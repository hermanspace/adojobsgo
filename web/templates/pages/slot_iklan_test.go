package pages

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Tiap slot iklan harus berupa satu form multipart dengan satu tombol simpan.
// Sebelumnya teks dan berkas dipisah ke dua form: admin memilih gambar lalu
// menekan tombol simpan milik form teks, berkasnya tidak ikut terkirim, dan
// halaman kembali dengan pesan "tersimpan" tanpa gambar. Tidak ada galat yang
// muncul di mana pun — kegagalannya sepenuhnya diam.
func TestSlotIklanSatuFormMultipart(t *testing.T) {
	teks := bacaTemplPengaturan(t)

	if strings.Contains(teks, `form="form-slot-iklan"`) {
		t.Error("slot iklan kembali memakai form terpisah yang ditautkan atribut form=")
	}

	// Form slot harus multipart, kalau tidak berkasnya tidak akan terkirim.
	form := regexp.MustCompile(`(?s)<form[^>]*/admin/pengaturan/iklan/[^>]*>`)
	cocok := form.FindAllString(teks, -1)
	if len(cocok) == 0 {
		t.Fatal("form slot iklan tidak ditemukan")
	}
	for _, f := range cocok {
		if !strings.Contains(f, `enctype="multipart/form-data"`) {
			t.Errorf("form slot iklan tanpa enctype multipart:\n%s", strings.TrimSpace(f))
		}
	}
}

// Isian berkas tidak boleh wajib: admin harus bisa mengubah teks saja tanpa
// dipaksa mengunggah ulang gambarnya.
func TestBerkasSlotIklanTidakWajib(t *testing.T) {
	teks := bacaTemplPengaturan(t)
	input := regexp.MustCompile(`(?s)<input[^>]*name="gambar"[^>]*/>`)
	cocok := input.FindString(teks)
	if cocok == "" {
		t.Fatal("isian berkas slot iklan tidak ditemukan")
	}
	if strings.Contains(cocok, "required") {
		t.Error("isian berkas slot iklan bertanda required; mengubah teks saja jadi mustahil")
	}
}

// Materi iklan diunggah sebagai berkas, bukan ditempel sebagai URL.
func TestSlotIklanTidakPunyaIsianURLGambar(t *testing.T) {
	if strings.Contains(bacaTemplPengaturan(t), "[gambar_url]") {
		t.Error("slot iklan kembali memakai isian URL gambar; materinya harus diunggah")
	}
}

func bacaTemplPengaturan(t *testing.T) string {
	t.Helper()
	isi, err := os.ReadFile("admin_pengaturan.templ")
	if err != nil {
		t.Fatal(err)
	}
	return string(isi)
}
