# Sistem desain Adojobs — web ↔ Android

Dokumen ini dibaca saat mulai aplikasi Flutter. Tujuannya satu: aplikasi
Android terlihat dan terasa **sama persis** dengan web tanpa menyalin
keputusan desain secara manual. Setiap bagian menyebut sumber tunggalnya di
repo — bila dokumen dan sumber berbeda, sumbernya yang benar.

## 1. Sumber tunggal

| Hal | Sumber di repo | Dipakai web oleh | Dipakai Android oleh |
|---|---|---|---|
| Warna, bayangan, radius, spasi, huruf, ukuran komponen | `web/static/design/tokens.json` | `tailwind.config.js` (→ `:root`/`.dark` + skala Tailwind) | `ThemeData` (baca berkas yang sama sebagai aset) |
| Set ikon | `web/templates/components/icons.templ` | komponen `Icon(nama)` | `web/static/design/ikon/<nama>.svg` + `index.json` (hasil `make ikon`, `flutter_svg`) |
| Ikon aplikasi | `web/static/img/favicon.svg` + warna merek | favicon, manifest PWA | `ikon-512.png`, `ikon-maskable-512.png` (hasil `make ikon`) |
| Isi menu utama | `internal/view/menu.go` (`MenuUtama`) | sheet tombol melayang | `GET /api/v1/config` → `menu` |
| Isi Tentang & Panduan | `internal/view/panduan.go` (`Panduan`) | `/tentang` | `GET /api/v1/help` |
| Judul/isi notifikasi | `internal/view` (`IsiNotifikasi`) | halaman notifikasi | payload push (`data.type`, `data.tautan`) |
| Kontrak API | `docs/openapi.yaml` | — | seluruh layar |

Font di-self-host: **Bricolage Grotesque** (judul, `display`) dan **Plus
Jakarta Sans** (teks, `sans`); berkas WOFF2 di `web/static/fonts`. Untuk
Flutter, konversi ke TTF dari sumber font yang sama dan daftarkan dengan nama
keluarga persis seperti di `tokens.huruf.keluarga`.

## 2. Token → Flutter

```
tokens.warna.terang/gelap   → ColorScheme (light/dark) + ThemeExtension untuk
                               nama non-Material (surface-raised, sorot-*, nav-blur)
tokens.bayangan             → BoxShadow (subtle, raised, pop)
tokens.radius               → BorderRadius (xs 4, sm 6, DEFAULT 8, md 10, lg 12, xl 16)
tokens.spasi                → skala padding/margin 4/8/12/16/24/32/48/64
tokens.huruf.skala          → TextTheme (ukuran, tinggiBaris, spasiHuruf)
tokens.komponen             → konstanta ukuran: sentuh-min 44, nav-bawah-tinggi 56,
                               fab 56, fab-jarak-tepi 16, sheet-radius-atas 16,
                               avatar-sm/md/lg 32/40/56
tokens.gerak                → Curves.easeOutCubic-setara (0.22,1,0.36,1); durasi 120/200/260 ms
```

Aturan yang harus dipertahankan di Android, karena itu identitas visualnya:

- **Pemisahan memakai garis, bukan bayangan.** Kartu = latar `surface-raised`
  + garis `border` 1 px; bayangan hanya untuk elemen melayang (FAB, sheet).
- **Satu gradasi merek** (`primary-from → primary-to`, 103°) hanya pada tombol
  utama, FAB, badge, garis tab aktif, dan angka statistik. Tidak di latar.
- **Mode gelap menaikkan permukaan, bukan bayangan.** Elevasi di gelap =
  permukaan lebih terang (`surface` → `surface-raised`).
- **Dua sorotan kartu**: sorotan jasa keemasan (`sorot-*`), penyedia pilihan
  warna merek (`sorot-penyedia-*`), keduanya tipis; bila keduanya berlaku,
  keemasan menang.
- **Tanpa ilustrasi dekoratif, tanpa emoji.** Ikon garis 1,75 px dari set ikon.

## 3. Kamus komponen

| Komponen web | Kelas / templ | Padanan Flutter | Catatan |
|---|---|---|---|
| Navigasi bawah | `BottomNav` (`.bottom-nav-item`) | `NavigationBar` 5 tujuan | Tinggi 56; tab aktif = warna merek + garis gradasi 2 px di atas |
| Tombol melayang | `.fab` (`data-menu-buka`) | `FloatingActionButton` (56, bulat, gradasi) | Badge belum dibaca di sudut; sembunyi saat keyboard terbuka & di ruang obrolan |
| Sheet menu utama | `<dialog class="sheet">` | `showModalBottomSheet` (radius atas 16, maks 85% tinggi) | Isi dari `menu` di `/config`; item berjenis `tautan/keluar/tema/aplikasi` |
| Item sheet | `.sheet-item`, `.is-utama` | `ListTile` / tombol penuh gradasi untuk `utama` | Ikon dalam kotak 36 px `surface`, radius md |
| Tombol | `.btn-primary/-secondary/-ghost`, `-sm/-lg` | `FilledButton` gradasi / `OutlinedButton` / `TextButton` | Tinggi min 40 (lg 48), radius 8, huruf 600 |
| Input | `.input`, `.field-label` | `TextField` `OutlineInputBorder` radius 10 | Label di atas, petunjuk & error di bawah |
| Chip filter | `.chip`, `.is-active` | `ChoiceChip` | Baris geser horizontal; chip aktif digeser ke layar |
| Kartu jasa | `.listing-card` (+`.is-sorot`, `.is-sorot-penyedia`) | `Card` kustom | Foto 4:3, judul 2 baris, harga, rating, jarak (`jarak_km`) |
| Kartu penyedia | `penyedia.templ` | `Card` kustom | Avatar 56, lencana terverifikasi, rating ringkas, jumlah jasa |
| Avatar | `Avatar(url, nama, size)` | `CircleAvatar` | Inisial 2 huruf bila tanpa foto, latar `primary-soft` |
| Rating | `Rating`, `RatingRingkas` | ikon `bintang` warna `star` | Selalu `avg` 1 desimal + `(total)` |
| Badge belum dibaca | `BadgeBelumDibacaInline` | `Badge` | Gradasi merek, huruf 10 px bold |
| Empty state | `EmptyState(judul, pesan)` | kolom teks tengah | Tanpa ilustrasi |
| Slot iklan | `iklan.templ` (`SlotIklan`) | widget gambar + label "IKLAN" | Ambil dari `GET /ads?slot=…`; klik → `klik_url` |
| FAQ | `<details class="faq">` | `ExpansionTile` | Tanda `+`/`–` di kanan |
| Langkah bernomor | `.langkah`, `.langkah-nomor` | `ListTile` dengan lingkaran nomor 32 px | Nomor warna merek di `primary-soft` |
| Tile statistik | `.stat-tile`, `.stat-angka` | `Container` + teks gradasi | Angka dari `/help.ringkasan` |
| Flash / toast | `FlashMessage` | `SnackBar` | Jenis sukses/info/peringatan/galat |

## 4. Navigasi & tautan

Semua tautan dari server (menu, notifikasi, push, panduan) adalah **path
web**. Aplikasi memetakannya dengan satu tabel; path yang tidak dikenal
dibuka di layar web-view atau diabaikan.

| Path | Layar Android |
|---|---|
| `/` | Beranda |
| `/cari?…` | Pencarian (query, kategori, kecamatan, urut, lat/lng) |
| `/jasa/{id}` | Detail jasa |
| `/penyedia` · `/penyedia/{slug}` | Direktori · Profil penyedia |
| `/pesan` · `/pesan/{id}` | Daftar obrolan · Ruang obrolan (SSE) |
| `/pesanan/{id}` | Detail pesanan |
| `/dasbor` · `/akun/profil` | Akun · Ubah profil |
| `/provider/daftar` · `/provider/profil` · `/provider/lokasi` · `/provider/portofolio` | Alur penyedia |
| `/jasa/baru` · `/jasa/{id}/ubah` | Form jasa |
| `/promosi` · `/promosi/baru?jenis=…` · `/promosi/{id}` | Promosi saya · Ajukan · Detail |
| `/notifikasi` | Notifikasi |
| `/tentang#{anchor}` · `/bantuan` | Tentang & Panduan, gulir ke anchor |
| `/masuk` · `/daftar` · `/keluar` | Masuk · Daftar · (POST logout) |

Struktur navigasi utama identik: **bottom nav 5 tujuan** (Beranda, Cari,
Jual jasa/Pasang, Pesan, Akun) + **FAB menu** sebagai peta lengkap. Tidak ada
fitur yang hanya bisa dicapai lewat satu jalur.

## 5. Perilaku yang wajib sama

- **Lokasi**: minta izin saat pengguna menekan "pakai lokasi saya", bukan
  saat aplikasi dibuka; tanpa izin, pilih kecamatan (daftar berkoordinat dari
  `/config.lokasi.kecamatan`).
- **Obrolan**: SSE `/conversations/{id}/stream`, sambung ulang otomatis,
  kejar dengan `messages?since=`; gulir ke pesan terbaru setelah kirim.
- **Pesanan**: tombol "Kirim permintaan pesanan" selalu di atas dan setara
  atau lebih menonjol dari "Kirim pesan".
- **WhatsApp**: tidak ditampilkan kecuali `situs.whatsapp_aktif` true.
- **Tema**: terang/gelap mengikuti pilihan pengguna (bukan hanya sistem);
  simpan lokal seperti `localStorage` di web.
- **Iklan**: satu pemilihan per tampilan slot (`GET /ads`), bukan cache
  klien; tayangan dihitung server.
- **Harga**: format `Rp 1.250.000` (`id-ID`), tanpa desimal; "nego" untuk
  `price_type=negotiable`.

## 6. Menambah sesuatu

1. Warna/ukuran baru → `tokens.json`, lalu `make css`. Jangan tulis heksa di CSS/templ.
2. Ikon baru → `icons.templ`, lalu `make ikon`. Uji `TestEksporIkonSelaras` menjaga.
3. Item menu / isi panduan → Go di `internal/view`, bukan template; uji di paket yang sama.
4. Endpoint baru → `internal/handler/api/mobile.go` + `router.go` + `docs/openapi.yaml` + kasus di `TestAPIUntukAndroid`.
