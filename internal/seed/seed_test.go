package seed

import (
	"strings"
	"testing"
)

// Regresi: kategori induk "Tukang & Bangunan" dan anaknya "Tukang Bangunan"
// pernah menghasilkan slug sama. Karena slug adalah kunci unik dan seeder
// memakai ON CONFLICT DO UPDATE, baris induk tertimpa oleh anaknya sendiri
// sehingga parent_id menunjuk ke id-nya sendiri — dan penelusuran rekursif
// sub-kategori berputar tanpa henti, menggantung halaman detail jasa.
func TestSlugKategoriTidakBertabrakan(t *testing.T) {
	if err := periksaSlugKategori(); err != nil {
		t.Fatalf("data kategori memiliki tabrakan slug: %v", err)
	}
}

func TestPeriksaSlugKategoriMendeteksiTabrakan(t *testing.T) {
	asli := daftarKategori
	t.Cleanup(func() { daftarKategori = asli })

	daftarKategori = []kategoriBenih{
		{Nama: "Tukang & Bangunan", Ikon: "rumah-bangun", Anak: []string{"Tukang Bangunan"}},
	}

	err := periksaSlugKategori()
	if err == nil {
		t.Fatal("tabrakan slug seharusnya terdeteksi")
	}
	if !strings.Contains(err.Error(), "tukang-bangunan") {
		t.Errorf("pesan galat tidak menyebut slug yang bertabrakan: %v", err)
	}
}

func TestSetiapKategoriPunyaAnak(t *testing.T) {
	for _, induk := range daftarKategori {
		if len(induk.Anak) == 0 {
			t.Errorf("kategori induk %q tidak punya sub-kategori", induk.Nama)
		}
		if induk.Ikon == "" {
			t.Errorf("kategori induk %q tidak punya ikon", induk.Nama)
		}
	}
}

// Setiap jasa contoh harus menunjuk slug kategori yang benar-benar ada,
// karena seeder gagal bila kategorinya tidak ditemukan.
func TestJasaContohMemakaiKategoriYangAda(t *testing.T) {
	tersedia := map[string]bool{}
	for _, induk := range daftarKategori {
		tersedia[slug(induk.Nama)] = true
		for _, anak := range induk.Anak {
			tersedia[slug(anak)] = true
		}
	}

	for _, p := range daftarProvider {
		if len(p.Jasa) == 0 {
			t.Errorf("provider contoh %q tidak punya jasa", p.Nama)
		}
		for _, j := range p.Jasa {
			if !tersedia[j.Kategori] {
				t.Errorf("jasa %q memakai kategori %q yang tidak ada di daftar kategori",
					j.Judul, j.Kategori)
			}
			// Harga wajib diisi kecuali jenis harganya nego.
			if j.JenisHarga != "negotiable" && j.HargaMin <= 0 {
				t.Errorf("jasa %q berjenis harga %q tapi tidak punya harga", j.Judul, j.JenisHarga)
			}
		}
	}
}
