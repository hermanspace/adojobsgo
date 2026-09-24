package view

import (
	"strconv"
	"strings"
	"time"

	"github.com/hermansyah/adojobsid/internal/model"
)

// PromosiData adalah halaman daftar pengajuan milik pengguna.
type PromosiData struct {
	Base
	Items      []model.Promosi
	Paket      map[int64]model.PaketPromosi
	IsProvider bool
}

// NamaPaket mengembalikan nama paket sebuah pengajuan, atau keterangan bila
// ditetapkan admin tanpa paket.
func (d PromosiData) NamaPaket(p model.Promosi) string {
	if p.PaketID == nil {
		return "Ditetapkan pengelola"
	}
	if pk, ok := d.Paket[*p.PaketID]; ok {
		return pk.Nama
	}
	return "—"
}

// PromosiBaruData adalah form pengajuan; isinya mengikuti jenis.
type PromosiBaruData struct {
	Base
	Jenis     string
	Paket     []model.PaketPromosi
	Listings  []model.ServiceCard
	Kecamatan []string
	Form      Form
}

func (d PromosiBaruData) LabelJenis() string { return model.LabelJenisPromosi(d.Jenis) }

// KecamatanDipilih membaca pilihan dari form; nilainya disimpan dipisah koma.
func (d PromosiBaruData) KecamatanDipilih(k string) bool {
	for _, v := range strings.Split(d.Form.Value("target_kecamatan"), ",") {
		if v == k {
			return true
		}
	}
	return false
}

// PromosiDetailData adalah halaman satu pengajuan.
type PromosiDetailData struct {
	Base
	Promosi    model.Promosi
	Paket      *model.PaketPromosi
	JudulJasa  string
	Pembayaran model.PengaturanPromosi
}

func (d PromosiDetailData) LabelStatus() string { return model.LabelStatusPromosi(d.Promosi.Status) }
func (d PromosiDetailData) LabelJenis() string  { return model.LabelJenisPromosi(d.Promosi.Jenis) }

// PerluBayar: disetujui, paket berbayar, dan uji coba gratis tidak menyala.
// Bukti yang sudah diunggah tetap ditampilkan sebagai "menunggu konfirmasi".
func (d PromosiDetailData) PerluBayar() bool {
	return d.Promosi.Status == model.StatusDisetujui && d.Paket != nil &&
		!d.Paket.Gratis() && !d.Pembayaran.UjiCobaGratis
}

func (d PromosiDetailData) SudahUnggahBukti() bool { return d.Promosi.BuktiBayarURL != nil }

func (d PromosiDetailData) BisaBatal() bool {
	return model.BolehTransisi(d.Promosi.Status, model.StatusDibatalkan, model.AktorPemohon)
}

// SisaHari menghitung sisa masa tayang, dibulatkan ke atas.
func (d PromosiDetailData) SisaHari() int {
	if d.Promosi.SelesaiAt == nil {
		return 0
	}
	sisa := time.Until(*d.Promosi.SelesaiAt)
	if sisa <= 0 {
		return 0
	}
	return int((sisa + 24*time.Hour - time.Nanosecond) / (24 * time.Hour))
}

// CTR adalah rasio klik per tayang iklan ini.
func (d PromosiDetailData) CTR() string { return persenKlik(d.Promosi.Tayang, d.Promosi.Klik) }

// TayangPerHari adalah rata-rata tayang per hari sejak mulai tayang —
// pembanding yang adil antara iklan yang baru sehari dan yang sudah sebulan.
func (d PromosiDetailData) TayangPerHari() string {
	if d.Promosi.MulaiAt == nil {
		return "—"
	}
	hari := time.Since(*d.Promosi.MulaiAt).Hours() / 24
	if hari < 1 {
		hari = 1
	}
	return strconv.FormatFloat(float64(d.Promosi.Tayang)/hari, 'f', 0, 64)
}

// KelasStatusPromosi memilih gaya penanda status.
func KelasStatusPromosi(status string) string {
	switch status {
	case model.StatusAktif:
		return "tanda tanda-baik"
	case model.StatusMenunggu, model.StatusDisetujui, model.StatusDijeda:
		return "tanda tanda-sorot"
	case model.StatusDitolak, model.StatusDihentikan:
		return "tanda tanda-bahaya"
	default:
		return "tanda"
	}
}
