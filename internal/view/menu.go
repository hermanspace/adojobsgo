package view

// Menu utama (tombol melayang) disusun di sini — satu-satunya tempat — dan
// dipakai dua arah: templ merendernya sebagai sheet di web, API mengirimnya
// apa adanya ke aplikasi Android. Aturan "siapa melihat apa" karena itu
// tidak pernah ditulis dua kali.

// Jenis item menu. Tautan biasa dibuka sebagai halaman; sisanya butuh
// perlakuan khusus klien (kirim form keluar, panggil prompt pasang, ganti
// tema) — klien memutuskan lewat Jenis, bukan lewat menebak dari Tautan.
const (
	MenuTautan   = "tautan"
	MenuKeluar   = "keluar"
	MenuAplikasi = "aplikasi"
	MenuTema     = "tema"
)

// Keadaan item "Pasang aplikasi". Sengaja dua, bukan tiga: aplikasi Android
// tidak menampilkan item ini sama sekali, dan web memutuskan sendiri lewat
// AplikasiKeadaan mana yang berlaku.
const (
	AplikasiPlaystore = "playstore" // tautan toko sudah diisi admin
	AplikasiPWA       = "pwa"       // belum ada di toko: tawarkan pasang dari peramban
)

type MenuItem struct {
	Ikon       string `json:"ikon"`
	Label      string `json:"label"`
	Keterangan string `json:"keterangan,omitempty"`
	// Tautan adalah path web; aplikasi memetakannya ke layar dengan satu
	// tabel — konvensi yang sama dengan notifikasi dan push.
	Tautan string `json:"tautan,omitempty"`
	Jenis  string `json:"jenis"`
	Badge  int    `json:"badge,omitempty"`
	// Keadaan hanya berisi untuk Jenis aplikasi.
	Keadaan string `json:"keadaan,omitempty"`
	// Utama menandai satu-dua aksi yang ditonjolkan (tombol penuh, bukan baris).
	Utama bool `json:"utama,omitempty"`
}

type MenuBagian struct {
	Judul string     `json:"judul"`
	Item  []MenuItem `json:"item"`
}

// MenuKonteks adalah masukan minimum untuk menyusun menu: siapa yang
// melihat, berapa yang belum dibaca, dan pengaturan situs yang relevan.
type MenuKonteks struct {
	User        *CurrentUser
	BelumDibaca int
	Notifikasi  int
	Situs       SitusRingkas
}

// KonteksMenu mengambil konteks menu dari data halaman.
func (b Base) KonteksMenu() MenuKonteks {
	return MenuKonteks{User: b.User, BelumDibaca: b.BelumDibaca, Situs: b.Situs}
}

// MenuUtama menyusun bagian-bagian menu sesuai peran pengunjung. Setiap
// tautan yang butuh masuk diarahkan ke halamannya langsung: RequireWeb
// sudah mengirim tamu ke /masuk dengan alamat kembali, jadi menu tidak
// perlu tahu aturan itu.
func MenuUtama(k MenuKonteks) []MenuBagian {
	masuk := k.User != nil
	penyedia := masuk && k.User.IsProvider

	aksi := []MenuItem{{Ikon: "cari", Label: "Cari jasa", Keterangan: "Tukang, servis, dan jasa di sekitar Anda", Tautan: "/cari", Jenis: MenuTautan}}
	switch {
	case penyedia:
		aksi = append(aksi,
			MenuItem{Ikon: "tambah", Label: "Pasang jasa", Keterangan: "Tawarkan jasa baru", Tautan: "/jasa/baru", Jenis: MenuTautan, Utama: true},
			MenuItem{Ikon: "pesanan", Label: "Pesan & pesanan", Tautan: "/pesan", Jenis: MenuTautan, Badge: k.BelumDibaca},
			MenuItem{Ikon: "kunci-inggris", Label: "Jasa saya", Tautan: "/dasbor", Jenis: MenuTautan},
		)
	case masuk:
		aksi = append(aksi,
			MenuItem{Ikon: "pesanan", Label: "Pesan & pesanan", Tautan: "/pesan", Jenis: MenuTautan, Badge: k.BelumDibaca},
			MenuItem{Ikon: "tambah", Label: "Jadi penyedia jasa", Keterangan: "Gratis, disetujui admin", Tautan: "/provider/daftar", Jenis: MenuTautan, Utama: true},
		)
	default:
		aksi = append(aksi,
			MenuItem{Ikon: "tambah", Label: "Jadi penyedia jasa", Keterangan: "Gratis, tanpa perantara", Tautan: "/provider/daftar", Jenis: MenuTautan, Utama: true},
		)
	}

	promosi := []MenuItem{{Ikon: "megafon", Label: "Pasang iklan", Keterangan: "Tampil di beranda dan hasil pencarian", Tautan: "/promosi/baru?jenis=iklan", Jenis: MenuTautan}}
	if penyedia {
		promosi = append(promosi,
			MenuItem{Ikon: "bintang", Label: "Sorot jasa", Keterangan: "Naikkan satu jasa ke urutan teratas", Tautan: "/promosi/baru?jenis=sorotan_jasa", Jenis: MenuTautan},
			MenuItem{Ikon: "terverifikasi", Label: "Jadi penyedia pilihan", Tautan: "/promosi/baru?jenis=penyedia_pilihan", Jenis: MenuTautan},
		)
	}
	if masuk {
		promosi = append(promosi, MenuItem{Ikon: "petir", Label: "Promosi saya", Tautan: "/promosi", Jenis: MenuTautan})
	}

	var akun []MenuItem
	if masuk {
		akun = append(akun,
			MenuItem{Ikon: "akun", Label: "Dasbor", Tautan: "/dasbor", Jenis: MenuTautan},
			MenuItem{Ikon: "ubah", Label: "Profil & kata sandi", Tautan: "/akun/profil", Jenis: MenuTautan},
		)
		if penyedia {
			akun = append(akun,
				MenuItem{Ikon: "lokasi", Label: "Lokasi & area layanan", Tautan: "/provider/lokasi", Jenis: MenuTautan},
				MenuItem{Ikon: "foto", Label: "Portofolio", Tautan: "/provider/portofolio", Jenis: MenuTautan},
			)
		}
		if k.User.IsAdmin {
			akun = append(akun, MenuItem{Ikon: "filter", Label: "Panel admin", Tautan: "/admin", Jenis: MenuTautan})
		}
		akun = append(akun, MenuItem{Ikon: "keluar", Label: "Keluar", Tautan: "/keluar", Jenis: MenuKeluar})
	} else {
		akun = append(akun,
			MenuItem{Ikon: "akun", Label: "Masuk", Tautan: "/masuk", Jenis: MenuTautan, Utama: true},
			MenuItem{Ikon: "tambah", Label: "Daftar akun baru", Keterangan: "Cukup nomor HP", Tautan: "/daftar", Jenis: MenuTautan},
		)
	}

	bantuan := []MenuItem{
		{Ikon: "buku", Label: "Panduan & pertanyaan umum", Tautan: "/tentang#panduan", Jenis: MenuTautan},
		{Ikon: "info", Label: "Tentang " + k.Situs.Nama, Tautan: "/tentang", Jenis: MenuTautan},
		{Ikon: "bulan", Label: "Tema terang / gelap", Jenis: MenuTema},
	}
	if item, ada := itemAplikasi(k.Situs); ada {
		bantuan = append([]MenuItem{item}, bantuan...)
	}

	return []MenuBagian{
		{Judul: "Aksi cepat", Item: aksi},
		{Judul: "Promosi", Item: promosi},
		{Judul: "Akun", Item: akun},
		{Judul: "Aplikasi & bantuan", Item: bantuan},
	}
}

// itemAplikasi memutuskan bentuk "Pasang aplikasi" dari pengaturan admin:
// bila tautan toko sudah ada, itu yang dipakai; bila belum, web menawarkan
// pemasangan PWA supaya fiturnya berguna hari ini, bukan menunggu rilis.
func itemAplikasi(s SitusRingkas) (MenuItem, bool) {
	if !s.AplikasiTampil {
		return MenuItem{}, false
	}
	if s.PlaystoreURL != "" {
		return MenuItem{Ikon: "unduh", Label: "Pasang aplikasi Android", Keterangan: "Unduh di Google Play", Tautan: s.PlaystoreURL, Jenis: MenuAplikasi, Keadaan: AplikasiPlaystore}, true
	}
	return MenuItem{Ikon: "unduh", Label: "Pasang di layar utama", Keterangan: "Buka lebih cepat, tanpa toko aplikasi", Jenis: MenuAplikasi, Keadaan: AplikasiPWA}, true
}
