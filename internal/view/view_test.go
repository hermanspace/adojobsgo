package view

import (
	"testing"

	"github.com/hermansyah/adojobsid/internal/model"
)

func TestRupiah(t *testing.T) {
	kasus := map[float64]string{
		0:        "0",
		1000:     "1.000",
		85000:    "85.000",
		150000:   "150.000",
		2500000:  "2.500.000",
		12345678: "12.345.678",
	}
	for masukan, harapan := range kasus {
		if got := Rupiah(masukan); got != harapan {
			t.Errorf("Rupiah(%v) = %q, harusnya %q", masukan, got, harapan)
		}
	}
}

func TestHargaLabel(t *testing.T) {
	min, max := 85000.0, 120000.0

	if got := HargaLabel(model.PriceFixed, &min, &max); got != "Rp 85.000–120.000" {
		t.Errorf("rentang harga salah: %q", got)
	}
	if got := HargaLabel(model.PriceFixed, &min, nil); got != "Rp 85.000" {
		t.Errorf("harga tunggal salah: %q", got)
	}
	if got := HargaLabel(model.PriceHourly, &min, nil); got != "Rp 85.000/jam" {
		t.Errorf("harga per jam salah: %q", got)
	}
	if got := HargaLabel(model.PriceNegotiable, &min, &max); got != "Nego" {
		t.Errorf("harga nego seharusnya mengabaikan angka: %q", got)
	}
	// Harga tetap tanpa angka tidak boleh menampilkan "Rp " kosong.
	if got := HargaLabel(model.PriceFixed, nil, nil); got != "Nego" {
		t.Errorf("harga kosong salah: %q", got)
	}
	// Batas atas yang sama dengan batas bawah ditampilkan sebagai satu angka.
	sama := 85000.0
	if got := HargaLabel(model.PriceFixed, &min, &sama); got != "Rp 85.000" {
		t.Errorf("rentang dengan nilai sama salah: %q", got)
	}
}

func TestLokasi(t *testing.T) {
	kec, kota := "Bantan", "Bengkalis"
	if got := Lokasi(&kec, &kota); got != "Bantan, Bengkalis" {
		t.Errorf("lokasi lengkap salah: %q", got)
	}
	if got := Lokasi(nil, &kota); got != "Bengkalis" {
		t.Errorf("lokasi tanpa kecamatan salah: %q", got)
	}
	if got := Lokasi(nil, nil); got != "Bengkalis" {
		t.Errorf("lokasi kosong harus jatuh ke Bengkalis: %q", got)
	}
	kosong := "   "
	if got := Lokasi(&kosong, &kota); got != "Bengkalis" {
		t.Errorf("kecamatan berisi spasi harus diabaikan: %q", got)
	}
}

func TestInitials(t *testing.T) {
	kasus := map[string]string{
		"Rizal Teknik AC":  "RA",
		"Hafiz":            "H",
		"  Dapur Melayu  ": "DM",
		"":                 "?",
	}
	for masukan, harapan := range kasus {
		if got := Initials(masukan); got != harapan {
			t.Errorf("Initials(%q) = %q, harusnya %q", masukan, got, harapan)
		}
	}
}

func TestSearchDataURLDengan(t *testing.T) {
	d := SearchData{Query: "servis ac", Kecamatan: "Bantan", Page: 3}

	// Mengganti kategori harus mempertahankan filter lain dan membuang halaman.
	got := d.URLDengan("kategori", "servis-ac")
	for _, wajib := range []string{"kategori=servis-ac", "kecamatan=Bantan", "q=servis+ac"} {
		if !contains(got, wajib) {
			t.Errorf("URL %q seharusnya memuat %q", got, wajib)
		}
	}
	if contains(got, "page=") {
		t.Errorf("URL %q seharusnya tidak membawa nomor halaman", got)
	}

	kosong := SearchData{}
	if kosong.URLDengan("kategori", "") != "/cari" {
		t.Error("tanpa filter seharusnya menghasilkan /cari polos")
	}
}

func TestSearchDataFilterAktif(t *testing.T) {
	d := SearchData{CategorySlug: "servis-ac", Kecamatan: "Bantan", Sort: "terbaru"}
	// Sort "terbaru" adalah nilai bawaan, tidak dihitung sebagai filter.
	if n := d.FilterAktif(); n != 2 {
		t.Errorf("FilterAktif() = %d, harusnya 2", n)
	}
	d.Sort = "rating"
	if n := d.FilterAktif(); n != 3 {
		t.Errorf("FilterAktif() = %d, harusnya 3", n)
	}
}

func TestPotong(t *testing.T) {
	if got := Potong("pendek", 20); got != "pendek" {
		t.Errorf("teks pendek tidak boleh dipotong: %q", got)
	}
	panjang := "Cuci AC split rumah lengkap dengan pengecekan tekanan freon"
	got := Potong(panjang, 20)
	if len([]rune(got)) > 21 {
		t.Errorf("hasil potong terlalu panjang: %q", got)
	}
	if !contains(got, "…") {
		t.Errorf("hasil potong harus diakhiri elipsis: %q", got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
