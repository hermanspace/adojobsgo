package view

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/hermansyah/adojobsid/internal/model"
)

// HomeData adalah data beranda.
type HomeData struct {
	Base
	Kategori []model.Category
	Terbaru  []model.ServiceCard
	Total    int
	Pilihan  []model.ProviderDetail
}

// SearchData adalah data halaman pencarian beserta nilai filter yang sedang aktif.
type SearchData struct {
	Base
	Query        string
	CategorySlug string
	Kecamatan    string
	PriceType    string
	Sort         string
	// RadiusKm 0 berarti tanpa batas jarak.
	RadiusKm      int
	RadiusMaks    int
	OnlyReachable bool
	Kategori      []model.Category
	KecamatanOps  []string
	Items         []model.ServiceCard
	Total         int
	Page          int
	TotalPages    int
}

func (d SearchData) HasNext() bool { return d.Page < d.TotalPages }

// FilterAktif menghitung berapa filter yang sedang dipakai, untuk badge tombol filter.
func (d SearchData) FilterAktif() int {
	n := 0
	for _, v := range []string{d.CategorySlug, d.Kecamatan, d.PriceType} {
		if strings.TrimSpace(v) != "" {
			n++
		}
	}
	if d.Sort != "" && d.Sort != "terbaru" {
		n++
	}
	if d.RadiusKm > 0 {
		n++
	}
	if d.OnlyReachable {
		n++
	}
	return n
}

// PilihanRadius menyusun opsi radius pada filter pencarian.
func (d SearchData) PilihanRadius() []int { return RadiusPilihan(d.RadiusMaks) }

// URLDengan membangun ulang URL pencarian dengan satu parameter diganti.
// Dipakai oleh chip kategori dan tautan halaman berikutnya.
func (d SearchData) URLDengan(key, value string) string {
	q := url.Values{}
	set := func(k, v string) {
		if strings.TrimSpace(v) != "" {
			q.Set(k, v)
		}
	}
	set("q", d.Query)
	set("kategori", d.CategorySlug)
	set("kecamatan", d.Kecamatan)
	set("harga", d.PriceType)
	if d.Sort != "" && d.Sort != "terbaru" {
		set("urut", d.Sort)
	}
	if d.RadiusKm > 0 {
		set("radius", strconv.Itoa(d.RadiusKm))
	}
	if d.OnlyReachable {
		set("menjangkau", "1")
	}

	if strings.TrimSpace(value) == "" {
		q.Del(key)
	} else {
		q.Set(key, value)
	}
	q.Del("page") // ganti filter selalu kembali ke halaman pertama

	if len(q) == 0 {
		return "/cari"
	}
	return "/cari?" + q.Encode()
}

// URLHalaman menghasilkan URL untuk halaman hasil berikutnya (dipakai HTMX).
func (d SearchData) URLHalaman(page int) string {
	base := d.URLDengan("", "")
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + "page=" + strconv.Itoa(page)
}

// NamaKategori mengembalikan nama kategori yang sedang dipilih, bila ada.
func (d SearchData) NamaKategori() string {
	if d.CategorySlug == "" {
		return ""
	}
	for _, k := range d.Kategori {
		if k.Slug == d.CategorySlug {
			return k.Name
		}
	}
	// Sub-kategori tidak ada di daftar chip (yang hanya memuat kategori induk),
	// jadi slug-nya dipakai apa adanya sebagai penjelas.
	return strings.ReplaceAll(d.CategorySlug, "-", " ")
}

// RingkasanFilter menyusun kalimat penjelas hasil pencarian.
func (d SearchData) RingkasanFilter() string {
	parts := make([]string, 0, 3)
	if q := strings.TrimSpace(d.Query); q != "" {
		parts = append(parts, fmt.Sprintf("%q", q))
	}
	if d.Kecamatan != "" {
		parts = append(parts, "di "+d.Kecamatan)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

// AuthData adalah data halaman masuk dan daftar.
type AuthData struct {
	Base
	Form Form
	Next string
}

// ProviderFormData adalah data form profil penyedia jasa.
type ProviderFormData struct {
	Base
	Form      Form
	IsEdit    bool
	Profile   *model.ProviderProfile
	Kecamatan []string
}

// ListingFormData adalah data form buat/edit listing jasa.
type ListingFormData struct {
	Base
	Form         Form
	IsEdit       bool
	SebagaiAdmin bool
	// StatusSaatIni dipakai untuk memperingatkan provider bahwa perubahan
	// pada bagian penting akan mengembalikan listing ke antrean peninjauan.
	StatusSaatIni model.ServiceStatus
	ServiceID     int64
	Kategori      []model.Category
	Images        []model.ServiceImage
	MaxImages     int
	MaxSizeMB     int64
}

// PeringatanTinjauUlang menandai form yang perlu memberi tahu provider bahwa
// menyimpan perubahan akan menurunkan listing dari peredaran.
func (d ListingFormData) PeringatanTinjauUlang() bool {
	return d.IsEdit && !d.SebagaiAdmin && d.StatusSaatIni == model.ServiceActive
}

// SisaFoto menghitung berapa foto lagi yang boleh diunggah.
func (d ListingFormData) SisaFoto() int {
	sisa := d.MaxImages - len(d.Images)
	if sisa < 0 {
		return 0
	}
	return sisa
}

// DashboardData adalah data halaman akun/dasbor.
type DashboardData struct {
	Base
	User     *model.User
	Provider *model.ProviderProfile
	Listings []model.ServiceCard
}

// ServiceDetailData adalah data halaman detail jasa.
type ServiceDetailData struct {
	Base
	Service *model.ServiceDetail
	IsOwner bool
	IsAdmin bool
	Serupa  []model.ServiceCard
	// JarakKm dari lokasi acuan pencari jasa ke penyedia.
	JarakKm *float64
	Form    Form
}

// Menjangkau menandai penyedia yang radius layanannya mencakup lokasi pencari.
func (d ServiceDetailData) Menjangkau() bool {
	return d.JarakKm != nil && *d.JarakKm <= float64(d.Service.Provider.ServiceRadiusKm)
}

// DapatDihubungi menandai pengunjung yang boleh memulai percakapan:
// sudah masuk, dan bukan pemilik listing itu sendiri.
func (d ServiceDetailData) DapatDihubungi() bool {
	return d.LoggedIn() && !d.IsOwner
}

// ErrorData dipakai halaman 404 dan 500.
type ErrorData struct {
	Base
	Code    int
	Heading string
	Message string
}

// TentangData adalah halaman Tentang & Panduan: isi dari Panduan(), angka
// hidup dari Ringkasan, dan paket dari database untuk bagian promosi.
type TentangData struct {
	Base
	Bagian    []PanduanBagian
	Paket     []model.PaketPromosi
	JasaAktif int
	Penyedia  int
	Kecamatan int
	KontakWA  string
	// UjiCobaGratis: semua paket lewat tanpa bayar — harga ditampilkan
	// dicoret supaya pengunjung tahu ini masa promosi, bukan harga tetap.
	UjiCobaGratis bool
}

// LabelPaket menulis nama paket dengan durasinya, kecuali namanya sudah
// menyebut durasi ("Dasar 7 hari") — supaya tidak tampil "7 hari · 7 hari".
func LabelPaket(p model.PaketPromosi) string {
	if strings.Contains(strings.ToLower(p.Nama), "hari") {
		return p.Nama
	}
	return p.Nama + " · " + strconv.Itoa(p.DurasiHari) + " hari"
}

// PaketUntuk menyaring paket menurut jenis promosi.
func (d TentangData) PaketUntuk(jenis string) []model.PaketPromosi {
	var out []model.PaketPromosi
	for _, p := range d.Paket {
		if p.Jenis == jenis {
			out = append(out, p)
		}
	}
	return out
}
