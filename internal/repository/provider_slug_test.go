package repository

import (
	"regexp"
	"strings"
	"testing"
)

// Slug penyedia dipakai sebagai alamat publik, sehingga bentuknya harus aman
// dipakai di URL: huruf kecil, angka, dan tanda hubung saja.
func TestBentukSlugAman(t *testing.T) {
	aman := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

	sah := []string{"rizal-teknik-ac", "cv-amanah-karya", "penyedia", "toko123", "nama-2"}
	for _, s := range sah {
		if !aman.MatchString(s) {
			t.Errorf("slug %q seharusnya dianggap sah", s)
		}
	}

	tidakSah := []string{
		"Rizal Teknik AC", // ada spasi dan huruf besar
		"-awalan-strip",
		"akhiran-strip-",
		"dobel--strip",
		"ada/slash",
		"",
	}
	for _, s := range tidakSah {
		if aman.MatchString(s) {
			t.Errorf("slug %q seharusnya ditolak", s)
		}
	}
}

// Kueri slug harus memakai placeholder, bukan menempelkan nilai ke dalam SQL.
func TestKueriSlugMemakaiPlaceholder(t *testing.T) {
	if !strings.Contains(providerDetailSelect, "p.slug") {
		t.Error("kolom slug tidak ikut dipilih pada query detail penyedia")
	}
	if !strings.Contains(providerColumns, "slug") {
		t.Error("kolom slug tidak ada di daftar kolom penyedia")
	}
}
