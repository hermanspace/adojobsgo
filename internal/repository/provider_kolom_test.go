package repository

import (
	"strings"
	"testing"

	"github.com/hermansyah/adojobsid/internal/model"
)

// kolomProviderDetail memecah providerDetailKolom menjadi nama kolomnya.
func kolomProviderDetail() []string {
	var kolom []string
	for _, bagian := range strings.Split(providerDetailKolom, ",") {
		if b := strings.TrimSpace(bagian); b != "" {
			kolom = append(kolom, b)
		}
	}
	return kolom
}

// Jumlah kolom SELECT dan jumlah target Scan harus selalu sama. Pemisahan ini
// pernah menyimpang diam-diam: kolom slug ditambahkan di satu query saja,
// query-nya tetap sukses, dan tautan penyedia jadi /penyedia/ tanpa slug.
func TestTargetProviderDetailSepadanDenganKolomnya(t *testing.T) {
	var d model.ProviderDetail
	target := targetProviderDetail(&d)
	kolom := kolomProviderDetail()

	if len(target) != len(kolom) {
		t.Fatalf("providerDetailKolom punya %d kolom tapi targetProviderDetail punya %d target.\nkolom: %v",
			len(kolom), len(target), kolom)
	}
}

// Setiap kolom harus berprefiks p. atau u. karena dipakai di beberapa query yang
// alias tabelnya berbeda-beda isinya tapi selalu memakai dua alias ini.
func TestKolomProviderDetailSelaluBeralias(t *testing.T) {
	for _, k := range kolomProviderDetail() {
		if !strings.HasPrefix(k, "p.") && !strings.HasPrefix(k, "u.") {
			t.Errorf("kolom %q tanpa alias tabel; query yang menggabung tabel lain jadi ambigu", k)
		}
	}
}

// ServiceRepository.GetDetail harus memakai fragmen bersama, bukan menyalinnya.
func TestGetDetailMemakaiKolomBersama(t *testing.T) {
	if !strings.Contains(providerDetailSelect, providerDetailKolom) {
		t.Error("providerDetailSelect tidak lagi memakai providerDetailKolom")
	}
	// slug adalah kolom yang dulu tercecer; dijaga eksplisit.
	if !strings.Contains(providerDetailKolom, "p.slug") {
		t.Error("p.slug hilang dari providerDetailKolom; tautan /penyedia/:slug akan kosong")
	}
}
