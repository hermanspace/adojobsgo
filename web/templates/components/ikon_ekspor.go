package components

import (
	"regexp"
	"sort"
	"strings"
)

// Set ikon hidup di icons.templ — satu-satunya sumber. EksporIkon
// mengurainya menjadi berkas SVG mandiri per nama untuk aset aplikasi
// Android (flutter_svg), sehingga kedua platform menggambar garis yang
// sama persis. Dipanggil `make ikon`; uji penjaga memastikan hasil ekspor
// tidak tertinggal dari template.

var (
	polaKasus = regexp.MustCompile(`(?s)case "([a-z-]+)":\s*(<svg.*?</svg>)`)
	polaSpasi = regexp.MustCompile(`(?m)^\t+`)
)

// EksporIkon mengembalikan peta nama → SVG mandiri dari isi icons.templ.
func EksporIkon(templ string) map[string]string {
	out := map[string]string{}
	for _, m := range polaKasus.FindAllStringSubmatch(templ, -1) {
		svg := m[2]
		svg = strings.Replace(svg, ` class={ class }`, ` xmlns="http://www.w3.org/2000/svg"`, 1)
		svg = strings.Replace(svg, ` aria-hidden="true"`, "", 1)
		svg = polaSpasi.ReplaceAllString(svg, "")
		out[m[1]] = svg + "\n"
	}
	return out
}

// NamaIkon mengembalikan daftar nama ikon terurut.
func NamaIkon(templ string) []string {
	ikon := EksporIkon(templ)
	nama := make([]string, 0, len(ikon))
	for n := range ikon {
		nama = append(nama, n)
	}
	sort.Strings(nama)
	return nama
}
