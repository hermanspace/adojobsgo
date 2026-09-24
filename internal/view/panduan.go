package view

import "strings"

// Panduan adalah isi halaman "Tentang & Panduan" dalam bentuk data, bukan
// markup. Templ merendernya jadi HTML; API mengirim struktur yang sama ke
// aplikasi Android. Ubah satu kalimat di sini, kedua platform ikut.
//
// Setiap bagian punya ID tetap yang menjadi anchor halaman (/tentang#faq) —
// tautan dari menu, notifikasi, dan aplikasi mengandalkannya.

const (
	PanduanTeks   = "teks"   // paragraf ringkas
	PanduanJalur  = "jalur"  // dua kolom langkah: pencari vs penyedia
	PanduanFitur  = "fitur"  // grid kartu fitur
	PanduanPaket  = "paket"  // diisi handler dari tabel paket_promosi
	PanduanTanya  = "tanya"  // pertanyaan umum
	PanduanKontak = "kontak" // pengelola & aturan
)

type PanduanLangkah struct {
	Ikon   string `json:"ikon"`
	Judul  string `json:"judul"`
	Isi    string `json:"isi"`
	Tautan string `json:"tautan,omitempty"`
	// Kunci mengaitkan item dengan jenis paket promosi (iklan, sorotan_jasa,
	// penyedia_pilihan) supaya harganya bisa ditempel dari database.
	Kunci string `json:"kunci,omitempty"`
}

type PanduanJalurData struct {
	Judul   string           `json:"judul"`
	Langkah []PanduanLangkah `json:"langkah"`
}

type PanduanTanyaJawab struct {
	Tanya string `json:"tanya"`
	Jawab string `json:"jawab"`
}

type PanduanBagian struct {
	ID      string `json:"id"`
	Jenis   string `json:"jenis"`
	Judul   string `json:"judul"`
	Ringkas string `json:"ringkas,omitempty"`
	// Salah satu berikut terisi, sesuai Jenis.
	Paragraf []string            `json:"paragraf,omitempty"`
	Jalur    []PanduanJalurData  `json:"jalur,omitempty"`
	Fitur    []PanduanLangkah    `json:"fitur,omitempty"`
	Tanya    []PanduanTanyaJawab `json:"tanya,omitempty"`
}

// Panduan menyusun seluruh bagian untuk situs bernama nama. Bagian paket
// dikembalikan tanpa isi: harganya hidup di database dan diisi pemanggil,
// supaya panduan tidak pernah menampilkan harga basi.
func Panduan(nama string) []PanduanBagian {
	r := strings.NewReplacer("{situs}", nama)
	bagian := []PanduanBagian{
		{ID: "tentang", Jenis: PanduanTeks, Judul: "Apa itu {situs}",
			Paragraf: []string{
				"{situs} mempertemukan penyedia jasa lokal di Kabupaten Bengkalis — tukang, teknisi, jasa kebersihan, sampai les privat — dengan warga yang membutuhkannya. Tanpa perantara, tanpa biaya pendaftaran.",
				"Semua terjadi di dalam aplikasi: mencari berdasarkan jarak dari lokasi Anda, mengobrol, mengirim permintaan pesanan, dan memberi ulasan setelah pekerjaan selesai. Nomor telepon tidak dibagikan sebelum kedua pihak sepakat.",
			}},
		{ID: "cara-kerja", Jenis: PanduanJalur, Judul: "Cara kerja", Ringkas: "Dua sisi, satu alur yang sama-sama sederhana.",
			Jalur: []PanduanJalurData{
				{Judul: "Mencari jasa", Langkah: []PanduanLangkah{
					{Ikon: "cari", Judul: "Cari", Isi: "Ketik kebutuhan Anda atau pilih kategori. Izinkan lokasi supaya hasil diurutkan dari yang terdekat.", Tautan: "/cari"},
					{Ikon: "pesanan", Judul: "Hubungi", Isi: "Kirim pesan untuk bertanya, atau langsung kirim permintaan pesanan dengan catatan dan tanggal yang diinginkan."},
					{Ikon: "sukses", Judul: "Selesaikan", Isi: "Penyedia menerima pesanan dan menandainya selesai. Anda memberi rating dan ulasan — itu yang menjaga kualitas di sini."},
				}},
				{Judul: "Menawarkan jasa", Langkah: []PanduanLangkah{
					{Ikon: "akun", Judul: "Daftar sebagai penyedia", Isi: "Cukup nomor HP dan deskripsi singkat. Tetapkan lokasi dan radius layanan agar Anda muncul di pencarian sekitar.", Tautan: "/provider/daftar"},
					{Ikon: "tambah", Judul: "Pasang jasa", Isi: "Judul, kategori, harga (tetap, per jam, atau nego), dan foto. Admin meninjau sebelum tayang, biasanya kurang dari sehari.", Tautan: "/jasa/baru"},
					{Ikon: "bintang", Judul: "Terima pesanan & kumpulkan ulasan", Isi: "Balas pesan, terima atau tolak pesanan, tandai selesai. Ulasan dan lencana terverifikasi menaikkan kepercayaan."},
				}},
			}},
		{ID: "fitur", Jenis: PanduanFitur, Judul: "Fitur utama",
			Fitur: []PanduanLangkah{
				{Ikon: "lokasi", Judul: "Pencarian sadar-lokasi", Isi: "Hasil diurutkan dari yang terdekat dan bisa disaring hanya penyedia yang menjangkau alamat Anda."},
				{Ikon: "pesanan", Judul: "Obrolan di dalam aplikasi", Isi: "Pesan tersimpan bersama pesanan, jadi kesepakatan selalu bisa dirujuk ulang."},
				{Ikon: "sukses", Judul: "Pesanan & ulasan", Isi: "Status pesanan jelas dari diajukan sampai selesai; ulasan hanya dari pesanan yang benar-benar selesai."},
				{Ikon: "terverifikasi", Judul: "Penyedia terverifikasi", Isi: "Lencana untuk penyedia yang identitasnya sudah diperiksa admin."},
				{Ikon: "foto", Judul: "Portofolio", Isi: "Penyedia memamerkan hasil pekerjaan sebelumnya dengan foto."},
				{Ikon: "megafon", Judul: "Promosi mandiri", Isi: "Iklan, sorotan jasa, dan penyedia pilihan bisa diajukan sendiri, disetujui admin."},
			}},
		{ID: "promosi", Jenis: PanduanPaket, Judul: "Promosi", Ringkas: "Tiga cara tampil lebih menonjol. Semua diajukan dari akun Anda, ditinjau admin, dan dibayar lewat transfer dengan bukti.",
			Fitur: []PanduanLangkah{
				{Kunci: "iklan", Ikon: "megafon", Judul: "Iklan", Isi: "Gambar Anda tampil di slot beranda dan hasil pencarian. Pilih satu atau beberapa kecamatan sasaran; harga tidak berubah.", Tautan: "/promosi/baru?jenis=iklan"},
				{Kunci: "sorotan_jasa", Ikon: "bintang", Judul: "Sorotan jasa", Isi: "Satu jasa naik ke urutan teratas kategorinya dengan kartu keemasan.", Tautan: "/promosi/baru?jenis=sorotan_jasa"},
				{Kunci: "penyedia_pilihan", Ikon: "terverifikasi", Judul: "Penyedia pilihan", Isi: "Profil Anda tampil bergiliran di blok penyedia pilihan beranda, diutamakan untuk penonton sekecamatan.", Tautan: "/promosi/baru?jenis=penyedia_pilihan"},
			}},
		{ID: "faq", Jenis: PanduanTanya, Judul: "Pertanyaan umum",
			Tanya: []PanduanTanyaJawab{
				{"Apakah {situs} berbayar?", "Tidak. Mendaftar, memasang jasa, mengobrol, dan memesan semuanya gratis. Yang berbayar hanya promosi, dan itu pilihan."},
				{"Bagaimana cara membayar penyedia?", "Langsung ke penyedia, sesuai kesepakatan di obrolan. {situs} tidak menahan atau memproses pembayaran jasa."},
				{"Kenapa jasa saya belum tayang?", "Setiap jasa baru ditinjau admin. Bila lebih dari sehari belum berubah, periksa notifikasi — biasanya ada catatan yang perlu dilengkapi."},
				{"Bagaimana mendapat lencana terverifikasi?", "Lengkapi profil, lokasi, dan portofolio, lalu admin memeriksa identitas Anda. Belum bisa diajukan sendiri; admin menghubungi lewat notifikasi."},
				{"Apakah nomor HP saya terlihat?", "Tidak. Semua komunikasi lewat obrolan di aplikasi. Nomor hanya dipakai untuk masuk dan pemulihan akun."},
				{"Bisakah saya mengubah atau membatalkan pesanan?", "Pesanan yang masih menunggu bisa dibatalkan pencari. Setelah diterima, sepakati perubahannya lewat obrolan; penyedia yang menandai selesai."},
				{"Bagaimana cara pesan tersampaikan saat aplikasi ditutup?", "Di web, hitungan belum dibaca diperbarui saat halaman dibuka. Di aplikasi Android, notifikasi masuk ke ponsel."},
				{"Apa yang terjadi bila iklan saya ditolak?", "Anda menerima notifikasi berisi alasannya dan boleh mengajukan ulang dengan materi yang diperbaiki. Tidak ada biaya untuk pengajuan yang ditolak."},
				{"Berapa lama promosi berjalan?", "Sesuai paket yang dipilih, dihitung sejak admin mengaktifkannya. Anda diingatkan sehari sebelum berakhir."},
				{"Bagaimana melaporkan penyedia atau pengguna bermasalah?", "Hubungi pengelola lewat kontak di bawah dengan tautan profil atau nomor pesanan dan bukti yang jelas. Akun yang terbukti melanggar dinonaktifkan; pengelola tidak memberikan kompensasi atau menangani urusan di luar itu."},
			}},
		{ID: "kontak", Jenis: PanduanKontak, Judul: "Pengelola & aturan",
			Paragraf: []string{
				"{situs} dikelola secara mandiri di Bengkalis. Untuk pertanyaan, laporan, atau kerja sama, gunakan kontak yang tertera di bawah halaman.",
				"{situs} hanya mempertemukan pencari dan penyedia jasa. Kesepakatan harga, pengerjaan, dan pembayaran terjadi langsung di antara keduanya. Karena itu pengelola tidak bertanggung jawab atas kerugian, sengketa, atau penyalahgunaan yang timbul dari penggunaan aplikasi ini, dan tidak memberikan kompensasi atau ganti rugi dalam bentuk apa pun.",
				"Bila Anda menemukan penipuan, pelecehan, atau pelanggaran lain, laporkan kepada pengelola dengan bukti yang jelas. Setelah laporan diperiksa dan pelanggaran terbukti, tindakan pengelola adalah menonaktifkan akun yang bersangkutan. Tidak ada tindakan lain di luar itu; urusan hukum atau ganti rugi tetap menjadi hak dan tanggung jawab pihak yang dirugikan.",
				"Data yang disimpan hanya yang dibutuhkan untuk layanan: nama, nomor HP, lokasi yang Anda tetapkan sendiri, dan riwayat pesanan. Tidak dibagikan ke pihak lain.",
			}},
	}
	for i := range bagian {
		b := &bagian[i]
		b.Judul, b.Ringkas = r.Replace(b.Judul), r.Replace(b.Ringkas)
		for j := range b.Paragraf {
			b.Paragraf[j] = r.Replace(b.Paragraf[j])
		}
		for j := range b.Tanya {
			b.Tanya[j].Tanya, b.Tanya[j].Jawab = r.Replace(b.Tanya[j].Tanya), r.Replace(b.Tanya[j].Jawab)
		}
	}
	return bagian
}
