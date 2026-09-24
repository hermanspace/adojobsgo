package pages

import (
	"os"
	"strings"
	"testing"
)

// Kartu yang sekaligus sorotan jasa dan milik penyedia pilihan harus tampil
// kuning keemasan. Yang menentukan itu semata urutan penulisan di CSS:
// kekhususan kedua kelas sama, jadi yang ditulis belakangan menang. Menukar
// urutannya membalik aturan tanpa error apa pun — CSS tetap sah, kartunya
// hanya berubah warna diam-diam.
func TestUrutanKelasSorotDiCSS(t *testing.T) {
	isi, err := os.ReadFile("../../static/css/source.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(isi)

	penyedia := strings.Index(css, "  .is-sorot-penyedia {")
	jasa := strings.Index(css, "  .is-sorot {")

	if penyedia < 0 || jasa < 0 {
		t.Fatalf("aturan sorot tidak ditemukan (penyedia=%d, jasa=%d)", penyedia, jasa)
	}
	if jasa < penyedia {
		t.Error(".is-sorot ditulis sebelum .is-sorot-penyedia; kartu yang keduanya " +
			"akan tampil biru keunguan, bukan kuning keemasan")
	}
}
