package seed

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

// gambarDemo membuat gambar placeholder yang pantas dilihat: gradasi dua
// warna bernuansa dari benih, dengan beberapa lingkaran tembus pandang
// sebagai tekstur. Tanpa teks (tidak ada font di image akhir) dan tanpa
// berkas eksternal, sehingga seeder tetap mandiri.
func gambarDemo(lebar, tinggi int, benih int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, lebar, tinggi))
	h1 := float64((benih*47)%360) / 360
	h2 := math.Mod(h1+0.09, 1)
	dari, ke := hsl(h1, 0.55, 0.42), hsl(h2, 0.6, 0.6)
	for y := 0; y < tinggi; y++ {
		for x := 0; x < lebar; x++ {
			t := (float64(x)/float64(lebar)*0.7 + float64(y)/float64(tinggi)*0.3)
			img.SetRGBA(x, y, campur(dari, ke, t))
		}
	}
	// Tiga lingkaran lembut di posisi yang ditentukan benih.
	for i := 0; i < 3; i++ {
		cx := float64((benih*131 + i*97) % lebar)
		cy := float64((benih*71 + i*53) % tinggi)
		r := float64(minInt(lebar, tinggi)) * (0.18 + 0.08*float64(i))
		terang := hsl(math.Mod(h1+0.5, 1), 0.5, 0.8)
		for y := int(math.Max(0, cy-r)); y < minInt(tinggi, int(cy+r)); y++ {
			for x := int(math.Max(0, cx-r)); x < minInt(lebar, int(cx+r)); x++ {
				d := math.Hypot(float64(x)-cx, float64(y)-cy) / r
				if d > 1 {
					continue
				}
				a := 0.22 * (1 - d*d)
				img.SetRGBA(x, y, campur(img.RGBAAt(x, y), terang, a))
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func campur(a, b color.RGBA, t float64) color.RGBA {
	l := func(x, y uint8) uint8 { return uint8(math.Round(float64(x) + (float64(y)-float64(x))*t)) }
	return color.RGBA{l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), 255}
}

func hsl(h, s, l float64) color.RGBA {
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h*6, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch int(h * 6) {
	case 0:
		r, g, b = c, x, 0
	case 1:
		r, g, b = x, c, 0
	case 2:
		r, g, b = 0, c, x
	case 3:
		r, g, b = 0, x, c
	case 4:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return color.RGBA{uint8((r + m) * 255), uint8((g + m) * 255), uint8((b + m) * 255), 255}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
