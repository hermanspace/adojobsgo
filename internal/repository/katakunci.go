package repository

import (
	"fmt"
	"strconv"
	"strings"
)

// Pencocokan kata kunci.
//
// Dulu seluruh frasa dicocokkan sebagai satu substring utuh: "service ac"
// menuntut kedua kata muncul persis berdampingan, dan ejaan "service" tidak
// pernah sama dengan "servis". Sekarang frasa dipecah per kata, dan tiap kata
// harus muncul — sebagai substring ATAU sebagai kata yang mirip menurut
// trigram pg_trgm — di teks gabungan judul, kategori, nama penyedia, dan
// deskripsi. Semua kata harus cocok (AND), tetapi urutan dan jaraknya bebas.

// ambangMirip adalah batas word_similarity agar dua kata dianggap sama.
// Diambil dari angka nyata: "service"~"servis" 0,63 dan "servise"~"servis"
// 0,75 harus lolos; "pipa"~"atap bocor" 0,2 harus gagal. 0,45 memberi ruang
// ke kedua arah.
const ambangMirip = 0.45

// kataAbai adalah kata penghubung yang tidak membawa makna pencarian dan akan
// cocok dengan hampir semua teks bila ikut diwajibkan.
var kataAbai = map[string]bool{
	"di": true, "ke": true, "dari": true, "dan": true, "yang": true,
	"untuk": true, "atau": true, "dengan": true, "the": true, "of": true,
}

// pecahKata memecah kata kunci menjadi kata-kata yang layak dicocokkan:
// huruf kecil, tanpa kata penghubung, tanpa duplikat, tanpa yang terlalu
// pendek untuk punya trigram yang berarti.
func pecahKata(q string) []string {
	var out []string
	sudah := map[string]bool{}
	for _, k := range strings.FieldsFunc(strings.ToLower(q), func(r rune) bool {
		return r == ' ' || r == ',' || r == '/' || r == ';' || r == '\t' || r == '\n'
	}) {
		k = strings.Trim(k, ".-_\"'()")
		if len([]rune(k)) < 2 || kataAbai[k] || sudah[k] {
			continue
		}
		sudah[k] = true
		out = append(out, k)
	}
	return out
}

// kondisiKataKunci menyusun klausa WHERE untuk kata-kata hasil pecahKata
// terhadap satu ekspresi teks. Tiap kata ditambahkan sebagai argumen sendiri;
// nomornya dihitung dari panjang args yang masuk.
func kondisiKataKunci(kata []string, teks string, args []any) (string, []any) {
	bagian := make([]string, 0, len(kata))
	for _, k := range kata {
		args = append(args, k)
		n := len(args)
		bagian = append(bagian, fmt.Sprintf(
			"(%[1]s ILIKE '%%' || $%[2]d || '%%' OR word_similarity($%[2]d, %[1]s) >= %[3]s)",
			teks, n, strconv.FormatFloat(ambangMirip, 'f', 2, 64)))
	}
	return "(" + strings.Join(bagian, " AND ") + ")", args
}

// ekspresiRelevansi menyusun skor untuk ORDER BY: jumlah kemiripan tiap kata
// terhadap judul dan kategori. Memakai argumen kata yang sama dengan klausa
// WHERE — mulai adalah indeks argumen pertama kata-kata itu.
func ekspresiRelevansi(kata []string, teks string, mulai int) string {
	if len(kata) == 0 {
		return ""
	}
	bagian := make([]string, 0, len(kata))
	for i := range kata {
		bagian = append(bagian, fmt.Sprintf("word_similarity($%d, %s)", mulai+i, teks))
	}
	return "(" + strings.Join(bagian, " + ") + ")"
}

// teksListing adalah teks gabungan yang dicocokkan pada pencarian jasa.
// Nama kategori induk ikut, karena pengguna lazim mengetik istilah umum
// ("kebersihan", "dokumentasi") yang tidak muncul di judul maupun nama
// sub-kategorinya.
const teksListing = "(s.title || ' ' || c.name || ' ' || COALESCE(pc.name, '') || ' ' || u.full_name || ' ' || COALESCE(s.description, ''))"

// teksRelevansiListing lebih sempit: hanya judul dan kategori, supaya hasil
// yang kata kuncinya cuma tersebut di deskripsi tidak menyalip yang memang
// jasanya itu.
const teksRelevansiListing = "(s.title || ' ' || c.name)"

// teksPenyedia adalah teks gabungan pada direktori penyedia.
const teksPenyedia = "(u.full_name || ' ' || COALESCE(p.bio, ''))"
