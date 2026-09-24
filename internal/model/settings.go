package model

// Kunci pengaturan aplikasi yang tersimpan di tabel app_settings.
const (
	SettingUmum     = "umum"
	SettingLokasi   = "lokasi"
	SettingIklan    = "iklan"
	SettingTampilan = "tampilan"
	SettingPromosi  = "promosi"
)

// PengaturanUmum memuat identitas platform yang bisa diubah admin
// tanpa perlu membangun ulang aplikasi.
type PengaturanUmum struct {
	NamaSitus string `json:"nama_situs"`
	// Tagline dipakai sebagai <meta name="description"> beranda.
	Tagline    string `json:"tagline"`
	LogoURL    string `json:"logo_url"`
	KontakWA   string `json:"kontak_wa"`
	TeksFooter string `json:"teks_footer"`
	// WhatsappAktif menentukan apakah tombol "Hubungi lewat WhatsApp"
	// ditampilkan di halaman jasa dan halaman penyedia. Dimatikan supaya
	// seluruh percakapan tetap berada di dalam aplikasi — nomornya tetap
	// tersimpan, jadi menyalakannya kembali tidak perlu mengumpulkan ulang.
	WhatsappAktif bool `json:"whatsapp_aktif"`
}

// PengaturanTampilan mengatur elemen visual beranda yang boleh diganti admin
// tanpa menyentuh kode.
type PengaturanTampilan struct {
	HeroGambarURL string `json:"hero_gambar_url"`
	// HeroOverlay adalah kepekatan lapisan gelap di atas gambar, dalam persen.
	// Bisa disetel karena foto yang diunggah admin terang-gelapnya berbeda-beda;
	// nilai tetap akan membuat teks hilang pada foto yang terlalu terang.
	HeroOverlay int `json:"hero_overlay"`

	// AplikasiTampil menentukan apakah lencana unduh muncul di hero.
	// Dipisahkan dari PlaystoreURL supaya lencana "segera hadir" bisa
	// ditampilkan sebelum aplikasinya benar-benar ada di Play Store.
	AplikasiTampil bool `json:"aplikasi_tampil"`
	// PlaystoreURL boleh kosong. Saat kosong, lencananya dirender sebagai
	// keterangan biasa yang tidak bisa diklik, bukan tautan yang menuju
	// halaman Play Store yang belum ada.
	PlaystoreURL       string `json:"playstore_url"`
	PlaystoreTeksAtas  string `json:"playstore_teks_atas"`
	PlaystoreTeksBawah string `json:"playstore_teks_bawah"`
}

// PengaturanLokasi mengatur perilaku pencarian berbasis jarak.
type PengaturanLokasi struct {
	PusatLatitude   float64 `json:"pusat_latitude"`
	PusatLongitude  float64 `json:"pusat_longitude"`
	RadiusDefaultKm int     `json:"radius_default_km"`
	RadiusMaksKm    int     `json:"radius_maks_km"`
}

// SlotIklan adalah satu slot penempatan iklan.
// Penayangan iklan belum diaktifkan; struktur ini disiapkan supaya admin
// sudah bisa menyusun slotnya lebih dulu.
type SlotIklan struct {
	Kunci     string `json:"kunci"`
	Nama      string `json:"nama"`
	Aktif     bool   `json:"aktif"`
	GambarURL string `json:"gambar_url"`
	TautanURL string `json:"tautan_url"`
	Teks      string `json:"teks"`
}

// LayakTayang menandai slot yang benar-benar punya sesuatu untuk ditampilkan.
// Slot bisa saja aktif tetapi materinya sudah dihapus belakangan, dan halaman
// publik tidak boleh menampilkan kotak kosong karenanya.
func (s SlotIklan) LayakTayang() bool {
	return s.Aktif && (s.GambarURL != "" || s.Teks != "")
}

// DapatDiklik menandai iklan yang punya tujuan. Iklan tanpa tautan tetap
// ditayangkan sebagai gambar atau teks biasa, bukan tautan mati.
func (s SlotIklan) DapatDiklik() bool { return s.TautanURL != "" }

// PengaturanIklan memuat seluruh slot iklan yang tersedia.
type PengaturanIklan struct {
	Slot []SlotIklan `json:"slot"`
}

// Cari mencari satu slot berdasarkan kunci. Mengembalikan nil bila slotnya
// tidak ada atau belum layak tayang, sehingga pemanggil cukup memeriksa nil.
func (p PengaturanIklan) Cari(kunci string) *SlotIklan {
	for i := range p.Slot {
		if p.Slot[i].Kunci == kunci && p.Slot[i].LayakTayang() {
			return &p.Slot[i]
		}
	}
	return nil
}

// PengaturanPromosi mengatur pembayaran manual promosi.
type PengaturanPromosi struct {
	// UjiCobaGratis membuat setiap paket lewat tanpa pembayaran: pengajuan
	// yang disetujui langsung tayang. Menyala sejak awal — jalur bayarnya
	// sudah siap, tinggal dimatikan saat harga mulai diberlakukan.
	UjiCobaGratis bool   `json:"uji_coba_gratis"`
	Bank          string `json:"bank"`
	NomorRekening string `json:"nomor_rekening"`
	AtasNama      string `json:"atas_nama"`
	PetunjukBayar string `json:"petunjuk_bayar"`
}

// Pengaturan menggabungkan seluruh kelompok pengaturan.
type Pengaturan struct {
	Umum     PengaturanUmum     `json:"umum"`
	Lokasi   PengaturanLokasi   `json:"lokasi"`
	Iklan    PengaturanIklan    `json:"iklan"`
	Tampilan PengaturanTampilan `json:"tampilan"`
	Promosi  PengaturanPromosi  `json:"promosi"`
}

// HeroOverlayMin menjaga teks hero tetap terbaca sekalipun admin menurunkan
// overlay serendah mungkin pada foto yang terang.
const (
	HeroOverlayMin    = 25
	HeroOverlayMaks   = 85
	HeroOverlayBawaan = 55
)

// PengaturanBawaan dipakai saat tabel app_settings masih kosong, sehingga
// aplikasi tetap berjalan sebelum admin menyentuh halaman pengaturan.
func PengaturanBawaan() Pengaturan {
	return Pengaturan{
		Umum: PengaturanUmum{
			NamaSitus: "Adojobs",
			// Tagline adalah deskripsi meta beranda — kalimat yang tampil di
			// bawah judul pada hasil pencarian Google. Sengaja lebih kaya
			// daripada teks footer, karena keduanya punya pembaca berbeda.
			Tagline:    "Marketplace jasa lokal Kabupaten Bengkalis: servis AC, tukang, kebersihan, dekorasi, dokumentasi acara.",
			TeksFooter: "Marketplace jasa lokal Kabupaten Bengkalis, Riau.",
		},
		Lokasi: PengaturanLokasi{
			PusatLatitude:   PusatBengkalis.Latitude,
			PusatLongitude:  PusatBengkalis.Longitude,
			RadiusDefaultKm: 25,
			RadiusMaksKm:    100,
		},
		Iklan: PengaturanIklan{Slot: SlotIklanBawaan()},
		Tampilan: PengaturanTampilan{
			HeroOverlay: HeroOverlayBawaan,
			// Bawaannya "segera hadir" karena aplikasinya memang belum terbit;
			// admin tinggal menggantinya jadi "Dapatkan di" setelah tayang.
			PlaystoreTeksAtas:  "Segera hadir di",
			PlaystoreTeksBawah: "Google Play",
		},
		Promosi: PengaturanPromosi{UjiCobaGratis: true},
	}
}

// SlotIklanBawaan mendefinisikan penempatan yang tersedia di halaman publik.
func SlotIklanBawaan() []SlotIklan {
	return []SlotIklan{
		{Kunci: "beranda_atas", Nama: "Beranda — bawah hero"},
		{Kunci: "beranda_tengah", Nama: "Beranda — antara kategori dan listing"},
		{Kunci: "cari_atas", Nama: "Halaman pencarian — atas hasil"},
		{Kunci: "detail_samping", Nama: "Detail jasa — kolom samping"},
	}
}
