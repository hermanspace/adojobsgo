package service

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func gambarUji(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	return img
}

func TestPerkecil(t *testing.T) {
	// Sisi terpanjang dibatasi, rasio dipertahankan.
	hasil := perkecil(gambarUji(4000, 3000), 1600)
	if got := hasil.Bounds().Dx(); got != 1600 {
		t.Errorf("lebar = %d, harusnya 1600", got)
	}
	if got := hasil.Bounds().Dy(); got != 1200 {
		t.Errorf("tinggi = %d, harusnya 1200 (rasio 4:3 dipertahankan)", got)
	}

	// Potret: sisi terpanjangnya tinggi.
	potret := perkecil(gambarUji(3000, 4000), 1600)
	if potret.Bounds().Dy() != 1600 || potret.Bounds().Dx() != 1200 {
		t.Errorf("potret = %dx%d, harusnya 1200x1600",
			potret.Bounds().Dx(), potret.Bounds().Dy())
	}

	// Gambar yang sudah kecil tidak diperbesar: memperbesar hanya menambah
	// ukuran berkas tanpa menambah detail.
	kecil := gambarUji(400, 300)
	if hasil := perkecil(kecil, 1600); hasil.Bounds().Dx() != 400 {
		t.Errorf("gambar kecil ikut diperbesar menjadi %d px", hasil.Bounds().Dx())
	}

	// maks 0 berarti tanpa pembatasan.
	if hasil := perkecil(gambarUji(4000, 3000), 0); hasil.Bounds().Dx() != 4000 {
		t.Error("maks 0 seharusnya membiarkan ukuran apa adanya")
	}
}

func TestSandikanMenghasilkanWebP(t *testing.T) {
	data, ekstensi, err := sandikan(gambarUji(320, 240), 80)
	if err != nil {
		t.Fatalf("penyandian gagal: %v", err)
	}
	if ekstensi != ".webp" {
		t.Errorf("ekstensi = %q, harusnya .webp", ekstensi)
	}
	// Berkas WebP diawali kontainer RIFF dengan penanda "WEBP".
	if len(data) < 12 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		t.Errorf("keluaran bukan berkas WebP yang sah (%d byte)", len(data))
	}
}

func TestProsesGambarMemperkecilDanMenyandikanUlang(t *testing.T) {
	var asli bytes.Buffer
	if err := jpeg.Encode(&asli, gambarUji(3000, 2000), &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}

	hasil, err := prosesGambar(bytes.NewReader(asli.Bytes()), ProfilListing)
	if err != nil {
		t.Fatalf("pemrosesan gagal: %v", err)
	}

	if hasil.Lebar != 1600 {
		t.Errorf("lebar hasil = %d, harusnya dibatasi 1600", hasil.Lebar)
	}
	if len(hasil.Thumb) == 0 {
		t.Error("profil listing seharusnya menghasilkan thumbnail")
	}
	if len(hasil.Thumb) >= len(hasil.Utama) {
		t.Errorf("thumbnail (%d B) tidak lebih kecil dari gambar utama (%d B)",
			len(hasil.Thumb), len(hasil.Utama))
	}
	if len(hasil.Utama) >= asli.Len() {
		t.Errorf("hasil (%d B) tidak lebih kecil dari berkas asli (%d B)",
			len(hasil.Utama), asli.Len())
	}

	// Profil tanpa thumbnail tidak menghasilkan varian tambahan.
	tanpaThumb, err := prosesGambar(bytes.NewReader(asli.Bytes()), ProfilAvatar)
	if err != nil {
		t.Fatalf("pemrosesan avatar gagal: %v", err)
	}
	if len(tanpaThumb.Thumb) != 0 {
		t.Error("profil avatar seharusnya tidak membuat thumbnail")
	}
	if tanpaThumb.Lebar != ProfilAvatar.MaksSisi {
		t.Errorf("lebar avatar = %d, harusnya %d", tanpaThumb.Lebar, ProfilAvatar.MaksSisi)
	}
}

// Berkas terkompresi kecil bisa mengembang menjadi gambar raksasa saat
// di-decode. Penjagaannya harus terjadi dari header, sebelum satu piksel pun
// dialokasikan.
func TestProsesGambarMenolakResolusiBerlebihan(t *testing.T) {
	// PNG 30000x30000 berisi satu warna: berkasnya kecil, tetapi decode-nya
	// membutuhkan sekitar 3,6 GB.
	bom := pngPalsuBesar(30000, 30000)

	_, err := prosesGambar(bytes.NewReader(bom), ProfilListing)
	if err == nil {
		t.Fatal("gambar beresolusi berlebihan seharusnya ditolak")
	}
	e, ok := AsError(err)
	if !ok || e.Code != CodeInvalidInput {
		t.Errorf("seharusnya ditolak sebagai input tidak valid, dapat %v", err)
	}
}

func TestProsesGambarMenolakBerkasRusak(t *testing.T) {
	if _, err := prosesGambar(bytes.NewReader([]byte("ini bukan gambar sama sekali")), ProfilListing); err == nil {
		t.Error("berkas yang bukan gambar seharusnya ditolak")
	}
}

// ---------- orientasi EXIF ----------

func TestBacaOrientasiEXIF(t *testing.T) {
	// Berkas tanpa EXIF dianggap normal.
	var polos bytes.Buffer
	_ = jpeg.Encode(&polos, gambarUji(64, 64), nil)
	if got := bacaOrientasiEXIF(bytes.NewReader(polos.Bytes())); got != orientasiNormal {
		t.Errorf("JPEG tanpa EXIF = %d, harusnya %d", got, orientasiNormal)
	}

	// Bukan JPEG sama sekali.
	if got := bacaOrientasiEXIF(bytes.NewReader([]byte("bukan jpeg"))); got != orientasiNormal {
		t.Errorf("berkas bukan JPEG = %d, harusnya %d", got, orientasiNormal)
	}

	// JPEG dengan tag Orientation, dua-duanya urutan byte.
	for _, besarDulu := range []bool{true, false} {
		for _, nilai := range []int{orientasiPutar90, orientasiPutar180, orientasiPutar270} {
			data := jpegDenganOrientasi(t, nilai, besarDulu)
			if got := bacaOrientasiEXIF(bytes.NewReader(data)); got != nilai {
				t.Errorf("orientasi (besarDulu=%v) = %d, harusnya %d", besarDulu, got, nilai)
			}
		}
	}
}

func TestTerapkanOrientasi(t *testing.T) {
	src := gambarUji(100, 60)

	// Orientasi normal tidak mengubah apa pun.
	if hasil := terapkanOrientasi(src, orientasiNormal); hasil != src {
		t.Error("orientasi normal seharusnya mengembalikan gambar yang sama")
	}

	// Orientasi 5–8 menukar sumbu, sehingga lebar dan tinggi bertukar.
	for _, o := range []int{orientasiCerminHPutar270, orientasiPutar90, orientasiCerminHPutar90, orientasiPutar270} {
		hasil := terapkanOrientasi(src, o)
		if hasil.Bounds().Dx() != 60 || hasil.Bounds().Dy() != 100 {
			t.Errorf("orientasi %d menghasilkan %dx%d, harusnya 60x100",
				o, hasil.Bounds().Dx(), hasil.Bounds().Dy())
		}
	}

	// Orientasi 2–4 hanya mencerminkan, ukuran tetap.
	for _, o := range []int{orientasiCerminH, orientasiPutar180, orientasiCerminV} {
		hasil := terapkanOrientasi(src, o)
		if hasil.Bounds().Dx() != 100 || hasil.Bounds().Dy() != 60 {
			t.Errorf("orientasi %d mengubah ukuran menjadi %dx%d",
				o, hasil.Bounds().Dx(), hasil.Bounds().Dy())
		}
	}

	// Nilai di luar rentang dibiarkan apa adanya.
	if hasil := terapkanOrientasi(src, 99); hasil != src {
		t.Error("orientasi tak dikenal seharusnya diabaikan")
	}
}

// Foto potret dari ponsel tersimpan lanskap dengan tag Orientation 6.
// Tanpa penanganan ini, fotonya tampil miring setelah metadatanya dibuang.
func TestProsesGambarMenghormatiOrientasi(t *testing.T) {
	data := jpegDenganOrientasi(t, orientasiPutar90, true)

	hasil, err := prosesGambar(bytes.NewReader(data), ProfilAvatar)
	if err != nil {
		t.Fatalf("pemrosesan gagal: %v", err)
	}
	// Sumbernya 120x80 (lanskap); dengan orientasi 6 hasilnya harus potret.
	if hasil.Lebar >= hasil.Tinggi {
		t.Errorf("hasil %dx%d masih lanskap — orientasi tidak diterapkan",
			hasil.Lebar, hasil.Tinggi)
	}
}

// ---------- pembantu ----------

// jpegDenganOrientasi menyusun JPEG lengkap dengan segmen APP1 berisi satu
// tag EXIF Orientation.
func jpegDenganOrientasi(t *testing.T, orientasi int, besarDulu bool) []byte {
	t.Helper()

	var dasar bytes.Buffer
	if err := jpeg.Encode(&dasar, gambarUji(120, 80), nil); err != nil {
		t.Fatal(err)
	}
	asli := dasar.Bytes()

	var urutan binary.ByteOrder = binary.BigEndian
	penanda := []byte{'M', 'M'}
	if !besarDulu {
		urutan = binary.LittleEndian
		penanda = []byte{'I', 'I'}
	}

	// Blok TIFF: header (8 byte) + satu entri IFD.
	tiff := new(bytes.Buffer)
	tiff.Write(penanda)
	_ = binary.Write(tiff, urutan, uint16(42))
	_ = binary.Write(tiff, urutan, uint32(8))         // offset IFD0
	_ = binary.Write(tiff, urutan, uint16(1))         // jumlah entri
	_ = binary.Write(tiff, urutan, uint16(0x0112))    // tag Orientation
	_ = binary.Write(tiff, urutan, uint16(3))         // tipe SHORT
	_ = binary.Write(tiff, urutan, uint32(1))         // jumlah nilai
	_ = binary.Write(tiff, urutan, uint16(orientasi)) // nilai
	_ = binary.Write(tiff, urutan, uint16(0))         // padding 4 byte
	_ = binary.Write(tiff, urutan, uint32(0))         // offset IFD berikutnya

	isi := append([]byte("Exif\x00\x00"), tiff.Bytes()...)
	panjang := len(isi) + 2

	out := new(bytes.Buffer)
	out.Write(asli[:2]) // SOI
	out.Write([]byte{0xFF, 0xE1})
	out.WriteByte(byte(panjang >> 8))
	out.WriteByte(byte(panjang & 0xFF))
	out.Write(isi)
	out.Write(asli[2:])
	return out.Bytes()
}

// pngPalsuBesar menyusun PNG dengan header berukuran raksasa. Isi datanya
// sengaja tidak sah — penolakan harus terjadi dari header, sebelum decode.
func pngPalsuBesar(w, h int) []byte {
	ihdr := new(bytes.Buffer)
	_ = binary.Write(ihdr, binary.BigEndian, uint32(w))
	_ = binary.Write(ihdr, binary.BigEndian, uint32(h))
	ihdr.Write([]byte{8, 2, 0, 0, 0}) // 8 bit, truecolor

	out := new(bytes.Buffer)
	out.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	out.Write(potonganPNG("IHDR", ihdr.Bytes()))
	out.Write(potonganPNG("IDAT", []byte{0x78, 0x9c, 0x03, 0x00, 0x00, 0x00, 0x00, 0x01}))
	out.Write(potonganPNG("IEND", nil))
	return out.Bytes()
}

func potonganPNG(jenis string, data []byte) []byte {
	out := new(bytes.Buffer)
	_ = binary.Write(out, binary.BigEndian, uint32(len(data)))
	out.WriteString(jenis)
	out.Write(data)
	_ = binary.Write(out, binary.BigEndian, crcPNG(append([]byte(jenis), data...)))
	return out.Bytes()
}

func crcPNG(b []byte) uint32 {
	var tabel [256]uint32
	for i := range tabel {
		c := uint32(i)
		for k := 0; k < 8; k++ {
			if c&1 != 0 {
				c = 0xedb88320 ^ (c >> 1)
			} else {
				c >>= 1
			}
		}
		tabel[i] = c
	}
	c := uint32(0xffffffff)
	for _, x := range b {
		c = tabel[(c^uint32(x))&0xff] ^ (c >> 8)
	}
	return c ^ 0xffffffff
}
