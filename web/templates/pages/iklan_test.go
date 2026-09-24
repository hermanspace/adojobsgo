package pages

import (
	"os"
	"strings"
	"testing"
)

// Setiap kunci slot yang didefinisikan di kode harus benar-benar dipasang di
// salah satu halaman publik. Slot yang hanya ada di panel admin akan membuat
// admin mengisi materi, menekan simpan, dan tidak pernah melihat hasilnya —
// persis kegagalan yang dilaporkan saat penayangan belum dipasang sama sekali.
func TestSetiapSlotIklanDipasangDiHalaman(t *testing.T) {
	halaman := []string{"home.templ", "search.templ", "service_detail.templ"}

	var gabungan strings.Builder
	for _, nama := range halaman {
		isi, err := os.ReadFile(nama)
		if err != nil {
			t.Fatal(err)
		}
		gabungan.Write(isi)
	}
	teks := gabungan.String()

	// Kunci diambil dari definisi bawaan di model, bukan ditulis ulang di sini,
	// sehingga menambah slot baru tanpa memasangnya langsung ketahuan.
	for _, kunci := range kunciSlotBawaan(t) {
		if !strings.Contains(teks, `SlotIklan("`+kunci+`")`) {
			t.Errorf("slot %q ada di pengaturan tapi tidak dipasang di halaman publik mana pun", kunci)
		}
	}
}

// Iklan di halaman pencarian harus berada di luar fragmen yang ditukar HTMX.
// Di dalamnya, iklan akan dimuat ulang tiap kali filter berubah.
func TestIklanPencarianDiLuarFragmenHTMX(t *testing.T) {
	isi, err := os.ReadFile("search.templ")
	if err != nil {
		t.Fatal(err)
	}
	teks := string(isi)

	iklan := strings.Index(teks, `SlotIklan("cari_atas")`)
	fragmen := strings.Index(teks, `<div id="hasil-pencarian">`)
	if iklan < 0 || fragmen < 0 {
		t.Fatalf("penanda tidak ditemukan (iklan=%d, fragmen=%d)", iklan, fragmen)
	}
	if iklan > fragmen {
		t.Error("iklan pencarian berada di dalam fragmen HTMX; akan dimuat ulang tiap filter berubah")
	}
}

// Iklan wajib berlabel dan memakai rel=sponsored: pengunjung berhak tahu mana
// konten berbayar, dan mesin pencari tidak boleh menganggapnya rekomendasi.
func TestIklanBerlabelDanSponsored(t *testing.T) {
	isi, err := os.ReadFile("../components/iklan.templ")
	if err != nil {
		t.Fatal(err)
	}
	teks := string(isi)

	if !strings.Contains(teks, `class="iklan-label">Iklan<`) {
		t.Error("materi iklan tayang tanpa label yang terbaca")
	}
	if !strings.Contains(teks, `rel="sponsored noopener"`) {
		t.Error("tautan iklan tanpa rel=sponsored")
	}
}

func kunciSlotBawaan(t *testing.T) []string {
	t.Helper()
	isi, err := os.ReadFile("../../../internal/model/settings.go")
	if err != nil {
		t.Fatal(err)
	}

	var kunci []string
	for _, baris := range strings.Split(string(isi), "\n") {
		const awalan = `{Kunci: "`
		i := strings.Index(baris, awalan)
		if i < 0 {
			continue
		}
		sisa := baris[i+len(awalan):]
		if j := strings.Index(sisa, `"`); j > 0 {
			kunci = append(kunci, sisa[:j])
		}
	}
	if len(kunci) == 0 {
		t.Fatal("tidak ada kunci slot yang terbaca dari definisi bawaan")
	}
	return kunci
}
