# Adojobs Bengkalis

Marketplace jasa hyperlocal untuk Kabupaten Bengkalis, Riau. Mempertemukan
penyedia jasa perorangan dan UMKM — tukang, teknisi, jasa kebersihan, dekorasi,
dokumentasi acara — dengan warga yang membutuhkan.

Seluruh service berjalan di Docker. Tidak ada dependensi yang perlu dipasang di
host, termasuk Go, Node, maupun klien PostgreSQL.

## Jalankan di lokal

```bash
make setup
```

Perintah itu membuat `.env` berisi kredensial acak, menjalankan seluruh service,
menjalankan migrasi, lalu mengisi data contoh. Aplikasi terbuka di
<http://localhost:3000>.

Bila `.env` sudah ada dan database sudah terisi, cukup:

```bash
make up
```

Akun penyedia contoh hasil seeding memakai kata sandi `rahasia123`, misalnya
nomor `08117512001` (Rizal Teknik AC). Akun-akun ini hanya untuk pengembangan.

Untuk membuka panel admin, isi `ADMIN_PHONE` dan `ADMIN_PASSWORD` di `.env` lalu:

```bash
make admin-create
```

Masuk lewat `/masuk` memakai nomor tersebut; admin otomatis diarahkan ke `/admin`.
Bila nomornya sudah terdaftar sebagai pengguna biasa, akun itu dinaikkan
perannya, bukan dibuat ganda.

## Perintah

Seluruh target berlaku untuk kedua lingkungan. Tambahkan `ENV=prod` untuk
menjalankannya di server lewat SSH — tidak ada logika yang ditulis dua kali.

| Perintah | Kegunaan |
|---|---|
| `make up` / `make down` | Jalankan / hentikan semua service |
| `make build` / `make rebuild` | Build ulang image app |
| `make logs` / `make logs-app` | Tail log semua service / service app |
| `make migrate` / `make migrate-down` | Migrasi ke versi terbaru / rollback satu langkah |
| `make seed` | Isi kategori jasa dan penyedia contoh |

Migrasi **tersemat di dalam image** (`//go:embed`), jadi `make migrate` menjalankan migrasi yang dikenal image yang sedang ada. Setelah menambah berkas migrasi, jalankan `make build` lebih dulu — kalau tidak, image lama akan melapor "sudah pada versi terbaru" dengan jujur, karena bagi dirinya memang begitu.
| `make admin-create` | Buat admin pertama dari `ADMIN_PHONE` & `ADMIN_PASSWORD` di `.env` |
| `make shell` / `make psql` / `make redis-cli` | Masuk ke container app / psql / redis-cli |
| `make test` / `make lint` / `make fmt` | Test, pemeriksaan, dan format kode |
| `make css` / `make generate` | Bangun ulang CSS Tailwind / berkas templ |
| `make backup-db` / `make restore-db FILE=…` | Cadangkan / pulihkan database |
| `make deploy` | Build image produksi, kirim ke server, migrasi, restart |
| `make clean` | Hentikan service dan hapus volume (data ikut terhapus) |

Alat bantu pengembangan dijalankan lewat profil terpisah supaya tidak pernah
ikut berjalan di server:

```bash
docker compose --profile tools up -d adminer   # http://localhost:8080
docker compose --profile tools up -d tailwind  # pantau perubahan CSS
```

## Arsitektur

```
cmd/server            entrypoint; juga menjalankan subcommand migrate & seed
internal/handler/api  endpoint JSON  /api/v1/*   (disiapkan untuk aplikasi Flutter)
internal/handler/web  halaman HTML   (templ + HTMX), termasuk panel admin /admin
internal/service      seluruh aturan bisnis — dipakai kedua handler di atas
internal/repository   satu-satunya tempat query SQL ditulis
internal/model        struct domain
internal/view         data dan helper untuk template
migrations            migrasi SQL bernomor, ditanam ke dalam binary
web/templates         berkas .templ
web/static            css, js, dan font yang di-self-host
```

Handler web dan handler API memanggil service layer yang sama persis, sehingga
aturan validasi dan otorisasi tidak pernah berbeda antara web dan API.

**Stack:** Go + Fiber, PostgreSQL 16, Redis (sesi dan cache), templ, HTMX,
Alpine.js, Tailwind CSS, Leaflet. Rendering seluruhnya di server.

### Keputusan yang perlu diketahui

- **Sesi, bukan JWT.** ID sesi acak disimpan di cookie `HttpOnly`; seluruh
  datanya ada di Redis sehingga sesi bisa dicabut kapan saja dari sisi server.
- **Rating di-cache.** `avg_rating` dan `total_reviews` disimpan di
  `provider_profiles` dan hanya dihitung ulang saat ada ulasan baru — bukan
  diagregasi setiap kali listing muncul di pencarian.
- **Satu query per kartu listing.** Foto sampul diambil lewat `LEFT JOIN LATERAL`,
  sehingga halaman pencarian tidak menimbulkan N+1 query.
- **Cache pencarian.** Hasil pencarian dan daftar kategori disimpan di Redis dan
  dibersihkan otomatis setiap kali ada listing yang berubah.
- **Migrasi ditanam di binary.** `make migrate` berjalan identik di lokal dan di
  server tanpa image migrator terpisah.
- **Alpine build CSP.** Aplikasi memakai Alpine versi *CSP-friendly* yang tidak
  memanggil `new Function`, sehingga `Content-Security-Policy` bisa tetap ketat
  tanpa `unsafe-eval`. Konsekuensinya direktif Alpine hanya boleh menunjuk nama
  properti atau method — lihat komponen yang didaftarkan di `web/static/js/app.js`.
  Interaksi yang murni presentasional (galeri foto, penyorotan pilihan harga)
  ditangani CSS, tanpa JavaScript sama sekali.
- **Aset berversi isi.** URL CSS dan JS membawa hash isinya, sehingga boleh
  di-cache lama di peramban namun pembaruan tetap sampai ke pengguna.
- **Admin adalah pengguna biasa dengan `role = admin`.** Tidak ada jalur
  autentikasi kedua: sesi, cookie, dan middleware yang sama dipakai ulang.
  Panel admin menjawab **404** bagi yang tidak berhak, bukan 403, agar
  keberadaannya tidak terkonfirmasi.
- **Penangguhan mencabut sesi seketika.** Redis menyimpan indeks sesi per
  pengguna, sehingga seluruh sesi aktif akun yang ditangguhkan langsung
  dihapus — bukan menunggu cookie-nya kedaluwarsa.
- **Pencarian jarak memakai `cube` + `earthdistance`,** ekstensi yang sudah
  tersedia di image `postgres:16-alpine` — tidak perlu mengganti image ke
  PostGIS. Penyaringan radius memanfaatkan indeks GiST lewat `earth_box`,
  sehingga tidak memindai seluruh tabel.
- **Gambar unggahan selalu disandikan ulang, tidak pernah disimpan apa adanya.**
  Foto diperkecil ke sisi terpanjang 1600 px, dikonversi ke WebP, dan
  dibuatkan thumbnail 480 px untuk kartu pencarian. Penyandian ulang dari
  piksel sekaligus membuang seluruh metadata bawaan — termasuk koordinat GPS
  yang lazim disematkan kamera ponsel, hal yang penting karena penyedia jasa
  memotret pekerjaan di rumah pelanggan. Orientasi EXIF dibaca lebih dulu dan
  diterapkan, supaya foto potret tidak tampil miring setelah metadatanya hilang.
  Encoder WebP-nya pure Go (WASM), jadi build tetap `CGO_ENABLED=0`.
- **Tidak ada geocoding pihak ketiga.** Tebakan kecamatan dari koordinat
  dilakukan di dalam aplikasi terhadap daftar 11 kecamatan Bengkalis, jadi
  posisi pengguna tidak pernah meninggalkan server ini. Yang keluar hanyalah
  permintaan gambar tile peta dari peramban ke OpenStreetMap.
- **Chat memakai polling HTMX,** bukan SSE atau WebSocket, supaya tidak ada
  penyetelan proxy tambahan di Nginx Proxy Manager. Ruang obrolan menarik
  hanya pesan yang lebih baru dari yang sudah tampil.
- **Tidak ada JavaScript sebaris sama sekali.** `script-src 'self'` melarang
  atribut `onclick`/`onchange`, `hx-on`, dan `hx-vals`/`hx-headers` berisi
  objek — htmx mem-parsing dua atribut terakhir dengan `Function()`, bukan
  `JSON.parse`, sehingga ikut diblokir. Semua perilaku itu ditangani listener
  yang didelegasikan di `web/static/js/app.js`, dan `htmx.config.allowEval`
  dimatikan.

`go test ./web/templates/pages/` memindai seluruh template untuk dua kelas
kesalahan yang **gagal diam-diam di peramban** — tidak ada error di sisi
server, tombolnya sekadar tidak bekerja:

1. JavaScript sebaris dan atribut htmx yang dievaluasi (`hx-on`, `hx-vals`,
   `hx-headers`), yang diblokir CSP.
2. `hx-target="#id"` yang menunjuk elemen tidak ada, yang memunculkan
   `htmx:targetError` di konsol.

## Desain

Token warna didefinisikan sebagai CSS variable di `web/static/css/source.css` dan
dipakai konsisten di seluruh halaman — tidak ada nilai heksadesimal yang ditulis
langsung di markup. Mode gelap diaktifkan lewat toggle di navigasi, disimpan di
`localStorage`, dan menyesuaikan tingkat elevasi antar mode, bukan sekadar
membalik warna.

Tata letak dirancang mobile-first dari lebar 375px; breakpoint tablet dan desktop
adalah penambahan. Navigasi utama di ponsel memakai bottom-bar.

Tipografi memakai dua font yang di-self-host: **Bricolage Grotesque** untuk
heading dan **Plus Jakarta Sans** untuk teks isi. Gradasi hanya dipakai pada
elemen aksen kecil — tombol utama, garis tab aktif, indikator navigasi — tidak
pernah sebagai latar blok besar.

## Panel admin

Tersedia di `/admin` untuk akun berperan `admin`.

| Halaman | Isi |
|---|---|
| Ringkasan | Angka pengguna, penyedia, listing, dan kategori; daftar hal yang perlu ditindak; grafik listing per kategori |
| Pengguna | Cari dan saring akun; verifikasi penyedia; tetapkan penyedia pilihan; tangguhkan dan aktifkan kembali; ubah peran |
| Listing jasa | Moderasi seluruh listing lintas penyedia; nonaktifkan/aktifkan; tetapkan listing pilihan; saring listing tanpa foto |
| Antrean | Tinjau listing baru dan perubahan penting: setujui, tolak dengan alasan, atau perbaiki sendiri |
| Kategori | Tambah, ubah, dan hapus kategori serta sub-kategorinya |
| Pengaturan | Nama situs, tagline, logo, kontak; gambar latar beranda & lencana aplikasi; titik pusat & radius; slot iklan |

Seluruh tindakan bersifat **reversible** — tidak ada penghapusan permanen atas
akun maupun listing. Penangguhan wajib disertai alasan, dan alasannya tersimpan.

Dampak penangguhan sebuah akun:

- seluruh sesi aktifnya langsung dicabut;
- tidak bisa masuk kembali, dengan pesan berisi alasannya;
- seluruh listing miliknya hilang dari pencarian dan halaman detail;
- statusnya sebagai penyedia pilihan ikut berhenti tampil di beranda.

**Penyedia pilihan** dan **listing pilihan** disetel manual oleh admin dengan
pilihan lama tayang (7/14/30/90 hari). Ini murni kontrol pengelola: belum ada
kaitan dengan pembayaran, sesuai batasan MVP. Listing yang sedang disorot naik
ke urutan teratas hasil pencarian; penyedia pilihan tampil di blok tersendiri
di beranda. Keduanya berhenti dengan sendirinya setelah masa tayang lewat —
kolom `featured_until` tidak perlu dibersihkan.

Keduanya dibedakan lewat warna kontainer kartu:

| Keadaan | Tampilan kartu |
|---|---|
| Sorotan jasa | Kuning keemasan — keluarga warna yang sama dengan bintang rating, karena keduanya berarti "diangkat ke depan" |
| Penyedia pilihan | Biru keunguan, warna merek aplikasi |
| Keduanya | Kuning keemasan; sorotan jasa lebih spesifik, jadi ia yang menang |

Masing-masing disertai pita tipis di tepi atas dengan warna yang sama. Nuansa
bidangnya sengaja sangat tipis: yang membedakan kartu tetap garisnya. Di mode
gelap keduanya menjadi permukaan gelap bernuansa, bukan versi lebih pucat,
karena tint terang justru menyilaukan di latar gelap.

Aturan "keduanya menang kuning" ditentukan **semata oleh urutan penulisan di
`source.css`**: kekhususan kedua kelas sama, jadi yang ditulis belakangan yang
berlaku. Menukar urutannya membalik aturan tanpa error apa pun, karena CSS-nya
tetap sah — satu uji penjaga mengunci urutan itu.

Warna tidak pernah jadi satu-satunya penanda: tiap kartu bersorot membawa teks
tersembunyi yang menyebutkan jenis sorotannya untuk pembaca layar.

Dua penjagaan yang selalu berlaku: admin tidak dapat menangguhkan atau mencabut
peran akunnya sendiri, dan admin aktif terakhir tidak dapat diturunkan
perannya — sehingga panel tidak pernah menjadi tidak bisa diakses siapa pun.

## Gambar unggahan

Setiap gambar melewati pipeline yang sama di `internal/service/gambar.go`:

| Tahap | Alasan |
|---|---|
| Periksa dimensi dari header | Menolak *decompression bomb* sebelum satu piksel dialokasikan (batas 50 MP) |
| Baca orientasi EXIF | Datanya hilang setelah penyandian ulang, jadi harus dibaca lebih dulu |
| Perkecil | Foto ponsel 4000 px tidak pernah ditampilkan seukuran itu |
| Sandikan ke WebP | Lebih kecil dari JPEG pada mutu setara; JPEG dipakai bila WebP gagal |
| Buat thumbnail | Kartu pencarian memuat belasan gambar sekaligus |

Ukuran per jenis unggahan diatur lewat `ProfilGambar`: listing 1600 px + thumbnail
480 px, avatar dan logo 400 px tanpa thumbnail.

Contoh terukur, foto 4000×3000 (764 KB):

| Varian | Ukuran | Dipakai di |
|---|---|---|
| Tampilan 1600 px WebP | 44 KB | Halaman detail jasa |
| Thumbnail 480 px WebP | 5 KB | Kartu di halaman pencarian |

Pemrosesan dibatasi berjalan paling banyak empat sekaligus, karena satu encode
mengalokasikan puluhan megabita sementara container produksi dibatasi 512 MB.

Konversi ini **tidak** mempercepat unggahan — transfer jaringan sudah selesai
sebelum server melihat berkasnya. Yang dipercepat adalah pemuatan halaman bagi
pengunjung, dan yang dihemat adalah ruang simpan.

Gambar yang diunggah sebelum pipeline ini ada tidak punya thumbnail; tampilan
jatuh kembali ke gambar aslinya lewat `COALESCE(thumb_url, image_url)`.

## Tampilan yang diatur admin

Beranda dan jalur kontak dikendalikan dari `/admin/pengaturan`, tanpa perlu
membangun ulang aplikasi.

| Pengaturan | Pengaruhnya |
|---|---|
| Gambar latar beranda | Hero memakai foto dengan lapisan gelap di atasnya. Kosong berarti hero kembali ke latar polos bertipografi |
| Kepekatan overlay | 25–85 persen. Dinaikkan untuk foto yang terang supaya judul dan kolom pencarian tetap terbaca |
| Tampilkan lencana aplikasi | Menyalakan lencana unduh di hero |
| Teks lencana (dua baris) | Bisa diubah, misalnya "Segera hadir di" / "Google Play" |
| Tautan Google Play | Boleh kosong selama aplikasinya belum terbit |
| Tampilkan tombol WhatsApp | Mematikannya membuat seluruh percakapan berlangsung di dalam aplikasi |

Lencana aplikasi dikendalikan sakelarnya sendiri, bukan oleh ada-tidaknya
tautan. Pemisahan itu disengaja: selama aplikasi Android belum terbit,
lencana "Segera hadir di Google Play" tetap layak ditampilkan. Saat tautannya
kosong, lencana dirender sebagai `<span>` — bukan `<a>` tanpa `href`, yang
masih dibacakan sebagai tautan oleh pembaca layar dan masih bisa difokus lewat
papan ketik meski tidak menuju ke mana pun.

Foto hero melewati pipeline gambar yang sama dengan foto listing: dikecilkan ke
lebar maksimum 1920 piksel dan dikonversi ke WEBP. Kualitasnya ditekan lebih
rendah daripada profil lain karena gambarnya tertutup overlay — cacat kompresi
tidak terlihat, sementara ukurannya turun drastis.

Warna teks hero tidak pernah ditulis sebagai style sebaris. Semuanya memakai
kelas (`hero-judul-aksen`, `hero-teks-lembut`, `hero-teks-samar`) yang nilainya
mengikuti tema pada keadaan biasa, lalu ditimpa menjadi terang ketika ada foto
latar. Tanpa pemisahan itu, warna sebaris akan mengalahkan penimpaan dan teks
gelap akan hilang di atas foto.

**Tombol WhatsApp mati secara bawaan.** Nomor WhatsApp penyedia tetap
dikumpulkan dan disimpan, jadi menyalakannya kembali tidak memerlukan
pengumpulan ulang. Satu uji penjaga memastikan setiap `TautanWhatsapp` di
template selalu berada di dalam pemeriksaan `Situs.WhatsappAktif` — tanpa itu,
satu tautan yang lupa dibungkus akan lolos diam-diam, karena halamannya tetap
dirender dan yang berubah hanya ke mana pengguna dibawa.

## Direktori penyedia

`/penyedia` menjawab pertanyaan yang berbeda dari `/cari`: bukan "jasa apa yang
tersedia", melainkan "siapa yang mengerjakannya di dekat sini". Kartunya
menampilkan nama, badge terverifikasi, rating, jumlah jasa, dan jarak dari
lokasi aktif pengunjung.

Penyaringnya: kata kunci (nama atau bio), kecamatan, radius, hanya
terverifikasi, dan hanya yang punya jasa aktif. Urutannya bisa dibalik ke
terdekat, rating tertinggi, atau yang terbaru bergabung — bawaannya paling
banyak jasa. Filter radius memakai `earth_box` dengan indeks GiST yang sama
seperti pencarian listing, jadi penyaringan jaraknya terjadi di indeks, bukan
setelah seluruh baris terbaca.

## Halaman penyedia

Tersedia di `/penyedia/<slug>`, misalnya `/penyedia/rizal-teknik-ac`. Slug
diturunkan dari nama penyedia saat profil dibuat dan **tidak ikut berubah**
ketika namanya diganti, sehingga tautan yang sudah dibagikan tetap berlaku.
Nama kembar dibedakan dengan akhiran angka.

Isi halamannya, urut mengikuti pertanyaan pencari jasa:

| Bagian | Menjawab |
|---|---|
| Kepala | Siapa orang ini — foto, badge terverifikasi, rating, lokasi dan jarak dari pengunjung, bio |
| Jasa | Apa yang ditawarkan — seluruh listing aktif |
| Portofolio | Apa buktinya — foto pekerjaan yang pernah diselesaikan |
| Ulasan | Kata siapa — rata-rata, sebaran bintang, dan ulasan terbaru |
| Samping | Cara menghubungi, area layanan, dan peta lokasi beserta radiusnya |

Pemilik dan admin melihat seluruh listing termasuk yang belum tayang, sehingga
halaman ini bisa dipakai memeriksa keadaan sebenarnya. Penyedia yang
ditangguhkan hilang dari halaman publik, sama seperti listing-listingnya.

**Portofolio** dikelola penyedia di `/provider/portofolio`, maksimal 24 karya,
melewati pipeline gambar yang sama dengan foto listing.

**Ulasan** hanya bisa ditulis pemesan, hanya setelah pesanan ditandai selesai,
dan satu pesanan menghasilkan tepat satu ulasan — dijaga indeks unik pada
`order_id`, sehingga dua permintaan bersamaan pun tidak menggandakannya.
Formnya muncul di ruang obrolan pesanan itu. Bintang penilaiannya memakai
radio yang seluruh tampilannya ditentukan CSS, tanpa JavaScript sama sekali.

Menulis ulasan menyegarkan `avg_rating` dan `total_reviews` di
`provider_profiles` — satu-satunya saat kolom cache itu dihitung ulang.

## Lokasi &amp; jarak

Penyedia menentukan titik lokasinya dengan menggeser pin di peta, ditambah
radius layanan — sejauh mana ia bersedia mendatangi pelanggan. Pencari jasa
menentukan titik acuannya lewat GPS peramban atau memilih kecamatan; pilihan
itu disimpan di cookie dan dipakai untuk menghitung jarak.

Di hasil pencarian, jarak menggantikan nama wilayah pada kartu listing, hasil
bisa disaring per radius, dan urutan "terdekat" tersedia. Penyedia yang berada
di luar radius layanannya sendiri tetap ditampilkan tetapi diberi keterangan
jujur bahwa ia belum tentu bersedia datang.

Peta memakai Leaflet yang di-self-host; hanya gambar tile-nya yang diambil dari
OpenStreetMap. Karena itu `img-src` pada CSP dilonggarkan khusus untuk domain
tile — `script-src` tetap `'self'` tanpa `unsafe-eval`. Bila peta gagal dimuat
atau pengguna menolak akses GPS, tersedia daftar kecamatan sebagai jalan keluar.

## Persetujuan listing

Listing baru berstatus **menunggu** dan belum tayang sampai admin menyetujui.
Setelah tayang, provider tetap bebas mengedit — dengan satu aturan:

| Yang diubah | Akibatnya |
|---|---|
| Harga saja | Tetap tayang |
| Judul, deskripsi, atau kategori | Kembali ke antrean, sementara tidak tayang |
| Foto (tambah/hapus) | Kembali ke antrean, sementara tidak tayang |
| Apa pun, oleh admin | Tetap tayang — admin adalah peninjaunya |

Aturan ini menutup *bait-and-switch* (listing lolos dengan konten bersih lalu
diganti isinya) tanpa menghambat perbaikan kecil seperti memperbarui harga.
Penolakan wajib disertai alasan, alasannya tampil di halaman listing bagi
pemiliknya, dan perbaikan yang disimpan otomatis diajukan ulang.

## Uji alur end-to-end

Perakitan aplikasi — koneksi, service, middleware, rute — kini hidup di
`internal/app` sebagai `app.New(ctx, cfg, versi)`; `main.go` tinggal
memanggilnya lalu mendengarkan. Pemisahan ini yang membuat alur utuh bisa
diuji tanpa menjalankan binary: uji membangun aplikasi yang sama persis dengan
produksi, lalu mengirim permintaan HTTP lewat `app.Test`.

```bash
make up        # Postgres & Redis harus berjalan
make test-e2e
```

Uji berjalan di jaringan compose pada **database terpisah** bernama
`<POSTGRES_DB>_e2e` yang dibuat ulang dari nol setiap kali (migrasi + seed),
dan Redis DB 15 yang dikosongkan lebih dulu. Ia menolak berjalan bila nama
database-nya tidak berakhiran `_e2e`, sehingga tidak mungkin menyentuh data
asli. Tanpa `E2E=1` (mis. `make test` biasa) seluruh uji ini dilewati.

`TestAlurUtama` menelusuri satu transaksi dari kedua sisi: pencari jasa
mendaftar → penyedia mendaftar tanpa nomor WhatsApp → memasang jasa (masuk
antrean, belum tayang di pencarian) → admin menyetujui (tayang) → pencari
memesan (percakapan terbentuk, isi permintaan jadi pesan pertama) → penyedia
menerima lalu menyelesaikan → pencari mengulas (rating penyedia terhitung,
ulasan tampil di halaman publik) → kontrak API (tanpa `whatsapp_number`,
menolak POST tanpa header CSRF). `TestHalamanPublik` memastikan halaman tanpa
login terender dan halaman terlindung mengalihkan atau menyembunyikan diri
sebagaimana dirancang.

Sebelum ini handler berada di 0% cakupan uji dan tidak ada satu pun uji yang
melewati lebih dari satu lapisan. Uji unit tetap menjaga potongan logika; uji
ini menjaga bahwa potongan-potongan itu masih tersambung.

## `.env.example` dijaga dua arah

Tiga uji di `internal/config` membaca kode dan berkas konfigurasi lalu
mencocokkannya:

- setiap kunci yang dibaca `config.go` atau `main.go` harus ada di
  `.env.example` — kunci yang tidak terdokumentasi jatuh ke bawaannya di
  server tanpa ada yang tahu bahwa ia bisa disetel;
- setiap kunci di `.env.example` harus dibaca kode, Makefile, atau compose —
  kunci yang tidak dibaca siapa pun hanya membingungkan orang yang menyalinnya;
- setiap `${VAR}` di berkas compose harus ada di `.env.example` — variabel
  compose berasal dari `.env`, dan yang hilang menjadi string kosong diam-diam
  saat `make up` di server baru.

Penjaga ini langsung menemukan dua kunci nyata saat pertama dijalankan:
`POSTGRES_HOST` dan `REDIS_HOST` dibaca kode tetapi tidak pernah tertulis.

## Promosi: iklan, sorotan jasa, penyedia pilihan

Ketiganya berbagi **satu tabel `promosi`** dengan kolom `jenis`, satu mesin
status, dan nantinya satu antrean admin — bukan tiga subsistem. Paket
(`paket_promosi`) menentukan durasi, harga, bobot rotasi, dan slot penempatan
(khusus iklan); admin mengelolanya di `/admin/paket`, seeder menanam paket
bawaan dengan harga nol.

Mesin statusnya ada di `model.BolehTransisi` sebagai tabel murni — status asal
→ tujuan → aktor yang berhak — dan service hanya memeriksanya:

```
menunggu ──tolak──▶ ditolak            (admin, wajib catatan)
   │──setujui──▶ disetujui ──bayar dikonfirmasi──▶ aktif ──habis──▶ selesai
   │              (gratis / uji coba gratis: langsung aktif)   │
   └──batal (pemohon, hanya sebelum aktif)──▶ dibatalkan       └─jeda ⇄ aktif, hentikan
```

`featured_until` di listing dan penyedia — yang sudah menggerakkan urutan
pencarian, kartu berwarna, dan blok beranda — kini **turunan** dari promosi
aktif, dijaga trigger `sinkron_featured_promosi` yang menghitung ulang dari
seluruh baris aktif yang tersisa. Sorotan yang ditetapkan admin sebelum sistem
ini ada dipindahkan menjadi promosi bersumber `admin` saat migrasi. Sampai
Fase 3, tombol sorot sekali-klik yang lama masih menulis `featured_until`
langsung; setelah itu ia lewat mekanisme yang sama.

Yang dijaga database, bukan kode: satu pengajuan hidup per listing dan per
penyedia (indeks unik parsial), penolakan wajib beralasan, promosi aktif wajib
berjadwal, iklan wajib berjudul (CHECK). Paket yang pernah dipakai tidak bisa
dihapus (FK RESTRICT) — dinonaktifkan saja.

**Pengajuan (Fase 2).** `/promosi` memuat seluruh pengajuan pengguna;
`/promosi/baru?jenis=…` satu form yang isinya mengikuti jenis — sorotan memilih
jasa milik sendiri yang sudah tayang, penyedia pilihan menulis alasan singkat,
iklan mengisi judul, gambar (pipeline WebP), tautan, dan kecamatan sasaran
(boleh lebih dari satu; kosong = seluruh kabupaten). `/promosi/:id`
menampilkan status, jadwal, tayang/klik, catatan penolakan, dan — bila
disetujui, berbayar, dan uji coba gratis mati — petunjuk transfer beserta form
bukti pembayaran. Pemohon boleh membatalkan hanya sebelum tayang.

Batas: satu pengajuan hidup per listing/penyedia (indeks unik DB), maksimal 3
iklan hidup per akun, hanya jasa yang sudah tayang yang bisa disorot. Iklan
boleh diajukan siapa pun yang terdaftar; sorotan & penyedia pilihan hanya
penyedia — pengguna biasa dialihkan ke pembuatan profil dengan pesan, bukan
403. Setiap perubahan status oleh admin atau sistem mengirim notifikasi
`promosi_diperbarui` ke pemohon. Pintu masuk: blok "Promosikan" di dasbor,
tautan "Sorot" di kartu listing yang belum disorot, dan tombol "Ajukan jadi
penyedia pilihan" di halaman penyedia milik sendiri.

**Peninjauan admin (Fase 3).** `/admin/promosi` adalah antrean dengan empat
tab yang mengikuti tahap keputusan — *Menunggu*, *Menunggu bayar*, *Tayang*,
*Riwayat* — dan saringan per jenis; yang paling lama menunggu tampil lebih
dulu. Badge sidebar menghitung pengajuan baru **dan** bukti bayar yang belum
dikonfirmasi. Halaman detail menampilkan pemohon, sasaran, paket, materi,
alasan, bukti bayar, dan hanya merender tombol yang sah pada status saat ini
(`model.BolehTransisi`): setujui, tolak (wajib alasan — form dirender ulang
dengan galat di kolomnya, bukan dialihkan), konfirmasi pembayaran, jeda,
lanjutkan, hentikan.

Tombol sorot sekali-klik yang lama di daftar listing dan pengguna **tetap
ada**, tetapi kini membuat promosi bersumber `admin` lewat
`PromosiService.TetapkanAdmin` — riwayatnya satu dengan pengajuan pengguna,
dan `featured_until` tetap hanya ditulis trigger. Mencabut (`hari=0`)
menghentikan sorotan yang sedang tayang; pengajuan pengguna yang masih
menunggu tidak disentuh — admin memutuskannya di antrean, bukan diam-diam
lewat tombol cabut. Menyorot sasaran yang sudah punya pengajuan hidup ditolak
dengan pesan, bukan ditimpa.

Bagian **Pembayaran promosi** di `/admin/pengaturan` memuat sakelar uji coba
gratis dan rekening tujuan; mematikan uji coba tanpa rekening ditolak, karena
pemohon yang disetujui akan diminta membayar ke mana-mana.

**Penayangan iklan (Fase 4).** Untuk tiap slot yang dirender, `model.PilihIklan`
memilih satu iklan dalam tiga tahap berurutan: **paket** menentukan slot mana
yang boleh (iklan Dasar tidak pernah muncul di beranda atas), **kecamatan
penonton** menyaring (diturunkan dari lokasi aktif → kecamatan terdekat;
penonton yang belum memilih lokasi melihat semua), lalu **rotasi berbobot**
per tampilan halaman — bobot 3 berarti tiga dari empat tampilan. Rotasinya
per tampilan, bukan carousel bergantian: tanpa JavaScript, tanpa permintaan
tambahan, dan secara statistik sama adilnya.

Daftar iklan aktif di-cache utuh (`promosi:iklan-tayang`, dibersihkan setiap
status bergerak) dan pemilihannya terjadi di Go: **nol query per tampilan**.
Pemilihan berjalan tepat saat slot dirender — `Base.PilihIklan` adalah
fungsi, bukan nilai — supaya tayangan hanya dihitung untuk slot yang memang
ada di halaman. Tidak ada iklan berbayar yang layak → slot jatuh ke **materi
bawaan** dari pengaturan.

Tayangan dihitung `INCR` di Redis dan disalin ke kolom `tayang` tiap menit;
klik lewat `/iklan/:id/klik` (+1, lalu dialihkan ke tautan pengiklan). Satu
goroutine latar di `app.New` menjalankan penyalinan itu dan menutup promosi
yang masa tayangnya habis tiap sepuluh menit — keduanya idempoten, tanpa
container cron. Blok **Penyedia pilihan** di beranda diambil hingga 12 lalu
dipilih 4, mengutamakan yang sekecamatan dengan penonton dan menggilir sisanya.

**Statistik & pengingat (Fase 5).** Dasbor admin merangkum promosi dalam
satu query agregat: yang sedang tayang per jenis, tayangan & klik seluruh
iklan aktif beserta CTR, yang menunggu (ditinjau / bayar), dan yang berakhir
dalam 30 hari; pengajuan baru dan bukti bayar masuk ikut di blok "Perlu
perhatian". Halaman pengiklan menambahkan rasio klik dan rata-rata tayang per
hari — pembanding yang adil antara iklan yang baru sehari dan yang sudah
sebulan — dengan catatan bahwa iklan bertarget kecamatan wajar tayangannya
lebih rendah. Ticker yang sama mengirim **pengingat H-1** ke pemohon sehari
sebelum masa tayang habis, tepat satu kali per promosi (kolom
`diingatkan_at`, migrasi 000022), supaya sempat mengajukan lagi.

Sakelar **uji coba gratis** (`pengaturan.promosi.uji_coba_gratis`, bawaan
menyala) membuat setiap pengajuan yang disetujui langsung tayang; jalur
pembayaran manual sudah ada di data dan mesin status, tinggal dimatikan saat
harga diberlakukan. Form pengajuan pengguna, antrean admin, dan penayangan
iklan menyusul di fase berikutnya.

## Pencarian yang tidak kaku

Kata kunci dipecah per kata. Tiap kata harus cocok — sebagai substring **atau**
sebagai kata yang mirip menurut trigram `pg_trgm` (`word_similarity ≥ 0,45`) —
di teks gabungan judul, kategori (termasuk induknya), nama penyedia, dan
deskripsi. Semua kata wajib cocok, tetapi urutan dan jaraknya bebas: "service
ac", "ac servis", dan salah ketik "servise" sama-sama menemukan jasa di
kategori Servis AC. Kata penghubung ("di", "dan", "untuk") dan kata di bawah
dua huruf diabaikan.

Ambangnya diambil dari angka nyata, bukan tebakan: "service"~"servis" 0,63 dan
"servise"~"servis" 0,75 harus lolos; "pipa"~"atap bocor" 0,2 harus gagal.

Tanpa pilihan urutan, hasil diurutkan menurut kemiripan kata kunci terhadap
judul dan kategori saja — bukan deskripsi — supaya jasa yang memang itu
namanya naik di atas jasa yang cuma menyebutnya sambil lalu. Listing pilihan
tetap paling atas.

Hasil pencarian di-cache 5 menit. Setelah men-deploy perubahan pada logika
pencarian, jalankan `make cache-clear` supaya hasil lama tidak tersaji sampai
cache-nya kedaluwarsa sendiri.

`word_similarity()` dalam bentuk fungsi tidak memakai indeks; pada skala
kabupaten (ratusan sampai ribuan listing yang sudah tersaring status aktif) itu
tidak terasa. Bila kelak melewati puluhan ribu baris, ganti ke operator `<%`
dengan `SET pg_trgm.word_similarity_threshold` supaya indeks GIN trigram yang
sudah ada ikut bekerja.

## Pool koneksi diperiksa saat menyala

Saat aplikasi dinyalakan, ukuran pool (`POSTGRES_MAX_CONNS`) dibandingkan
dengan `max_connections − superuser_reserved_connections` di server. Melebihi
→ aplikasi menolak menyala dengan pesan yang menyebut ketiga angkanya; lebih
dari separuh → peringatan di log, karena migrasi, alat admin, dan instance
kedua semuanya memakai server yang sama. Ketidakselarasan seperti ini tidak
terasa saat sepi dan baru meledak sebagai "too many clients" di tengah puncak —
lebih baik ketahuan di detik pertama.

## Area layanan diturunkan, bukan diketik

Dulu ada tiga cara menyatakan "di mana" seorang penyedia: kecamatan domisili
di akun, teks bebas `service_area` di profil, dan pin lokasi + radius. Teks
bebasnya tidak pernah sinkron dengan radius — penyedia menulis "Bengkalis,
Bantan" lalu menggeser radius ke 5 km, dan keduanya tampil berdampingan tanpa
saling tahu.

Kolomnya dihapus (migrasi 000019). Area layanan kini **diturunkan** oleh
`ProviderProfile.AreaLayanan`: kecamatan yang pusatnya berada dalam radius
layanan dari pin, diurutkan dari yang terdekat. Bila radiusnya lebih kecil
daripada jarak ke pusat kecamatan mana pun, kecamatan terdekat tetap disebut
supaya daftarnya tidak pernah kosong bagi yang sudah menaruh pin. Tanpa pin,
jatuh ke kecamatan domisili. Satu sumber kebenaran: pin di peta.

Karena itu dasbor penyedia kini menampilkan ajakan **"Tandai lokasi"** di atas
daftar listing selama pinnya belum ada — tanpa pin, jasanya tidak ikut
terhitung dekat bagi siapa pun dan area layanannya tidak bisa diturunkan.

## `is_provider` ditulis satu pihak saja

`users.is_provider` adalah denormalisasi dari keberadaan baris
`provider_profiles`. Sebelumnya kode aplikasi yang menyalakannya dalam
transaksi yang sama — benar, tetapi dua sumber kebenaran tanpa penjaga akan
menyimpang begitu ada jalur lain (seeder, perbaikan manual lewat SQL,
penghapusan profil kelak).

Migrasi 000020 memasang trigger `AFTER INSERT OR DELETE` pada
`provider_profiles` yang menyalakan dan mematikan penanda itu, meluruskan
data yang sudah ada, dan kode aplikasi berhenti menyentuh kolomnya sama sekali.
Uji e2e memverifikasi kedua arah lewat SQL langsung — jalur yang persis
melewati kode aplikasi — lalu memeriksa invarian menyeluruh: tidak satu pun
pengguna yang penandanya berbeda dari keberadaan profilnya.

## Keadaan kosong yang mengajak bertindak

Halaman penyedia dan dasbor punya tiga bagian yang sering kosong bagi penyedia
baru — jasa, portofolio, ulasan. Masing-masing kini membedakan siapa yang
melihat:

| Bagian | Pemilik | Pengunjung |
|---|---|---|
| Jasa | ajakan memasang jasa pertama | ajakan melihat penyedia lain |
| Portofolio | ajakan menambah karya | **bagian disembunyikan** — judul tanpa isi hanya menegaskan kekurangan |
| Ulasan | penjelasan bahwa ulasan datang dari pesanan yang diselesaikan, tautan ke pesanan | ajakan jadi yang pertama memesan (bila ada jasa dan sudah masuk) |

## Pengaturan yang punya satu makna

**Deskripsi beranda** (`Tagline` di pengaturan) adalah `<meta name="description">`
halaman utama — kalimat di bawah judul pada hasil pencarian Google. Sebelumnya
field ini tersimpan tetapi tidak dibaca template mana pun, sementara kalimat
metanya hidup sebagai salinan di kode; keduanya kini satu. Teks footer tetap
field terpisah karena pembacanya berbeda: pengunjung yang sudah di halaman,
bukan mesin pencari.

**Nomor WhatsApp penyedia opsional.** Percakapan berlangsung di dalam aplikasi,
dan data wajib yang tidak dipakai hanya jadi beban pendaftaran. Bila diisi,
formatnya tetap diperiksa — tombol WhatsApp, kalau nanti dinyalakan, tidak
boleh menuju nomor yang salah — dan tombolnya hanya dirender bila sakelarnya
menyala **dan** nomornya ada. Kolomnya tetap `NOT NULL` dengan bawaan string
kosong (migrasi 000018), bukan `NULL`: seluruh kode membaca nomor sebagai
string, dan kosong sudah cukup berarti "tidak diisi".

## Peta hanya dimuat di halaman berpeta

Leaflet (≈150 KB JS + CSS, di-self-host) dimuat hanya bila handler menandai
halamannya dengan `DenganPeta()` — tiga halaman: lokasi penyedia, profil publik
penyedia, dan pengaturan admin. Sebelumnya ia dimuat di setiap halaman,
sehingga sembilan dari sepuluh tampilan membayar untuk skrip yang tidak
dipakai. Penandanya harus di handler, karena layout dirender lebih dulu
daripada isi halaman dan tidak bisa tahu sendiri apakah di bawahnya ada peta.

Halaman berpeta yang handler-nya lupa menandai tidak gagal di mana pun —
petanya cuma tidak muncul, karena `app.js` memang diam bila `L` tidak ada.
Satu uji penjaga membaca daftar halaman berpeta dari template (`data-peta`)
dan memastikan tiap handler yang merendernya memanggil `DenganPeta()`.

## Cache pencarian per sel ~1 km

Hasil pencarian berlokasi di-cache per **sel 0,01° (≈1,1 km)**, bukan per
titik GPS. Dengan pembulatan 100 m sebelumnya, tiap pengguna GPS jatuh di
selnya sendiri dan cache nyaris tak pernah kena — tepat di query termahal
(`earth_box`). Sel dipakai untuk kunci cache dan pusat query saja.

Jarak yang dilihat pengguna **tidak** memakai sel: titik persisnya disimpan,
lalu jarak tiap hasil dihitung ulang darinya setelah keluar cache
(`HitungUlangJarak`), dan diurutkan ulang bila pengguna meminta "terdekat".
Dua orang di sel yang sama berbagi satu hasil query tetapi melihat angka
jaraknya masing-masing. Satu-satunya efek pembulatan ada di tepi saringan
radius: sekitar 0,8 km pada radius 5–100 km, yang tidak terasa.

## Menu utama (tombol melayang) & pasang aplikasi

Tombol melayang di kanan bawah membuka *sheet* berisi peta lengkap aplikasi:
aksi cepat, promosi, akun, aplikasi & bantuan — isinya kontekstual (tamu,
pengguna, penyedia, admin) dan disusun `view.MenuUtama`, sumber yang sama
dengan API Android. Sheet adalah elemen `dialog` bawaan peramban (perangkap
fokus, Esc, latar gelap gratis); JavaScript-nya hanya buka/tutup dan
sinkron badge. Di ponsel ia naik dari bawah, di desktop menjadi kartu di dekat
tombol (padanan Flutter: `FloatingActionButton` + `showModalBottomSheet`).
Tombol menyingkir sendiri lewat CSS saat pengguna mengetik, dan tidak dirender
di ruang obrolan.

"Pasang aplikasi" punya dua wajah dari satu pengaturan admin: bila tautan
Play Store diisi, item membuka toko; bila belum, web menawarkan pemasangan
PWA lewat `/manifest.webmanifest` (dibuat server dari pengaturan situs, tanpa
service worker) dan item baru tampil ketika peramban benar-benar menawarkan
pemasangan. Ikon PNG-nya dihasilkan `make ikon` dari `favicon.svg` dan warna
di `tokens.json`.

## Halaman Tentang & Panduan

`/tentang` (alias `/bantuan` → `/tentang#cara-kerja`) dirender dari
`view.Panduan`: data terstruktur berisi enam bagian ber-anchor tetap
(`tentang`, `cara-kerja`, `fitur`, `promosi`, `faq`, `kontak`), bukan markup.
Struktur yang sama dikirim ke Android lewat `GET /api/v1/help`, jadi mengubah
satu kalimat memperbarui kedua platform. Angka hidup (jasa aktif, penyedia,
kecamatan) datang dari `Catalog.Ringkasan` yang di-cache 10 menit, dan harga
paket promosi ditempel dari tabel `paket_promosi` supaya tidak pernah basi.
FAQ memakai elemen `details` tanpa JavaScript. `GET /api/v1/config` kini juga
membawa `menu` — isi menu utama untuk peran pemanggil, sumber yang sama
dengan sheet di web.

## Token desain: satu sumber untuk web dan Android

`web/static/design/tokens.json` memegang seluruh warna (terang/gelap),
bayangan, radius, spasi, skala huruf, dan ukuran komponen. `tailwind.config.js`
membacanya: skala Tailwind diambil langsung, dan blok `:root` / `.dark`
dihasilkan lewat plugin — `source.css` tidak lagi menulis variabel warna.
Aplikasi Android membaca berkas yang sama untuk `ThemeData`.

Ubah token → `make css` (atau `make build`, yang membangun CSS di dalam
image). Uji penjaga `TestTokensJSONSumberTunggal` gagal bila mode terang dan
gelap tidak sepadan, bila template memakai `var(--x)` yang tidak ada di
tokens, atau bila `app.css` tertinggal dari tokens.

Dua sumber data lain yang sengaja dipakai web dan API bersama-sama:
`view.MenuUtama` (isi menu utama menurut peran) dan `view.Panduan` (isi
halaman Tentang & Panduan). Keduanya diuji di `internal/view`.

Set ikon antarmuka hidup di `icons.templ`; `make ikon` mengekspornya sebagai
SVG mandiri per nama ke `web/static/design/ikon/` (plus `index.json`) untuk
aset Flutter, dan uji `TestEksporIkonSelaras` memastikan ekspor tidak
tertinggal. Kamus komponen beserta padanan widget Flutter-nya ada di
[`docs/design-system.md`](docs/design-system.md).

## API untuk aplikasi Android

Kontraknya ada di [`docs/openapi.yaml`](docs/openapi.yaml). Seluruh endpoint
memanggil service yang sama dengan situs web — tidak ada logika bisnis yang
hidup dua kali. Inti kontraknya:

- **Bearer token.** `POST /api/v1/auth/login` mengembalikan `token`, yaitu ID
  sesi yang sama dengan cookie web; kirim sebagai `Authorization: Bearer`.
  Karena satu sesi di Redis, `logout-all` dan penangguhan akun mencabut
  keduanya sekaligus. Permintaan ber-`Authorization` dilewatkan penjaga CSRF
  — peramban tidak pernah menambahkan header itu sendiri.
- **`GET /config`** adalah satu panggilan saat aplikasi dibuka: `base_url`
  (semua `*_url` di respons bersifat relatif), identitas situs, hero, pusat &
  radius, 11 kecamatan berkoordinat, pohon kategori, paket promosi, rekening.
- **Pencarian sadar-lokasi** lewat `lat`, `lng`, `radius`, `terjangkau`, dan
  `urut=terdekat` — aturan cache sel ~1 km dan hitung-ulang jarak yang sama.
- **Obrolan**: `GET /conversations/{id}/stream` adalah Server-Sent Events dari
  broker dalam proses (`service.Siaran`) yang disuplai `ChatService.Kirim` —
  pesan dari web maupun API sama-sama tersiar. Ditutup server tiap 5 menit;
  klien menyambung ulang dan mengejar lewat `messages?since=`.
- **Push** lewat FCM HTTP v1 **tanpa SDK**: JWT service account ditandatangani
  `crypto/rsa` bawaan lalu ditukar token OAuth2. Setiap notifikasi yang
  tersimpan diteruskan lewat hook `NotificationRepository.SetelahBuat` —
  tidak satu pun pemanggil `Create` berubah — dengan judul dari
  `view.IsiNotifikasi` yang sama dengan halaman web. Kosongkan
  `FCM_PROJECT_ID`/`FCM_SERVICE_ACCOUNT_FILE` untuk mematikannya; token
  UNREGISTERED dibuang otomatis.
- **Iklan** lewat `GET /ads?slot=…` memakai pemilih yang sama (paket →
  kecamatan → rotasi berbobot), tayangan dihitung di sana karena aplikasi
  memang akan menampilkannya.

Uji e2e `TestAPIUntukAndroid` meniru aplikasi native tanpa cookie sama sekali.

## Keamanan API &amp; pembatasan laju

**Setiap permintaan API yang mengubah data (POST, PATCH, DELETE) wajib membawa
header `X-Requested-With`** — nilainya bebas, lazimnya `XMLHttpRequest`. Tanpa
itu server menjawab 403. Ini penjaga CSRF untuk `/api/v1`: API memakai cookie
sesi yang sama dengan halaman web, sedangkan form HTML tidak punya token untuk
endpoint JSON. Form lintas-situs tidak bisa menambah header kustom, dan fetch
lintas-situs yang membawanya memicu preflight CORS yang tidak pernah diizinkan
— jadi kehadiran header itu saja sudah membuktikan asal permintaannya, tanpa
token yang perlu disimpan dan diputar. GET tidak terpengaruh.

```bash
curl -X POST http://localhost:3000/api/v1/auth/login \
  -H 'Content-Type: application/json' -H 'X-Requested-With: XMLHttpRequest' \
  -d '{"identifier":"0812xxxxxxx","password":"..."}'
```

**Nomor WhatsApp penyedia tidak keluar lewat API publik** (`/services/:id`,
`/providers/:id`) selama sakelar WhatsApp di pengaturan mati. Halaman HTML
sudah menggerbang tombolnya, tetapi JSON memuat struct apa adanya — tanpa
gerbang ini, nomor pribadi terbaca siapa pun yang membuka endpoint. Endpoint
milik penyedia sendiri (buat/ubah profil) tetap mengembalikannya.

**Unggahan dibatasi 40 berkas per menit per akun**, menyeluruh untuk semua
rute berkas (foto listing, portofolio, logo, hero, materi iklan, dan API).
Kuncinya akun, bukan IP: pengguna di balik satu NAT kantor atau operator
seluler berbagi IP, dan satu orang yang rajin mengunggah tidak boleh menutup
jalan semua orang di jaringan yang sama. Pengunjung tanpa akun jatuh ke IP.
Melampaui kuota menghasilkan 429 — halaman galat di web, amplop JSON
`rate_limited` di API. Satu uji penjaga memindai handler yang membaca berkas
dan memastikan setiap rutenya memasang pembatas ini; rute unggah baru yang
lupa memasangnya tidak gagal di mana pun, ia cuma jadi jalan masuk tanpa
penjaga.

**Penjaga konfigurasi produksi.** Dengan `APP_ENV=prod`, aplikasi menolak
menyala bila `SESSION_COOKIE_SECURE` bukan `true` atau `SESSION_SECRET` masih
nilai contoh — salah konfigurasi lebih baik berhenti di detik pertama daripada
diam-diam mengirim cookie sesi lewat HTTP. HSTS (180 hari) dikirim hanya di
produksi; di lokal tanpa HTTPS ia justru mengunci peramban dari localhost.

**Pendaftaran dibatasi** 10 per menit per IP (web dan API) — longgar untuk
kantor yang berbagi IP, cukup untuk menutup skrip pembuat akun.

**Bukti transfer bersifat pribadi.** Berkas di `/uploads/bukti/…` hanya bisa
dibuka pemohonnya dan admin; pengunjung lain mendapat 404, diperiksa sebelum
handler statis karena handler statis tidak tahu apa-apa soal sesi.

## Pesan &amp; pesanan

Percakapan berpangkal dari listing: satu pencari jasa punya satu ruang obrolan
per listing, sehingga riwayatnya tidak terpecah. Permintaan order membuat
pesanan sekaligus menautkannya ke ruang obrolan, dan isi permintaan dikirim
sebagai pesan pertama agar penyedia melihat konteksnya langsung.

Ruang obrolan memeriksa pesan baru tiap 3 detik lewat HTMX. Tanda "sudah
dibaca" hanya ditulis bila poll itu memang membawa pesan baru — sebelumnya
`UPDATE … WHERE read_at IS NULL` dijalankan setiap poll, 1.200 write per jam
per ruang yang sunyi, tanpa satu pun yang mengubah apa-apa. Diverifikasi di
tingkat statement lewat `log_statement = 'mod'`: 15 poll ruang sunyi
menghasilkan nol `UPDATE`.

Panel kontak di halaman jasa disusun mengikuti nilai tindakannya: **Kirim
permintaan order** berada paling atas sebagai tombol utama seukuran penuh,
**Kirim pesan dulu** di bawahnya sebagai tombol sekunder. Formulir ordernya
tetap dibungkus `<details>` supaya panelnya ringkas, tetapi pemicunya berupa
tombol — bukan tautan teks seperti sebelumnya, yang membuat tindakan
terpenting justru paling samar.

Alur status pesanan dijaga di service layer: penyedia yang menerima, menolak,
dan menyelesaikan; pencari jasa hanya dapat membatalkan. Transisi mundur
ditolak. Admin **tidak** dapat membaca isi percakapan — moderasi platform tidak
memerlukan akses ke obrolan pribadi antar pengguna.

## Deploy ke server

Rantai trafik di produksi: **Cloudflare (proxied) → Nginx Proxy Manager
(NPM) → container `app`**. Aplikasi tidak mempublikasikan port apa pun ke
host; NPM bergabung ke jaringan Docker `${DOCKER_NETWORK}` (subnet
`${DOCKER_SUBNET}`) dan meneruskan ke `adojobsid-app-1:3000`. Karena itu
tidak ada port host yang perlu dipilih atau bisa bentrok dengan stack lain.

### 1. Menyiapkan server (sekali)

Diuji pada Ubuntu 24.04, 2 vCPU, 4 GB RAM, Docker 29 / Compose v5.

```bash
# swap 2 GB sebagai pengaman OOM — VPS kecil tanpa swap mematikan proses
# secara acak saat memori penuh
sudo fallocate -l 2G /swapfile && sudo chmod 600 /swapfile && sudo mkswap /swapfile && sudo swapon /swapfile && echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
# lepaskan image/cache build yang tak terpakai (disk sebaiknya < 70%)
docker system df && docker system prune -f --filter "until=720h"
# folder aplikasi & rahasia
sudo mkdir -p /opt/adojobsid/secrets && sudo chown -R $USER:$USER /opt/adojobsid
```

### 2. `.env` produksi

Salin `.env` lokal ke `/opt/adojobsid/.env` lalu ubah nilai berikut:

| Kunci | Nilai produksi |
|---|---|
| `APP_BASE_URL` | `https://<domain>` |
| `APP_TRUSTED_PROXIES` | sama dengan `DOCKER_SUBNET`, mis. `172.23.0.0/24` |
| `APP_PROXY_HEADER` | `CF-Connecting-IP` |
| `DOCKER_SUBNET` | subnet yang belum dipakai `docker network ls` (server ini: 172.17–172.22 terpakai → `172.23.0.0/24`) |
| `SESSION_SECRET`, `POSTGRES_PASSWORD`, `REDIS_PASSWORD` | nilai acak baru, bukan dari lokal |
| `DEPLOY_HOST`, `DEPLOY_USER`, `DEPLOY_PATH` | tujuan SSH; user harus anggota grup `docker` |
| `FCM_SERVICE_ACCOUNT_FILE` | `/app/secrets/fcm.json` bila push dipakai (taruh berkasnya di `/opt/adojobsid/secrets/`) |

`APP_ENV=prod` dan `SESSION_COOKIE_SECURE=true` dipaksa oleh
`docker-compose.prod.yml`; aplikasi menolak menyala bila `SESSION_SECRET`
masih nilai contoh.

### 3. Deploy pertama

Dari Mac, di folder proyek (`.env` lokal berisi `DEPLOY_HOST` dsb.):

```bash
make deploy
```

Target itu membangun image `linux/amd64`, menyalin berkas compose, mengirim
image lewat SSH (tanpa registry), menjalankan migrasi, lalu `up`. Setiap
deploy juga menandai image dengan stempel waktu untuk rollback.

Setelah container hidup, sambungkan NPM ke jaringan aplikasi (sekali):

```bash
ssh <user>@<server> 'docker network connect adojobsid_internal nginx-proxy-manager'
```

### 4. Nginx Proxy Manager

Buat **Proxy Host**: domain → scheme `http`, forward host
`adojobsid-app-1`, port `3000`, *Block Common Exploits* aktif, *Websockets
Support* tidak perlu. SSL: sertifikat Let's Encrypt, *Force SSL*, *HTTP/2*.
Di tab **Advanced** tambahkan, karena ruang obrolan memakai Server-Sent
Events yang tidak boleh ditahan buffer nginx:

```nginx
proxy_buffering off;
proxy_read_timeout 120s;
client_max_body_size 40m;
```

Bila domain di Cloudflare berstatus *proxied*, batasi proxy host ini hanya
untuk rentang IP Cloudflare (NPM → *Access Lists*, atau firewall server),
supaya `CF-Connecting-IP` tidak bisa dipalsukan dengan mengakses origin
langsung. Mode SSL Cloudflare: **Full (strict)**.

### 5. Setelah hidup

```bash
make admin-create ENV=prod      # admin pertama dari ADMIN_PHONE/ADMIN_PASSWORD di .env server
make logs-app ENV=prod
make releases                   # daftar tag image di server
make rollback TAG=20260924-1015
make backup ENV=prod            # database + arsip foto ditarik ke backups/ di Mac
```

Cadangan terjadwal di server (database + volume foto), tiap hari 02:00:

```bash
( crontab -l 2>/dev/null; echo '0 2 * * * cd /opt/adojobsid && docker compose -f docker-compose.yml -f docker-compose.prod.yml exec -T postgres pg_dump -U adojobs -d adojobs --clean --if-exists | gzip > /opt/adojobsid/backups/db-$(date +\%F).sql.gz && docker run --rm -v adojobsid_uploads:/u:ro -v /opt/adojobsid/backups:/b alpine tar czf /b/uploads-$(date +\%F).tgz -C /u . && find /opt/adojobsid/backups -mtime +14 -delete' ) | crontab -
```

Cron itu menyimpan cadangan di server yang sama, dan itu bukan cadangan
sesungguhnya. Tarik salinannya ke luar server secara berkala dengan
`make backup ENV=prod` dari Mac (atau rclone ke R2/Drive bila nanti ada).

## Status pengerjaan

Tahap 1 — **selesai**: registrasi dan login, profil penyedia, buat dan ubah
listing jasa lengkap dengan foto, pencarian dengan filter kategori dan lokasi,
halaman detail jasa, mode gelap.

Panel admin — **selesai**: kelola pengguna dan penyedia, moderasi listing,
kelola kategori, penyedia dan listing pilihan, dasbor statistik, antrean
persetujuan, dan pengaturan aplikasi.

Lokasi &amp; jarak — **selesai**: titik lokasi penyedia lewat peta, radius layanan,
lokasi aktif pencari jasa, filter radius, urutan terdekat, dan tampilan jarak.

Pesan &amp; pesanan — **selesai**: chat dalam aplikasi dengan notifikasi, permintaan
order yang menyatu dengan percakapan, dan alur status pesanan.

Halaman penyedia — **selesai**: direktori `/penyedia` dengan penyaring dan
radius, profil publik ber-slug, daftar jasa, portofolio, ulasan, dan peta
lokasi.

Tampilan &amp; aplikasi mobile — **sebagian**: gambar latar beranda, kepekatan
overlay, dan lencana unduh Google Play sudah bisa diatur admin. Aplikasi
Android-nya sendiri belum dibangun, jadi tautannya dibiarkan kosong dan
lencananya tidak tampil sampai diisi.

Tahap 3 — **selesai**: rating dan ulasan setelah order selesai, serta
portofolio penyedia, keduanya tampil di halaman publik penyedia.

Slot iklan — **selesai untuk penayangan sendiri**: admin menyusun slot,
mengunggah materinya, dan slot yang aktif langsung tayang di halaman publik.
Yang belum ada adalah jaringan iklan pihak ketiga, penargetan, dan penghitungan
tayang/klik — semuanya di luar cakupan MVP.

Empat penempatannya:

| Kunci | Letak |
|---|---|
| `beranda_atas` | Beranda, antara hero dan daftar kategori |
| `beranda_tengah` | Beranda, sebelum blok penyedia pilihan |
| `cari_atas` | Halaman pencarian, di atas hasil |
| `detail_samping` | Detail jasa, kolom samping **di bawah panel kontak** |

Di halaman detail, iklan sengaja ditaruh setelah panel kontak: menghubungi
penyedia lebih berharga bagi kedua pihak daripada klik iklan, jadi tombol
ordernya tidak boleh terdorong turun oleh materi berbayar. Di halaman
pencarian, iklan berada **di luar** fragmen yang ditukar HTMX — di dalamnya, ia
akan dimuat ulang setiap kali filter berubah.

Slot hanya tayang bila ditandai aktif **dan** punya gambar atau teks; slot yang
aktif tetapi materinya sudah dihapus tidak menyisakan kotak kosong. Setiap
iklan diberi label "Iklan" yang terbaca, bentuknya sengaja berbeda dari kartu
jasa (melebar penuh, bergaris putus-putus) supaya tidak tertukar dengan hasil
pencarian, dan tautannya memakai `rel="sponsored"` agar mesin pencari tidak
memperlakukannya sebagai rekomendasi editorial.

Satu uji penjaga memastikan setiap kunci slot yang didefinisikan di model
benar-benar dipasang di salah satu halaman publik. Tanpa itu, menambah slot
baru di pengaturan akan membuat admin mengisi materi, menekan simpan, dan tidak
pernah melihat hasilnya — tanpa galat apa pun.

Materi iklan diunggah sebagai berkas, bukan ditempel sebagai URL, dan melewati
pipeline WEBP yang sama.

Tiap slot adalah **satu form multipart dengan satu tombol simpan**: teks dan
berkas dikirim bersama. Percobaan pertama memisahkannya — form unggah sendiri
untuk berkas, satu form bersama untuk semua teks — dan itu gagal dengan cara
yang paling buruk: admin memilih gambar, menekan tombol simpan yang terlihat
paling utama (milik form teks), berkasnya tidak ikut terkirim, dan halaman
kembali dengan pesan "tersimpan" tanpa gambar. Tidak ada galat di mana pun.
Penyatuan formnya menghapus kemungkinan itu sepenuhnya, dan uji penjaga
memastikan setiap form slot tetap `multipart/form-data` serta isian berkasnya
tidak bertanda `required` — sebab mengubah teks saja harus tetap mungkin tanpa
mengunggah ulang gambar.

Skema database untuk ketiga tahap sudah dibuat sejak awal, termasuk tabel
`orders`, `reviews`, `portfolios`, dan `notifications`.

Di luar cakupan dan sengaja belum dikerjakan: pembayaran atau escrow,
penagihan untuk listing dan penyedia pilihan (kontrol adminnya sudah ada,
tetapi tidak terkait pembayaran apa pun), dan aplikasi mobile.
