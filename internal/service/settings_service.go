package service

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"strings"

	"github.com/hermansyah/adojobsid/internal/database"
	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/repository"
	"github.com/hermansyah/adojobsid/internal/validator"
)

// SettingsService melayani pengaturan aplikasi yang bisa diubah admin.
// Pengaturan dibaca hampir di setiap permintaan, karena itu hasilnya di-cache
// di Redis dan hanya dibersihkan saat admin menyimpan perubahan.
type SettingsService struct {
	repos  *repository.Repositories
	cache  *database.Cache
	upload *UploadService
}

const cacheKeyPengaturan = "settings:semua"

// Get mengembalikan seluruh pengaturan. Kelompok yang belum pernah disimpan
// memakai nilai bawaan, sehingga aplikasi tetap jalan sebelum admin
// menyentuh halaman pengaturan sama sekali.
func (s *SettingsService) Get(ctx context.Context) model.Pengaturan {
	hasil, err := database.Remember(ctx, s.cache, cacheKeyPengaturan, 0,
		func() (model.Pengaturan, error) {
			p := model.PengaturanBawaan()

			if err := s.repos.Settings.Get(ctx, model.SettingUmum, &p.Umum); err != nil &&
				!errors.Is(err, repository.ErrNotFound) {
				return p, err
			}
			if err := s.repos.Settings.Get(ctx, model.SettingLokasi, &p.Lokasi); err != nil &&
				!errors.Is(err, repository.ErrNotFound) {
				return p, err
			}
			if err := s.repos.Settings.Get(ctx, model.SettingIklan, &p.Iklan); err != nil &&
				!errors.Is(err, repository.ErrNotFound) {
				return p, err
			}
			if err := s.repos.Settings.Get(ctx, model.SettingTampilan, &p.Tampilan); err != nil &&
				!errors.Is(err, repository.ErrNotFound) {
				return p, err
			}
			if err := s.repos.Settings.Get(ctx, model.SettingPromosi, &p.Promosi); err != nil &&
				!errors.Is(err, repository.ErrNotFound) {
				return p, err
			}
			return normalkanPengaturan(p), nil
		})
	if err != nil {
		// Kegagalan membaca pengaturan tidak boleh menjatuhkan halaman;
		// nilai bawaan sudah cukup untuk merender aplikasi.
		return model.PengaturanBawaan()
	}
	return normalkanPengaturan(hasil)
}

// normalkanPengaturan mengisi nilai yang kosong atau tidak masuk akal dengan
// bawaannya, supaya data lama tetap aman dipakai setelah struktur berubah.
func normalkanPengaturan(p model.Pengaturan) model.Pengaturan {
	bawaan := model.PengaturanBawaan()

	if strings.TrimSpace(p.Umum.NamaSitus) == "" {
		p.Umum.NamaSitus = bawaan.Umum.NamaSitus
	}
	// Tagline kosong berarti beranda tanpa deskripsi meta; bawaannya dipakai.
	if strings.TrimSpace(p.Umum.Tagline) == "" {
		p.Umum.Tagline = bawaan.Umum.Tagline
	}
	if p.Lokasi.RadiusDefaultKm <= 0 {
		p.Lokasi.RadiusDefaultKm = bawaan.Lokasi.RadiusDefaultKm
	}
	if p.Lokasi.RadiusMaksKm <= 0 {
		p.Lokasi.RadiusMaksKm = bawaan.Lokasi.RadiusMaksKm
	}
	if p.Lokasi.RadiusDefaultKm > p.Lokasi.RadiusMaksKm {
		p.Lokasi.RadiusDefaultKm = p.Lokasi.RadiusMaksKm
	}
	// Overlay di luar rentang aman dikembalikan ke bawaan supaya teks hero
	// tidak pernah hilang di atas foto, apa pun isi datanya.
	if p.Tampilan.HeroOverlay < model.HeroOverlayMin || p.Tampilan.HeroOverlay > model.HeroOverlayMaks {
		p.Tampilan.HeroOverlay = bawaan.Tampilan.HeroOverlay
	}
	// Lencana tanpa teks tidak berarti apa-apa, jadi teks kosong — termasuk
	// pada data yang tersimpan sebelum kedua kolom ini ada — kembali ke bawaan.
	if strings.TrimSpace(p.Tampilan.PlaystoreTeksAtas) == "" {
		p.Tampilan.PlaystoreTeksAtas = bawaan.Tampilan.PlaystoreTeksAtas
	}
	if strings.TrimSpace(p.Tampilan.PlaystoreTeksBawah) == "" {
		p.Tampilan.PlaystoreTeksBawah = bawaan.Tampilan.PlaystoreTeksBawah
	}
	if !model.KoordinatValid(p.Lokasi.PusatLatitude, p.Lokasi.PusatLongitude) {
		p.Lokasi.PusatLatitude = bawaan.Lokasi.PusatLatitude
		p.Lokasi.PusatLongitude = bawaan.Lokasi.PusatLongitude
	}

	// Slot yang belum pernah dikonfigurasi tetap muncul agar admin bisa
	// mengisinya; slot lama yang sudah tidak dikenal dibuang.
	tersimpan := make(map[string]model.SlotIklan, len(p.Iklan.Slot))
	for _, s := range p.Iklan.Slot {
		tersimpan[s.Kunci] = s
	}
	gabungan := make([]model.SlotIklan, 0, len(bawaan.Iklan.Slot))
	for _, def := range bawaan.Iklan.Slot {
		if ada, ok := tersimpan[def.Kunci]; ok {
			ada.Nama = def.Nama // nama penempatan selalu mengikuti definisi kode
			gabungan = append(gabungan, ada)
			continue
		}
		gabungan = append(gabungan, def)
	}
	p.Iklan.Slot = gabungan
	return p
}

type PengaturanUmumInput struct {
	NamaSitus     string
	Tagline       string
	KontakWA      string
	TeksFooter    string
	WhatsappAktif bool
}

func (s *SettingsService) SimpanUmum(ctx context.Context, adminID int64, in PengaturanUmumInput) error {
	errs := validator.New()

	nama := errs.Required("nama_situs", "Nama situs", in.NamaSitus)
	errs.Length("nama_situs", "Nama situs", nama, 2, 60)
	errs.Length("tagline", "Tagline", in.Tagline, 0, 160)
	errs.Length("teks_footer", "Teks footer", in.TeksFooter, 0, 200)

	wa := strings.TrimSpace(in.KontakWA)
	if wa != "" {
		wa = validator.NormalizePhone(wa)
		if !validator.ValidPhone(wa) {
			errs.Add("kontak_wa", "Format nomor WhatsApp tidak valid.")
		}
	}
	if errs.Any() {
		return Invalid(errs)
	}

	saat := s.Get(ctx)
	saat.Umum.NamaSitus = nama
	saat.Umum.Tagline = strings.TrimSpace(in.Tagline)
	saat.Umum.KontakWA = wa
	saat.Umum.TeksFooter = strings.TrimSpace(in.TeksFooter)
	saat.Umum.WhatsappAktif = in.WhatsappAktif

	if err := s.repos.Settings.Set(ctx, model.SettingUmum, saat.Umum, adminID); err != nil {
		return Internal(err)
	}
	s.invalidate()
	return nil
}

// SimpanLogo mengunggah logo baru dan menghapus berkas logo sebelumnya.
func (s *SettingsService) SimpanLogo(ctx context.Context, adminID int64, fh *multipart.FileHeader) error {
	g, err := s.upload.SaveImage(fh, ProfilLogo)
	if err != nil {
		return err
	}

	saat := s.Get(ctx)
	lama := saat.Umum.LogoURL
	saat.Umum.LogoURL = g.URL

	if err := s.repos.Settings.Set(ctx, model.SettingUmum, saat.Umum, adminID); err != nil {
		s.upload.Delete(g.URL)
		return Internal(err)
	}
	if lama != "" {
		s.upload.Delete(lama)
	}
	s.invalidate()
	return nil
}

// HapusLogo mengembalikan tampilan ke wordmark tipografis bawaan.
func (s *SettingsService) HapusLogo(ctx context.Context, adminID int64) error {
	saat := s.Get(ctx)
	lama := saat.Umum.LogoURL
	if lama == "" {
		return nil
	}
	saat.Umum.LogoURL = ""

	if err := s.repos.Settings.Set(ctx, model.SettingUmum, saat.Umum, adminID); err != nil {
		return Internal(err)
	}
	s.upload.Delete(lama)
	s.invalidate()
	return nil
}

type PengaturanLokasiInput struct {
	PusatLatitude   float64
	PusatLongitude  float64
	RadiusDefaultKm int
	RadiusMaksKm    int
}

func (s *SettingsService) SimpanLokasi(ctx context.Context, adminID int64, in PengaturanLokasiInput) error {
	errs := validator.New()

	if !model.KoordinatValid(in.PusatLatitude, in.PusatLongitude) {
		errs.Add("pusat", "Titik pusat peta tidak valid.")
	}
	if in.RadiusMaksKm < 1 || in.RadiusMaksKm > 500 {
		errs.Add("radius_maks_km", "Radius maksimum harus antara 1 dan 500 km.")
	}
	if in.RadiusDefaultKm < 1 || in.RadiusDefaultKm > in.RadiusMaksKm {
		errs.Add("radius_default_km", "Radius bawaan harus antara 1 km dan radius maksimum.")
	}
	if errs.Any() {
		return Invalid(errs)
	}

	saat := s.Get(ctx)
	saat.Lokasi = model.PengaturanLokasi{
		PusatLatitude:   in.PusatLatitude,
		PusatLongitude:  in.PusatLongitude,
		RadiusDefaultKm: in.RadiusDefaultKm,
		RadiusMaksKm:    in.RadiusMaksKm,
	}
	if err := s.repos.Settings.Set(ctx, model.SettingLokasi, saat.Lokasi, adminID); err != nil {
		return Internal(err)
	}
	s.invalidate()
	return nil
}

// SlotIklanInput adalah isian satu slot. Teks dan gambar disimpan bersama
// dalam satu permintaan: pemisahannya dulu membuat admin memilih berkas lalu
// menekan tombol simpan yang salah, dan berkasnya hilang tanpa pesan apa pun.
type SlotIklanInput struct {
	Aktif     bool
	TautanURL string
	Teks      string
}

// SimpanSlotIklan menyimpan satu slot beserta materinya bila ada berkas baru.
// Slot dicari berdasarkan kunci yang terdaftar di kode, sehingga kunci asing
// dari kiriman klien tidak bisa membuat slot baru.
func (s *SettingsService) SimpanSlotIklan(
	ctx context.Context, adminID int64, kunci string,
	in SlotIklanInput, fh *multipart.FileHeader,
) error {
	saat := s.Get(ctx)
	idx := indeksSlot(saat.Iklan.Slot, kunci)
	if idx < 0 {
		return NotFound("Slot iklan tidak dikenal.")
	}

	tautan := strings.TrimSpace(in.TautanURL)
	teks := strings.TrimSpace(in.Teks)

	errs := validator.New()
	if tautan != "" && !strings.HasPrefix(tautan, "https://") && !strings.HasPrefix(tautan, "http://") {
		errs.Add("tautan_"+kunci, "Tautan harus diawali http:// atau https://")
	}
	if errs.Any() {
		return Invalid(errs)
	}

	lama := saat.Iklan.Slot[idx].GambarURL
	baru := lama

	// Berkas hanya disentuh bila admin benar-benar memilih satu; menyimpan
	// teks saja tidak boleh menghapus materi yang sudah ada.
	if fh != nil {
		g, err := s.upload.SaveImage(fh, ProfilIklan)
		if err != nil {
			return err
		}
		baru = g.URL
	}

	// Slot tidak bisa diaktifkan sebelum ada isi yang bisa ditampilkan.
	if in.Aktif && baru == "" && teks == "" {
		if baru != lama {
			s.upload.Delete(baru)
		}
		errs.Add("isi_"+kunci, "Unggah materi atau isi teks sebelum mengaktifkan slot ini.")
		return Invalid(errs)
	}

	saat.Iklan.Slot[idx].Aktif = in.Aktif
	saat.Iklan.Slot[idx].TautanURL = tautan
	saat.Iklan.Slot[idx].Teks = teks
	saat.Iklan.Slot[idx].GambarURL = baru

	if err := s.repos.Settings.Set(ctx, model.SettingIklan, saat.Iklan, adminID); err != nil {
		if baru != lama {
			s.upload.Delete(baru)
		}
		return Internal(err)
	}
	if baru != lama && lama != "" {
		s.upload.Delete(lama)
	}
	s.invalidate()
	return nil
}

// HapusGambarIklan membuang materi satu slot. Slot yang jadi kosong sekaligus
// dinonaktifkan, karena slot aktif tanpa isi tidak punya yang bisa ditayangkan.
func (s *SettingsService) HapusGambarIklan(ctx context.Context, adminID int64, kunci string) error {
	saat := s.Get(ctx)
	idx := indeksSlot(saat.Iklan.Slot, kunci)
	if idx < 0 {
		return NotFound("Slot iklan tidak dikenal.")
	}

	lama := saat.Iklan.Slot[idx].GambarURL
	if lama == "" {
		return nil
	}
	saat.Iklan.Slot[idx].GambarURL = ""
	if saat.Iklan.Slot[idx].Teks == "" {
		saat.Iklan.Slot[idx].Aktif = false
	}

	if err := s.repos.Settings.Set(ctx, model.SettingIklan, saat.Iklan, adminID); err != nil {
		return Internal(err)
	}
	s.upload.Delete(lama)
	s.invalidate()
	return nil
}

func indeksSlot(slot []model.SlotIklan, kunci string) int {
	for i := range slot {
		if slot[i].Kunci == kunci {
			return i
		}
	}
	return -1
}

func (s *SettingsService) invalidate() {
	if s.cache == nil {
		return
	}
	s.cache.Forget(contextBackground(), "settings:*")
}

// ---------- tampilan beranda ----------

type PengaturanTampilanInput struct {
	HeroOverlay        int
	AplikasiTampil     bool
	PlaystoreURL       string
	PlaystoreTeksAtas  string
	PlaystoreTeksBawah string
}

// SimpanTampilan menyimpan pengaturan visual beranda. Gambar hero diunggah
// lewat SimpanHero terpisah supaya menyimpan tautan tidak mengharuskan admin
// mengunggah ulang fotonya.
func (s *SettingsService) SimpanTampilan(ctx context.Context, adminID int64, in PengaturanTampilanInput) error {
	errs := validator.New()

	if in.HeroOverlay < model.HeroOverlayMin || in.HeroOverlay > model.HeroOverlayMaks {
		errs.Add("hero_overlay", fmt.Sprintf("Kepekatan overlay harus antara %d dan %d persen.",
			model.HeroOverlayMin, model.HeroOverlayMaks))
	}

	play := strings.TrimSpace(in.PlaystoreURL)
	if play != "" && !strings.HasPrefix(play, "https://") {
		errs.Add("playstore_url", "Tautan Play Store harus diawali https://")
	}

	atas := strings.TrimSpace(in.PlaystoreTeksAtas)
	bawah := strings.TrimSpace(in.PlaystoreTeksBawah)
	errs.Length("playstore_teks_atas", "Baris atas lencana", atas, 0, 30)
	errs.Length("playstore_teks_bawah", "Baris bawah lencana", bawah, 0, 30)

	// Lencana yang dinyalakan tanpa teks apa pun akan tampil sebagai kotak
	// kosong, jadi ditolak di sini alih-alih diam-diam diisi bawaan.
	if in.AplikasiTampil && atas == "" && bawah == "" {
		errs.Add("playstore_teks_bawah", "Isi teks lencana sebelum menampilkannya.")
	}
	if errs.Any() {
		return Invalid(errs)
	}

	saat := s.Get(ctx)
	saat.Tampilan.HeroOverlay = in.HeroOverlay
	saat.Tampilan.AplikasiTampil = in.AplikasiTampil
	saat.Tampilan.PlaystoreURL = play
	saat.Tampilan.PlaystoreTeksAtas = atas
	saat.Tampilan.PlaystoreTeksBawah = bawah

	if err := s.repos.Settings.Set(ctx, model.SettingTampilan, saat.Tampilan, adminID); err != nil {
		return Internal(err)
	}
	s.invalidate()
	return nil
}

// SimpanHero mengganti gambar latar hero dan membuang berkas lamanya.
func (s *SettingsService) SimpanHero(ctx context.Context, adminID int64, fh *multipart.FileHeader) error {
	g, err := s.upload.SaveImage(fh, ProfilHero)
	if err != nil {
		return err
	}

	saat := s.Get(ctx)
	lama := saat.Tampilan.HeroGambarURL
	saat.Tampilan.HeroGambarURL = g.URL

	if err := s.repos.Settings.Set(ctx, model.SettingTampilan, saat.Tampilan, adminID); err != nil {
		s.upload.Delete(g.URL)
		return Internal(err)
	}
	if lama != "" {
		s.upload.Delete(lama)
	}
	s.invalidate()
	return nil
}

// HapusHero mengembalikan hero ke latar polos bertipografi.
func (s *SettingsService) HapusHero(ctx context.Context, adminID int64) error {
	saat := s.Get(ctx)
	lama := saat.Tampilan.HeroGambarURL
	if lama == "" {
		return nil
	}
	saat.Tampilan.HeroGambarURL = ""

	if err := s.repos.Settings.Set(ctx, model.SettingTampilan, saat.Tampilan, adminID); err != nil {
		return Internal(err)
	}
	s.upload.Delete(lama)
	s.invalidate()
	return nil
}

// ---------- pembayaran promosi ----------

type PengaturanPromosiInput struct {
	UjiCobaGratis bool
	Bank          string
	NomorRekening string
	AtasNama      string
	PetunjukBayar string
}

// SimpanPromosi menyimpan sakelar uji coba dan rekening. Mematikan uji coba
// tanpa rekening ditolak: pemohon yang disetujui akan diminta membayar ke
// mana-mana.
func (s *SettingsService) SimpanPromosi(ctx context.Context, adminID int64, in PengaturanPromosiInput) error {
	errs := validator.New()
	bank, rek, nama := strings.TrimSpace(in.Bank), strings.TrimSpace(in.NomorRekening), strings.TrimSpace(in.AtasNama)
	errs.Length("bank", "Bank", bank, 0, 60)
	errs.Length("nomor_rekening", "Nomor rekening", rek, 0, 40)
	errs.Length("atas_nama", "Atas nama", nama, 0, 80)
	errs.Length("petunjuk_bayar", "Petunjuk", in.PetunjukBayar, 0, 500)
	if !in.UjiCobaGratis && (bank == "" || rek == "" || nama == "") {
		errs.Add("nomor_rekening", "Isi bank, nomor rekening, dan atas nama sebelum mematikan uji coba gratis.")
	}
	if errs.Any() {
		return Invalid(errs)
	}

	saat := s.Get(ctx)
	saat.Promosi = model.PengaturanPromosi{
		UjiCobaGratis: in.UjiCobaGratis, Bank: bank, NomorRekening: rek, AtasNama: nama,
		PetunjukBayar: strings.TrimSpace(in.PetunjukBayar),
	}
	if err := s.repos.Settings.Set(ctx, model.SettingPromosi, saat.Promosi, adminID); err != nil {
		return Internal(err)
	}
	s.invalidate()
	return nil
}
