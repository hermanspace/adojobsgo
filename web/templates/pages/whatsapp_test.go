package pages

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tombol WhatsApp membawa percakapan keluar dari aplikasi, jadi kemunculannya
// harus selalu bergantung pada sakelar di pengaturan admin. Tanpa penjaga ini,
// satu tautan yang lupa dibungkus kondisi akan lolos diam-diam: halamannya
// tetap dirender, hanya saja pengguna dibawa keluar aplikasi.
func TestTautanWhatsappSelaluBersyarat(t *testing.T) {
	berkas, err := filepath.Glob("*.templ")
	if err != nil {
		t.Fatal(err)
	}
	if len(berkas) == 0 {
		t.Fatal("tidak ada berkas .templ yang diperiksa")
	}

	for _, nama := range berkas {
		isi, err := os.ReadFile(nama)
		if err != nil {
			t.Fatal(err)
		}
		teks := string(isi)
		if !strings.Contains(teks, "TautanWhatsapp") {
			continue
		}
		if !strings.Contains(teks, "Situs.WhatsappAktif") {
			t.Errorf("%s memakai TautanWhatsapp tanpa memeriksa Situs.WhatsappAktif", nama)
		}
	}
}
