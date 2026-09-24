// Perintah ikon menghasilkan dua hal untuk aplikasi Android dan PWA:
//   - ikon aplikasi PNG dari bentuk favicon.svg + warna merek tokens.json
//     (web/static/img/ikon-*.png), dan
//   - set ikon antarmuka sebagai SVG mandiri per nama, diekspor dari
//     icons.templ (web/static/design/ikon/*.svg + index.json).
//
// Dijalankan lewat `make ikon`; hasilnya ikut ke image.
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"golang.org/x/image/vector"

	"github.com/hermansyah/adojobsid/web/templates/components"
)

type titik struct{ x, y float64 }

// Huruf "A" dari favicon.svg (viewBox 32), sudah dijabarkan ke koordinat
// absolut. Kontur luar searah jarum jam, lubang berlawanan — aturan
// nonzero yang dipakai rasterizer membuat lubangnya benar-benar berlubang.
var hurufLuar = []titik{{10, 22.5}, {15.2, 9.5}, {17.1, 9.5}, {22.3, 22.5}, {19.3, 22.5}, {18.2, 19.6}, {13, 19.6}, {11.9, 22.5}, {9, 22.5}}
var hurufLubang = []titik{{14.8, 17.2}, {18.4, 17.2}, {16.6, 12.4}}

func main() {
	dari, ke := warnaMerek()
	tujuan := "web/static/img"
	for _, spek := range []struct {
		nama     string
		ukuran   int
		maskable bool
	}{{"ikon-192.png", 192, false}, {"ikon-512.png", 512, false}, {"ikon-maskable-512.png", 512, true}} {
		img := gambar(spek.ukuran, spek.maskable, dari, ke)
		f, err := os.Create(filepath.Join(tujuan, spek.nama))
		if err != nil {
			panic(err)
		}
		if err := png.Encode(f, img); err != nil {
			panic(err)
		}
		_ = f.Close()
		fmt.Printf("  %s (%dpx)\n", spek.nama, spek.ukuran)
	}
	eksporSetIkon()
}

// eksporSetIkon menulis SVG per ikon dan index.json berisi daftar namanya.
func eksporSetIkon() {
	src, err := os.ReadFile("web/templates/components/icons.templ")
	if err != nil {
		panic(err)
	}
	dir := "web/static/design/ikon"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		panic(err)
	}
	// Bersihkan berkas usang supaya ikon yang dihapus dari templ ikut hilang.
	lama, _ := filepath.Glob(filepath.Join(dir, "*.svg"))
	for _, f := range lama {
		_ = os.Remove(f)
	}
	ikon := components.EksporIkon(string(src))
	for nama, svg := range ikon {
		if err := os.WriteFile(filepath.Join(dir, nama+".svg"), []byte(svg), 0o644); err != nil {
			panic(err)
		}
	}
	indeks, _ := json.MarshalIndent(components.NamaIkon(string(src)), "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "index.json"), append(indeks, '\n'), 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("  %d ikon SVG → %s\n", len(ikon), dir)
}

func warnaMerek() (color.RGBA, color.RGBA) {
	raw, err := os.ReadFile("web/static/design/tokens.json")
	if err != nil {
		panic(err)
	}
	var t struct {
		Warna struct {
			Terang map[string]string `json:"terang"`
		} `json:"warna"`
	}
	if err := json.Unmarshal(raw, &t); err != nil {
		panic(err)
	}
	return hex(t.Warna.Terang["primary-from"]), hex(t.Warna.Terang["primary-to"])
}

func hex(s string) color.RGBA {
	var r, g, b uint8
	if _, err := fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &b); err != nil {
		panic("warna bukan #rrggbb: " + s)
	}
	return color.RGBA{r, g, b, 255}
}

// gambar melukis latar gradasi diagonal (dipotong sudut membulat untuk ikon
// biasa; penuh untuk maskable, yang dipotong sistem sendiri) lalu huruf
// putih. Untuk maskable huruf diperkecil ke zona aman 80%.
func gambar(ukuran int, maskable bool, dari, ke color.RGBA) *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, ukuran, ukuran))
	latar := image.NewRGBA(out.Bounds())
	for y := 0; y < ukuran; y++ {
		for x := 0; x < ukuran; x++ {
			t := float64(x+y) / float64(2*ukuran)
			latar.SetRGBA(x, y, color.RGBA{lerp(dari.R, ke.R, t), lerp(dari.G, ke.G, t), lerp(dari.B, ke.B, t), 255})
		}
	}
	if maskable {
		draw.Draw(out, out.Bounds(), latar, image.Point{}, draw.Src)
	} else {
		mask := image.NewAlpha(out.Bounds())
		r := vector.NewRasterizer(ukuran, ukuran)
		sudutBulat(r, float64(ukuran), float64(ukuran)/4) // rx 8 dari 32
		r.Draw(mask, mask.Bounds(), image.Opaque, image.Point{})
		draw.DrawMask(out, out.Bounds(), latar, image.Point{}, mask, image.Point{}, draw.Over)
	}

	skala, geser := float64(ukuran)/32, 0.0
	if maskable {
		skala *= 0.8
		geser = float64(ukuran) * 0.1
	}
	r := vector.NewRasterizer(ukuran, ukuran)
	for _, kontur := range [][]titik{hurufLuar, hurufLubang} {
		for i, p := range kontur {
			x, y := float32(p.x*skala+geser), float32(p.y*skala+geser)
			if i == 0 {
				r.MoveTo(x, y)
			} else {
				r.LineTo(x, y)
			}
		}
		r.ClosePath()
	}
	huruf := image.NewAlpha(out.Bounds())
	r.Draw(huruf, huruf.Bounds(), image.Opaque, image.Point{})
	draw.DrawMask(out, out.Bounds(), image.NewUniform(color.White), image.Point{}, huruf, image.Point{}, draw.Over)
	return out
}

func lerp(a, b uint8, t float64) uint8 {
	return uint8(math.Round(float64(a) + (float64(b)-float64(a))*t))
}

// sudutBulat menggambar persegi ukuran s dengan sudut berjari-jari rad,
// memakai kurva kubik pendekatan busur (k = 0.5523).
func sudutBulat(r *vector.Rasterizer, s, rad float64) {
	k := 0.5523 * rad
	f := func(v float64) float32 { return float32(v) }
	r.MoveTo(f(rad), 0)
	r.LineTo(f(s-rad), 0)
	r.CubeTo(f(s-rad+k), 0, f(s), f(rad-k), f(s), f(rad))
	r.LineTo(f(s), f(s-rad))
	r.CubeTo(f(s), f(s-rad+k), f(s-rad+k), f(s), f(s-rad), f(s))
	r.LineTo(f(rad), f(s))
	r.CubeTo(f(rad-k), f(s), 0, f(s-rad+k), 0, f(s-rad))
	r.LineTo(0, f(rad))
	r.CubeTo(0, f(rad-k), f(rad-k), 0, f(rad), 0)
	r.ClosePath()
}
