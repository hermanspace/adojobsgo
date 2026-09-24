package service

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	_ "image/png" // mendaftarkan dekoder PNG
	"io"
	"runtime"

	"github.com/gen2brain/webp"
	xdraw "golang.org/x/image/draw"
)

// ProfilGambar menentukan bagaimana satu jenis unggahan diproses.
type ProfilGambar struct {
	Subdir string
	// MaksSisi membatasi sisi terpanjang gambar. Foto ponsel lazimnya
	// 4000 px, jauh di atas yang pernah ditampilkan halaman mana pun.
	MaksSisi int
	Kualitas int
	// ThumbSisi 0 berarti jenis unggahan ini tidak memerlukan thumbnail.
	ThumbSisi     int
	ThumbKualitas int
}

// Profil unggahan yang dipakai aplikasi.
var (
	// Foto listing: satu ukuran tampilan untuk halaman detail, satu thumbnail
	// untuk kartu di halaman pencarian yang memuat belasan gambar sekaligus.
	ProfilListing = ProfilGambar{
		Subdir: "services", MaksSisi: 1600, Kualitas: 80,
		ThumbSisi: 480, ThumbKualitas: 75,
	}
	ProfilAvatar = ProfilGambar{Subdir: "avatars", MaksSisi: 400, Kualitas: 82}
	ProfilLogo   = ProfilGambar{Subdir: "situs", MaksSisi: 400, Kualitas: 90}
	// Hero dipakai selebar layar, jadi sisinya paling panjang di antara semua
	// profil. Kualitasnya ditekan lebih rendah karena gambarnya tertutup
	// overlay gelap — cacat kompresi tidak terlihat, ukurannya jauh lebih kecil.
	ProfilHero = ProfilGambar{Subdir: "situs", MaksSisi: 1920, Kualitas: 72}
	// Materi iklan biasanya berupa spanduk berisi teks kecil, jadi kualitasnya
	// dijaga lebih tinggi daripada hero: di sini tidak ada overlay yang
	// menyamarkan cacat kompresi.
	ProfilIklan = ProfilGambar{Subdir: "iklan", MaksSisi: 1200, Kualitas: 85}
	// Bukti transfer cukup terbaca oleh admin; tidak perlu tajam.
	ProfilBukti = ProfilGambar{Subdir: "bukti", MaksSisi: 1400, Kualitas: 75}
)

// maksPiksel membatasi jumlah piksel gambar yang boleh di-decode.
// Berkas terkompresi berukuran kecil bisa mengembang menjadi gambar berukuran
// raksasa saat di-decode (decompression bomb); batas ini diperiksa dari header
// gambar sebelum satu piksel pun dialokasikan. 50 MP setara 8660x5773,
// jauh di atas kamera ponsel mana pun.
const maksPiksel = 50_000_000

// gerbangProses membatasi jumlah gambar yang diproses bersamaan.
// Satu encode mengalokasikan puluhan megabita, sementara container produksi
// dibatasi 512 MB — tanpa pembatas ini, unggahan serentak bisa menghabiskannya.
var gerbangProses = make(chan struct{}, maksProsesSerentak())

func maksProsesSerentak() int {
	n := runtime.NumCPU()
	if n > 4 {
		n = 4
	}
	if n < 1 {
		n = 1
	}
	return n
}

// HasilGambar adalah gambar yang sudah diproses dan siap disimpan.
type HasilGambar struct {
	Utama  []byte
	Thumb  []byte
	Lebar  int
	Tinggi int
	// Ekstensi berkas hasil, mengikuti format yang benar-benar dipakai.
	Ekstensi string
}

// prosesGambar membaca berkas unggahan, memperkecilnya, dan menyandikannya
// ulang ke WebP.
//
// Penyandian ulang dari piksel membuat seluruh metadata bawaan ikut hilang,
// termasuk koordinat GPS yang lazim disematkan kamera ponsel — penting karena
// penyedia jasa memotret pekerjaan di rumah pelanggan.
func prosesGambar(r io.ReadSeeker, profil ProfilGambar) (*HasilGambar, error) {
	// Dimensi diperiksa dari header lebih dulu, sebelum gambar di-decode utuh.
	cfg, _, err := image.DecodeConfig(r)
	if err != nil {
		return nil, InvalidMsg("Berkas gambar tidak dapat dibaca.")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, InvalidMsg("Ukuran gambar tidak valid.")
	}
	if cfg.Width*cfg.Height > maksPiksel {
		return nil, InvalidMsg("Resolusi gambar terlalu besar. Perkecil dulu sebelum mengunggah.")
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, Internal(err)
	}

	// Orientasi EXIF dibaca sebelum decode, karena datanya hilang begitu
	// gambar disandikan ulang.
	orientasi := bacaOrientasiEXIF(r)
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, Internal(err)
	}

	gerbangProses <- struct{}{}
	defer func() { <-gerbangProses }()

	src, _, err := image.Decode(r)
	if err != nil {
		return nil, InvalidMsg("Berkas gambar rusak atau formatnya tidak didukung.")
	}
	src = terapkanOrientasi(src, orientasi)

	utama := perkecil(src, profil.MaksSisi)
	dataUtama, ekstensi, err := sandikan(utama, profil.Kualitas)
	if err != nil {
		return nil, Internal(err)
	}

	hasil := &HasilGambar{
		Utama:    dataUtama,
		Lebar:    utama.Bounds().Dx(),
		Tinggi:   utama.Bounds().Dy(),
		Ekstensi: ekstensi,
	}

	if profil.ThumbSisi > 0 {
		thumb := perkecil(utama, profil.ThumbSisi)
		dataThumb, _, err := sandikan(thumb, profil.ThumbKualitas)
		if err != nil {
			return nil, Internal(err)
		}
		hasil.Thumb = dataThumb
	}
	return hasil, nil
}

// sandikan menyandikan gambar ke WebP. Bila penyandian WebP gagal karena
// alasan apa pun, JPEG dipakai sebagai cadangan supaya unggahan tetap berhasil
// alih-alih menggagalkan pekerjaan pengguna.
func sandikan(img image.Image, kualitas int) ([]byte, string, error) {
	var buf bytes.Buffer
	if err := webp.Encode(&buf, img, webp.Options{Quality: kualitas}); err == nil {
		return buf.Bytes(), ".webp", nil
	}

	buf.Reset()
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: kualitas}); err != nil {
		return nil, "", fmt.Errorf("sandikan gambar: %w", err)
	}
	return buf.Bytes(), ".jpg", nil
}

// perkecil mengubah ukuran gambar agar sisi terpanjangnya tidak melebihi maks.
// Gambar yang sudah lebih kecil dibiarkan apa adanya — memperbesar hanya
// menambah ukuran berkas tanpa menambah detail.
func perkecil(src image.Image, maks int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if maks <= 0 || (w <= maks && h <= maks) {
		return src
	}

	var lebarBaru, tinggiBaru int
	if w >= h {
		lebarBaru = maks
		tinggiBaru = int(float64(h) * float64(maks) / float64(w))
	} else {
		tinggiBaru = maks
		lebarBaru = int(float64(w) * float64(maks) / float64(h))
	}
	if lebarBaru < 1 {
		lebarBaru = 1
	}
	if tinggiBaru < 1 {
		tinggiBaru = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, lebarBaru, tinggiBaru))
	// CatmullRom memberi hasil paling tajam untuk pengecilan foto.
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

// ---------- orientasi EXIF ----------

// Nilai tag Orientation pada EXIF (TIFF tag 0x0112).
const (
	orientasiNormal          = 1
	orientasiCerminH         = 2
	orientasiPutar180        = 3
	orientasiCerminV         = 4
	orientasiCerminHPutar270 = 5
	orientasiPutar90         = 6
	orientasiCerminHPutar90  = 7
	orientasiPutar270        = 8
)

// bacaOrientasiEXIF mengambil tag Orientation dari segmen APP1 sebuah JPEG.
//
// Hanya satu tag yang dibutuhkan, sehingga membaca sendiri lebih ringan
// daripada menarik pustaka EXIF utuh. Berkas yang tidak punya EXIF, atau yang
// strukturnya tidak dikenali, dianggap berorientasi normal.
func bacaOrientasiEXIF(r io.ReadSeeker) int {
	data := make([]byte, 64*1024) // orientasi selalu berada di awal berkas
	n, err := io.ReadFull(r, data)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return orientasiNormal
	}
	data = data[:n]

	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return orientasiNormal // bukan JPEG
	}

	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			return orientasiNormal
		}
		penanda := data[i+1]
		panjang := int(data[i+2])<<8 | int(data[i+3])
		if panjang < 2 || i+2+panjang > len(data) {
			return orientasiNormal
		}

		if penanda == 0xE1 { // APP1, tempat EXIF berada
			isi := data[i+4 : i+2+panjang]
			if len(isi) > 6 && string(isi[:6]) == "Exif\x00\x00" {
				return orientasiDariTIFF(isi[6:])
			}
		}
		if penanda == 0xDA { // mulai data gambar; EXIF tidak akan muncul lagi
			return orientasiNormal
		}
		i += 2 + panjang
	}
	return orientasiNormal
}

// orientasiDariTIFF menelusuri IFD0 sebuah blok TIFF untuk menemukan tag 0x0112.
func orientasiDariTIFF(t []byte) int {
	if len(t) < 8 {
		return orientasiNormal
	}

	var besarDulu bool
	switch {
	case t[0] == 'M' && t[1] == 'M':
		besarDulu = true
	case t[0] == 'I' && t[1] == 'I':
		besarDulu = false
	default:
		return orientasiNormal
	}

	u16 := func(b []byte) int {
		if besarDulu {
			return int(b[0])<<8 | int(b[1])
		}
		return int(b[1])<<8 | int(b[0])
	}
	u32 := func(b []byte) int {
		if besarDulu {
			return int(b[0])<<24 | int(b[1])<<16 | int(b[2])<<8 | int(b[3])
		}
		return int(b[3])<<24 | int(b[2])<<16 | int(b[1])<<8 | int(b[0])
	}

	offsetIFD := u32(t[4:8])
	if offsetIFD < 8 || offsetIFD+2 > len(t) {
		return orientasiNormal
	}

	jumlah := u16(t[offsetIFD : offsetIFD+2])
	awal := offsetIFD + 2
	for k := 0; k < jumlah; k++ {
		p := awal + k*12
		if p+12 > len(t) {
			return orientasiNormal
		}
		if u16(t[p:p+2]) != 0x0112 {
			continue
		}
		nilai := u16(t[p+8 : p+10])
		if nilai >= orientasiNormal && nilai <= orientasiPutar270 {
			return nilai
		}
		return orientasiNormal
	}
	return orientasiNormal
}

// terapkanOrientasi memutar dan mencerminkan gambar sesuai tag EXIF, sehingga
// foto potret dari ponsel tidak tampil miring setelah metadatanya dibuang.
func terapkanOrientasi(src image.Image, orientasi int) image.Image {
	if orientasi <= orientasiNormal || orientasi > orientasiPutar270 {
		return src
	}

	b := src.Bounds()
	w, h := b.Dx(), b.Dy()

	// Orientasi 5–8 menukar sumbu, sehingga lebar dan tinggi bertukar.
	putar := orientasi >= orientasiCerminHPutar270
	lebarBaru, tinggiBaru := w, h
	if putar {
		lebarBaru, tinggiBaru = h, w
	}

	dst := image.NewRGBA(image.Rect(0, 0, lebarBaru, tinggiBaru))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var nx, ny int
			switch orientasi {
			case orientasiCerminH:
				nx, ny = w-1-x, y
			case orientasiPutar180:
				nx, ny = w-1-x, h-1-y
			case orientasiCerminV:
				nx, ny = x, h-1-y
			case orientasiCerminHPutar270:
				nx, ny = y, x
			case orientasiPutar90:
				nx, ny = h-1-y, x
			case orientasiCerminHPutar90:
				nx, ny = h-1-y, w-1-x
			case orientasiPutar270:
				nx, ny = y, w-1-x
			default:
				nx, ny = x, y
			}
			dst.Set(nx, ny, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}
