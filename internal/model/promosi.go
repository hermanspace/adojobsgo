package model

import (
	"strconv"
	"time"
)

// Jenis promosi. Ketiganya berbagi tabel, alur, dan mesin status; yang
// berbeda hanya apa yang ditampilkan dan di mana.
const (
	PromosiIklan    = "iklan"
	PromosiSorotan  = "sorotan_jasa"
	PromosiPenyedia = "penyedia_pilihan"
)

// JenisPromosi adalah daftar jenis yang sah, berurutan seperti di UI.
var JenisPromosi = []string{PromosiIklan, PromosiSorotan, PromosiPenyedia}

// LabelJenisPromosi menerjemahkan jenis ke teks yang dibaca orang.
func LabelJenisPromosi(jenis string) string {
	switch jenis {
	case PromosiIklan:
		return "Iklan"
	case PromosiSorotan:
		return "Sorotan jasa"
	case PromosiPenyedia:
		return "Penyedia pilihan"
	}
	return jenis
}

// Status promosi.
//
//	menunggu  ─ diajukan, belum ditinjau
//	disetujui ─ disetujui admin, menunggu pembayaran dikonfirmasi
//	aktif     ─ sedang tayang
//	dijeda    ─ dihentikan sementara oleh admin, bisa dilanjutkan
//	selesai   ─ masa tayang habis
//	ditolak / dibatalkan / dihentikan ─ berhenti sebelum atau di tengah jalan
const (
	StatusMenunggu   = "menunggu"
	StatusDisetujui  = "disetujui"
	StatusAktif      = "aktif"
	StatusDijeda     = "dijeda"
	StatusSelesai    = "selesai"
	StatusDitolak    = "ditolak"
	StatusDibatalkan = "dibatalkan"
	StatusDihentikan = "dihentikan"
)

// Sumber promosi: diajukan pengguna, atau ditetapkan admin langsung.
const (
	SumberPengguna = "pengguna"
	SumberAdmin    = "admin"
)

// Aktor yang meminta perubahan status.
type AktorPromosi int

const (
	AktorPemohon AktorPromosi = iota
	AktorAdmin
	AktorSistem
)

// transisiPromosi memetakan status asal → status tujuan → aktor yang berhak.
// Ini satu-satunya sumber aturan; service hanya memeriksanya, tidak mengulangi.
var transisiPromosi = map[string]map[string][]AktorPromosi{
	StatusMenunggu: {
		StatusDisetujui:  {AktorAdmin},
		StatusAktif:      {AktorAdmin}, // paket gratis: langsung tayang
		StatusDitolak:    {AktorAdmin},
		StatusDibatalkan: {AktorPemohon},
	},
	StatusDisetujui: {
		StatusAktif:      {AktorAdmin}, // pembayaran dikonfirmasi
		StatusDibatalkan: {AktorPemohon},
		StatusDihentikan: {AktorAdmin},
	},
	StatusAktif: {
		StatusDijeda:     {AktorAdmin},
		StatusDihentikan: {AktorAdmin},
		StatusSelesai:    {AktorSistem},
	},
	StatusDijeda: {
		StatusAktif:      {AktorAdmin},
		StatusDihentikan: {AktorAdmin},
		StatusSelesai:    {AktorSistem},
	},
}

// BolehTransisi menjawab apakah aktor boleh memindahkan promosi dari satu
// status ke status lain. Status akhir tidak punya jalan keluar.
func BolehTransisi(dari, ke string, aktor AktorPromosi) bool {
	for _, a := range transisiPromosi[dari][ke] {
		if a == aktor {
			return true
		}
	}
	return false
}

// StatusAkhir menandai status yang tidak bisa berubah lagi.
func StatusAkhir(status string) bool {
	return len(transisiPromosi[status]) == 0
}

// StatusHidup menandai pengajuan yang masih menempati kuota "satu per
// listing / satu per penyedia" — selaras dengan indeks unik parsial di DB.
func StatusHidup(status string) bool {
	switch status {
	case StatusMenunggu, StatusDisetujui, StatusAktif, StatusDijeda:
		return true
	}
	return false
}

// LabelStatusPromosi menerjemahkan status ke teks yang dibaca orang.
func LabelStatusPromosi(status string) string {
	switch status {
	case StatusMenunggu:
		return "Menunggu peninjauan"
	case StatusDisetujui:
		return "Menunggu pembayaran"
	case StatusAktif:
		return "Sedang tayang"
	case StatusDijeda:
		return "Dijeda"
	case StatusSelesai:
		return "Selesai"
	case StatusDitolak:
		return "Ditolak"
	case StatusDibatalkan:
		return "Dibatalkan"
	case StatusDihentikan:
		return "Dihentikan"
	}
	return status
}

// PaketPromosi adalah pilihan durasi, harga, dan penempatan yang ditawarkan.
type PaketPromosi struct {
	ID         int64     `json:"id"`
	Jenis      string    `json:"jenis"`
	Nama       string    `json:"nama"`
	Deskripsi  string    `json:"deskripsi"`
	DurasiHari int       `json:"durasi_hari"`
	Harga      int64     `json:"harga"`
	Bobot      int       `json:"bobot"`
	SlotIklan  []string  `json:"slot_iklan,omitempty"`
	Aktif      bool      `json:"aktif"`
	Urutan     int       `json:"urutan"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Gratis menandai paket yang tidak memerlukan pembayaran.
func (p PaketPromosi) Gratis() bool { return p.Harga == 0 }

// Promosi adalah satu pengajuan beserta seluruh riwayat hidupnya.
type Promosi struct {
	ID         int64  `json:"id"`
	Jenis      string `json:"jenis"`
	UserID     int64  `json:"user_id"`
	ProviderID *int64 `json:"provider_id,omitempty"`
	ServiceID  *int64 `json:"service_id,omitempty"`
	PaketID    *int64 `json:"paket_id,omitempty"`
	Status     string `json:"status"`
	Sumber     string `json:"sumber"`

	Judul           *string  `json:"judul,omitempty"`
	Deskripsi       *string  `json:"deskripsi,omitempty"`
	GambarURL       *string  `json:"gambar_url,omitempty"`
	TautanURL       *string  `json:"tautan_url,omitempty"`
	TargetKecamatan []string `json:"target_kecamatan"`

	Alasan       *string    `json:"alasan,omitempty"`
	CatatanAdmin *string    `json:"catatan_admin,omitempty"`
	DitinjauOleh *int64     `json:"ditinjau_oleh,omitempty"`
	DitinjauAt   *time.Time `json:"ditinjau_at,omitempty"`

	BuktiBayarURL    *string    `json:"bukti_bayar_url,omitempty"`
	DibayarAt        *time.Time `json:"dibayar_at,omitempty"`
	DikonfirmasiOleh *int64     `json:"dikonfirmasi_oleh,omitempty"`

	MulaiAt   *time.Time `json:"mulai_at,omitempty"`
	SelesaiAt *time.Time `json:"selesai_at,omitempty"`
	Tayang    int64      `json:"tayang"`
	Klik      int64      `json:"klik"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SedangTayang menandai promosi yang aktif dan masa tayangnya belum habis.
func (p Promosi) SedangTayang(kini time.Time) bool {
	return p.Status == StatusAktif && p.SelesaiAt != nil && p.SelesaiAt.After(kini)
}

// MenargetkanKecamatan menjawab apakah promosi tampil untuk penonton di
// kecamatan tertentu. Tanpa target berarti seluruh kabupaten; penonton tanpa
// kecamatan (belum menentukan lokasi) melihat semuanya.
func (p Promosi) MenargetkanKecamatan(kecamatan string) bool {
	if len(p.TargetKecamatan) == 0 || kecamatan == "" {
		return true
	}
	for _, k := range p.TargetKecamatan {
		if k == kecamatan {
			return true
		}
	}
	return false
}

// IklanTayang adalah iklan aktif beserta penempatan dan bobot paketnya —
// bentuk yang di-cache untuk pemilihan per tampilan tanpa menyentuh database.
type IklanTayang struct {
	Promosi
	Slot  []string `json:"slot"`
	Bobot int      `json:"bobot"`
}

func (ik IklanTayang) layakDi(slot string) bool {
	for _, s := range ik.Slot {
		if s == slot {
			return true
		}
	}
	return false
}

// PilihIklan memilih satu iklan untuk satu slot dan satu penonton, dalam tiga
// tahap berurutan: paket menentukan kelayakan slot, kecamatan penonton
// menyaring, lalu di antara yang lolos dipilih acak dengan peluang sebanding
// bobot paket. acak(n) mengembalikan bilangan [0, n) — disuntikkan supaya
// pilihannya bisa diuji secara deterministik.
//
// Rotasinya per tampilan halaman, bukan bergantian di dalam satu halaman:
// tanpa JavaScript, tanpa permintaan tambahan, dan secara statistik sama
// adilnya.
func PilihIklan(daftar []IklanTayang, slot, kecamatan string, kini time.Time, acak func(n int) int) *IklanTayang {
	var lolos []int
	total := 0
	for i := range daftar {
		ik := &daftar[i]
		if !ik.SedangTayang(kini) || !ik.layakDi(slot) || !ik.MenargetkanKecamatan(kecamatan) {
			continue
		}
		bobot := ik.Bobot
		if bobot < 1 {
			bobot = 1
		}
		lolos = append(lolos, i)
		total += bobot
	}
	if len(lolos) == 0 {
		return nil
	}
	undian := acak(total)
	for _, i := range lolos {
		bobot := daftar[i].Bobot
		if bobot < 1 {
			bobot = 1
		}
		if undian < bobot {
			return &daftar[i]
		}
		undian -= bobot
	}
	return &daftar[lolos[len(lolos)-1]]
}

// SlotDariIklan mengubah iklan terpilih menjadi bentuk yang dipahami
// komponen tampilan slot. Tautan diarahkan lewat rute klik supaya terhitung;
// iklan tanpa tautan tetap tidak bisa diklik.
func SlotDariIklan(ik IklanTayang, kunci string) SlotIklan {
	s := SlotIklan{Kunci: kunci, Nama: kunci, Aktif: true}
	if ik.GambarURL != nil {
		s.GambarURL = *ik.GambarURL
	}
	if ik.Judul != nil {
		s.Teks = *ik.Judul
	}
	if ik.TautanURL != nil && *ik.TautanURL != "" {
		s.TautanURL = "/iklan/" + strconv.FormatInt(ik.ID, 10) + "/klik"
	}
	return s
}

// PilihPenyediaPilihan memilih n penyedia pilihan untuk ditampilkan, lebih
// mengutamakan yang berada di kecamatan penonton, lalu sisanya diacak supaya
// yang tampil bergilir antar tampilan. kecamatanDari mengembalikan kecamatan
// tiap penyedia; acak(n) seperti pada PilihIklan.
func PilihPenyediaPilihan(daftar []ProviderDetail, kecamatan string, n int, acak func(n int) int) []ProviderDetail {
	if len(daftar) <= n {
		return daftar
	}
	var dekat, jauh []ProviderDetail
	for _, p := range daftar {
		if kecamatan != "" && kecamatanPenyedia(p) == kecamatan {
			dekat = append(dekat, p)
		} else {
			jauh = append(jauh, p)
		}
	}
	kocok := func(xs []ProviderDetail) {
		for i := len(xs) - 1; i > 0; i-- {
			j := acak(i + 1)
			xs[i], xs[j] = xs[j], xs[i]
		}
	}
	kocok(dekat)
	kocok(jauh)
	out := append(dekat, jauh...)
	return out[:n]
}

func kecamatanPenyedia(p ProviderDetail) string {
	if p.PunyaLokasi() {
		k, _ := KecamatanTerdekat(*p.Latitude, *p.Longitude)
		return k.Nama
	}
	if p.Kecamatan != nil {
		return *p.Kecamatan
	}
	return ""
}
