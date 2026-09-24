package repository

import (
	"strings"
	"testing"
)

func TestBuildFilterSelaluMembatasiListingAktif(t *testing.T) {
	where, args := buildFilter(ServiceFilter{}, nil)
	if !strings.Contains(where, "s.status = 'active'") {
		t.Errorf("filter kosong harus tetap membatasi listing aktif: %q", where)
	}
	if len(args) != 0 {
		t.Errorf("filter kosong seharusnya tanpa argumen, dapat %d", len(args))
	}
}

func TestBuildFilterMenomoriArgumenBerurutan(t *testing.T) {
	where, args := buildFilter(ServiceFilter{
		Query:       "servis ac",
		CategoryIDs: []int64{1, 2},
		Kecamatan:   "Bantan",
		PriceType:   "fixed",
	}, nil)

	// "servis ac" menjadi dua argumen kata, lalu kategori, kecamatan, harga.
	if len(args) != 5 {
		t.Fatalf("jumlah argumen = %d, harusnya 5", len(args))
	}
	// Setiap placeholder harus punya argumen yang bersesuaian.
	for _, p := range []string{"$1", "$2", "$3", "$4", "$5"} {
		if !strings.Contains(where, p) {
			t.Errorf("placeholder %s tidak ada di %q", p, where)
		}
	}
	if args[0] != "servis" || args[1] != "ac" {
		t.Errorf("argumen kata kunci = %v, harusnya servis, ac", args[:2])
	}
}

func TestOrderClause(t *testing.T) {
	if !strings.Contains(orderClause("rating", false, ""), "avg_rating DESC") {
		t.Error("urutan rating harus memakai avg_rating")
	}
	if !strings.Contains(orderClause("termurah", false, ""), "price_min ASC") {
		t.Error("urutan termurah harus memakai price_min menaik")
	}
	// Nilai yang tidak dikenal jatuh ke urutan bawaan, bukan SQL kosong.
	bawaan := orderClause("entah-apa", false, "")
	if !strings.HasPrefix(bawaan, "ORDER BY") {
		t.Errorf("urutan bawaan tidak valid: %q", bawaan)
	}
	if !strings.Contains(bawaan, "featured_until") {
		t.Error("urutan bawaan harus mendahulukan listing berbayar yang masih berlaku")
	}
}

func TestCacheKeyBerbedaPerFilter(t *testing.T) {
	a := ServiceFilter{Query: "AC", Kecamatan: "Bantan", Limit: 12}
	b := ServiceFilter{Query: "AC", Kecamatan: "Rupat", Limit: 12}
	if a.CacheKey() == b.CacheKey() {
		t.Error("filter berbeda seharusnya menghasilkan kunci cache berbeda")
	}
	// Kapitalisasi tidak boleh membuat entri cache terpisah.
	c := ServiceFilter{Query: "ac", Kecamatan: "bantan", Limit: 12}
	if a.CacheKey() != c.CacheKey() {
		t.Error("perbedaan huruf besar-kecil seharusnya memakai kunci cache sama")
	}
}

// Pencarian teks harus mencakup nama kategori, karena pengguna lazim mengetik
// istilah kategori ("kebersihan") yang tidak muncul di judul listing.
func TestBuildFilterMencocokkanNamaKategori(t *testing.T) {
	where, _ := buildFilter(ServiceFilter{Query: "kebersihan"}, nil)
	for _, kolom := range []string{"s.title", "s.description", "u.full_name", "c.name", "pc.name"} {
		if !strings.Contains(where, kolom) {
			t.Errorf("klausa pencarian tidak mencocokkan %s: %q", kolom, where)
		}
	}
}

// Penyaringan radius harus memanfaatkan indeks GiST lewat earth_box,
// bukan menghitung jarak untuk seluruh baris lalu menyaringnya.
func TestBuildFilterRadiusMemakaiEarthBox(t *testing.T) {
	lat, lng := 1.4667, 102.1
	where, args := buildFilter(ServiceFilter{
		Latitude: &lat, Longitude: &lng, RadiusKm: 25,
	}, []any{lat, lng})

	if !strings.Contains(where, "earth_box(ll_to_earth($1, $2)") {
		t.Errorf("klausa radius tidak memakai earth_box: %q", where)
	}
	if !strings.Contains(where, "earth_distance") {
		t.Errorf("klausa radius tidak menyaring sudut kotak dengan earth_distance: %q", where)
	}
	// Radius dikirim dalam meter karena earth_distance bekerja dalam meter.
	if len(args) != 3 || args[2] != float64(25000) {
		t.Errorf("argumen radius = %v, harusnya 25000 meter", args)
	}
}

// Filter keterjangkauan membandingkan jarak dengan radius layanan penyedia.
func TestBuildFilterKeterjangkauan(t *testing.T) {
	lat, lng := 1.4667, 102.1
	where, _ := buildFilter(ServiceFilter{
		Latitude: &lat, Longitude: &lng, OnlyReachable: true,
	}, []any{lat, lng})

	if !strings.Contains(where, "p.service_radius_km * 1000") {
		t.Errorf("filter keterjangkauan tidak membandingkan radius penyedia: %q", where)
	}
}

// Tanpa titik lokasi, tidak boleh ada klausa jarak sama sekali.
func TestBuildFilterTanpaLokasi(t *testing.T) {
	where, args := buildFilter(ServiceFilter{RadiusKm: 25, OnlyReachable: true}, nil)
	if strings.Contains(where, "earth_") {
		t.Errorf("tanpa koordinat seharusnya tidak ada klausa jarak: %q", where)
	}
	if len(args) != 0 {
		t.Errorf("tanpa koordinat seharusnya tanpa argumen, dapat %v", args)
	}
}

// Query COUNT tidak punya kolom jarak, jadi koordinat hanya boleh ikut
// dikirim bila ada klausa yang benar-benar merujuknya — Postgres menolak
// argumen yang tidak dipakai placeholder mana pun.
func TestPerluKoordinat(t *testing.T) {
	lat, lng := 1.4667, 102.1
	dasar := ServiceFilter{Latitude: &lat, Longitude: &lng}

	if dasar.perluKoordinat() {
		t.Error("tanpa radius atau keterjangkauan, koordinat tidak dirujuk klausa mana pun")
	}

	denganRadius := dasar
	denganRadius.RadiusKm = 10
	if !denganRadius.perluKoordinat() {
		t.Error("filter radius merujuk koordinat")
	}

	denganJangkau := dasar
	denganJangkau.OnlyReachable = true
	if !denganJangkau.perluKoordinat() {
		t.Error("filter keterjangkauan merujuk koordinat")
	}

	if (ServiceFilter{RadiusKm: 10}).perluKoordinat() {
		t.Error("tanpa koordinat, filter radius tidak bisa dipakai")
	}
}

func TestOrderClauseTerdekat(t *testing.T) {
	// Tanpa lokasi, urutan terdekat tidak mungkin dihitung dan harus
	// jatuh ke urutan bawaan yang tetap sah.
	if strings.Contains(orderClause("terdekat", false, ""), "jarak_km") {
		t.Error("tanpa lokasi, urutan tidak boleh memakai kolom jarak")
	}
	dengan := orderClause("terdekat", true, "")
	if !strings.Contains(dengan, "jarak_km ASC") {
		t.Errorf("urutan terdekat salah: %q", dengan)
	}
	// Penyedia tanpa titik lokasi diletakkan paling belakang, bukan paling depan.
	if !strings.Contains(dengan, "NULLS LAST") {
		t.Errorf("penyedia tanpa lokasi harus di urutan terakhir: %q", dengan)
	}
}

// Kunci cache harus membedakan lokasi dan radius, jika tidak hasil pencarian
// satu pengguna bisa tersaji ke pengguna di lokasi lain.
func TestCacheKeyMembedakanLokasi(t *testing.T) {
	a, b := 1.4667, 102.1
	c, d := 1.85, 101.6

	f1 := ServiceFilter{Latitude: &a, Longitude: &b, RadiusKm: 10, Limit: 12}
	f2 := ServiceFilter{Latitude: &c, Longitude: &d, RadiusKm: 10, Limit: 12}
	if f1.CacheKey() == f2.CacheKey() {
		t.Error("lokasi berbeda harus menghasilkan kunci cache berbeda")
	}

	f3 := f1
	f3.RadiusKm = 50
	if f1.CacheKey() == f3.CacheKey() {
		t.Error("radius berbeda harus menghasilkan kunci cache berbeda")
	}

	tanpa := ServiceFilter{Limit: 12}
	if tanpa.CacheKey() == f1.CacheKey() {
		t.Error("dengan dan tanpa lokasi harus berbeda kunci cache")
	}
}

// Antrean peninjauan hanya memuat listing berstatus menunggu.
func TestBuildAdminServiceFilterAntrean(t *testing.T) {
	where, _ := buildAdminServiceFilter(AdminServiceFilter{Antrean: true})
	if !strings.Contains(where, "s.status = 'pending'") {
		t.Errorf("filter antrean tidak menyaring status menunggu: %q", where)
	}
}
