package components

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Aset SVG di web/static/design/ikon harus persis hasil ekspor icons.templ
// saat ini: ikon baru yang belum diekspor, atau berkas usang yang ikonnya
// sudah dihapus, sama-sama membuat Android menggambar set yang berbeda.
func TestEksporIkonSelaras(t *testing.T) {
	src, err := os.ReadFile("icons.templ")
	if err != nil {
		t.Fatal(err)
	}
	ikon := EksporIkon(string(src))
	if len(ikon) < 30 {
		t.Fatalf("hanya %d ikon terurai; pola parser tidak cocok lagi dengan icons.templ", len(ikon))
	}
	dir := "../../static/design/ikon"
	for nama, svg := range ikon {
		isi, err := os.ReadFile(filepath.Join(dir, nama+".svg"))
		if err != nil || string(isi) != svg {
			t.Errorf("%s.svg tertinggal atau hilang; jalankan `make ikon`", nama)
		}
		if !strings.Contains(svg, `xmlns="http://www.w3.org/2000/svg"`) || strings.Contains(svg, "{ class }") {
			t.Errorf("%s: SVG tidak mandiri: %s", nama, svg)
		}
	}
	entri, _ := os.ReadDir(dir)
	for _, e := range entri {
		nama := strings.TrimSuffix(e.Name(), ".svg")
		if strings.HasSuffix(e.Name(), ".svg") && ikon[nama] == "" {
			t.Errorf("%s usang: ikon %q tidak ada lagi di icons.templ", e.Name(), nama)
		}
	}
}
