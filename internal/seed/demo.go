package seed

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/repository"
	"github.com/hermansyah/adojobsid/internal/service"
)

// Demo mengisi konten peragaan yang utuh untuk produksi: enam penyedia
// contoh (dari daftarProvider) lengkap dengan foto jasa, avatar, portofolio,
// dan lencana terverifikasi; empat pencari jasa; pesanan dengan obrolan dan
// ulasan sehingga rating terhitung; serta promosi yang sedang tayang (iklan,
// sorotan jasa, penyedia pilihan) plus satu pengajuan menunggu untuk
// antrean admin.
//
// Kata sandi seluruh akun demo diambil dari DEMO_PASSWORD bila diisi, atau
// dibuat acak dan dicetak sekali di log. Seeder aman diulang: bila akun
// pencari pertama sudah ada, seluruh proses dilewati.
func Demo(ctx context.Context, repos *repository.Repositories, upload *service.UploadService) error {
	if sudahAda, err := demoSudahAda(ctx, repos); err != nil {
		return err
	} else if sudahAda {
		slog.Info("akun & konten demo sudah ada; mengamankan nomor dan melengkapi riwayat kunjungan bila kosong")
		if err := amankanNomorDemo(ctx, repos); err != nil {
			return fmt.Errorf("amankan nomor demo: %w", err)
		}
		return buatKunjungan(ctx, repos)
	}

	sandi := os.Getenv("DEMO_PASSWORD")
	if sandi == "" {
		b := make([]byte, 9)
		if _, err := rand.Read(b); err != nil {
			return err
		}
		sandi = "demo-" + base64.RawURLEncoding.EncodeToString(b)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(sandi), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	if err := Dasar(ctx, repos); err != nil {
		return err
	}
	if err := seedProviderDenganHash(ctx, repos, string(hash), "(lihat akhir log)"); err != nil {
		return err
	}
	penyedia, err := lengkapiPenyedia(ctx, repos, upload)
	if err != nil {
		return fmt.Errorf("lengkapi penyedia: %w", err)
	}
	pencari, err := buatPencari(ctx, repos, upload, string(hash))
	if err != nil {
		return fmt.Errorf("pencari: %w", err)
	}
	if err := buatPesanan(ctx, repos, penyedia, pencari); err != nil {
		return fmt.Errorf("pesanan: %w", err)
	}
	if err := buatPromosi(ctx, repos, upload, penyedia); err != nil {
		return fmt.Errorf("promosi: %w", err)
	}
	if err := buatKunjungan(ctx, repos); err != nil {
		return fmt.Errorf("kunjungan: %w", err)
	}
	if err := amankanNomorDemo(ctx, repos); err != nil {
		return fmt.Errorf("amankan nomor demo: %w", err)
	}
	slog.Info("konten demo selesai", "penyedia", len(penyedia), "pencari", len(pencari), "katasandi_semua_akun_demo", sandi, "masuk_dengan", "email <slug-nama>@demo.adojobs.id")
	return nil
}

// penyediaDemo adalah penyedia contoh beserta jasa-jasanya setelah tersimpan.
type penyediaDemo struct {
	User     *model.User
	Profil   *model.ProviderProfile
	Jasa     []model.ServiceCard
	Terverif bool
}

var pencariDemo = []struct {
	Nama, Phone, Kecamatan string
}{
	{"Rina Marlina", "628117512011", "Bengkalis"},
	{"Dedi Kurniawan", "628117512012", "Bantan"},
	{"Sari Yulianti", "628117512013", "Bengkalis"},
	{"Andi Saputra", "628117512014", "Bukit Batu"},
}

// lengkapiPenyedia menambahkan avatar, foto jasa, portofolio, dan lencana
// terverifikasi pada penyedia contoh.
func lengkapiPenyedia(ctx context.Context, repos *repository.Repositories, upload *service.UploadService) ([]penyediaDemo, error) {
	var out []penyediaDemo
	for i, p := range daftarProvider {
		user, err := repos.User.GetByPhone(ctx, p.Phone)
		if err != nil {
			return nil, err
		}
		profil, err := repos.Provider.GetByUserID(ctx, user.ID)
		if err != nil {
			return nil, err
		}
		if user.AvatarURL == nil {
			g, err := upload.SaveImageBytes(gambarDemo(400, 400, 100+i), service.ProfilAvatar)
			if err != nil {
				return nil, err
			}
			user.AvatarURL = &g.URL
			if err := repos.User.Update(ctx, user); err != nil {
				return nil, err
			}
		}
		jasa, err := repos.Service.ListByProvider(ctx, profil.ID, true)
		if err != nil {
			return nil, err
		}
		for j, s := range jasa {
			if n, _ := repos.Service.CountImages(ctx, s.ID); n > 0 {
				continue
			}
			for k := 0; k < 2; k++ {
				g, err := upload.SaveImageBytes(gambarDemo(1200, 900, i*10+j*3+k), service.ProfilListing)
				if err != nil {
					return nil, err
				}
				im := &model.ServiceImage{ServiceID: s.ID, ImageURL: g.URL, SortOrder: k,
					Width: &g.Lebar, Height: &g.Tinggi, Bytes: &g.Bytes}
				if g.ThumbURL != "" {
					im.ThumbURL = &g.ThumbURL
				}
				if err := repos.Service.AddImage(ctx, im); err != nil {
					return nil, err
				}
			}
		}
		terverif := i%2 == 0 // tiga dari enam
		if terverif {
			if err := repos.Provider.SetVerified(ctx, profil.ID, true); err != nil {
				return nil, err
			}
		}
		if i < 4 {
			judul := []string{"Pekerjaan di Jl. Ahmad Yani", "Pesanan rumah dua lantai", "Proyek kantor kecamatan"}
			for k, t := range judul {
				g, err := upload.SaveImageBytes(gambarDemo(1200, 800, 200+i*7+k), service.ProfilPortofolio)
				if err != nil {
					return nil, err
				}
				selesai := time.Now().AddDate(0, -k-1, -i*3)
				pf := &model.Portfolio{ProviderID: profil.ID, Title: t, ImageURL: g.URL,
					Width: &g.Lebar, Height: &g.Tinggi, Bytes: &g.Bytes, CompletedAt: &selesai}
				if g.ThumbURL != "" {
					pf.ThumbURL = &g.ThumbURL
				}
				if err := repos.Portfolio.Create(ctx, pf); err != nil {
					return nil, err
				}
			}
		}
		out = append(out, penyediaDemo{User: user, Profil: profil, Jasa: jasa, Terverif: terverif})
	}
	return out, nil
}

func buatPencari(ctx context.Context, repos *repository.Repositories, upload *service.UploadService, hash string) ([]*model.User, error) {
	var out []*model.User
	kota := "Bengkalis"
	for i, p := range pencariDemo {
		kec := p.Kecamatan
		u := &model.User{FullName: p.Nama, Phone: p.Phone, PasswordHash: hash, City: &kota, Kecamatan: &kec}
		if err := repos.User.Create(ctx, u); err != nil {
			return nil, err
		}
		if i < 2 {
			g, err := upload.SaveImageBytes(gambarDemo(400, 400, 300+i), service.ProfilAvatar)
			if err != nil {
				return nil, err
			}
			u.AvatarURL = &g.URL
			if err := repos.User.Update(ctx, u); err != nil {
				return nil, err
			}
		}
		out = append(out, u)
	}
	return out, nil
}

// pesananDemo menggambarkan satu alur: obrolan, pesanan, dan (bila selesai)
// ulasan. Indeks menunjuk ke urutan penyedia contoh dan jasanya.
type pesananDemo struct {
	Pencari, Penyedia, Jasa int
	Status                  model.OrderStatus
	Catatan                 string
	Pesan                   []string // bergantian: pencari, penyedia, pencari, …
	Rating                  int
	Ulasan                  string
}

var daftarPesanan = []pesananDemo{
	{0, 0, 0, model.OrderCompleted, "Cuci 2 unit AC 1 PK di rumah, bisa Sabtu pagi?",
		[]string{"Selamat pagi, AC saya 2 unit, sudah setahun tidak dicuci. Bisa Sabtu pagi?", "Bisa, Bu. Sabtu jam 9 kami datang. Untuk 2 unit sekitar 1,5 jam.", "Oke, ditunggu ya."},
		5, "Datang tepat waktu, kerjanya rapi, AC jadi dingin lagi. Harga sesuai yang tertulis."},
	{1, 1, 0, model.OrderCompleted, "Atap dapur bocor di dua titik, tolong survei dulu.",
		[]string{"Pak, atap dapur bocor kalau hujan deras. Bisa disurvei dulu?", "Siap, besok sore saya cek lokasi dulu, gratis survei."},
		5, "Survei cepat, dijelaskan penyebabnya, dikerjakan sehari selesai. Sudah dua kali hujan besar tidak bocor lagi."},
	{2, 2, 0, model.OrderCompleted, "Nasi kotak 40 porsi untuk rapat kantor Kamis siang.",
		[]string{"Halo, butuh 40 nasi kotak untuk Kamis jam 12. Menu ayam bakar bisa?", "Bisa, Bu. Ayam bakar + sayur + sambal + buah. Kami antar jam 11.30."},
		4, "Rasanya enak dan porsinya pas. Pengantaran sedikit terlambat 15 menit, selebihnya bagus."},
	{3, 3, 0, model.OrderCompleted, "Dokumentasi akad nikah di rumah, tanggal 12 bulan depan.",
		[]string{"Assalamualaikum, mau tanya paket foto akad di rumah di Sungai Pakning.", "Waalaikumsalam. Untuk akad di rumah kami hitung paket setengah hari, hasil 150 foto edit + album digital."},
		5, "Fotonya bagus-bagus, momen keluarga banyak yang tertangkap. Hasil dikirim seminggu setelah acara."},
	{0, 4, 1, model.OrderCompleted, "Cuci sofa L dan 2 kasur, ada noda kopi.",
		[]string{"Sofa L sama 2 kasur, ada bekas kopi di sofa. Bisa hilang?", "Noda kopi biasanya bisa, Bu. Kami bawa mesin injeksi. Kering 4 jam."},
		4, "Sofa bersih dan wangi, noda kopi hilang. Kasur agak lama keringnya tapi hasilnya memuaskan."},
	{1, 5, 0, model.OrderAccepted, "Motor mogok di Sungai Pakning, minta datang ke lokasi.",
		[]string{"Bang, motor mogok di dekat pelabuhan Sungai Pakning. Bisa datang?", "Bisa, 20 menit saya sampai. Tolong kirim titik lokasinya."}, 0, ""},
	{2, 0, 1, model.OrderPending, "Pindah rumah minggu depan, AC 1 unit perlu dibongkar pasang.",
		[]string{"Mau bongkar pasang AC 1 unit untuk pindah rumah minggu depan, masih di Bengkalis kota."}, 0, ""},
	{3, 1, 1, model.OrderCompleted, "Butuh tukang harian 3 hari untuk renovasi kamar mandi.",
		[]string{"Pak, perlu tukang harian 3 hari untuk kamar mandi. Material sudah ada.", "Baik, 2 orang tukang, mulai Senin. Upah per hari sesuai yang tertera."},
		5, "Tukangnya rapi dan jujur, selesai sesuai jadwal."},
}

func buatPesanan(ctx context.Context, repos *repository.Repositories, penyedia []penyediaDemo, pencari []*model.User) error {
	for _, d := range daftarPesanan {
		if d.Penyedia >= len(penyedia) || d.Jasa >= len(penyedia[d.Penyedia].Jasa) {
			continue
		}
		p := penyedia[d.Penyedia]
		jasa := p.Jasa[d.Jasa]
		seeker := pencari[d.Pencari]

		conv, err := repos.Chat.EnsureConversation(ctx, jasa.ID, seeker.ID, p.Profil.ID)
		if err != nil {
			return err
		}
		for i, isi := range d.Pesan {
			pengirim := seeker.ID
			if i%2 == 1 {
				pengirim = p.User.ID
			}
			if err := repos.Chat.AddMessage(ctx, &model.Message{ConversationID: conv.ID, SenderID: pengirim, Body: isi}); err != nil {
				return err
			}
		}
		catatan := d.Catatan
		jadwal := time.Now().AddDate(0, 0, 3)
		order := &model.Order{SeekerID: seeker.ID, ProviderID: p.Profil.ID, ServiceID: jasa.ID,
			Status: d.Status, Notes: &catatan, ScheduledDate: &jadwal}
		if err := repos.Order.Create(ctx, order); err != nil {
			return err
		}
		if err := repos.Chat.AttachOrder(ctx, conv.ID, order.ID); err != nil {
			return err
		}
		if d.Status == model.OrderCompleted && d.Rating > 0 {
			ulasan := d.Ulasan
			if err := repos.Review.Create(ctx, &model.Review{OrderID: order.ID, Rating: d.Rating, Comment: &ulasan}); err != nil {
				return err
			}
			if err := repos.Provider.RecalculateRating(ctx, p.Profil.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// buatPromosi menanam promosi yang sedang tayang — trigger database yang
// menurunkan featured_until dari promosi aktif ikut bekerja — dan satu
// pengajuan menunggu untuk memperlihatkan antrean admin.
func buatPromosi(ctx context.Context, repos *repository.Repositories, upload *service.UploadService, penyedia []penyediaDemo) error {
	paket, err := repos.Paket.List(ctx, true)
	if err != nil {
		return err
	}
	cari := func(jenis, nama string) *int64 {
		for _, p := range paket {
			if p.Jenis == jenis && p.Nama == nama {
				id := p.ID
				return &id
			}
		}
		return nil
	}
	kini := time.Now()
	mulai := kini.AddDate(0, 0, -2)
	str := func(s string) *string { return &s }
	id := func(v int64) *int64 { return &v }

	iklan := []struct {
		Penyedia  int
		Paket     string
		Judul     string
		Deskripsi string
		Target    []string
		Status    string
		Hari      int
	}{
		{0, "Utama 14 hari", "Servis AC panggilan, garansi 30 hari", "Cuci, isi freon, dan perbaikan AC rumah. Teknisi bersertifikat, datang ke seluruh Bengkalis.", []string{}, model.StatusAktif, 14},
		{2, "Dasar 14 hari", "Katering nasi kotak mulai 15 ribu", "Menu harian dan acara, pengantaran gratis di Bengkalis dan Bantan.", []string{"Bengkalis", "Bantan"}, model.StatusAktif, 14},
		{5, "Dasar 7 hari", "Servis motor panggilan Bukit Batu", "Ganti oli, setel rantai, dan tangani mogok di jalan.", []string{"Bukit Batu", "Siak Kecil"}, model.StatusMenunggu, 7},
	}
	for i, ik := range iklan {
		if ik.Penyedia >= len(penyedia) {
			continue
		}
		p := penyedia[ik.Penyedia]
		g, err := upload.SaveImageBytes(gambarDemo(1200, 400, 400+i), service.ProfilIklan)
		if err != nil {
			return err
		}
		pr := &model.Promosi{Jenis: model.PromosiIklan, UserID: p.User.ID, ProviderID: id(p.Profil.ID),
			PaketID: cari(model.PromosiIklan, ik.Paket), Status: ik.Status, Sumber: model.SumberPengguna,
			Judul: str(ik.Judul), Deskripsi: str(ik.Deskripsi), GambarURL: str(g.URL),
			TautanURL: str("/penyedia/" + p.Profil.Slug), TargetKecamatan: ik.Target}
		if ik.Status == model.StatusAktif {
			selesai := mulai.AddDate(0, 0, ik.Hari)
			pr.MulaiAt, pr.SelesaiAt = &mulai, &selesai
		}
		if err := repos.Promosi.Create(ctx, pr); err != nil {
			return err
		}
	}
	// Sorotan jasa: jasa pertama penyedia pertama.
	if len(penyedia) > 0 && len(penyedia[0].Jasa) > 0 {
		selesai := mulai.AddDate(0, 0, 14)
		if err := repos.Promosi.Create(ctx, &model.Promosi{Jenis: model.PromosiSorotan, UserID: penyedia[0].User.ID,
			ProviderID: id(penyedia[0].Profil.ID), ServiceID: id(penyedia[0].Jasa[0].ID),
			PaketID: cari(model.PromosiSorotan, "14 hari"), Status: model.StatusAktif, Sumber: model.SumberPengguna,
			TargetKecamatan: []string{}, MulaiAt: &mulai, SelesaiAt: &selesai}); err != nil {
			return err
		}
	}
	// Penyedia pilihan: penyedia kedua dan keempat.
	for _, idx := range []int{1, 3} {
		if idx >= len(penyedia) {
			continue
		}
		selesai := mulai.AddDate(0, 0, 30)
		if err := repos.Promosi.Create(ctx, &model.Promosi{Jenis: model.PromosiPenyedia, UserID: penyedia[idx].User.ID,
			ProviderID: id(penyedia[idx].Profil.ID), PaketID: cari(model.PromosiPenyedia, "30 hari"),
			Status: model.StatusAktif, Sumber: model.SumberPengguna, TargetKecamatan: []string{},
			MulaiAt: &mulai, SelesaiAt: &selesai}); err != nil {
			return err
		}
	}
	return nil
}

// buatKunjungan menanam riwayat kunjungan 30 hari untuk setiap jasa aktif
// yang masih nol, supaya angka "dilihat" dan panel statistik langsung
// berisi. Pola dibuat wajar: jasa dengan urutan lebih awal lebih ramai,
// akhir pekan sedikit lebih tinggi, dan ada variasi harian deterministik
// (tanpa acak supaya hasilnya sama di setiap lingkungan). Jasa yang sudah
// punya kunjungan tidak disentuh.
func buatKunjungan(ctx context.Context, repos *repository.Repositories) error {
	jasa, err := repos.Service.Search(ctx, repository.ServiceFilter{Limit: 100})
	if err != nil {
		return err
	}
	hariIni := time.Now().Truncate(24 * time.Hour)
	total, diisi := 0, 0
	for i, j := range jasa {
		// Jasa yang sudah punya kunjungan (sungguhan atau demo) tidak disentuh.
		if j.TotalKunjungan > 0 {
			continue
		}
		diisi++
		dasar := 3 + (len(jasa)-i)%7 // 3–9 kunjungan per hari
		for d := 29; d >= 1; d-- {   // hari ini dibiarkan diisi kunjungan sungguhan
			tgl := hariIni.AddDate(0, 0, -d)
			n := dasar + int((j.ID*7+int64(d)*3)%5)
			if wd := tgl.Weekday(); wd == time.Saturday || wd == time.Sunday {
				n += 3
			}
			unik := n - n/4
			if err := repos.Kunjungan.Tambah(ctx, j.ID, tgl, int64(n), int64(unik)); err != nil {
				return err
			}
			total += n
		}
	}
	slog.Info("riwayat kunjungan demo tersimpan", "jasa", diisi, "kunjungan", total)
	return nil
}

// DomainEmailDemo adalah domain email akun demo. Sejak nomor HP demo
// diamankan, akun demo masuk lewat email ini.
const DomainEmailDemo = "demo.adojobs.id"

// demoSudahAda memeriksa akun pencari demo pertama lewat nomor asli maupun
// nomor yang sudah diamankan.
func demoSudahAda(ctx context.Context, repos *repository.Repositories) (bool, error) {
	for _, phone := range []string{pencariDemo[0].Phone, penandaNomorDemo(pencariDemo[0].Nama)} {
		if _, err := repos.User.GetByPhone(ctx, phone); err == nil {
			return true, nil
		} else if !errors.Is(err, repository.ErrNotFound) {
			return false, err
		}
	}
	return false, nil
}

// penandaNomorDemo mengganti nomor HP akun demo dengan penanda yang tidak
// bisa dihubungi: bukan digit, jadi tidak pernah cocok dengan nomor yang
// diketik siapa pun, tidak bisa dipanggil, dan tidak memblokir pendaftaran
// pemilik nomor sungguhan. Kolom phone VARCHAR(20), maka dibuat ringkas.
func penandaNomorDemo(nama string) string {
	p := "demo-" + slug(nama)
	if len(p) > 20 {
		p = p[:20]
	}
	return p
}

// amankanNomorDemo mengganti nomor HP dan WhatsApp akun demo (yang dibuat
// dari daftar penyedia contoh dan pencari demo) dengan penanda, serta
// memberi email supaya akun tetap bisa dipakai masuk. Aman diulang.
func amankanNomorDemo(ctx context.Context, repos *repository.Repositories) error {
	type akun struct{ Nama, Phone string }
	var daftar []akun
	for _, p := range daftarProvider {
		daftar = append(daftar, akun{p.Nama, p.Phone})
	}
	for _, p := range pencariDemo {
		daftar = append(daftar, akun{p.Nama, p.Phone})
	}
	diamankan := 0
	for _, a := range daftar {
		user, err := repos.User.GetByPhone(ctx, a.Phone)
		if errors.Is(err, repository.ErrNotFound) {
			continue // sudah diamankan atau memang tidak ada
		} else if err != nil {
			return err
		}
		email := slug(a.Nama) + "@" + DomainEmailDemo
		user.Phone = penandaNomorDemo(a.Nama)
		if user.Email == nil || *user.Email == "" {
			user.Email = &email
		}
		if err := repos.User.UpdateByAdmin(ctx, user); err != nil {
			return err
		}
		if user.IsProvider {
			profil, err := repos.Provider.GetByUserID(ctx, user.ID)
			if err != nil {
				return err
			}
			profil.WhatsappNumber = ""
			if err := repos.Provider.Update(ctx, profil); err != nil {
				return err
			}
		}
		diamankan++
	}
	if diamankan > 0 {
		slog.Info("nomor akun demo diamankan", "akun", diamankan, "masuk_dengan", "email <slug-nama>@"+DomainEmailDemo)
	}
	return nil
}
