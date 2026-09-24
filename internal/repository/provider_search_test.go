package repository

import (
	"strings"
	"testing"
)

// Penyedia yang ditangguhkan tidak boleh pernah muncul di direktori publik,
// apa pun filternya.
func TestBuildProviderFilterSelaluMenyembunyikanYangDitangguhkan(t *testing.T) {
	where, args := buildProviderFilter(ProviderFilter{}, nil)
	if !strings.Contains(where, "u.suspended_at IS NULL") {
		t.Errorf("direktori harus menyembunyikan akun ditangguhkan: %q", where)
	}
	if len(args) != 0 {
		t.Errorf("filter kosong seharusnya tanpa argumen, dapat %d", len(args))
	}
}

func TestBuildProviderFilterPencarianTeks(t *testing.T) {
	where, args := buildProviderFilter(ProviderFilter{Query: "servis ac"}, nil)

	// Frasa dipecah per kata; tiap kata jadi argumen sendiri.
	if len(args) < 2 || args[0] != "servis" || args[1] != "ac" {
		t.Errorf("argumen pencarian = %v, harusnya [servis ac ...]", args)
	}
	if !strings.Contains(where, "word_similarity") {
		t.Error("pencarian penyedia tidak memakai kemiripan trigram")
	}
	// Pencarian menyentuh nama dan bio — pengguna bisa mengetik keahlian,
	// bukan hanya nama. Area layanan bukan kolom lagi; ia diturunkan dari
	// pin dan radius, jadi penyaringan wilayah memakai kecamatan & jarak.
	for _, kolom := range []string{"u.full_name", "p.bio"} {
		if !strings.Contains(where, kolom) {
			t.Errorf("pencarian tidak mencakup %s: %q", kolom, where)
		}
	}
}

func TestBuildProviderFilterPenyaringTambahan(t *testing.T) {
	where, _ := buildProviderFilter(ProviderFilter{
		HanyaTerverifikasi: true,
		HanyaBerjasa:       true,
	}, nil)

	if !strings.Contains(where, "p.is_verified") {
		t.Errorf("filter terverifikasi tidak diterapkan: %q", where)
	}
	if !strings.Contains(where, "COALESCE(sv.jumlah, 0) > 0") {
		t.Errorf("filter punya jasa aktif tidak diterapkan: %q", where)
	}
}

// Penyaringan radius harus memanfaatkan indeks GiST lewat earth_box.
func TestBuildProviderFilterRadius(t *testing.T) {
	lat, lng := 1.4667, 102.1
	where, args := buildProviderFilter(ProviderFilter{
		Latitude: &lat, Longitude: &lng, RadiusKm: 25,
	}, []any{lat, lng})

	if !strings.Contains(where, "earth_box(ll_to_earth($1, $2)") {
		t.Errorf("filter radius tidak memakai earth_box: %q", where)
	}
	if len(args) != 3 || args[2] != float64(25000) {
		t.Errorf("radius harus dikirim dalam meter, dapat %v", args)
	}
}

// Query COUNT tidak punya kolom jarak, jadi koordinat hanya boleh ikut bila
// ada klausa yang benar-benar merujuknya.
func TestProviderFilterPerluKoordinat(t *testing.T) {
	lat, lng := 1.4667, 102.1
	dasar := ProviderFilter{Latitude: &lat, Longitude: &lng}

	if dasar.perluKoordinat() {
		t.Error("tanpa radius, koordinat tidak dirujuk klausa mana pun")
	}
	denganRadius := dasar
	denganRadius.RadiusKm = 10
	if !denganRadius.perluKoordinat() {
		t.Error("filter radius merujuk koordinat")
	}
	if (ProviderFilter{RadiusKm: 10}).perluKoordinat() {
		t.Error("tanpa koordinat, filter radius tidak bisa dipakai")
	}
}

func TestProviderOrderClause(t *testing.T) {
	// Tanpa lokasi, urutan terdekat tidak mungkin dihitung dan harus jatuh
	// ke urutan bawaan yang tetap sah.
	tanpa := providerOrderClause("terdekat", false)
	if strings.Contains(tanpa, "jarak_km") {
		t.Errorf("tanpa lokasi tidak boleh mengurutkan berdasarkan jarak: %q", tanpa)
	}
	if !strings.HasPrefix(tanpa, "ORDER BY") {
		t.Errorf("urutan bawaan tidak sah: %q", tanpa)
	}

	dengan := providerOrderClause("terdekat", true)
	if !strings.Contains(dengan, "jarak_km ASC") || !strings.Contains(dengan, "NULLS LAST") {
		t.Errorf("urutan terdekat salah: %q", dengan)
	}

	// Urutan bawaan mendahulukan penyedia pilihan yang masa tayangnya berlaku.
	if !strings.Contains(providerOrderClause("", false), "featured_until") {
		t.Error("urutan bawaan harus mendahulukan penyedia pilihan")
	}
}

// Kartu direktori harus menghitung jasa aktif saja: listing yang menunggu
// tinjauan atau disembunyikan tidak boleh ikut terhitung.
func TestProviderCardHanyaMenghitungJasaTayang(t *testing.T) {
	sel := providerCardSelect(tanpaJarak)
	if !strings.Contains(sel, "s.status = 'active'") {
		t.Errorf("penghitung jasa tidak menyaring status tayang: %q", sel)
	}
	if !strings.Contains(sel, "LEFT JOIN LATERAL") {
		t.Error("penghitung jasa seharusnya memakai LATERAL agar tidak menimbulkan N+1")
	}
}

func TestProviderCardMenjangkau(t *testing.T) {
	jarak := 12.0
	c := ProviderCard{JarakKm: &jarak}
	c.ServiceRadiusKm = 20
	if !c.Menjangkau() {
		t.Error("jarak 12 km dalam radius 20 km seharusnya terjangkau")
	}
	c.ServiceRadiusKm = 5
	if c.Menjangkau() {
		t.Error("jarak 12 km di luar radius 5 km seharusnya tidak terjangkau")
	}
	c.JarakKm = nil
	if c.Menjangkau() {
		t.Error("tanpa jarak, keterjangkauan tidak boleh diklaim")
	}
}
