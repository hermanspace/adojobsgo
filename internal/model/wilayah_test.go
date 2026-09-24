package model

import (
	"math"
	"testing"
)

func TestJarakKm(t *testing.T) {
	// Titik yang sama berjarak nol.
	if j := JarakKm(1.4667, 102.1, 1.4667, 102.1); j != 0 {
		t.Errorf("jarak titik yang sama = %v, harusnya 0", j)
	}

	// Bengkalis ke Bantan: sekitar 15 km menurut koordinat pusat keduanya.
	j := JarakKm(1.4667, 102.1, 1.5167, 102.2333)
	if j < 13 || j > 18 {
		t.Errorf("jarak Bengkalis–Bantan = %.1f km, di luar rentang wajar 13–18 km", j)
	}

	// Satu derajat lintang kira-kira 111 km, di mana pun bujurnya.
	d := JarakKm(0, 102, 1, 102)
	if math.Abs(d-111.19) > 1 {
		t.Errorf("satu derajat lintang = %.2f km, harusnya sekitar 111 km", d)
	}

	// Jarak bersifat simetris.
	a := JarakKm(1.4667, 102.1, 1.85, 101.6)
	b := JarakKm(1.85, 101.6, 1.4667, 102.1)
	if math.Abs(a-b) > 0.0001 {
		t.Errorf("jarak tidak simetris: %.4f vs %.4f", a, b)
	}
}

func TestKecamatanTerdekat(t *testing.T) {
	// Titik tepat di pusat Bengkalis harus mengembalikan Bengkalis.
	kec, jarak := KecamatanTerdekat(1.4667, 102.1)
	if kec.Nama != "Bengkalis" {
		t.Errorf("kecamatan terdekat = %q, harusnya Bengkalis", kec.Nama)
	}
	if jarak > 0.1 {
		t.Errorf("jarak ke pusatnya sendiri = %.2f km, harusnya nyaris nol", jarak)
	}

	// Titik di Rupat Utara tidak boleh tertebak sebagai Bengkalis.
	kec2, _ := KecamatanTerdekat(2.03, 101.65)
	if kec2.Nama != "Rupat Utara" {
		t.Errorf("kecamatan terdekat dari Rupat Utara = %q", kec2.Nama)
	}

	// Titik yang sangat jauh tetap mengembalikan sesuatu, disertai jarak besar
	// sehingga pemanggil bisa memutuskan untuk tidak memakai namanya.
	_, jauh := KecamatanTerdekat(-6.2, 106.8) // Jakarta
	if jauh < 500 {
		t.Errorf("jarak Jakarta ke Bengkalis = %.0f km, terlalu kecil", jauh)
	}
}

func TestCariKecamatan(t *testing.T) {
	if _, ok := CariKecamatan("Bantan"); !ok {
		t.Error("kecamatan yang ada tidak ditemukan")
	}
	if _, ok := CariKecamatan("Entah Di Mana"); ok {
		t.Error("kecamatan yang tidak ada seharusnya tidak ditemukan")
	}
}

func TestKoordinatValid(t *testing.T) {
	if !KoordinatValid(1.4667, 102.1) {
		t.Error("koordinat Bengkalis ditolak")
	}
	// Nol-nol adalah nilai kosong yang lazim, bukan lokasi yang dimaksud.
	if KoordinatValid(0, 0) {
		t.Error("koordinat 0,0 seharusnya ditolak sebagai nilai kosong")
	}
	for _, k := range [][2]float64{{91, 102}, {-91, 102}, {1.4, 181}, {1.4, -181}} {
		if KoordinatValid(k[0], k[1]) {
			t.Errorf("koordinat di luar rentang %v seharusnya ditolak", k)
		}
	}
}

func TestDaftarKecamatanKonsisten(t *testing.T) {
	if len(KecamatanBengkalis) != len(KecamatanBengkalisKoordinat) {
		t.Errorf("daftar nama (%d) dan daftar koordinat (%d) tidak sama panjang",
			len(KecamatanBengkalis), len(KecamatanBengkalisKoordinat))
	}
	if len(KecamatanBengkalisKoordinat) != 11 {
		t.Errorf("Kabupaten Bengkalis punya 11 kecamatan, terdaftar %d",
			len(KecamatanBengkalisKoordinat))
	}
	for _, k := range KecamatanBengkalisKoordinat {
		if !KoordinatValid(k.Latitude, k.Longitude) {
			t.Errorf("koordinat %q tidak valid", k.Nama)
		}
		// Seluruh kecamatan harus berada di sekitar Riau, bukan salah ketik.
		if k.Latitude < 0.5 || k.Latitude > 2.5 || k.Longitude < 100.5 || k.Longitude > 103 {
			t.Errorf("koordinat %q (%.4f, %.4f) di luar wilayah Riau",
				k.Nama, k.Latitude, k.Longitude)
		}
	}
}

func TestServiceStatus(t *testing.T) {
	for _, s := range []ServiceStatus{ServicePending, ServiceActive, ServiceInactive, ServiceRejected} {
		if !s.Valid() {
			t.Errorf("status %q ditolak padahal sah", s)
		}
	}
	if ServiceStatus("entah").Valid() {
		t.Error("status tidak dikenal seharusnya ditolak")
	}

	// Provider tidak boleh menyetel sendiri status menunggu maupun ditolak.
	if ServicePending.DapatDipilihProvider() || ServiceRejected.DapatDipilihProvider() {
		t.Error("status hasil peninjauan tidak boleh dapat dipilih provider")
	}
	if !ServiceActive.DapatDipilihProvider() || !ServiceInactive.DapatDipilihProvider() {
		t.Error("provider harus bisa menayangkan dan menyembunyikan listingnya")
	}

	// Hanya listing aktif yang benar-benar tayang.
	if !ServiceActive.Tayang() {
		t.Error("listing aktif harus dianggap tayang")
	}
	for _, s := range []ServiceStatus{ServicePending, ServiceInactive, ServiceRejected} {
		if s.Tayang() {
			t.Errorf("status %q tidak boleh dianggap tayang", s)
		}
	}
}

func TestServiceCardMenjangkau(t *testing.T) {
	jarak := 10.0
	c := ServiceCard{ProviderRadiusKm: 15, JarakKm: &jarak}
	if !c.Menjangkau() {
		t.Error("jarak 10 km dalam radius 15 km seharusnya terjangkau")
	}

	c.ProviderRadiusKm = 5
	if c.Menjangkau() {
		t.Error("jarak 10 km di luar radius 5 km seharusnya tidak terjangkau")
	}

	// Tanpa titik lokasi pencari jasa, keterjangkauan tidak bisa disimpulkan.
	c.JarakKm = nil
	if c.Menjangkau() {
		t.Error("tanpa jarak, keterjangkauan tidak boleh diklaim")
	}
}
