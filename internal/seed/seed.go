// Package seed mengisi data awal untuk pengembangan: kategori jasa dan
// beberapa penyedia contoh. Aman dijalankan berulang kali.
package seed

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/repository"
	"github.com/hermansyah/adojobsid/internal/validator"
)

// slug memakai fungsi yang sama dengan validator agar slug kategori hasil
// seeding identik dengan slug yang dihasilkan di tempat lain.
func slug(s string) string { return validator.Slugify(s) }

// kategoriBenih adalah kategori jasa yang benar-benar lazim ditawarkan
// di Kabupaten Bengkalis, bukan kategori contoh generik.
type kategoriBenih struct {
	Nama string
	Ikon string
	Anak []string
}

var daftarKategori = []kategoriBenih{
	{Nama: "Servis & Elektronik", Ikon: "petir", Anak: []string{
		"Servis AC", "Servis Kulkas & Mesin Cuci", "Servis TV & Elektronik", "Instalasi Listrik",
	}},
	{Nama: "Bangunan & Renovasi", Ikon: "rumah-bangun", Anak: []string{
		"Tukang Bangunan", "Tukang Kayu & Mebel", "Cat & Plafon", "Atap & Bocor", "Las & Pagar",
	}},
	{Nama: "Air & Sanitasi", Ikon: "kunci-inggris", Anak: []string{
		"Tukang Ledeng", "Sedot WC", "Bor Sumur",
	}},
	{Nama: "Kebersihan", Ikon: "sapu", Anak: []string{
		"Bersih Rumah", "Cuci Sofa & Kasur", "Bersih Pasca Renovasi", "Taman & Rumput",
	}},
	{Nama: "Acara & Dekorasi", Ikon: "pesta", Anak: []string{
		"Dekorasi Pernikahan", "Tenda & Kursi", "Katering", "MC & Hiburan",
	}},
	{Nama: "Dokumentasi", Ikon: "kamera", Anak: []string{
		"Foto Pernikahan", "Video Acara", "Foto Produk", "Drone",
	}},
	{Nama: "Kendaraan", Ikon: "truk", Anak: []string{
		"Servis Motor", "Servis Mobil", "Cuci Kendaraan", "Angkutan & Pindahan",
	}},
	{Nama: "Digital & Usaha", Ikon: "laptop", Anak: []string{
		"Servis Komputer & Laptop", "Desain Grafis", "Pembuatan Website", "Cetak & Sablon",
	}},
	{Nama: "Perawatan Diri", Ikon: "gunting", Anak: []string{
		"Potong Rambut", "Rias Pengantin", "Pijat & Terapi",
	}},
	{Nama: "Pendidikan", Ikon: "buku", Anak: []string{
		"Les Privat", "Mengaji", "Kursus Komputer",
	}},
}

// Dasar mengisi data yang dibutuhkan setiap instalasi — kategori jasa dan
// paket promosi — tanpa satu pun data contoh. Ini yang dijalankan di
// produksi; Upsert membuatnya aman diulang.
func Dasar(ctx context.Context, repos *repository.Repositories) error {
	if err := periksaSlugKategori(); err != nil {
		return err
	}
	if err := seedKategori(ctx, repos); err != nil {
		return fmt.Errorf("seed kategori: %w", err)
	}
	if err := seedPaket(ctx, repos); err != nil {
		return fmt.Errorf("seed paket promosi: %w", err)
	}
	slog.Info("seed dasar selesai")
	return nil
}

// Run menjalankan seluruh proses seeding untuk pengembangan: data dasar
// ditambah penyedia dan jasa contoh.
func Run(ctx context.Context, repos *repository.Repositories) error {
	if err := periksaSlugKategori(); err != nil {
		return err
	}
	if err := periksaSlugPenyedia(); err != nil {
		return err
	}
	if err := seedKategori(ctx, repos); err != nil {
		return fmt.Errorf("seed kategori: %w", err)
	}
	if err := seedProvider(ctx, repos); err != nil {
		return fmt.Errorf("seed provider: %w", err)
	}
	if err := seedPaket(ctx, repos); err != nil {
		return fmt.Errorf("seed paket promosi: %w", err)
	}
	slog.Info("seeding selesai")
	return nil
}

// periksaSlugKategori memastikan tidak ada dua kategori yang menghasilkan slug
// sama. Slug adalah kunci unik, sehingga tabrakan akan membuat satu baris
// menimpa baris lain lewat ON CONFLICT — dan bila yang tertimpa adalah induk
// dari kategori penimpanya, hasilnya adalah baris yang menjadi induk bagi
// dirinya sendiri. Lebih baik seeding gagal terang-terangan di sini.
func periksaSlugKategori() error {
	asal := map[string]string{}
	var tabrakan []string

	catat := func(nama string) {
		s := slug(nama)
		if sebelumnya, ada := asal[s]; ada {
			tabrakan = append(tabrakan, fmt.Sprintf("%q dan %q sama-sama menjadi %q", sebelumnya, nama, s))
			return
		}
		asal[s] = nama
	}

	for _, induk := range daftarKategori {
		catat(induk.Nama)
		for _, anak := range induk.Anak {
			catat(anak)
		}
	}
	if len(tabrakan) > 0 {
		return fmt.Errorf("slug kategori bertabrakan: %s", strings.Join(tabrakan, "; "))
	}
	return nil
}

// periksaSlugPenyedia memastikan tidak ada dua penyedia contoh yang
// menghasilkan slug sama. Slug penyedia adalah kunci unik sekaligus alamat
// publiknya, jadi tabrakan akan menggagalkan seeding di tengah jalan dengan
// pesan yang tidak menjelaskan apa-apa.
func periksaSlugPenyedia() error {
	asal := map[string]string{}
	var tabrakan []string

	for _, p := range daftarProvider {
		s := slug(p.Nama)
		if s == "" {
			tabrakan = append(tabrakan, fmt.Sprintf("%q menghasilkan slug kosong", p.Nama))
			continue
		}
		if sebelumnya, ada := asal[s]; ada {
			tabrakan = append(tabrakan,
				fmt.Sprintf("%q dan %q sama-sama menjadi %q", sebelumnya, p.Nama, s))
			continue
		}
		asal[s] = p.Nama
	}
	if len(tabrakan) > 0 {
		return fmt.Errorf("slug penyedia bertabrakan: %s", strings.Join(tabrakan, "; "))
	}
	return nil
}

func seedKategori(ctx context.Context, repos *repository.Repositories) error {
	total := 0
	for _, induk := range daftarKategori {
		ikon := induk.Ikon
		parent := &model.Category{
			Name: induk.Nama,
			Slug: slug(induk.Nama),
			Icon: &ikon,
		}
		if err := repos.Category.Upsert(ctx, parent); err != nil {
			return err
		}
		total++

		for _, namaAnak := range induk.Anak {
			anak := &model.Category{
				Name:     namaAnak,
				Slug:     slug(namaAnak),
				ParentID: &parent.ID,
				Icon:     &ikon,
			}
			if err := repos.Category.Upsert(ctx, anak); err != nil {
				return err
			}
			total++
		}
	}
	slog.Info("kategori tersimpan", "jumlah", total)
	return nil
}

// providerBenih adalah penyedia contoh untuk pengembangan.
type providerBenih struct {
	Nama      string
	Phone     string
	Kecamatan string
	Bio       string
	// Titik lokasi dan radius layanan, supaya pencarian berbasis jarak
	// langsung bisa dicoba setelah seeding.
	Lat      float64
	Lng      float64
	RadiusKm int
	Alamat   string
	Jasa     []jasaBenih
}

type jasaBenih struct {
	Judul      string
	Kategori   string // slug kategori
	Deskripsi  string
	JenisHarga model.PriceType
	HargaMin   float64
	HargaMax   float64
}

var daftarProvider = []providerBenih{
	{
		Nama: "Rizal Teknik AC", Phone: "628117512001", Kecamatan: "Bengkalis",
		Bio: "Teknisi pendingin ruangan sejak 2014. Menangani cuci AC, isi freon, perbaikan tidak dingin, sampai bongkar-pasang unit untuk rumah, ruko, dan kantor di Pulau Bengkalis.",
		Lat: 1.4680, Lng: 102.1020, RadiusKm: 20, Alamat: "Jl. Hangtuah, Bengkalis",
		Jasa: []jasaBenih{
			{
				Judul: "Cuci AC split rumah 0,5–2 PK", Kategori: "servis-ac",
				Deskripsi:  "Pembersihan unit indoor dan outdoor memakai mesin steam, cek tekanan freon, dan pengecekan kebocoran. Pengerjaan sekitar 45 menit per unit. Biaya freon dihitung terpisah bila perlu ditambah.",
				JenisHarga: model.PriceFixed, HargaMin: 85000, HargaMax: 120000,
			},
			{
				Judul: "Bongkar pasang AC pindah rumah", Kategori: "servis-ac",
				Deskripsi:  "Pembongkaran unit di lokasi lama, pengamanan freon, pemasangan kembali di lokasi baru termasuk braket. Pipa tambahan dihitung per meter. Bergaransi pemasangan 30 hari.",
				JenisHarga: model.PriceFixed, HargaMin: 350000, HargaMax: 550000,
			},
		},
	},
	{
		Nama: "CV Amanah Karya", Phone: "628117512002", Kecamatan: "Bantan",
		Bio: "Tim tukang bangunan berpengalaman 10 tahun untuk renovasi rumah, pembuatan dapur, dan perbaikan atap bocor. Bekerja harian maupun borongan dengan rincian material terbuka.",
		Lat: 1.5180, Lng: 102.2340, RadiusKm: 30, Alamat: "Selatbaru, Bantan",
		Jasa: []jasaBenih{
			{
				Judul: "Perbaikan atap bocor dan ganti seng", Kategori: "atap-bocor",
				Deskripsi:  "Pemeriksaan titik bocor, penggantian seng atau genteng yang rusak, penambalan nok, dan perbaikan talang. Survei lokasi gratis untuk area Bantan dan Bengkalis. Material bisa disediakan pemilik rumah.",
				JenisHarga: model.PriceNegotiable,
			},
			{
				Judul: "Tukang harian renovasi rumah", Kategori: "tukang-bangunan",
				Deskripsi:  "Tenaga tukang berpengalaman untuk pekerjaan renovasi: pasang keramik, plester dinding, pembuatan dapur, dan pekerjaan sipil ringan lainnya. Minimal tiga hari kerja.",
				JenisHarga: model.PriceHourly, HargaMin: 25000,
			},
		},
	},
	{
		Nama: "Dapur Melayu Bengkalis", Phone: "628117512003", Kecamatan: "Bengkalis",
		Bio: "Melayani katering acara keluarga, syukuran, dan rapat kantor dengan menu Melayu Bengkalis. Sudah biasa menangani pesanan 50 sampai 500 porsi dengan pemberitahuan tiga hari sebelumnya.",
		Lat: 1.4650, Lng: 102.0980, RadiusKm: 15, Alamat: "Pasar Bengkalis",
		Jasa: []jasaBenih{
			{
				Judul: "Katering nasi kotak acara kantor", Kategori: "katering",
				Deskripsi:  "Nasi kotak lengkap dengan lauk, sayur, buah, dan air mineral. Menu bisa dipilih dari daftar rotasi mingguan. Minimal pesanan 30 kotak, pengantaran gratis dalam kota Bengkalis.",
				JenisHarga: model.PriceFixed, HargaMin: 28000, HargaMax: 45000,
			},
		},
	},
	{
		Nama: "Wan Dokumentasi", Phone: "628117512004", Kecamatan: "Bengkalis",
		Bio: "Fotografer dan videografer acara di Bengkalis sejak 2017. Fokus pada dokumentasi pernikahan adat Melayu, khitanan, dan acara resmi instansi.",
		Lat: 1.4700, Lng: 102.1100, RadiusKm: 40, Alamat: "Jl. Pramuka, Bengkalis",
		Jasa: []jasaBenih{
			{
				Judul: "Dokumentasi foto akad dan resepsi", Kategori: "foto-pernikahan",
				Deskripsi:  "Dua fotografer, liputan akad sampai resepsi, seluruh file hasil seleksi diserahkan dalam flashdisk, plus 40 foto cetak album. Penambahan jam liputan bisa dinegosiasikan.",
				JenisHarga: model.PriceFixed, HargaMin: 2500000, HargaMax: 4500000,
			},
			{
				Judul: "Video dokumentasi acara instansi", Kategori: "video-acara",
				Deskripsi:  "Peliputan kegiatan rapat, pelantikan, atau seremoni instansi. Hasil berupa video ringkasan 3–5 menit plus rekaman penuh. Termasuk pengambilan gambar dari drone bila lokasi memungkinkan.",
				JenisHarga: model.PriceNegotiable,
			},
		},
	},
	{
		Nama: "Bersih Kilat Bengkalis", Phone: "628117512005", Kecamatan: "Bengkalis",
		Bio: "Jasa kebersihan rumah dan kantor. Tim beranggotakan empat orang, membawa peralatan dan bahan pembersih sendiri. Melayani pembersihan rutin maupun pasca renovasi.",
		Lat: 1.4620, Lng: 102.0950, RadiusKm: 25, Alamat: "Kelapapati, Bengkalis",
		Jasa: []jasaBenih{
			{
				Judul: "Bersih rumah menyeluruh 2 kamar", Kategori: "bersih-rumah",
				Deskripsi:  "Menyapu, mengepel, membersihkan kamar mandi, dapur, kaca jendela, dan merapikan ruangan. Perkiraan tiga jam untuk rumah dua kamar. Bahan pembersih disediakan tim.",
				JenisHarga: model.PriceFixed, HargaMin: 180000, HargaMax: 250000,
			},
			{
				Judul: "Cuci sofa dan kasur di tempat", Kategori: "cuci-sofa-kasur",
				Deskripsi:  "Pencucian memakai mesin vakum injeksi, menghilangkan debu, tungau, dan noda ringan. Dikerjakan di rumah pelanggan, kering sekitar empat jam. Harga per unit.",
				JenisHarga: model.PriceFixed, HargaMin: 150000, HargaMax: 300000,
			},
		},
	},
	{
		Nama: "Hafiz Servis Motor", Phone: "628117512006", Kecamatan: "Bukit Batu",
		Bio: "Bengkel motor panggilan untuk servis ringan, ganti oli, dan perbaikan mogok di jalan. Siap datang ke lokasi di wilayah Bukit Batu dan Siak Kecil.",
		Lat: 1.3170, Lng: 102.1170, RadiusKm: 35, Alamat: "Sungai Pakning, Bukit Batu",
		Jasa: []jasaBenih{
			{
				Judul: "Servis motor panggilan ke rumah", Kategori: "servis-motor",
				Deskripsi:  "Servis ringan meliputi ganti oli, setel rantai, cek rem, bersih karburator atau throttle body, dan cek kelistrikan. Suku cadang dihitung terpisah sesuai kebutuhan.",
				JenisHarga: model.PriceFixed, HargaMin: 60000, HargaMax: 150000,
			},
		},
	},
}

// katasandiBenih dipakai untuk seluruh akun contoh — hanya untuk pengembangan.
const katasandiBenih = "rahasia123"

func seedProvider(ctx context.Context, repos *repository.Repositories) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(katasandiBenih), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return seedProviderDenganHash(ctx, repos, string(hash), katasandiBenih)
}

// seedProviderDenganHash menanam penyedia contoh dengan hash kata sandi yang
// ditentukan pemanggil; label hanya untuk log.
func seedProviderDenganHash(ctx context.Context, repos *repository.Repositories, hash string, label string) error {
	dibuat, dilewati := 0, 0
	for _, p := range daftarProvider {
		// Seeder aman diulang: akun yang sudah ada dilewati.
		if _, err := repos.User.GetByPhone(ctx, p.Phone); err == nil {
			dilewati++
			continue
		} else if !errors.Is(err, repository.ErrNotFound) {
			return err
		}

		kecamatan := p.Kecamatan
		kota := "Bengkalis"
		user := &model.User{
			FullName:     p.Nama,
			Phone:        p.Phone,
			PasswordHash: hash,
			City:         &kota,
			Kecamatan:    &kecamatan,
			IsProvider:   true,
		}
		if err := repos.User.Create(ctx, user); err != nil {
			return err
		}

		bio := p.Bio
		profile := &model.ProviderProfile{
			UserID: user.ID,
			// Slug wajib diisi: kolomnya NOT NULL dan unik. Seeder memanggil
			// repository langsung, jadi tidak melewati pembuatan slug di
			// service layer dan harus menyusunnya sendiri.
			Slug:           slug(p.Nama),
			Bio:            &bio,
			WhatsappNumber: p.Phone,
		}
		if err := repos.Provider.Create(ctx, profile); err != nil {
			return err
		}
		// Titik lokasi disetel terpisah karena Create hanya menangani
		// kolom profil dasar.
		lat, lng, alamat := p.Lat, p.Lng, p.Alamat
		if err := repos.Provider.SetLocation(ctx, profile.ID, &lat, &lng, &alamat, p.RadiusKm); err != nil {
			return err
		}

		for _, j := range p.Jasa {
			kategori, err := repos.Category.GetBySlug(ctx, j.Kategori)
			if err != nil {
				return fmt.Errorf("kategori %q tidak ditemukan: %w", j.Kategori, err)
			}

			svc := &model.Service{
				ProviderID:  profile.ID,
				CategoryID:  kategori.ID,
				Title:       j.Judul,
				Description: j.Deskripsi,
				PriceType:   j.JenisHarga,
				Status:      model.ServiceActive,
			}
			if j.JenisHarga != model.PriceNegotiable {
				min := j.HargaMin
				svc.PriceMin = &min
				if j.HargaMax > 0 {
					max := j.HargaMax
					svc.PriceMax = &max
				}
			}
			if err := repos.Service.Create(ctx, svc); err != nil {
				return err
			}
		}
		dibuat++
	}

	slog.Info("provider contoh tersimpan", "dibuat", dibuat, "dilewati", dilewati,
		"katasandi", label)
	return nil
}

// seedPaket menanam paket promosi awal. Harganya nol: selama sakelar "uji
// coba gratis" menyala harga memang tidak berlaku, dan admin mengisinya lewat
// panel saat siap. Upsert berdasarkan nama per jenis, sehingga aman dijalankan
// berulang tanpa menimpa status aktif yang disetel admin.
func seedPaket(ctx context.Context, repos *repository.Repositories) error {
	semuaSlot := make([]string, 0, 4)
	for _, s := range model.SlotIklanBawaan() {
		semuaSlot = append(semuaSlot, s.Kunci)
	}
	paket := []model.PaketPromosi{
		{Jenis: model.PromosiSorotan, Nama: "7 hari", DurasiHari: 7, Bobot: 1, Urutan: 1,
			Deskripsi: "Listing naik ke atas hasil pencarian dan kartunya ditandai."},
		{Jenis: model.PromosiSorotan, Nama: "14 hari", DurasiHari: 14, Bobot: 1, Urutan: 2},
		{Jenis: model.PromosiSorotan, Nama: "30 hari", DurasiHari: 30, Bobot: 1, Urutan: 3},
		{Jenis: model.PromosiPenyedia, Nama: "30 hari", DurasiHari: 30, Bobot: 1, Urutan: 1,
			Deskripsi: "Tampil di blok Penyedia pilihan beranda dan kartunya ditandai."},
		{Jenis: model.PromosiPenyedia, Nama: "90 hari", DurasiHari: 90, Bobot: 1, Urutan: 2},
		{Jenis: model.PromosiIklan, Nama: "Dasar 7 hari", DurasiHari: 7, Bobot: 1, Urutan: 1,
			SlotIklan: []string{"cari_atas", "detail_samping"},
			Deskripsi: "Tayang di halaman pencarian dan detail jasa."},
		{Jenis: model.PromosiIklan, Nama: "Dasar 14 hari", DurasiHari: 14, Bobot: 1, Urutan: 2,
			SlotIklan: []string{"cari_atas", "detail_samping"}},
		{Jenis: model.PromosiIklan, Nama: "Utama 14 hari", DurasiHari: 14, Bobot: 3, Urutan: 3,
			SlotIklan: semuaSlot, Deskripsi: "Semua slot termasuk beranda atas, tiga kali lebih sering terpilih."},
		{Jenis: model.PromosiIklan, Nama: "Utama 30 hari", DurasiHari: 30, Bobot: 3, Urutan: 4, SlotIklan: semuaSlot},
	}
	for i := range paket {
		paket[i].Aktif = true
		if paket[i].SlotIklan == nil {
			paket[i].SlotIklan = []string{}
		}
		if err := repos.Paket.Upsert(ctx, &paket[i]); err != nil {
			return fmt.Errorf("%s / %s: %w", paket[i].Jenis, paket[i].Nama, err)
		}
	}
	slog.Info("paket promosi tersimpan", "jumlah", len(paket))
	return nil
}
