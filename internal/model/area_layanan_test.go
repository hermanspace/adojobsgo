package model

import "testing"

func TestAreaLayananDariPinDanRadius(t *testing.T) {
	lat, lng := PusatBengkalis.Latitude, PusatBengkalis.Longitude
	p := ProviderProfile{Latitude: &lat, Longitude: &lng, ServiceRadiusKm: 20}

	area := p.AreaLayanan("Mandau") // domisili diabaikan bila ada pin
	if len(area) == 0 || area[0] != "Bengkalis" {
		t.Fatalf("kecamatan terdekat harus pertama, dapat %v", area)
	}
	for _, nama := range area {
		var k *Kecamatan
		for i := range KecamatanBengkalisKoordinat {
			if KecamatanBengkalisKoordinat[i].Nama == nama {
				k = &KecamatanBengkalisKoordinat[i]
			}
		}
		if k == nil {
			t.Fatalf("%q bukan kecamatan yang dikenal", nama)
		}
		if km := JarakKm(lat, lng, k.Latitude, k.Longitude); km > 20 {
			t.Errorf("%s berjarak %.1f km, di luar radius 20 km", nama, km)
		}
	}
	for _, nama := range area {
		if nama == "Mandau" {
			t.Error("Mandau (>90 km) ikut terhitung dalam radius 20 km")
		}
	}
}

// Radius yang lebih kecil dari jarak ke pusat kecamatan mana pun tetap
// menyebut kecamatan terdekat — daftarnya tidak boleh kosong bagi penyedia
// yang sudah menaruh pin.
func TestAreaLayananRadiusKecilTetapSebutTerdekat(t *testing.T) {
	lat, lng := 1.40, 102.05 // di antara Bengkalis dan Bukit Batu, jauh dari keduanya
	p := ProviderProfile{Latitude: &lat, Longitude: &lng, ServiceRadiusKm: 1}
	if area := p.AreaLayanan(""); len(area) != 1 {
		t.Errorf("radius 1 km harusnya menyebut tepat satu kecamatan terdekat, dapat %v", area)
	}
}

func TestAreaLayananTanpaPin(t *testing.T) {
	var p ProviderProfile
	if area := p.AreaLayanan("Bantan"); len(area) != 1 || area[0] != "Bantan" {
		t.Errorf("tanpa pin harus jatuh ke domisili, dapat %v", area)
	}
	if area := p.AreaLayanan("  "); area != nil {
		t.Errorf("tanpa pin dan domisili harus kosong, dapat %v", area)
	}
}
