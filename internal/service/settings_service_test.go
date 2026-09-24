package service

import (
	"testing"

	"github.com/hermansyah/adojobsid/internal/model"
)

// Overlay yang tersimpan di luar rentang aman — termasuk nol dari data lama
// yang belum punya kolom ini — harus kembali ke bawaan, karena overlay terlalu
// tipis membuat teks hero hilang di atas foto yang terang.
func TestNormalkanOverlayHero(t *testing.T) {
	kasus := map[int]int{
		0:                         model.HeroOverlayBawaan, // data lama tanpa nilai
		model.HeroOverlayMin - 1:  model.HeroOverlayBawaan,
		model.HeroOverlayMaks + 1: model.HeroOverlayBawaan,
		-20:                       model.HeroOverlayBawaan,
		model.HeroOverlayMin:      model.HeroOverlayMin,
		model.HeroOverlayMaks:     model.HeroOverlayMaks,
		60:                        60,
	}
	for masuk, harapan := range kasus {
		p := model.PengaturanBawaan()
		p.Tampilan.HeroOverlay = masuk
		if got := normalkanPengaturan(p).Tampilan.HeroOverlay; got != harapan {
			t.Errorf("overlay %d dinormalkan jadi %d, harusnya %d", masuk, got, harapan)
		}
	}
}

// WhatsApp mati secara bawaan: seluruh percakapan berlangsung di dalam aplikasi
// sampai admin menyalakannya sendiri.
func TestWhatsappMatiSecaraBawaan(t *testing.T) {
	if model.PengaturanBawaan().Umum.WhatsappAktif {
		t.Error("tombol WhatsApp tidak boleh aktif tanpa admin menyalakannya")
	}
}

// Tautan Play Store bawaan kosong supaya lencana unduh tidak muncul sebelum
// aplikasinya benar-benar terbit.
func TestPlaystoreKosongSecaraBawaan(t *testing.T) {
	if model.PengaturanBawaan().Tampilan.PlaystoreURL != "" {
		t.Error("tautan Play Store harus kosong sebelum aplikasinya terbit")
	}
}

// Teks lencana kosong — termasuk pada data yang tersimpan sebelum kedua kolom
// ini ada — harus kembali ke bawaan, supaya lencananya tidak pernah tampil
// sebagai kotak tanpa tulisan.
func TestNormalkanTeksLencana(t *testing.T) {
	p := model.PengaturanBawaan()
	p.Tampilan.PlaystoreTeksAtas = ""
	p.Tampilan.PlaystoreTeksBawah = "   "

	hasil := normalkanPengaturan(p).Tampilan
	if hasil.PlaystoreTeksAtas != "Segera hadir di" {
		t.Errorf("baris atas = %q", hasil.PlaystoreTeksAtas)
	}
	if hasil.PlaystoreTeksBawah != "Google Play" {
		t.Errorf("baris bawah = %q", hasil.PlaystoreTeksBawah)
	}
}

// Bawaannya "segera hadir", bukan "dapatkan di", karena aplikasinya memang
// belum terbit di Play Store.
func TestLencanaBawaanMengatakanSegeraHadir(t *testing.T) {
	tmp := model.PengaturanBawaan().Tampilan
	if tmp.AplikasiTampil {
		t.Error("lencana tidak boleh tampil sebelum admin menyalakannya")
	}
	if tmp.PlaystoreTeksAtas != "Segera hadir di" {
		t.Errorf("teks bawaan = %q, harusnya menandai aplikasi belum terbit", tmp.PlaystoreTeksAtas)
	}
}
