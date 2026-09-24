package service

import (
	"testing"

	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/repository"
)

func ptr(v float64) *float64 { return &v }

// Dua pengguna dalam satu sel ~1 km harus berbagi satu kunci cache; yang
// berbeda sel harus punya kunci berbeda. Inilah yang dulu gagal: pembulatan
// 100 m memberi tiap pengguna GPS selnya sendiri.
func TestKunciCacheBerbagiPerSel(t *testing.T) {
	kunci := func(lat, lng float64) string {
		sl, sg := BulatkanSel(lat), BulatkanSel(lng)
		f := repository.ServiceFilter{Latitude: &sl, Longitude: &sg, RadiusKm: 25, Limit: 20}
		return f.CacheKey()
	}
	a := kunci(1.4667, 102.1000)
	b := kunci(1.4692, 102.1038) // ~300 m dari a, masih sel yang sama
	c := kunci(1.4867, 102.1000) // ~2 km ke utara, sel lain
	if a != b {
		t.Errorf("dua titik dalam satu sel dapat kunci berbeda:\n  %s\n  %s", a, b)
	}
	if a == c {
		t.Errorf("dua sel berbeda dapat kunci sama: %s", a)
	}
}

// Jarak yang dilihat pengguna dihitung dari titiknya sendiri, bukan dari
// pusat sel — kalau tidak, dua orang di sel yang sama melihat angka yang sama
// padahal posisinya beda ratusan meter.
func TestHitungUlangJarakDariTitikPersis(t *testing.T) {
	items := []model.ServiceCard{
		{ProviderLat: ptr(1.4700), ProviderLng: ptr(102.1000)},
		{ProviderLat: ptr(1.4667), ProviderLng: ptr(102.1000)},
		{}, // penyedia tanpa lokasi
	}
	// Pengguna persis di penyedia kedua.
	HitungUlangJarak(items, 1.4667, 102.1000, true)

	if items[0].JarakKm == nil || *items[0].JarakKm > 0.01 {
		t.Errorf("penyedia di titik pengguna harusnya berjarak ~0, dapat %v", items[0].JarakKm)
	}
	if items[1].JarakKm == nil || *items[1].JarakKm < 0.3 || *items[1].JarakKm > 0.45 {
		t.Errorf("penyedia 0,0033° ke utara harusnya ~0,37 km, dapat %v", items[1].JarakKm)
	}
	if items[2].JarakKm != nil {
		t.Error("penyedia tanpa koordinat tidak boleh diberi jarak")
	}
	// urutan terdekat: 0 km, 0,37 km, lalu yang tanpa jarak di belakang.
	if items[2].ProviderLat != nil {
		t.Error("penyedia tanpa lokasi harus berada di urutan terakhir")
	}
}

// Tanpa permintaan urut terdekat, urutan hasil (mis. terbaru) tidak boleh
// diacak oleh perhitungan jarak.
func TestHitungUlangJarakTidakMengubahUrutan(t *testing.T) {
	items := []model.ServiceCard{
		{ProviderLat: ptr(1.50), ProviderLng: ptr(102.10)},   // jauh
		{ProviderLat: ptr(1.4667), ProviderLng: ptr(102.10)}, // dekat
	}
	items[0].ID, items[1].ID = 1, 2
	HitungUlangJarak(items, 1.4667, 102.10, false)
	if items[0].ID != 1 {
		t.Error("urutan berubah padahal tidak diminta urut terdekat")
	}
}
