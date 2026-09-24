package model

import (
	"testing"
	"time"
)

// Mesin status adalah satu-satunya sumber aturan; yang dijaga di sini adalah
// batas hak tiap aktor, bukan sekadar daftar transisinya.
func TestTransisiPromosiMenurutAktor(t *testing.T) {
	kasus := []struct {
		dari, ke string
		aktor    AktorPromosi
		boleh    bool
	}{
		{StatusMenunggu, StatusDisetujui, AktorAdmin, true},
		{StatusMenunggu, StatusAktif, AktorAdmin, true}, // paket gratis
		{StatusMenunggu, StatusDitolak, AktorAdmin, true},
		{StatusMenunggu, StatusDibatalkan, AktorPemohon, true},
		{StatusMenunggu, StatusDisetujui, AktorPemohon, false}, // pemohon tidak menyetujui dirinya
		{StatusMenunggu, StatusDibatalkan, AktorAdmin, false},  // admin menolak, bukan membatalkan
		{StatusDisetujui, StatusAktif, AktorAdmin, true},       // bayar dikonfirmasi
		{StatusDisetujui, StatusDibatalkan, AktorPemohon, true},
		{StatusAktif, StatusDijeda, AktorAdmin, true},
		{StatusAktif, StatusSelesai, AktorSistem, true},
		{StatusAktif, StatusSelesai, AktorAdmin, false}, // selesai hanya oleh waktu
		{StatusAktif, StatusDibatalkan, AktorPemohon, false},
		{StatusDijeda, StatusAktif, AktorAdmin, true},
		{StatusSelesai, StatusAktif, AktorAdmin, false},
		{StatusDitolak, StatusMenunggu, AktorPemohon, false},
		{StatusDihentikan, StatusAktif, AktorAdmin, false},
	}
	for _, k := range kasus {
		if got := BolehTransisi(k.dari, k.ke, k.aktor); got != k.boleh {
			t.Errorf("%s → %s oleh aktor %d = %v, harusnya %v", k.dari, k.ke, k.aktor, got, k.boleh)
		}
	}
}

func TestStatusAkhirDanHidup(t *testing.T) {
	for _, s := range []string{StatusSelesai, StatusDitolak, StatusDibatalkan, StatusDihentikan} {
		if !StatusAkhir(s) {
			t.Errorf("%s harus status akhir", s)
		}
		if StatusHidup(s) {
			t.Errorf("%s tidak boleh dianggap hidup", s)
		}
	}
	for _, s := range []string{StatusMenunggu, StatusDisetujui, StatusAktif, StatusDijeda} {
		if StatusAkhir(s) {
			t.Errorf("%s bukan status akhir", s)
		}
		if !StatusHidup(s) {
			t.Errorf("%s harus dianggap hidup", s)
		}
	}
}

func TestMenargetkanKecamatan(t *testing.T) {
	semua := Promosi{}
	if !semua.MenargetkanKecamatan("Rupat") {
		t.Error("tanpa target harus tampil ke semua kecamatan")
	}
	sebagian := Promosi{TargetKecamatan: []string{"Bengkalis", "Bantan"}}
	if !sebagian.MenargetkanKecamatan("Bantan") || sebagian.MenargetkanKecamatan("Rupat") {
		t.Error("target sebagian salah menyaring")
	}
	if !sebagian.MenargetkanKecamatan("") {
		t.Error("penonton tanpa kecamatan harus tetap melihat iklan bertarget")
	}
}

func TestSedangTayang(t *testing.T) {
	kini := time.Now()
	besok, kemarin := kini.Add(24*time.Hour), kini.Add(-24*time.Hour)
	if !(Promosi{Status: StatusAktif, SelesaiAt: &besok}).SedangTayang(kini) {
		t.Error("aktif dengan selesai_at di depan harus tayang")
	}
	if (Promosi{Status: StatusAktif, SelesaiAt: &kemarin}).SedangTayang(kini) {
		t.Error("aktif tapi sudah lewat tidak boleh tayang")
	}
	if (Promosi{Status: StatusDijeda, SelesaiAt: &besok}).SedangTayang(kini) {
		t.Error("dijeda tidak boleh tayang")
	}
}
