package view

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/service"
)

func TestURLPenyedia(t *testing.T) {
	if got := URLPenyedia("rizal-teknik-ac"); got != "/penyedia/rizal-teknik-ac" {
		t.Errorf("URLPenyedia = %q", got)
	}
}

func TestPenyediaDataMenjangkau(t *testing.T) {
	p := &model.ProviderDetail{}
	p.ServiceRadiusKm = 20

	dekat := 10.0
	if !(PenyediaData{Provider: p, JarakKm: &dekat}).Menjangkau() {
		t.Error("jarak 10 km dalam radius 20 km seharusnya terjangkau")
	}

	jauh := 35.0
	if (PenyediaData{Provider: p, JarakKm: &jauh}).Menjangkau() {
		t.Error("jarak 35 km di luar radius 20 km seharusnya tidak terjangkau")
	}

	// Tanpa lokasi pengunjung, keterjangkauan tidak bisa disimpulkan.
	if (PenyediaData{Provider: p}).Menjangkau() {
		t.Error("tanpa jarak, keterjangkauan tidak boleh diklaim")
	}
}

func TestPetaPenyedia(t *testing.T) {
	// Penyedia tanpa titik lokasi tidak menghasilkan konfigurasi peta.
	kosong := PenyediaData{Provider: &model.ProviderDetail{}}
	if got := kosong.PetaPenyedia(); got != "{}" {
		t.Errorf("tanpa lokasi = %q, harusnya {}", got)
	}

	lat, lng := 1.4667, 102.1
	p := &model.ProviderDetail{}
	p.Latitude, p.Longitude, p.ServiceRadiusKm = &lat, &lng, 25

	var cfg map[string]any
	if err := json.Unmarshal([]byte((PenyediaData{Provider: p}).PetaPenyedia()), &cfg); err != nil {
		t.Fatalf("konfigurasi peta bukan JSON yang sah: %v", err)
	}
	if cfg["lat"] != 1.4667 || cfg["lng"] != 102.1 {
		t.Errorf("koordinat peta salah: %v", cfg)
	}
	// Peta di halaman publik harus baca-saja: pengunjung tidak boleh
	// menggeser titik lokasi penyedia.
	if cfg["bacaSaja"] != true {
		t.Error("peta halaman publik harus ditandai baca-saja")
	}
	if cfg["radiusKm"] != float64(25) {
		t.Errorf("radius peta = %v, harusnya 25", cfg["radiusKm"])
	}
}

func TestRingkasanJasa(t *testing.T) {
	kasus := map[int]string{
		0: "Belum ada jasa yang dipasang",
		1: "1 jasa ditawarkan",
		3: "3 jasa ditawarkan",
	}
	for jumlah, harapan := range kasus {
		d := PenyediaData{Listings: make([]model.ServiceCard, jumlah)}
		if got := d.RingkasanJasa(); got != harapan {
			t.Errorf("%d listing = %q, harusnya %q", jumlah, got, harapan)
		}
	}
}

func TestBergabungSejakMemakaiBulanIndonesia(t *testing.T) {
	p := &model.ProviderDetail{}
	p.CreatedAt = time.Date(2026, time.August, 15, 0, 0, 0, 0, time.UTC)

	if got := (PenyediaData{Provider: p}).BergabungSejak(); got != "Agustus 2026" {
		t.Errorf("BergabungSejak = %q, harusnya Agustus 2026", got)
	}
}

func TestNamaBulan(t *testing.T) {
	kasus := map[time.Month]string{
		time.January: "Januari", time.March: "Maret", time.May: "Mei",
		time.August: "Agustus", time.December: "Desember",
	}
	for bulan, harapan := range kasus {
		if got := NamaBulan(bulan); got != harapan {
			t.Errorf("NamaBulan(%v) = %q, harusnya %q", bulan, got, harapan)
		}
	}
}

func TestBintangPenuh(t *testing.T) {
	kasus := map[int]string{
		0: "-----",
		3: "***--",
		5: "*****",
	}
	for rating, harapan := range kasus {
		var got string
		for _, penuh := range BintangPenuh(rating) {
			if penuh {
				got += "*"
			} else {
				got += "-"
			}
		}
		if got != harapan {
			t.Errorf("BintangPenuh(%d) = %q, harusnya %q", rating, got, harapan)
		}
	}
	if len(BintangPenuh(3)) != 5 {
		t.Error("BintangPenuh harus selalu mengembalikan 5 elemen")
	}
}

func TestPortofolioSisaKarya(t *testing.T) {
	d := PortofolioData{MaksKarya: 24, Items: make([]model.Portfolio, 20)}
	if got := d.SisaKarya(); got != 4 {
		t.Errorf("SisaKarya = %d, harusnya 4", got)
	}

	// Melebihi batas tidak boleh menghasilkan angka negatif.
	penuh := PortofolioData{MaksKarya: 24, Items: make([]model.Portfolio, 30)}
	if got := penuh.SisaKarya(); got != 0 {
		t.Errorf("SisaKarya saat melebihi batas = %d, harusnya 0", got)
	}
}

func TestRingkasanUlasanPersenBintang(t *testing.T) {
	r := service.RingkasanUlasan{
		Total:   10,
		Sebaran: map[int]int{5: 7, 4: 2, 1: 1},
	}
	if got := r.PersenBintang(5); got != 70 {
		t.Errorf("persen bintang 5 = %d, harusnya 70", got)
	}
	if got := r.PersenBintang(3); got != 0 {
		t.Errorf("bintang tanpa ulasan = %d, harusnya 0", got)
	}

	// Tanpa ulasan sama sekali, pembagian nol tidak boleh terjadi.
	kosong := service.RingkasanUlasan{Sebaran: map[int]int{}}
	if got := kosong.PersenBintang(5); got != 0 {
		t.Errorf("tanpa ulasan = %d, harusnya 0", got)
	}
}

func TestDaftarBintangMenurun(t *testing.T) {
	got := DaftarBintang()
	harapan := []int{5, 4, 3, 2, 1}
	if len(got) != len(harapan) {
		t.Fatalf("jumlah = %d, harusnya %d", len(got), len(harapan))
	}
	for i := range harapan {
		if got[i] != harapan[i] {
			t.Errorf("urutan bintang salah: %v", got)
			break
		}
	}
}

// Slug kosong pernah menghasilkan /penyedia/ yang selalu 404. Sekarang jatuh ke
// direktori penyedia, bukan ke halaman mati.
func TestURLPenyediaSlugKosong(t *testing.T) {
	if got := URLPenyedia(""); got != "/penyedia" {
		t.Errorf("URLPenyedia(\"\") = %q, harusnya /penyedia", got)
	}
}
