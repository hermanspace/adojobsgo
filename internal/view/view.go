// Package view berisi data dan helper yang dipakai template templ.
// Dipisahkan dari package handler agar template tidak pernah mengimpor handler
// (dan sebaliknya), sehingga tidak ada impor melingkar.
package view

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/validator"
)

// CurrentUser adalah ringkasan pengguna yang sedang masuk, diambil dari sesi.
type CurrentUser struct {
	ID         int64
	FullName   string
	IsProvider bool
	IsAdmin    bool
	ProviderID int64
	AvatarURL  string
}

// Initials dipakai sebagai avatar cadangan saat pengguna belum mengunggah foto.
func (u *CurrentUser) Initials() string { return Initials(u.FullName) }

// Base adalah data yang tersedia di setiap halaman.
type Base struct {
	Title       string
	Description string
	User        *CurrentUser
	ActiveNav   string // beranda | cari | jual | pesanan | akun
	Flash       *Flash
	Path        string
	Assets      *Assets
	// Lokasi acuan pencari jasa pada permintaan ini.
	Lokasi LokasiRingkas
	// BelumDibaca adalah jumlah pesan chat yang belum dibaca pengguna.
	BelumDibaca int
	// Pengaturan platform yang memengaruhi tampilan (logo, nama situs).
	Situs SitusRingkas
	// Iklan dibawa di Base karena slotnya tersebar di beberapa halaman;
	// menaruhnya di tiap data halaman berarti mengulang hal yang sama.
	Iklan model.PengaturanIklan
	// PilihIklan, bila disetel handler, mencoba iklan berbayar untuk satu slot
	// sebelum jatuh ke materi bawaan di Iklan. Berbentuk fungsi supaya
	// pemilihan — dan pencatatan tayangnya — terjadi tepat saat slot itu
	// dirender, bukan untuk slot yang tidak ada di halaman.
	PilihIklan func(kunci string) *model.SlotIklan
	// PakaiPeta menyalakan pemuatan Leaflet (≈150 KB JS + CSS). Hanya tiga
	// halaman yang punya peta; memuatnya di semua halaman berarti sembilan
	// dari sepuluh tampilan membayar untuk sesuatu yang tidak dipakai.
	PakaiPeta bool
}

// DenganPeta menandai halaman yang menggambar peta. Dipanggil di handler,
// karena layout dirender lebih dulu daripada isi halaman dan tidak bisa
// tahu sendiri apakah di bawahnya nanti ada peta.
func (b Base) DenganPeta() Base { b.PakaiPeta = true; return b }

// Iklan mencari slot yang siap tayang pada penempatan tertentu.
// Mengembalikan nil bila slotnya kosong, dinonaktifkan, atau tidak dikenal —
// template cukup memeriksa nil, tidak perlu tahu alasannya.
func (b Base) SlotIklan(kunci string) *model.SlotIklan {
	if b.PilihIklan != nil {
		if s := b.PilihIklan(kunci); s != nil {
			return s
		}
	}
	return b.Iklan.Cari(kunci)
}

// LokasiRingkas adalah bentuk lokasi aktif yang dipakai template.
type LokasiRingkas struct {
	Label     string
	Latitude  float64
	Longitude float64
	Tepat     bool
	Sumber    string
}

// Ditentukan menandai lokasi yang benar-benar dipilih pengguna.
func (l LokasiRingkas) Ditentukan() bool { return l.Sumber == "pilihan" }

// SitusRingkas memuat identitas platform dari pengaturan admin.
type SitusRingkas struct {
	Nama       string
	LogoURL    string
	TeksFooter string
	// WhatsappAktif menentukan apakah tombol WhatsApp ditampilkan sama sekali.
	WhatsappAktif bool
	// HeroGambarURL kosong berarti hero memakai latar polos bawaan.
	HeroGambarURL string
	HeroOverlay   int

	AplikasiTampil     bool
	PlaystoreURL       string
	PlaystoreTeksAtas  string
	PlaystoreTeksBawah string
}

// PakaiHeroGambar menandai hero yang punya foto latar.
func (s SitusRingkas) PakaiHeroGambar() bool { return s.HeroGambarURL != "" }

// AdaAplikasi menandai lencana aplikasi yang dinyalakan admin. Tautannya boleh
// kosong: selama aplikasinya belum terbit, lencana "segera hadir" tetap layak
// ditampilkan, hanya saja tidak bisa diklik.
func (s SitusRingkas) AdaAplikasi() bool { return s.AplikasiTampil }

// AplikasiDapatDiklik membedakan lencana yang benar-benar menuju Play Store
// dari lencana pengumuman yang belum punya tujuan.
func (s SitusRingkas) AplikasiDapatDiklik() bool { return s.PlaystoreURL != "" }

// GayaHero menyusun properti CSS latar hero sebagai atribut style.
// Ditulis sebagai style sebaris, bukan blok <style>, karena URL gambarnya
// berasal dari data dan Content-Security-Policy melarang skrip sebaris.
func (s SitusRingkas) GayaHero() string {
	if !s.PakaiHeroGambar() {
		return ""
	}
	return "background-image: url(" + s.HeroGambarURL + ")"
}

// GayaOverlayHero menyusun kepekatan lapisan gelap di atas foto hero.
func (s SitusRingkas) GayaOverlayHero() string {
	return "opacity: " + strconv.FormatFloat(float64(s.HeroOverlay)/100, 'f', 2, 64)
}

func (b Base) LoggedIn() bool { return b.User != nil }

// Flash adalah pesan sekali tampil setelah aksi berhasil atau gagal.
type Flash struct {
	Kind    string // sukses | galat | info
	Message string
}

// Form membawa nilai isian dan pesan kesalahan agar form bisa dirender ulang
// tanpa kehilangan input pengguna saat validasi gagal.
type Form struct {
	Values map[string]string
	Errors validator.Errors
}

func NewForm() Form {
	return Form{Values: map[string]string{}, Errors: validator.New()}
}

func (f Form) Value(key string) string { return f.Values[key] }

func (f Form) Err(key string) string {
	if f.Errors == nil {
		return ""
	}
	return f.Errors[key]
}

func (f Form) HasErr(key string) bool { return f.Err(key) != "" }

func (f Form) Set(key, value string) Form {
	if f.Values == nil {
		f.Values = map[string]string{}
	}
	f.Values[key] = value
	return f
}

// Selected membantu menandai <option> terpilih saat form dirender ulang.
func (f Form) Selected(key, value string) bool { return f.Values[key] == value }

// ---------- helper format ----------

// Rupiah memformat angka ke format mata uang Indonesia: 150.000
// RupiahLabel adalah Rupiah beserta awalan "Rp " — untuk nominal yang berdiri
// sendiri (harga paket, tagihan), bukan rentang harga yang sudah berawalan.
func RupiahLabel(v int64) string { return "Rp " + Rupiah(float64(v)) }

func Rupiah(v float64) string {
	n := int64(math.Round(v))
	neg := n < 0
	if neg {
		n = -n
	}
	s := fmt.Sprintf("%d", n)

	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// HargaLabel menyusun teks harga sesuai jenis harga listing.
func HargaLabel(priceType model.PriceType, min, max *float64) string {
	switch priceType {
	case model.PriceNegotiable:
		return "Nego"
	case model.PriceHourly:
		if min == nil {
			return "Nego"
		}
		return "Rp " + Rupiah(*min) + "/jam"
	default:
		if min == nil {
			return "Nego"
		}
		if max != nil && *max > *min {
			return "Rp " + Rupiah(*min) + "–" + Rupiah(*max)
		}
		return "Rp " + Rupiah(*min)
	}
}

// RatingLabel menampilkan rating satu angka desimal.
func RatingLabel(avg float64) string { return fmt.Sprintf("%.1f", avg) }

// Lokasi menggabungkan kecamatan dan kota menjadi satu baris yang ringkas.
func Lokasi(kecamatan, city *string) string {
	parts := make([]string, 0, 2)
	if kecamatan != nil && strings.TrimSpace(*kecamatan) != "" {
		parts = append(parts, strings.TrimSpace(*kecamatan))
	}
	if city != nil && strings.TrimSpace(*city) != "" {
		parts = append(parts, strings.TrimSpace(*city))
	}
	if len(parts) == 0 {
		return "Bengkalis"
	}
	return strings.Join(parts, ", ")
}

// WaktuRelatif menampilkan jarak waktu dalam bahasa Indonesia.
func WaktuRelatif(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "baru saja"
	case d < time.Hour:
		return fmt.Sprintf("%d menit lalu", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d jam lalu", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%d hari lalu", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%d bulan lalu", int(d.Hours()/24/30))
	default:
		return t.Format("Jan 2006")
	}
}

// Initials mengambil maksimal dua huruf awal dari nama.
func Initials(name string) string {
	fields := strings.Fields(strings.TrimSpace(name))
	if len(fields) == 0 {
		return "?"
	}
	out := strings.ToUpper(string([]rune(fields[0])[0]))
	if len(fields) > 1 {
		out += strings.ToUpper(string([]rune(fields[len(fields)-1])[0]))
	}
	return out
}

// Deref mengembalikan isi pointer string, atau string kosong bila nil.
func Deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Potong memotong teks pada batas kata terdekat.
func Potong(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	cut := string(r[:max])
	if idx := strings.LastIndex(cut, " "); idx > max/2 {
		cut = cut[:idx]
	}
	return cut + "…"
}
