package repository

import (
	"strings"
	"testing"
)

func TestPecahKata(t *testing.T) {
	kasus := []struct {
		masuk   string
		harapan []string
	}{
		{"service ac", []string{"service", "ac"}},
		{"  Servis   AC  ", []string{"servis", "ac"}},
		{"tukang di bengkalis", []string{"tukang", "bengkalis"}}, // kata penghubung dibuang
		{"cuci ac, ac", []string{"cuci", "ac"}},                  // duplikat dibuang
		{"a b c", nil},                                           // terlalu pendek untuk trigram
		{"", nil},
	}
	for _, k := range kasus {
		got := pecahKata(k.masuk)
		if strings.Join(got, "|") != strings.Join(k.harapan, "|") {
			t.Errorf("pecahKata(%q) = %v, harusnya %v", k.masuk, got, k.harapan)
		}
	}
}

// Tiap kata wajib cocok (AND), masing-masing sebagai substring atau mirip
// trigram, dan nomor argumennya melanjutkan argumen yang sudah ada.
func TestKondisiKataKunci(t *testing.T) {
	cond, args := kondisiKataKunci([]string{"service", "ac"}, "t", []any{"sudah-ada"})

	if len(args) != 3 || args[1] != "service" || args[2] != "ac" {
		t.Fatalf("args = %v", args)
	}
	if !strings.Contains(cond, "$2") || !strings.Contains(cond, "$3") || strings.Contains(cond, "$1") {
		t.Errorf("nomor argumen salah: %s", cond)
	}
	if strings.Count(cond, " AND ") != 1 {
		t.Errorf("dua kata harus digabung satu AND: %s", cond)
	}
	if strings.Count(cond, "word_similarity") != 2 || strings.Count(cond, "ILIKE") != 2 {
		t.Errorf("tiap kata harus punya cabang substring dan kemiripan: %s", cond)
	}
	if !strings.Contains(cond, "0.45") {
		t.Errorf("ambang kemiripan tidak tertulis: %s", cond)
	}
}

// buildFilter tidak boleh kembali ke pencocokan frasa utuh, dan kata kunci
// harus jadi argumen pertama yang ditambahkannya — relevansiPencarian
// bergantung pada urutan itu untuk menomori ulang argumen di ORDER BY.
func TestBuildFilterKataKunciPerKataDanPalingAwal(t *testing.T) {
	awal := []any{1.0, 2.0} // seolah lat/lng sudah ada
	where, args := buildFilter(ServiceFilter{Query: "service ac", Kecamatan: "Bengkalis"}, awal)

	for _, a := range args {
		if s, ok := a.(string); ok && strings.HasPrefix(s, "%") {
			t.Errorf("masih ada pencocokan frasa utuh: %q", s)
		}
	}
	if args[2] != "service" || args[3] != "ac" {
		t.Fatalf("kata kunci harus jadi argumen tepat setelah yang sudah ada: %v", args)
	}
	rel := relevansiPencarian(ServiceFilter{Query: "service ac"}, len(awal))
	if !strings.Contains(rel, "$3") || !strings.Contains(rel, "$4") {
		t.Errorf("relevansi menomori argumen kata dengan salah: %s", rel)
	}
	if !strings.Contains(where, "word_similarity($3") {
		t.Errorf("klausa WHERE tidak memakai argumen kata yang sama: %s", where)
	}
}

// Kunci cache tidak boleh membedakan spasi ganda atau huruf besar.
func TestCacheKeyMenormalkanKataKunci(t *testing.T) {
	a := ServiceFilter{Query: "Service  AC", Limit: 20}.CacheKey()
	b := ServiceFilter{Query: "service ac", Limit: 20}.CacheKey()
	if a != b {
		t.Errorf("kunci berbeda untuk kata kunci yang sama:\n%s\n%s", a, b)
	}
}
