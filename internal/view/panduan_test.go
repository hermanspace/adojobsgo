package view

import (
	"github.com/hermansyah/adojobsid/internal/model"
	"strings"
	"testing"
)

// Panduan dirender web dan Android dari data ini; yang dijaga: anchor unik,
// tidak ada placeholder tersisa, ikon ada, dan setiap bagian punya isi.
func TestPanduanKonsisten(t *testing.T) {
	ikon := ikonTersedia(t)
	bagian := Panduan("Adojobs")
	id := map[string]bool{}
	for _, b := range bagian {
		if id[b.ID] {
			t.Errorf("anchor %q ganda", b.ID)
		}
		id[b.ID] = true
		isi := len(b.Paragraf) + len(b.Jalur) + len(b.Fitur) + len(b.Tanya)
		if isi == 0 {
			t.Errorf("bagian %q kosong", b.ID)
		}
		semua := append([]PanduanLangkah{}, b.Fitur...)
		for _, j := range b.Jalur {
			semua = append(semua, j.Langkah...)
		}
		for _, l := range semua {
			if !ikon[l.Ikon] {
				t.Errorf("%s: ikon %q tidak ada", b.ID, l.Ikon)
			}
			if l.Tautan != "" && !strings.HasPrefix(l.Tautan, "/") {
				t.Errorf("%s: tautan %q bukan path web", b.ID, l.Tautan)
			}
		}
		for _, teks := range append(append([]string{b.Judul, b.Ringkas}, b.Paragraf...), tanyaJawab(b.Tanya)...) {
			if strings.Contains(teks, "{situs}") {
				t.Errorf("%s: placeholder {situs} belum diganti: %q", b.ID, teks)
			}
		}
	}
	for _, wajib := range []string{"tentang", "cara-kerja", "fitur", "promosi", "faq", "kontak"} {
		if !id[wajib] {
			t.Errorf("anchor #%s hilang; menu dan tautan bantuan mengandalkannya", wajib)
		}
	}
	if !strings.Contains(bagian[0].Judul, "Adojobs") {
		t.Error("nama situs tidak masuk ke judul")
	}
}

func tanyaJawab(d []PanduanTanyaJawab) []string {
	var out []string
	for _, tj := range d {
		out = append(out, tj.Tanya, tj.Jawab)
	}
	return out
}

func TestLabelPaketTidakMengulangDurasi(t *testing.T) {
	if got := LabelPaket(model.PaketPromosi{Nama: "Dasar 7 hari", DurasiHari: 7}); got != "Dasar 7 hari" {
		t.Errorf("nama berdurasi diulang: %q", got)
	}
	if got := LabelPaket(model.PaketPromosi{Nama: "Utama", DurasiHari: 30}); got != "Utama · 30 hari" {
		t.Errorf("nama tanpa durasi = %q", got)
	}
}
