package view

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/repository"
	"github.com/hermansyah/adojobsid/internal/service"
)

// PenyediaData adalah data halaman publik satu penyedia jasa.
type PenyediaData struct {
	Base
	Provider   *model.ProviderDetail
	Listings   []model.ServiceCard
	Portofolio []model.Portfolio
	Ulasan     *service.RingkasanUlasan
	// JarakKm dari lokasi acuan pengunjung ke penyedia.
	JarakKm *float64
	// IsOwner menandai penyedia yang sedang melihat halamannya sendiri.
	IsOwner bool
	IsAdmin bool
}

// URLPenyedia menyusun alamat publik satu penyedia.
// Slug kosong berarti data penyedianya tidak lengkap terbawa dari query.
// Tautan diarahkan ke direktori supaya pengunjung tidak mendarat di 404;
// kegagalannya tetap kelihatan karena halaman yang dituju bukan yang dimaksud.
// AreaLayanan menyusun daftar kecamatan terjangkau sebagai satu baris teks.
func AreaLayanan(p model.ProviderDetail) string {
	return strings.Join(p.AreaLayanan(Deref(p.Kecamatan)), ", ")
}

func URLPenyedia(slug string) string {
	if slug == "" {
		return "/penyedia"
	}
	return "/penyedia/" + slug
}

// Menjangkau menandai penyedia yang radius layanannya mencakup lokasi pengunjung.
func (d PenyediaData) Menjangkau() bool {
	return d.JarakKm != nil && *d.JarakKm <= float64(d.Provider.ServiceRadiusKm)
}

// DapatDihubungi menandai pengunjung yang boleh memulai percakapan:
// sudah masuk, dan bukan penyedia itu sendiri.
func (d PenyediaData) DapatDihubungi() bool { return d.LoggedIn() && !d.IsOwner }

// PetaPenyedia menyusun konfigurasi peta lokasi penyedia untuk app.js.
// Peta di halaman ini hanya menampilkan, tidak bisa digeser pinnya.
func (d PenyediaData) PetaPenyedia() string {
	if d.Provider == nil || !d.Provider.PunyaLokasi() {
		return "{}"
	}
	cfg := map[string]any{
		"lat":      *d.Provider.Latitude,
		"lng":      *d.Provider.Longitude,
		"adaPin":   true,
		"radiusKm": d.Provider.ServiceRadiusKm,
		"bacaSaja": true,
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

// RingkasanJasa menyusun kalimat pendek tentang cakupan kerja penyedia.
func (d PenyediaData) RingkasanJasa() string {
	n := len(d.Listings)
	switch {
	case n == 0:
		return "Belum ada jasa yang dipasang"
	case n == 1:
		return "1 jasa ditawarkan"
	default:
		return fmt.Sprintf("%d jasa ditawarkan", n)
	}
}

// BergabungSejak menampilkan bulan dan tahun penyedia mulai bergabung.
func (d PenyediaData) BergabungSejak() string {
	if d.Provider == nil {
		return ""
	}
	return NamaBulan(d.Provider.CreatedAt.Month()) + " " + strconv.Itoa(d.Provider.CreatedAt.Year())
}

// PortofolioData adalah data halaman kelola portofolio penyedia.
type PortofolioData struct {
	Base
	Items      []model.Portfolio
	Form       Form
	MaksKarya  int
	MaksSizeMB int64
}

// SisaKarya menghitung berapa karya lagi yang boleh ditambahkan.
func (d PortofolioData) SisaKarya() int {
	sisa := d.MaksKarya - len(d.Items)
	if sisa < 0 {
		return 0
	}
	return sisa
}

// NamaBulan menerjemahkan bulan ke bahasa Indonesia, karena paket time
// hanya menyediakan nama bulan dalam bahasa Inggris.
func NamaBulan(m interface{ String() string }) string {
	nama := map[string]string{
		"January": "Januari", "February": "Februari", "March": "Maret",
		"April": "April", "May": "Mei", "June": "Juni",
		"July": "Juli", "August": "Agustus", "September": "September",
		"October": "Oktober", "November": "November", "December": "Desember",
	}
	if v, ok := nama[m.String()]; ok {
		return v
	}
	return m.String()
}

// BintangPenuh dipakai template untuk menggambar deretan bintang penilaian.
func BintangPenuh(rating int) []bool {
	out := make([]bool, 5)
	for i := 0; i < 5; i++ {
		out[i] = i < rating
	}
	return out
}

// DaftarBintang mengembalikan nilai 5 sampai 1, untuk grafik sebaran ulasan.
func DaftarBintang() []int { return []int{5, 4, 3, 2, 1} }

// ---------- direktori penyedia ----------

// DaftarPenyediaData adalah data halaman daftar penyedia jasa.
type DaftarPenyediaData struct {
	Base
	Query              string
	Kecamatan          string
	HanyaTerverifikasi bool
	HanyaBerjasa       bool
	Sort               string
	RadiusKm           int
	RadiusMaks         int

	Items        []repository.ProviderCard
	Total        int
	Page         int
	TotalPages   int
	KecamatanOps []string
}

func (d DaftarPenyediaData) AdaSebelum() bool    { return d.Page > 1 }
func (d DaftarPenyediaData) AdaSesudah() bool    { return d.Page < d.TotalPages }
func (d DaftarPenyediaData) HalamanSebelum() int { return d.Page - 1 }
func (d DaftarPenyediaData) HalamanSesudah() int { return d.Page + 1 }

// PilihanRadius menyusun opsi radius pada filter direktori.
func (d DaftarPenyediaData) PilihanRadius() []int { return RadiusPilihan(d.RadiusMaks) }

// URLDengan membangun ulang URL direktori dengan satu parameter diganti.
func (d DaftarPenyediaData) URLDengan(key, value string) string {
	q := url.Values{}
	set := func(k, v string) {
		if strings.TrimSpace(v) != "" {
			q.Set(k, v)
		}
	}
	set("q", d.Query)
	set("kecamatan", d.Kecamatan)
	if d.Sort != "" && d.Sort != "jasa" {
		set("urut", d.Sort)
	}
	if d.RadiusKm > 0 {
		set("radius", strconv.Itoa(d.RadiusKm))
	}
	if d.HanyaTerverifikasi {
		set("terverifikasi", "1")
	}
	if d.HanyaBerjasa {
		set("berjasa", "1")
	}

	if strings.TrimSpace(value) == "" {
		q.Del(key)
	} else {
		q.Set(key, value)
	}
	q.Del("page")

	if len(q) == 0 {
		return "/penyedia"
	}
	return "/penyedia?" + q.Encode()
}

func (d DaftarPenyediaData) URLHalaman(page int) string {
	base := d.URLDengan("", "")
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + "page=" + strconv.Itoa(page)
}

// JumlahPenyedia menyusun teks jumlah hasil.
func (d DaftarPenyediaData) JumlahPenyedia() string {
	if d.Total == 0 {
		return "Tidak ada penyedia ditemukan"
	}
	return fmt.Sprintf("%d penyedia jasa", d.Total)
}

// RingkasJasa menyusun keterangan jumlah jasa satu penyedia.
func RingkasJasa(total int) string {
	if total == 0 {
		return "Belum ada jasa"
	}
	return fmt.Sprintf("%d jasa", total)
}
