package pages

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Aplikasi berjalan dengan Content-Security-Policy `script-src 'self'`:
// tidak ada JavaScript sebaris yang boleh dieksekusi, dan tidak ada string
// yang boleh dievaluasi sebagai kode. Pelanggarannya gagal diam-diam di
// peramban — tombolnya sekadar tidak berfungsi, tanpa error di sisi server —
// sehingga hanya ketahuan saat seseorang benar-benar mengklik. Test ini
// menjaganya di waktu build.
//
// Pernah terjadi: onchange="this.form.requestSubmit()" pada input berkas
// membuat unggah foto tidak pernah jalan, dan hx-headers pada <body>
// membuat htmx melempar EvalError di setiap elemen yang diprosesnya.
var polaTerlarang = []struct {
	nama     string
	pola     *regexp.Regexp
	alasan   string
	gantinya string
}{
	{
		nama:     "handler sebaris",
		pola:     regexp.MustCompile(`\son[a-z]+\s*=\s*"`),
		alasan:   "atribut seperti onclick/onchange adalah JavaScript sebaris",
		gantinya: "pasang listener yang didelegasikan di web/static/js/app.js",
	},
	{
		nama:     "hx-on",
		pola:     regexp.MustCompile(`\shx-on[:=]`),
		alasan:   "htmx mengevaluasi isi hx-on sebagai JavaScript",
		gantinya: "gunakan event htmx (htmx:afterRequest) di app.js",
	},
	{
		nama:     "hx-vals/hx-headers berisi ekspresi",
		pola:     regexp.MustCompile(`\shx-(vals|headers)\s*=`),
		alasan:   "htmx mem-parsingnya dengan Function('return (…)'), bukan JSON.parse",
		gantinya: "isi parameternya lewat event htmx:configRequest di app.js",
	},
	{
		nama:     "URL javascript:",
		pola:     regexp.MustCompile(`"javascript:`),
		alasan:   "navigasi javascript: adalah eksekusi sebaris",
		gantinya: "gunakan tombol biasa beserta listener-nya",
	},
	{
		nama:     "blok <script> sebaris",
		pola:     regexp.MustCompile(`<script(?:\s[^>]*)?>[^<]`),
		alasan:   "skrip sebaris tidak diizinkan; hanya <script src> yang boleh",
		gantinya: "pindahkan isinya ke berkas di web/static/js",
	},
}

func TestTemplateBebasJavaScriptSebaris(t *testing.T) {
	// Test dijalankan dari direktori paketnya, sehingga ".." mencakup
	// seluruh template: layout, components, dan pages.
	akar := ".."

	var diperiksa int
	err := filepath.WalkDir(akar, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".templ") {
			return err
		}
		isi, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		diperiksa++

		for _, baris := range strings.Split(string(isi), "\n") {
			for _, p := range polaTerlarang {
				if p.pola.MatchString(baris) {
					t.Errorf("%s: %s — %s.\n  Baris: %s\n  Gantinya: %s",
						path, p.nama, p.alasan, strings.TrimSpace(baris), p.gantinya)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("gagal memindai template: %v", err)
	}
	if diperiksa == 0 {
		t.Fatal("tidak ada berkas .templ yang terpindai — jalur akar kemungkinan salah")
	}
	t.Logf("%d berkas template terpindai", diperiksa)
}

// Aset pihak ketiga harus di-self-host supaya script-src 'self' cukup.
// Satu-satunya pengecualian adalah gambar tile peta, yang diminta Leaflet
// dari server OpenStreetMap saat runtime — bukan dari markup.
func TestTidakAdaAsetDariCDN(t *testing.T) {
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".templ") {
			return err
		}
		isi, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, baris := range strings.Split(string(isi), "\n") {
			// href boleh menunjuk ke luar (wa.me, atribusi peta) karena itu
			// tautan navigasi, bukan aset yang dieksekusi atau dirender.
			if !strings.Contains(baris, `src="http`) {
				continue
			}
			t.Errorf("%s memuat aset dari luar, seharusnya di-self-host: %s",
				path, strings.TrimSpace(baris))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("gagal memindai template: %v", err)
	}
}

// hx-target yang menunjuk id tidak ada gagal diam-diam: htmx melempar
// htmx:targetError ke konsol dan tombolnya sekadar tidak melakukan apa pun.
// Pernah terjadi pada tombol "Setujui & tayangkan" di antrean admin, yang
// menyasar #panel-antrean padahal id itu hanya ada di fragmennya — tidak
// pernah dirender pada halaman penuh.
func TestTargetHTMXAda(t *testing.T) {
	polaTarget := regexp.MustCompile(`hx-target="#([a-zA-Z0-9_-]+)"`)
	polaID := regexp.MustCompile(`id="([a-zA-Z0-9_-]+)"`)

	target := map[string]string{} // id -> berkas yang merujuknya
	tersedia := map[string]bool{}

	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".templ") {
			return err
		}
		isi, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		teks := string(isi)

		for _, m := range polaTarget.FindAllStringSubmatch(teks, -1) {
			if _, sudah := target[m[1]]; !sudah {
				target[m[1]] = path
			}
		}
		for _, m := range polaID.FindAllStringSubmatch(teks, -1) {
			tersedia[m[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("gagal memindai template: %v", err)
	}
	if len(target) == 0 {
		t.Fatal("tidak ada hx-target yang ditemukan — pemindaian kemungkinan gagal")
	}

	for id, berkas := range target {
		if !tersedia[id] {
			t.Errorf("%s memakai hx-target=\"#%s\", tetapi tidak ada elemen dengan id itu "+
				"di template mana pun. Bungkus isinya dengan <div id=\"%s\"> pada halaman penuhnya.",
				berkas, id, id)
		}
	}
	t.Logf("%d target HTMX diperiksa", len(target))
}
