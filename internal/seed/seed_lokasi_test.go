package seed

import (
	"testing"

	"github.com/hermansyah/adojobsid/internal/model"
)

// Regresi: koordinat penyedia contoh pernah hilang dari seeder sehingga
// pencarian berbasis jarak tidak menghasilkan apa pun pada basis data baru.
func TestProviderContohPunyaKoordinat(t *testing.T) {
	for _, p := range daftarProvider {
		if !model.KoordinatValid(p.Lat, p.Lng) {
			t.Errorf("penyedia %q tidak punya koordinat yang sah (%v, %v)", p.Nama, p.Lat, p.Lng)
		}
		if p.RadiusKm < 1 || p.RadiusKm > 200 {
			t.Errorf("radius layanan %q tidak wajar: %d km", p.Nama, p.RadiusKm)
		}
		if p.Alamat == "" {
			t.Errorf("penyedia %q tidak punya keterangan alamat", p.Nama)
		}

		// Titik penyedia harus dekat dengan kecamatan yang diklaimnya,
		// supaya data contoh tidak saling bertentangan.
		kec, ok := model.CariKecamatan(p.Kecamatan)
		if !ok {
			t.Errorf("kecamatan %q milik %q tidak ada di daftar", p.Kecamatan, p.Nama)
			continue
		}
		if jarak := model.JarakKm(p.Lat, p.Lng, kec.Latitude, kec.Longitude); jarak > 30 {
			t.Errorf("titik %q berjarak %.1f km dari pusat kecamatan %q — kemungkinan salah koordinat",
				p.Nama, jarak, p.Kecamatan)
		}
	}
}

// Regresi: seeder sempat membuat seluruh penyedia dengan slug kosong,
// sehingga penyedia kedua langsung bentrok dengan indeks unik dan seeding
// gagal di tengah jalan.
func TestPenyediaContohPunyaSlugUnik(t *testing.T) {
	if err := periksaSlugPenyedia(); err != nil {
		t.Fatalf("slug penyedia contoh bermasalah: %v", err)
	}

	terlihat := map[string]bool{}
	for _, p := range daftarProvider {
		s := slug(p.Nama)
		if s == "" {
			t.Errorf("penyedia %q menghasilkan slug kosong", p.Nama)
		}
		if terlihat[s] {
			t.Errorf("slug %q dipakai lebih dari satu penyedia", s)
		}
		terlihat[s] = true
	}
}
