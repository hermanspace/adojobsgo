package model

import (
	"math"
	"sort"
	"strings"
)

// Kecamatan adalah satu wilayah kecamatan beserta titik pusat perkiraannya.
type Kecamatan struct {
	Nama      string  `json:"nama"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// KecamatanBengkalisKoordinat memuat 11 kecamatan di Kabupaten Bengkalis
// beserta titik pusat perkiraannya.
//
// Koordinat ini PERKIRAAN pusat wilayah, bukan batas administratif resmi.
// Dipakai hanya untuk dua hal: menebak kecamatan terdekat dari posisi
// pengguna, dan menentukan titik awal peta. Jarak yang ditampilkan ke
// pengguna selalu dihitung dari titik pin yang ditentukan penyedia sendiri,
// bukan dari titik pusat ini.
var KecamatanBengkalisKoordinat = []Kecamatan{
	{Nama: "Bengkalis", Latitude: 1.4667, Longitude: 102.1000},
	{Nama: "Bantan", Latitude: 1.5167, Longitude: 102.2333},
	{Nama: "Bandar Laksamana", Latitude: 1.3167, Longitude: 102.1333},
	{Nama: "Bathin Solapan", Latitude: 1.2500, Longitude: 101.3500},
	{Nama: "Bukit Batu", Latitude: 1.3167, Longitude: 102.1167},
	{Nama: "Mandau", Latitude: 1.2833, Longitude: 101.2000},
	{Nama: "Pinggir", Latitude: 1.1000, Longitude: 101.2000},
	{Nama: "Rupat", Latitude: 1.8500, Longitude: 101.6000},
	{Nama: "Rupat Utara", Latitude: 2.0333, Longitude: 101.6500},
	{Nama: "Siak Kecil", Latitude: 1.2000, Longitude: 102.0000},
	{Nama: "Talang Muandau", Latitude: 1.1500, Longitude: 101.4000},
}

// PusatBengkalis adalah titik awal peta bila pengguna belum menentukan lokasi.
var PusatBengkalis = Kecamatan{Nama: "Bengkalis", Latitude: 1.4667, Longitude: 102.1000}

// KecamatanBengkalis adalah daftar nama saja, dipakai pada dropdown lokasi.
var KecamatanBengkalis = func() []string {
	out := make([]string, 0, len(KecamatanBengkalisKoordinat))
	for _, k := range KecamatanBengkalisKoordinat {
		out = append(out, k.Nama)
	}
	return out
}()

// KotaTerlayani membatasi pilihan kota/kabupaten pada MVP.
var KotaTerlayani = []string{"Bengkalis"}

// CariKecamatan mengembalikan data kecamatan berdasarkan namanya.
func CariKecamatan(nama string) (Kecamatan, bool) {
	for _, k := range KecamatanBengkalisKoordinat {
		if k.Nama == nama {
			return k, true
		}
	}
	return Kecamatan{}, false
}

// KecamatanTerdekat menebak kecamatan dari sebuah titik koordinat.
// Penebakan dilakukan di dalam aplikasi terhadap daftar di atas — tidak ada
// permintaan ke layanan geocoding luar, sehingga posisi pengguna tidak pernah
// meninggalkan server ini.
func KecamatanTerdekat(lat, lng float64) (Kecamatan, float64) {
	terdekat := KecamatanBengkalisKoordinat[0]
	jarakMin := JarakKm(lat, lng, terdekat.Latitude, terdekat.Longitude)

	for _, k := range KecamatanBengkalisKoordinat[1:] {
		if j := JarakKm(lat, lng, k.Latitude, k.Longitude); j < jarakMin {
			jarakMin, terdekat = j, k
		}
	}
	return terdekat, jarakMin
}

// JarakKm menghitung jarak lingkaran besar antara dua titik dalam kilometer
// memakai rumus haversine. Dipakai untuk perhitungan di sisi aplikasi;
// penyaringan radius pada pencarian dikerjakan di database agar terindeks.
func JarakKm(lat1, lng1, lat2, lng2 float64) float64 {
	const radiusBumiKm = 6371.0

	rad := func(d float64) float64 { return d * math.Pi / 180 }

	dLat := rad(lat2 - lat1)
	dLng := rad(lng2 - lng1)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rad(lat1))*math.Cos(rad(lat2))*math.Sin(dLng/2)*math.Sin(dLng/2)
	return radiusBumiKm * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// KoordinatValid memeriksa kewajaran sepasang koordinat.
func KoordinatValid(lat, lng float64) bool {
	return lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180 && !(lat == 0 && lng == 0)
}

// AreaLayanan menurunkan daftar kecamatan yang terjangkau dari titik lokasi
// dan radius layanan penyedia, diurutkan dari yang terdekat. Ini pengganti
// kolom teks bebas service_area yang dulu diisi manual dan tidak pernah
// sinkron dengan radius — tiga cara menyatakan "di mana" kini tinggal satu
// sumber: pin di peta.
//
// Bila radiusnya lebih kecil daripada jarak ke pusat kecamatan mana pun,
// kecamatan terdekat tetap disebut supaya daftarnya tidak pernah kosong bagi
// penyedia yang sudah menaruh pin. Tanpa pin, jatuh ke kecamatan domisili.
func (p ProviderProfile) AreaLayanan(domisili string) []string {
	if !p.PunyaLokasi() {
		if strings.TrimSpace(domisili) != "" {
			return []string{domisili}
		}
		return nil
	}

	type calon struct {
		nama string
		km   float64
	}
	var dalam []calon
	terdekat := calon{km: math.Inf(1)}
	for _, k := range KecamatanBengkalisKoordinat {
		km := JarakKm(*p.Latitude, *p.Longitude, k.Latitude, k.Longitude)
		if km < terdekat.km {
			terdekat = calon{k.Nama, km}
		}
		if km <= float64(p.ServiceRadiusKm) {
			dalam = append(dalam, calon{k.Nama, km})
		}
	}
	if len(dalam) == 0 {
		dalam = []calon{terdekat}
	}
	sort.Slice(dalam, func(i, j int) bool { return dalam[i].km < dalam[j].km })

	out := make([]string, 0, len(dalam))
	for _, c := range dalam {
		out = append(out, c.nama)
	}
	return out
}
