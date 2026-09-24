package api

import (
	"testing"

	"github.com/hermansyah/adojobsid/internal/model"
)

// Nomor WhatsApp hanya boleh keluar lewat API bila fiturnya dinyalakan.
// Halaman HTML sudah menggerbang tombolnya, tetapi JSON memuat struct apa
// adanya — pemeriksaan ini menjaga keduanya tetap satu aturan.
func TestSembunyikanKontakMengikutiSakelar(t *testing.T) {
	p := &model.ProviderDetail{}
	p.WhatsappNumber = "628117512001"

	sembunyikanKontak(true, p)
	if p.WhatsappNumber == "" {
		t.Error("saat WhatsApp aktif, nomornya justru dihapus")
	}

	sembunyikanKontak(false, p)
	if p.WhatsappNumber != "" {
		t.Errorf("saat WhatsApp mati, nomor masih bocor: %q", p.WhatsappNumber)
	}

	// nil tidak boleh membuat panik — endpoint tidak selalu membawa penyedia.
	sembunyikanKontak(false, nil)
}
