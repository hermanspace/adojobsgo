package web

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Leaflet hanya dimuat bila handler menandai halamannya lewat DenganPeta().
// Halaman berpeta yang handler-nya lupa menandai tidak gagal di mana pun:
// petanya cuma tidak muncul, tanpa galat, karena app.js memang diam bila
// Leaflet tidak ada. Penjaga ini menutup celah itu secara mekanis — daftar
// halaman berpetanya dibaca dari template, bukan diingat di sini.
func TestHalamanBerpetaMenandaiDenganPeta(t *testing.T) {
	templs, err := filepath.Glob("../../../web/templates/pages/*.templ")
	if err != nil || len(templs) == 0 {
		t.Fatalf("template tidak ditemukan: %v", err)
	}
	namaTempl := regexp.MustCompile(`(?m)^templ ([A-Z]\w*)\(`)

	// nama templ tingkat atas dari setiap halaman yang menggambar peta
	var berpeta []string
	for _, f := range templs {
		isi, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(isi), "data-peta") {
			continue
		}
		for _, m := range namaTempl.FindAllStringSubmatch(string(isi), -1) {
			berpeta = append(berpeta, m[1])
		}
	}
	if len(berpeta) == 0 {
		t.Fatal("tidak ada halaman berpeta yang ditemukan; penjaga ini jadi hampa")
	}

	handlers, _ := filepath.Glob("*.go")
	var diperiksa int
	for _, h := range handlers {
		if strings.HasSuffix(h, "_test.go") {
			continue
		}
		isi, err := os.ReadFile(h)
		if err != nil {
			t.Fatal(err)
		}
		teks := string(isi)
		for _, nama := range berpeta {
			if !strings.Contains(teks, "pages."+nama+"(") {
				continue
			}
			diperiksa++
			if !strings.Contains(teks, ".DenganPeta()") {
				t.Errorf("%s merender pages.%s (berpeta) tanpa menandai DenganPeta()", h, nama)
			}
		}
	}
	if diperiksa == 0 {
		t.Fatal("tidak ada handler yang merender halaman berpeta; penjaga ini jadi hampa")
	}
}
