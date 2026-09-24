package view

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/repository"
	"github.com/hermansyah/adojobsid/internal/service"
)

// ---------- lokasi ----------

// ProviderLokasiData adalah data form lokasi layanan penyedia.
type ProviderLokasiData struct {
	Base
	Form       Form
	Profile    *model.ProviderProfile
	PusatLat   float64
	PusatLng   float64
	RadiusMaks int
	Kecamatan  []model.Kecamatan
}

// PetaConfig menyusun konfigurasi peta sebagai JSON untuk dibaca app.js.
// Nilainya ditulis ke atribut data-, bukan ke dalam blok <script>, supaya
// Content-Security-Policy tetap melarang skrip sebaris.
func (d ProviderLokasiData) PetaConfig() string {
	lat, lng := d.PusatLat, d.PusatLng
	if d.Profile != nil && d.Profile.PunyaLokasi() {
		lat, lng = *d.Profile.Latitude, *d.Profile.Longitude
	}
	cfg := map[string]any{
		"lat":       lat,
		"lng":       lng,
		"adaPin":    d.Profile != nil && d.Profile.PunyaLokasi(),
		"radiusKm":  d.RadiusSaatIni(),
		"kecamatan": d.Kecamatan,
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

// RadiusSaatIni mengembalikan radius yang sedang berlaku pada form.
func (d ProviderLokasiData) RadiusSaatIni() int {
	if v := d.Form.Value("service_radius_km"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	if d.Profile != nil && d.Profile.ServiceRadiusKm > 0 {
		return d.Profile.ServiceRadiusKm
	}
	return 15
}

// PilihanRadius menyusun opsi radius yang ditawarkan ke penyedia.
// Radius yang sedang tersimpan selalu ikut disertakan sekalipun bukan salah
// satu nilai preset — tanpa itu, penyedia yang radiusnya tidak persis sama
// dengan preset akan diam-diam terreset ke nilai terkecil saat menyimpan.
func (d ProviderLokasiData) PilihanRadius() []int {
	kandidat := []int{5, 10, 15, 25, 50, 75, 100, 150, 200}
	set := map[int]bool{}
	out := make([]int, 0, len(kandidat)+1)

	tambah := func(v int) {
		if v > 0 && v <= d.RadiusMaks && !set[v] {
			set[v] = true
			out = append(out, v)
		}
	}
	for _, k := range kandidat {
		tambah(k)
	}
	tambah(d.RadiusSaatIni())

	sort.Ints(out)
	if len(out) == 0 {
		out = append(out, d.RadiusMaks)
	}
	return out
}

// ---------- pesan ----------

// PesanData adalah data halaman daftar percakapan.
type PesanData struct {
	Base
	Items []model.ConversationRow
}

// RuangPesanData adalah data satu ruang obrolan.
type RuangPesanData struct {
	Base
	Conv            *model.ConversationRow
	Messages        []model.MessageRow
	Order           *model.Order
	SebagaiPenyedia bool
	UserID          int64
	Form            Form
	// DapatDiulas menandai pesanan selesai yang belum diulas pemesannya.
	DapatDiulas bool
	// Ulasan terisi bila pesanan ini sudah pernah diulas.
	Ulasan *model.Review
}

// PesanTerakhirID dipakai polling HTMX sebagai penanda mulai.
func (d RuangPesanData) PesanTerakhirID() int64 {
	if len(d.Messages) == 0 {
		return 0
	}
	return d.Messages[len(d.Messages)-1].ID
}

// TindakanOrder mengembalikan status berikutnya yang boleh dipilih pengguna.
func (d RuangPesanData) TindakanOrder() []model.OrderStatus {
	if d.Order == nil {
		return nil
	}
	svc := &service.OrderService{}
	return svc.TindakanTersedia(d.Order, d.SebagaiPenyedia)
}

// LabelTindakan menyusun teks tombol untuk satu perubahan status.
func LabelTindakan(s model.OrderStatus) string {
	switch s {
	case model.OrderAccepted:
		return "Terima pesanan"
	case model.OrderRejected:
		return "Tolak pesanan"
	case model.OrderCompleted:
		return "Tandai selesai"
	case model.OrderCancelled:
		return "Batalkan pesanan"
	}
	return s.Label()
}

// ---------- notifikasi ----------

// NotifikasiData adalah data halaman notifikasi.
type NotifikasiData struct {
	Base
	Items []model.Notification
}

// IsiNotifikasi menerjemahkan payload JSON menjadi judul, keterangan, dan
// tautan yang bisa ditampilkan.
func IsiNotifikasi(n model.Notification) (judul, keterangan, tautan string) {
	var p map[string]any
	_ = json.Unmarshal(n.Payload, &p)

	teks := func(key string) string {
		if v, ok := p[key].(string); ok {
			return v
		}
		return ""
	}
	id := func(key string) string {
		if v, ok := p[key].(float64); ok {
			return strconv.FormatInt(int64(v), 10)
		}
		return ""
	}

	switch n.Type {
	case repository.NotifPesanBaru:
		judul = "Pesan baru"
		keterangan = teks("cuplikan")
		if s := teks("service_title"); s != "" {
			judul = "Pesan baru — " + s
		}
		if cid := id("conversation_id"); cid != "" {
			tautan = "/pesan/" + cid
		}
	case repository.NotifOrderBaru:
		judul = "Permintaan order baru"
		keterangan = teks("service_title")
		if cid := id("conversation_id"); cid != "" {
			tautan = "/pesan/" + cid
		}
	case repository.NotifOrderDiperbarui:
		judul = "Status pesanan diperbarui"
		keterangan = model.OrderStatus(teks("status")).Label()
		tautan = "/pesan"
	case repository.NotifListingDisetujui:
		judul = "Listing disetujui"
		keterangan = teks("judul") + " sudah tayang di pencarian."
		if sid := id("service_id"); sid != "" {
			tautan = "/jasa/" + sid
		}
	case repository.NotifListingDitolak:
		judul = "Listing perlu diperbaiki"
		keterangan = teks("alasan")
		if sid := id("service_id"); sid != "" {
			tautan = "/jasa/" + sid + "/ubah"
		}
	case repository.NotifPromosiDiperbarui:
		judul = "Promosi: " + model.LabelStatusPromosi(teks("status"))
		keterangan = model.LabelJenisPromosi(teks("jenis"))
		if teks("status") == service.StatusAkanSelesai {
			judul = "Promosi berakhir besok"
			keterangan = model.LabelJenisPromosi(teks("jenis")) + " Anda habis masa tayangnya dalam 24 jam. Ajukan lagi bila ingin melanjutkan."
		}
		if c := teks("catatan"); c != "" {
			keterangan += " — " + c
		}
		if pid := id("promosi_id"); pid != "" {
			tautan = "/promosi/" + pid
		}
	default:
		judul = n.Type
	}
	return judul, keterangan, tautan
}

// ---------- pengaturan admin ----------

// AdminPengaturanData adalah data halaman pengaturan aplikasi.
type AdminPengaturanData struct {
	AdminBase
	Pengaturan model.Pengaturan
	Form       Form
	Tab        string // umum | lokasi | iklan
	// MaksSizeMB diambil dari konfigurasi unggahan supaya batas yang
	// disebutkan di layar selalu sama dengan yang ditolak server.
	MaksSizeMB int64
}

// GayaPratinjauHero dan GayaPratinjauOverlay memakai jalur yang sama persis
// dengan beranda, sehingga yang dilihat admin di panel pengaturan adalah hasil
// yang sebenarnya, bukan perkiraan.
func (d AdminPengaturanData) GayaPratinjauHero() string {
	return SitusRingkas{HeroGambarURL: d.Pengaturan.Tampilan.HeroGambarURL}.GayaHero()
}

func (d AdminPengaturanData) GayaPratinjauOverlay() string {
	return SitusRingkas{HeroOverlay: d.Pengaturan.Tampilan.HeroOverlay}.GayaOverlayHero()
}

// LencanaPratinjau menyusun bentuk lencana persis seperti yang akan muncul di
// beranda, sehingga admin melihat hasil sebenarnya sebelum menyimpan.
func (d AdminPengaturanData) LencanaPratinjau() SitusRingkas {
	return SitusRingkas{
		AplikasiTampil:     true,
		PlaystoreURL:       d.Pengaturan.Tampilan.PlaystoreURL,
		PlaystoreTeksAtas:  d.Pengaturan.Tampilan.PlaystoreTeksAtas,
		PlaystoreTeksBawah: d.Pengaturan.Tampilan.PlaystoreTeksBawah,
	}
}

// PetaConfigPengaturan menyusun konfigurasi peta titik pusat platform.
func (d AdminPengaturanData) PetaConfigPengaturan() string {
	cfg := map[string]any{
		"lat":    d.Pengaturan.Lokasi.PusatLatitude,
		"lng":    d.Pengaturan.Lokasi.PusatLongitude,
		"adaPin": true,
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

// AdminAntreanData adalah data halaman antrean peninjauan listing.
type AdminAntreanData struct {
	AdminBase
	Items      []model.ServiceCard
	Total      int
	Page       int
	TotalPages int
}

func (d AdminAntreanData) AdaSebelum() bool    { return d.Page > 1 }
func (d AdminAntreanData) AdaSesudah() bool    { return d.Page < d.TotalPages }
func (d AdminAntreanData) HalamanSebelum() int { return d.Page - 1 }
func (d AdminAntreanData) HalamanSesudah() int { return d.Page + 1 }

func (d AdminAntreanData) URLHalaman(page int) string {
	return "/admin/antrean?page=" + strconv.Itoa(page)
}

// ---------- helper tampilan jarak ----------

// JarakLabel memformat jarak menjadi teks singkat yang mudah dibaca.
func JarakLabel(km *float64) string {
	if km == nil {
		return ""
	}
	v := *km
	switch {
	case v < 1:
		return fmt.Sprintf("%.0f m", v*1000)
	case v < 10:
		return fmt.Sprintf("%.1f km", v)
	default:
		return fmt.Sprintf("%.0f km", v)
	}
}

// StatusListingKelas memilih gaya penanda status listing di antarmuka.
func StatusListingKelas(s model.ServiceStatus) string {
	switch s {
	case model.ServiceActive:
		return "tanda tanda-baik"
	case model.ServiceRejected:
		return "tanda tanda-bahaya"
	case model.ServicePending:
		return "tanda tanda-sorot"
	default:
		return "tanda"
	}
}

// RadiusPilihan menyusun opsi radius pada filter pencarian.
func RadiusPilihan(maks int) []int {
	kandidat := []int{5, 10, 25, 50, 100}
	out := make([]int, 0, len(kandidat))
	for _, k := range kandidat {
		if k <= maks {
			out = append(out, k)
		}
	}
	return out
}

// TrimLabel memotong label lokasi yang terlalu panjang untuk navigasi.
func TrimLabel(s string, maks int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= maks {
		return s
	}
	return string(r[:maks]) + "…"
}

// StatusPesananKelas memilih gaya penanda status pesanan.
func StatusPesananKelas(status string) string {
	switch model.OrderStatus(status) {
	case model.OrderAccepted:
		return "tanda tanda-sorot"
	case model.OrderCompleted:
		return "tanda tanda-baik"
	case model.OrderRejected, model.OrderCancelled:
		return "tanda tanda-bahaya"
	default:
		return "tanda"
	}
}

// TautanWhatsapp membentuk tautan wa.me dengan pesan pembuka yang sudah terisi.
func TautanWhatsapp(nomor, judulJasa string) string {
	return service.WhatsappLink(nomor,
		"Halo, saya melihat jasa \""+judulJasa+"\" di Adojobs. Apakah masih tersedia?")
}
