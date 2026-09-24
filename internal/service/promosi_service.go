package service

import (
	"context"
	"errors"
	"math/rand/v2"
	"mime/multipart"
	"strconv"
	"strings"
	"time"

	"github.com/hermansyah/adojobsid/internal/database"
	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/repository"
	"github.com/hermansyah/adojobsid/internal/validator"
)

// ---------- paket ----------

type PaketService struct{ repos *repository.Repositories }

type PaketInput struct {
	Jenis      string
	Nama       string
	Deskripsi  string
	DurasiHari int
	Harga      int64
	Bobot      int
	SlotIklan  []string
	Aktif      bool
	Urutan     int
}

// validasiPaket murni supaya aturannya bisa diuji tanpa database.
func validasiPaket(in PaketInput) validator.Errors {
	errs := validator.New()

	jenisSah := false
	for _, j := range model.JenisPromosi {
		if in.Jenis == j {
			jenisSah = true
		}
	}
	if !jenisSah {
		errs.Add("jenis", "Jenis promosi tidak dikenal.")
	}
	nama := errs.Required("nama", "Nama paket", in.Nama)
	errs.Length("nama", "Nama paket", nama, 2, 80)
	errs.Length("deskripsi", "Deskripsi", in.Deskripsi, 0, 300)
	if in.DurasiHari < 1 || in.DurasiHari > 365 {
		errs.Add("durasi_hari", "Durasi harus antara 1 dan 365 hari.")
	}
	if in.Harga < 0 {
		errs.Add("harga", "Harga tidak boleh negatif.")
	}
	if in.Bobot < 1 || in.Bobot > 10 {
		errs.Add("bobot", "Bobot harus antara 1 dan 10.")
	}

	// Slot hanya bermakna untuk iklan: wajib ada, dan harus salah satu slot
	// yang dikenal kode. Jenis lain tidak boleh membawa slot.
	dikenal := map[string]bool{}
	for _, s := range model.SlotIklanBawaan() {
		dikenal[s.Kunci] = true
	}
	switch {
	case in.Jenis == model.PromosiIklan && len(in.SlotIklan) == 0:
		errs.Add("slot_iklan", "Pilih minimal satu slot penempatan.")
	case in.Jenis != model.PromosiIklan && len(in.SlotIklan) > 0:
		errs.Add("slot_iklan", "Slot penempatan hanya untuk paket iklan.")
	}
	for _, s := range in.SlotIklan {
		if !dikenal[s] {
			errs.Add("slot_iklan", "Slot "+s+" tidak dikenal.")
			break
		}
	}
	return errs
}

func (s *PaketService) Daftar(ctx context.Context, hanyaAktif bool) ([]model.PaketPromosi, error) {
	out, err := s.repos.Paket.List(ctx, hanyaAktif)
	if err != nil {
		return nil, Internal(err)
	}
	return out, nil
}

func (s *PaketService) Ambil(ctx context.Context, id int64) (*model.PaketPromosi, error) {
	p, err := s.repos.Paket.Get(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, NotFound("Paket tidak ditemukan.")
		}
		return nil, Internal(err)
	}
	return p, nil
}

func susunPaket(p *model.PaketPromosi, in PaketInput) {
	p.Jenis = in.Jenis
	p.Nama = strings.TrimSpace(in.Nama)
	p.Deskripsi = strings.TrimSpace(in.Deskripsi)
	p.DurasiHari = in.DurasiHari
	p.Harga = in.Harga
	p.Bobot = in.Bobot
	p.SlotIklan = in.SlotIklan
	if p.SlotIklan == nil {
		p.SlotIklan = []string{}
	}
	p.Aktif = in.Aktif
	p.Urutan = in.Urutan
}

func (s *PaketService) Buat(ctx context.Context, in PaketInput) (*model.PaketPromosi, error) {
	if errs := validasiPaket(in); errs.Any() {
		return nil, Invalid(errs)
	}
	var p model.PaketPromosi
	susunPaket(&p, in)
	if err := s.repos.Paket.Create(ctx, &p); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return nil, Conflict("Sudah ada paket dengan nama itu untuk jenis yang sama.")
		}
		return nil, Internal(err)
	}
	return &p, nil
}

func (s *PaketService) Ubah(ctx context.Context, id int64, in PaketInput) (*model.PaketPromosi, error) {
	if errs := validasiPaket(in); errs.Any() {
		return nil, Invalid(errs)
	}
	p, err := s.Ambil(ctx, id)
	if err != nil {
		return nil, err
	}
	susunPaket(p, in)
	if err := s.repos.Paket.Update(ctx, p); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return nil, Conflict("Sudah ada paket dengan nama itu untuk jenis yang sama.")
		}
		return nil, Internal(err)
	}
	return p, nil
}

// Hapus hanya untuk paket yang belum pernah dipakai; yang sudah dipakai
// cukup dinonaktifkan supaya riwayat promosi lama tetap terbaca.
func (s *PaketService) Hapus(ctx context.Context, id int64) error {
	err := s.repos.Paket.Delete(ctx, id)
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return NotFound("Paket tidak ditemukan.")
	case errors.Is(err, repository.ErrConflict):
		return Conflict("Paket ini sudah dipakai pengajuan. Nonaktifkan saja supaya riwayatnya tetap terbaca.")
	case err != nil:
		return Internal(err)
	}
	return nil
}

// ---------- promosi: mesin status ----------

type PromosiService struct {
	repos    *repository.Repositories
	cache    *database.Cache
	settings *SettingsService
	upload   *UploadService
}

// MaksIklanHidup membatasi iklan yang masih berjalan per akun, supaya satu
// pengiklan tidak memenuhi antrean dan rotasi sendirian.
const MaksIklanHidup = 3

// AjukanInput adalah isian form pengajuan; bagian yang terpakai bergantung
// pada jenisnya.
type AjukanInput struct {
	Jenis     string
	PaketID   int64
	ServiceID int64 // sorotan jasa
	Alasan    string
	// materi iklan
	Judul           string
	Deskripsi       string
	TautanURL       string
	TargetKecamatan []string
}

// validasiAjukan murni supaya aturan tiap jenis bisa diuji tanpa database.
// adaGambar menandai berkas materi iklan yang ikut dikirim.
func validasiAjukan(in AjukanInput, providerID int64, adaGambar bool) validator.Errors {
	errs := validator.New()
	if in.PaketID <= 0 {
		errs.Add("paket_id", "Pilih salah satu paket.")
	}
	switch in.Jenis {
	case model.PromosiSorotan:
		if providerID == 0 {
			errs.Add("jenis", "Sorotan jasa hanya untuk penyedia.")
		}
		if in.ServiceID <= 0 {
			errs.Add("service_id", "Pilih jasa yang ingin disorot.")
		}
	case model.PromosiPenyedia:
		if providerID == 0 {
			errs.Add("jenis", "Penyedia pilihan hanya untuk penyedia.")
		}
		errs.Length("alasan", "Alasan", in.Alasan, 0, 500)
	case model.PromosiIklan:
		judul := errs.Required("judul", "Judul iklan", in.Judul)
		errs.Length("judul", "Judul iklan", judul, 5, 120)
		errs.Length("deskripsi", "Keterangan", in.Deskripsi, 0, 300)
		if t := strings.TrimSpace(in.TautanURL); t != "" &&
			!strings.HasPrefix(t, "https://") && !strings.HasPrefix(t, "http://") {
			errs.Add("tautan_url", "Tautan harus diawali http:// atau https://")
		}
		if !adaGambar {
			errs.Add("gambar", "Unggah gambar iklan.")
		}
		dikenal := map[string]bool{}
		for _, k := range model.KecamatanBengkalis {
			dikenal[k] = true
		}
		for _, k := range in.TargetKecamatan {
			if !dikenal[k] {
				errs.Add("target_kecamatan", "Kecamatan "+k+" tidak dikenal.")
				break
			}
		}
	default:
		errs.Add("jenis", "Jenis promosi tidak dikenal.")
	}
	return errs
}

// Ajukan membuat pengajuan baru berstatus menunggu.
func (s *PromosiService) Ajukan(ctx context.Context, userID, providerID int64, in AjukanInput, gambar *multipart.FileHeader) (*model.Promosi, error) {
	if errs := validasiAjukan(in, providerID, gambar != nil); errs.Any() {
		return nil, Invalid(errs)
	}

	paket, err := s.repos.Paket.Get(ctx, in.PaketID)
	if err != nil || !paket.Aktif || paket.Jenis != in.Jenis {
		errs := validator.New()
		errs.Add("paket_id", "Paket tidak tersedia untuk jenis promosi ini.")
		return nil, Invalid(errs)
	}

	p := &model.Promosi{
		Jenis:           in.Jenis,
		UserID:          userID,
		PaketID:         &paket.ID,
		Status:          model.StatusMenunggu,
		Sumber:          model.SumberPengguna,
		TargetKecamatan: []string{},
	}
	switch in.Jenis {
	case model.PromosiSorotan:
		// Hanya listing milik sendiri yang sudah tayang — listing yang masih
		// antre atau disembunyikan tidak punya apa-apa untuk disorot.
		svc, err := s.repos.Service.GetByID(ctx, in.ServiceID)
		if err != nil || svc.ProviderID != providerID {
			return nil, NotFound("Jasa tidak ditemukan.")
		}
		if svc.Status != model.ServiceActive {
			return nil, Conflict("Hanya jasa yang sudah tayang yang bisa disorot.")
		}
		p.ProviderID, p.ServiceID = &providerID, &svc.ID
	case model.PromosiPenyedia:
		p.ProviderID = &providerID
		p.Alasan = strPtrOrNil(in.Alasan)
	case model.PromosiIklan:
		n, err := s.repos.Promosi.HitungHidupPengguna(ctx, userID, model.PromosiIklan)
		if err != nil {
			return nil, Internal(err)
		}
		if n >= MaksIklanHidup {
			return nil, Conflict("Anda sudah punya " + strconv.Itoa(MaksIklanHidup) +
				" iklan yang masih berjalan. Tunggu salah satunya selesai.")
		}
		g, err := s.upload.SaveImage(gambar, ProfilIklan)
		if err != nil {
			return nil, err
		}
		judul := strings.TrimSpace(in.Judul)
		p.Judul, p.GambarURL = &judul, &g.URL
		p.Deskripsi = strPtrOrNil(in.Deskripsi)
		p.TautanURL = strPtrOrNil(in.TautanURL)
		if in.TargetKecamatan != nil {
			p.TargetKecamatan = in.TargetKecamatan
		}
	}

	if err := s.repos.Promosi.Create(ctx, p); err != nil {
		if p.GambarURL != nil {
			s.upload.Delete(*p.GambarURL)
		}
		if errors.Is(err, repository.ErrConflict) {
			return nil, Conflict("Sudah ada pengajuan yang masih berjalan untuk ini. Tunggu sampai selesai atau batalkan yang lama.")
		}
		return nil, Internal(err)
	}
	return p, nil
}

// UnggahBukti menyimpan bukti transfer dari pemohon. Statusnya tidak berubah
// — tetap disetujui — sampai admin mengonfirmasi; dibayar_at mencatat kapan
// pemohon mengaku membayar.
func (s *PromosiService) UnggahBukti(ctx context.Context, userID, id int64, fh *multipart.FileHeader) (*model.Promosi, error) {
	p, err := s.Ambil(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.UserID != userID {
		return nil, NotFound("Pengajuan tidak ditemukan.")
	}
	if p.Status != model.StatusDisetujui {
		return nil, Conflict("Bukti pembayaran hanya untuk pengajuan yang sudah disetujui dan menunggu pembayaran.")
	}
	g, err := s.upload.SaveImage(fh, ProfilBukti)
	if err != nil {
		return nil, err
	}
	lama := p.BuktiBayarURL
	kini := time.Now()
	p.BuktiBayarURL, p.DibayarAt = &g.URL, &kini
	if err := s.repos.Promosi.Simpan(ctx, p); err != nil {
		s.upload.Delete(g.URL)
		return nil, Internal(err)
	}
	if lama != nil {
		s.upload.Delete(*lama)
	}
	return p, nil
}

// DaftarPengguna mengembalikan seluruh pengajuan milik pengguna, terbaru dulu.
func (s *PromosiService) DaftarPengguna(ctx context.Context, userID int64) ([]model.Promosi, error) {
	out, err := s.repos.Promosi.ListByUser(ctx, userID)
	if err != nil {
		return nil, Internal(err)
	}
	return out, nil
}

// AmbilMilik mengembalikan pengajuan hanya bila milik pengguna itu.
func (s *PromosiService) AmbilMilik(ctx context.Context, userID, id int64) (*model.Promosi, error) {
	p, err := s.Ambil(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.UserID != userID {
		return nil, NotFound("Pengajuan tidak ditemukan.")
	}
	return p, nil
}

func (s *PromosiService) Ambil(ctx context.Context, id int64) (*model.Promosi, error) {
	p, err := s.repos.Promosi.Get(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, NotFound("Pengajuan tidak ditemukan.")
		}
		return nil, Internal(err)
	}
	return p, nil
}

// ubahStatus adalah satu-satunya jalan mengubah status. Aturannya ada di
// model.BolehTransisi; di sini hanya efek sampingnya: jadwal tayang saat
// pertama kali aktif, dan penyegaran cache karena featured_until ikut berubah.
func (s *PromosiService) ubahStatus(ctx context.Context, p *model.Promosi, ke string, aktor model.AktorPromosi) error {
	if !model.BolehTransisi(p.Status, ke, aktor) {
		return Conflict("Pengajuan berstatus " + model.LabelStatusPromosi(p.Status) +
			" tidak bisa diubah menjadi " + model.LabelStatusPromosi(ke) + ".")
	}
	if ke == model.StatusAktif && p.MulaiAt == nil {
		durasi, err := s.durasiHari(ctx, p)
		if err != nil {
			return err
		}
		kini := time.Now()
		selesai := kini.Add(time.Duration(durasi) * 24 * time.Hour)
		p.MulaiAt, p.SelesaiAt = &kini, &selesai
	}
	p.Status = ke
	if err := s.repos.Promosi.Simpan(ctx, p); err != nil {
		return Internal(err)
	}
	// Pemohon diberi tahu setiap kali statusnya bergerak oleh pihak lain;
	// perubahan yang ia lakukan sendiri (batal) tidak perlu dikabarkan.
	if aktor != model.AktorPemohon {
		payload := map[string]any{"promosi_id": p.ID, "jenis": p.Jenis, "status": ke}
		if p.CatatanAdmin != nil {
			payload["catatan"] = *p.CatatanAdmin
		}
		_ = s.repos.Notif.Create(ctx, p.UserID, repository.NotifPromosiDiperbarui, payload)
	}
	// Sorotan & penyedia pilihan mengubah urutan dan kartu hasil pencarian.
	invalidateSearchCache(s.cache)
	if s.cache != nil {
		s.cache.Forget(contextBackground(), "promosi:*")
	}
	return nil
}

func (s *PromosiService) durasiHari(ctx context.Context, p *model.Promosi) (int, error) {
	if p.PaketID == nil {
		return 0, Conflict("Pengajuan ini tidak punya paket, jadi tidak diketahui berapa lama tayangnya.")
	}
	paket, err := s.repos.Paket.Get(ctx, *p.PaketID)
	if err != nil {
		return 0, Internal(err)
	}
	return paket.DurasiHari, nil
}

// Setujui menyetujui pengajuan. Paket gratis — atau sakelar uji coba gratis
// menyala — langsung tayang; selain itu menunggu pembayaran dikonfirmasi.
func (s *PromosiService) Setujui(ctx context.Context, adminID, id int64) (*model.Promosi, error) {
	p, err := s.Ambil(ctx, id)
	if err != nil {
		return nil, err
	}
	kini := time.Now()
	p.DitinjauOleh, p.DitinjauAt = &adminID, &kini

	ke := model.StatusDisetujui
	if s.settings.Get(ctx).Promosi.UjiCobaGratis {
		ke = model.StatusAktif
	} else if p.PaketID != nil {
		paket, err := s.repos.Paket.Get(ctx, *p.PaketID)
		if err != nil {
			return nil, Internal(err)
		}
		if paket.Gratis() {
			ke = model.StatusAktif
		}
	}
	if err := s.ubahStatus(ctx, p, ke, model.AktorAdmin); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *PromosiService) Tolak(ctx context.Context, adminID, id int64, catatan string) (*model.Promosi, error) {
	catatan = strings.TrimSpace(catatan)
	if len(catatan) < 5 {
		errs := validator.New()
		errs.Add("catatan", "Tuliskan alasan penolakan agar pemohon tahu apa yang perlu diperbaiki.")
		return nil, Invalid(errs)
	}
	p, err := s.Ambil(ctx, id)
	if err != nil {
		return nil, err
	}
	kini := time.Now()
	p.DitinjauOleh, p.DitinjauAt, p.CatatanAdmin = &adminID, &kini, &catatan
	if err := s.ubahStatus(ctx, p, model.StatusDitolak, model.AktorAdmin); err != nil {
		return nil, err
	}
	return p, nil
}

// KonfirmasiBayar menyatakan pembayaran diterima; promosi mulai tayang.
func (s *PromosiService) KonfirmasiBayar(ctx context.Context, adminID, id int64) (*model.Promosi, error) {
	p, err := s.Ambil(ctx, id)
	if err != nil {
		return nil, err
	}
	kini := time.Now()
	p.DikonfirmasiOleh, p.DibayarAt = &adminID, &kini
	if err := s.ubahStatus(ctx, p, model.StatusAktif, model.AktorAdmin); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *PromosiService) Jeda(ctx context.Context, id int64) (*model.Promosi, error) {
	return s.adminUbah(ctx, id, model.StatusDijeda)
}

// Lanjutkan mengaktifkan kembali promosi yang dijeda. Jadwal tayangnya tidak
// digeser: jeda memang memakan masa tayang, dan itu keputusan admin.
func (s *PromosiService) Lanjutkan(ctx context.Context, id int64) (*model.Promosi, error) {
	return s.adminUbah(ctx, id, model.StatusAktif)
}

func (s *PromosiService) Hentikan(ctx context.Context, id int64) (*model.Promosi, error) {
	return s.adminUbah(ctx, id, model.StatusDihentikan)
}

func (s *PromosiService) adminUbah(ctx context.Context, id int64, ke string) (*model.Promosi, error) {
	p, err := s.Ambil(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.ubahStatus(ctx, p, ke, model.AktorAdmin); err != nil {
		return nil, err
	}
	return p, nil
}

// Batalkan dipakai pemohon; hanya pengajuannya sendiri, hanya sebelum tayang.
func (s *PromosiService) Batalkan(ctx context.Context, userID, id int64) (*model.Promosi, error) {
	p, err := s.Ambil(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.UserID != userID {
		return nil, NotFound("Pengajuan tidak ditemukan.")
	}
	if err := s.ubahStatus(ctx, p, model.StatusDibatalkan, model.AktorPemohon); err != nil {
		return nil, err
	}
	return p, nil
}

// TandaiKedaluwarsa menutup promosi yang masa tayangnya sudah lewat.
// Dipanggil ticker; mengembalikan yang baru saja ditutup untuk dinotifikasi.
func (s *PromosiService) TandaiKedaluwarsa(ctx context.Context) ([]model.Promosi, error) {
	daftar, err := s.repos.Promosi.ListKedaluwarsa(ctx)
	if err != nil {
		return nil, Internal(err)
	}
	var ditutup []model.Promosi
	for i := range daftar {
		if err := s.ubahStatus(ctx, &daftar[i], model.StatusSelesai, model.AktorSistem); err != nil {
			return ditutup, err
		}
		ditutup = append(ditutup, daftar[i])
	}
	return ditutup, nil
}

// ---------- admin ----------

// TabPromosi memetakan tab antrean admin ke status-statusnya.
func TabPromosi(tab string) []string {
	switch tab {
	case "bayar":
		return []string{model.StatusDisetujui}
	case "tayang":
		return []string{model.StatusAktif, model.StatusDijeda}
	case "riwayat":
		return []string{model.StatusSelesai, model.StatusDitolak, model.StatusDibatalkan, model.StatusDihentikan}
	default:
		return []string{model.StatusMenunggu}
	}
}

func (s *PromosiService) DaftarAdmin(ctx context.Context, f repository.PromosiFilter) ([]repository.PromosiAdminRow, int, error) {
	items, err := s.repos.Promosi.ListAdmin(ctx, f)
	if err != nil {
		return nil, 0, Internal(err)
	}
	total, err := s.repos.Promosi.Count(ctx, f)
	if err != nil {
		return nil, 0, Internal(err)
	}
	return items, total, nil
}

// JumlahPerluTindakan dipakai badge sidebar admin; kegagalan cukup jadi nol.
func (s *PromosiService) JumlahPerluTindakan(ctx context.Context) int {
	n, err := s.repos.Promosi.CountPerluTindakan(ctx)
	if err != nil {
		return 0
	}
	return n
}

// TetapkanAdmin adalah jalur tombol sorot sekali-klik: admin menyorot listing
// atau menetapkan penyedia pilihan tanpa pengajuan. Dicatat sebagai promosi
// bersumber admin supaya riwayatnya satu dengan pengajuan pengguna, dan
// featured_until tetap hanya ditulis trigger.
//
// hari = 0 berarti mencabut: seluruh sorotan yang sedang tayang untuk sasaran
// itu dihentikan. Pengajuan pengguna yang masih menunggu tidak disentuh —
// admin memutuskannya di antrean, bukan diam-diam lewat tombol cabut.
func (s *PromosiService) TetapkanAdmin(ctx context.Context, adminID int64, jenis string, targetID int64, hari int) error {
	if jenis != model.PromosiSorotan && jenis != model.PromosiPenyedia {
		return InvalidMsg("Jenis sorotan tidak dikenal.")
	}
	hidup, err := s.repos.Promosi.ListHidupUntuk(ctx, jenis, targetID)
	if err != nil {
		return Internal(err)
	}

	if hari == 0 {
		for i := range hidup {
			if hidup[i].Status == model.StatusAktif || hidup[i].Status == model.StatusDijeda {
				if err := s.ubahStatus(ctx, &hidup[i], model.StatusDihentikan, model.AktorAdmin); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if hari < 1 || hari > 365 {
		return InvalidMsg("Durasi sorotan tidak tersedia.")
	}
	if len(hidup) > 0 {
		return Conflict("Sudah ada sorotan atau pengajuan yang berjalan untuk ini. Kelola lewat antrean promosi.")
	}

	p := &model.Promosi{Jenis: jenis, Sumber: model.SumberAdmin, Status: model.StatusAktif, TargetKecamatan: []string{}}
	switch jenis {
	case model.PromosiSorotan:
		svc, err := s.repos.Service.GetByID(ctx, targetID)
		if err != nil {
			return NotFound("Listing tidak ditemukan.")
		}
		prov, err := s.repos.Provider.GetByID(ctx, svc.ProviderID)
		if err != nil {
			return Internal(err)
		}
		p.UserID, p.ProviderID, p.ServiceID = prov.UserID, &prov.ID, &svc.ID
	case model.PromosiPenyedia:
		prov, err := s.repos.Provider.GetByID(ctx, targetID)
		if err != nil {
			return NotFound("Penyedia tidak ditemukan.")
		}
		p.UserID, p.ProviderID = prov.UserID, &prov.ID
	}
	kini := time.Now()
	selesai := kini.Add(time.Duration(hari) * 24 * time.Hour)
	p.MulaiAt, p.SelesaiAt, p.DitinjauOleh, p.DitinjauAt = &kini, &selesai, &adminID, &kini

	if err := s.repos.Promosi.Create(ctx, p); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return Conflict("Sudah ada sorotan atau pengajuan yang berjalan untuk ini.")
		}
		return Internal(err)
	}
	invalidateSearchCache(s.cache)
	if s.cache != nil {
		s.cache.Forget(contextBackground(), "promosi:*")
	}
	return nil
}

// ---------- penayangan ----------

const cacheIklanTayang = "promosi:iklan-tayang"

// IklanTayang mengembalikan daftar iklan aktif dari cache. Daftar ini kecil
// (puluhan baris paling banyak) dan dibaca di setiap tampilan halaman, jadi
// di-cache sepenuhnya; ia dibersihkan setiap kali status promosi bergerak.
func (s *PromosiService) IklanTayang(ctx context.Context) []model.IklanTayang {
	daftar, err := database.Remember(ctx, s.cache, cacheIklanTayang, time.Minute, func() ([]model.IklanTayang, error) {
		return s.repos.Promosi.ListIklanTayang(ctx)
	})
	if err != nil {
		return nil
	}
	return daftar
}

// PilihUntukSlot memilih iklan untuk satu slot dan penonton di kecamatan
// tertentu (kosong = tidak disaring). nil berarti tidak ada iklan berbayar
// yang layak; pemanggil jatuh ke materi bawaan.
func (s *PromosiService) PilihUntukSlot(ctx context.Context, slot, kecamatan string) *model.IklanTayang {
	return model.PilihIklan(s.IklanTayang(ctx), slot, kecamatan, time.Now(), rand.IntN)
}

// CatatTayang menaikkan penghitung tayang di Redis; disalin ke database oleh
// SalinPenghitungTayang.
func (s *PromosiService) CatatTayang(ctx context.Context, id int64) {
	if s.cache != nil {
		s.cache.Tambah(ctx, "promosi:tayang:"+strconv.FormatInt(id, 10))
	}
}

// SalinPenghitungTayang memindahkan penghitung Redis ke kolom tayang.
// Mengembalikan jumlah promosi yang diperbarui.
func (s *PromosiService) SalinPenghitungTayang(ctx context.Context) int {
	if s.cache == nil {
		return 0
	}
	hitung := s.cache.AmbilDanHapus(ctx, "promosi:tayang:*")
	n := 0
	for kunci, jumlah := range hitung {
		id, err := strconv.ParseInt(strings.TrimPrefix(kunci, "promosi:tayang:"), 10, 64)
		if err != nil {
			continue
		}
		if err := s.repos.Promosi.TambahTayang(ctx, id, jumlah); err == nil {
			n++
		}
	}
	return n
}

// Klik mencatat klik dan mengembalikan tautan tujuan iklan.
func (s *PromosiService) Klik(ctx context.Context, id int64) (string, error) {
	tautan, err := s.repos.Promosi.TambahKlik(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", NotFound("Iklan tidak ditemukan.")
		}
		return "", Internal(err)
	}
	return tautan, nil
}

// PenyediaPilihan memilih n penyedia pilihan untuk beranda, mengutamakan
// kecamatan penonton dan menggilir sisanya.
func (s *PromosiService) PenyediaPilihan(daftar []model.ProviderDetail, kecamatan string, n int) []model.ProviderDetail {
	return model.PilihPenyediaPilihan(daftar, kecamatan, n, rand.IntN)
}

// StatusAkanSelesai adalah penanda payload notifikasi pengingat H-1; bukan
// status promosi, hanya jenis kabar.
const StatusAkanSelesai = "akan_selesai"

// IngatkanAkanSelesai mengirim pengingat ke pemohon sehari sebelum masa
// tayang habis, supaya sempat mengajukan lagi. Dipanggil ticker; tiap
// promosi diingatkan tepat satu kali. Mengembalikan jumlah yang dikirim.
func (s *PromosiService) IngatkanAkanSelesai(ctx context.Context) int {
	daftar, err := s.repos.Promosi.ListAkanSelesai(ctx, 24*time.Hour)
	if err != nil {
		return 0
	}
	n := 0
	for _, p := range daftar {
		if milikKita, err := s.repos.Promosi.TandaiDiingatkan(ctx, p.ID); err != nil || !milikKita {
			continue
		}
		_ = s.repos.Notif.Create(ctx, p.UserID, repository.NotifPromosiDiperbarui, map[string]any{
			"promosi_id": p.ID, "jenis": p.Jenis, "status": StatusAkanSelesai,
		})
		n++
	}
	return n
}

// Statistik untuk dasbor admin.
func (s *PromosiService) Statistik(ctx context.Context) *repository.StatistikPromosi {
	st, err := s.repos.Promosi.Statistik(ctx)
	if err != nil {
		return &repository.StatistikPromosi{}
	}
	return st
}
