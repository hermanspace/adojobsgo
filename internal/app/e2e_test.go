package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hermansyah/adojobsid/internal/config"
	"github.com/hermansyah/adojobsid/internal/database"
	"github.com/hermansyah/adojobsid/internal/seed"
)

// Uji end-to-end menjalankan aplikasi utuh — rute, middleware, service,
// repository — di atas Postgres dan Redis sungguhan, pada database terpisah
// yang dibuat ulang setiap kali. Ia berjalan hanya bila E2E=1 (lihat
// `make test-e2e`), supaya `go test ./...` biasa tetap lolos tanpa layanan.
//
// Nilainya berbeda dari uji unit: yang dijaga di sini adalah alur yang
// benar-benar dilalui pengguna, bukan potongan logika — daftar, jadi
// penyedia, pasang jasa, disetujui admin, dipesan, diselesaikan, diulas.

func siapkan(t *testing.T) *App {
	t.Helper()
	if os.Getenv("E2E") != "1" {
		t.Skip("E2E=1 tidak disetel; jalankan lewat `make test-e2e`")
	}
	// Aset statis dirujuk relatif terhadap akar repo. Pindah hanya bila belum
	// di sana: setiap uji memanggil siapkan(), dan pindah dua kali akan
	// melompat keluar dari repo.
	if _, err := os.Stat("web/static"); err != nil {
		if err := os.Chdir("../.."); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("konfigurasi: %v", err)
	}
	if !strings.HasSuffix(cfg.Postgres.DB, "_e2e") {
		t.Fatalf("POSTGRES_DB harus berakhiran _e2e agar tidak menyentuh data asli, dapat %q", cfg.Postgres.DB)
	}
	cfg.Upload.Dir = t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	// Database uji dibuat ulang dari nol lewat koneksi ke database pemeliharaan.
	pemeliharaan := cfg.Postgres
	pemeliharaan.DB = "postgres"
	adminPool, err := database.NewPostgres(ctx, pemeliharaan)
	if err != nil {
		t.Fatalf("koneksi pemeliharaan: %v", err)
	}
	defer adminPool.Close()
	for _, q := range []string{
		fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, cfg.Postgres.DB),
		fmt.Sprintf(`CREATE DATABASE %q`, cfg.Postgres.DB),
	} {
		if _, err := adminPool.Exec(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if err := database.MigrateUp(cfg.Postgres); err != nil {
		t.Fatalf("migrasi: %v", err)
	}

	a, err := New(ctx, cfg, "e2e")
	if err != nil {
		t.Fatalf("rakit aplikasi: %v", err)
	}
	t.Cleanup(a.Close)

	if err := a.Rdb.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("bersihkan redis: %v", err)
	}
	if err := seed.Run(ctx, a.Repos); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return a
}

// klien meniru satu peramban: menyimpan cookie antar permintaan.
type klien struct {
	t      *testing.T
	app    *App
	cookie map[string]string
	// token, bila diisi, dikirim sebagai Authorization: Bearer — meniru
	// aplikasi native yang tidak menyimpan cookie.
	token string
}

// klienNative tidak pernah membawa cookie; hanya bearer dan header klien.
func (a *App) klienNative(t *testing.T) *klien {
	return &klien{t: t, app: a, cookie: nil}
}

func (a *App) klienBaru(t *testing.T) *klien {
	return &klien{t: t, app: a, cookie: map[string]string{}}
}

func (k *klien) kirim(req *http.Request) *http.Response {
	k.t.Helper()
	for nama, nilai := range k.cookie {
		req.AddCookie(&http.Cookie{Name: nama, Value: nilai})
	}
	if k.token != "" {
		req.Header.Set("Authorization", "Bearer "+k.token)
	}
	if k.cookie == nil {
		// aplikasi native selalu mengirim header ini (lihat docs/openapi.yaml)
		req.Header.Set("X-Requested-With", "adojobs-android")
	}
	res, err := k.app.Fiber.Test(req, -1)
	if err != nil {
		k.t.Fatalf("%s %s: %v", req.Method, req.URL.Path, err)
	}
	if k.cookie == nil {
		return res // klien native mengabaikan cookie sepenuhnya
	}
	for _, c := range res.Cookies() {
		if c.MaxAge < 0 || c.Value == "" {
			delete(k.cookie, c.Name)
			continue
		}
		k.cookie[c.Name] = c.Value
	}
	return res
}

// postJSON mengirim badan JSON dan mengurai amplop {data} / {error}.
func (k *klien) postJSON(path string, badan any) (int, map[string]any) {
	k.t.Helper()
	raw, _ := json.Marshal(badan)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	res := k.kirim(req)
	return res.StatusCode, uraiJSON(k.t, res)
}

func (k *klien) patchJSON(path string, badan any) (int, map[string]any) {
	k.t.Helper()
	raw, _ := json.Marshal(badan)
	req := httptest.NewRequest(http.MethodPatch, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	res := k.kirim(req)
	return res.StatusCode, uraiJSON(k.t, res)
}

func (k *klien) getJSON(path string) (int, map[string]any) {
	k.t.Helper()
	res := k.kirim(httptest.NewRequest(http.MethodGet, path, nil))
	return res.StatusCode, uraiJSON(k.t, res)
}

func uraiJSON(t *testing.T, res *http.Response) map[string]any {
	t.Helper()
	var out map[string]any
	isi := baca(res)
	if err := json.Unmarshal([]byte(isi), &out); err != nil {
		t.Fatalf("bukan JSON (%d): %.200s", res.StatusCode, isi)
	}
	return out
}

// data mengambil amplop data sebagai map; gagal bila bentuknya lain.
func data(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	d, ok := m["data"].(map[string]any)
	if !ok {
		t.Fatalf("amplop data bukan objek: %v", m)
	}
	return d
}

// labelMenuJSON meratakan label item menu dari amplop JSON untuk dicek.
func labelMenuJSON(menu any) string {
	var out []string
	bagian, _ := menu.([]any)
	for _, b := range bagian {
		bm, _ := b.(map[string]any)
		item, _ := bm["item"].([]any)
		for _, it := range item {
			im, _ := it.(map[string]any)
			out = append(out, fmt.Sprint(im["label"]))
		}
	}
	return strings.Join(out, "|")
}

func angka(v any) int64 {
	f, _ := v.(float64)
	return int64(f)
}

func (k *klien) get(path string) (int, string) {
	res := k.kirim(httptest.NewRequest(http.MethodGet, path, nil))
	return res.StatusCode, baca(res)
}

func (k *klien) getHX(path string) (int, string) {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("HX-Request", "true")
	res := k.kirim(req)
	return res.StatusCode, baca(res)
}

func (k *klien) post(path string, isi url.Values, hx bool) *http.Response {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(isi.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	return k.kirim(req)
}

func (k *klien) postMultipart(path string, isi map[string]string) *http.Response {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for nama, nilai := range isi {
		_ = w.WriteField(nama, nilai)
	}
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return k.kirim(req)
}

// postMultipartBerkas seperti postMultipart, ditambah satu berkas.
func (k *klien) postMultipartBerkas(path string, isi map[string]string, namaField, namaBerkas string, data []byte) *http.Response {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for nama, nilai := range isi {
		_ = w.WriteField(nama, nilai)
	}
	if fw, err := w.CreateFormFile(namaField, namaBerkas); err == nil {
		_, _ = fw.Write(data)
	}
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return k.kirim(req)
}

// pngUji membuat gambar PNG polos berukuran cukup untuk pipeline gambar.
func pngUji(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 600, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 600; x++ {
			img.Set(x, y, color.RGBA{R: 30, G: 100, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func baca(res *http.Response) string {
	b, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	return string(b)
}

func harusStatus(t *testing.T, res *http.Response, mau int, langkah string) {
	t.Helper()
	if res.StatusCode != mau {
		isi := baca(res)
		galat := regexp.MustCompile(`field-error[^<]*(?:<[^>]*>[^<]*)*`).FindString(isi)
		t.Fatalf("%s: status %d, harusnya %d\n%s", langkah, res.StatusCode, mau,
			strings.Join(strings.Fields(regexp.MustCompile(`<[^>]*>`).ReplaceAllString(galat, " ")), " "))
	}
}

func lokasi(res *http.Response) string { return res.Header.Get("Location") }

func (a *App) tanya(t *testing.T, q string, args ...any) string {
	t.Helper()
	var out string
	if err := a.Pool.QueryRow(context.Background(), q, args...).Scan(&out); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return out
}

func daftar(t *testing.T, k *klien, nama, phone string) {
	t.Helper()
	res := k.post("/daftar", url.Values{
		"full_name": {nama}, "phone": {phone}, "kecamatan": {"Bengkalis"},
		"password": {"rahasia-e2e-123"}, "password_confirm": {"rahasia-e2e-123"},
	}, false)
	harusStatus(t, res, http.StatusSeeOther, "daftar "+nama)
}

// TestAlurUtama menelusuri satu transaksi utuh dari kedua sisi.
func TestAlurUtama(t *testing.T) {
	a := siapkan(t)
	seeker := a.klienBaru(t)
	penyedia := a.klienBaru(t)
	admin := a.klienBaru(t)

	// --- pencari jasa mendaftar dan langsung masuk ---
	daftar(t, seeker, "Pencari Uji", "081311100001")
	if kode, _ := seeker.get("/dasbor"); kode != http.StatusOK {
		t.Fatalf("setelah daftar, /dasbor = %d; registrasi tidak memulai sesi", kode)
	}

	// --- penyedia mendaftar, jadi penyedia tanpa nomor WhatsApp ---
	daftar(t, penyedia, "Penyedia Uji", "081311100002")
	harusStatus(t, penyedia.postMultipart("/provider/daftar", map[string]string{
		"whatsapp_number": "",
		"bio":             "Teknisi pendingin ruangan untuk rumah dan ruko di Pulau Bengkalis sejak 2014.",
	}), http.StatusSeeOther, "jadi penyedia")
	if got := a.tanya(t, `SELECT whatsapp_number FROM provider_profiles p JOIN users u ON u.id=p.user_id WHERE u.phone='6281311100002'`); got != "" {
		t.Errorf("nomor WhatsApp kosong tersimpan sebagai %q", got)
	}

	// --- memasang jasa: masuk antrean, belum tayang ---
	kategori := a.tanya(t, `SELECT id::text FROM categories WHERE parent_id IS NOT NULL ORDER BY id LIMIT 1`)
	res := penyedia.postMultipart("/jasa/baru", map[string]string{
		"title":       "Cuci AC uji e2e",
		"category_id": kategori,
		"description": "Pembersihan unit indoor dan outdoor memakai mesin steam, cek tekanan freon, dan pengecekan kebocoran.",
		"price_type":  "fixed",
		"price_min":   "100000",
		"price_max":   "100000",
	})
	harusStatus(t, res, http.StatusSeeOther, "pasang jasa")
	m := regexp.MustCompile(`/jasa/(\d+)`).FindStringSubmatch(lokasi(res))
	if m == nil {
		t.Fatalf("pasang jasa tidak mengarah ke halaman jasa: %q", lokasi(res))
	}
	jasaID := m[1]
	if st := a.tanya(t, `SELECT status FROM services WHERE id=$1`, jasaID); st != "pending" {
		t.Fatalf("jasa baru berstatus %q, harusnya pending", st)
	}
	if _, isi := seeker.get("/cari?q=uji+e2e"); strings.Contains(isi, "Cuci AC uji e2e") {
		t.Error("jasa yang masih antre sudah muncul di pencarian")
	}

	// --- admin menyetujui ---
	if _, _, err := a.Services.Admin.CreateAdmin(context.Background(), "Admin E2E", "081311100009", "rahasia-e2e-123"); err != nil {
		t.Fatalf("buat admin: %v", err)
	}
	harusStatus(t, admin.post("/masuk", url.Values{"identifier": {"081311100009"}, "password": {"rahasia-e2e-123"}}, false),
		http.StatusSeeOther, "masuk admin")
	harusStatus(t, admin.post("/admin/antrean/"+jasaID+"/setujui", nil, true), http.StatusOK, "setujui listing")
	if st := a.tanya(t, `SELECT status FROM services WHERE id=$1`, jasaID); st != "active" {
		t.Fatalf("setelah disetujui status %q, harusnya active", st)
	}
	if _, isi := seeker.get("/cari?q=uji+e2e"); !strings.Contains(isi, "Cuci AC uji e2e") {
		t.Error("jasa yang sudah disetujui tidak muncul di pencarian")
	}

	// --- pencari jasa memesan; percakapan terbentuk ---
	res = seeker.post("/jasa/"+jasaID+"/order", url.Values{
		"notes": {"Cuci 2 unit AC 1 PK di rumah, lantai dua."}, "scheduled_date": {""},
	}, false)
	harusStatus(t, res, http.StatusSeeOther, "kirim permintaan order")
	m = regexp.MustCompile(`/pesan/(\d+)`).FindStringSubmatch(lokasi(res))
	if m == nil {
		t.Fatalf("order tidak mengarah ke ruang obrolan: %q", lokasi(res))
	}
	percakapan := m[1]
	orderID := a.tanya(t, `SELECT id::text FROM orders WHERE service_id=$1`, jasaID)
	if st := a.tanya(t, `SELECT status FROM orders WHERE id=$1`, orderID); st != "pending" {
		t.Fatalf("order baru berstatus %q", st)
	}
	if kode, isi := seeker.get("/pesan/" + percakapan); kode != http.StatusOK || strings.Contains(isi, "data-menu-buka") {
		t.Errorf("ruang obrolan = %d; tombol melayang tidak boleh menutupi kotak kirim", kode)
	}
	if kode, isi := seeker.getHX("/pesan/" + percakapan + "/baru?sejak=0"); kode != http.StatusOK || !strings.Contains(isi, "Cuci 2 unit AC") {
		t.Errorf("isi permintaan tidak muncul sebagai pesan pertama (status %d)", kode)
	}

	// --- penyedia menerima lalu menyelesaikan ---
	for _, status := range []string{"accepted", "completed"} {
		harusStatus(t, penyedia.post("/pesanan/"+orderID+"/status",
			url.Values{"status": {status}, "conversation_id": {percakapan}}, false),
			http.StatusSeeOther, "ubah status ke "+status)
		if st := a.tanya(t, `SELECT status FROM orders WHERE id=$1`, orderID); st != status {
			t.Fatalf("status order %q, harusnya %q", st, status)
		}
	}
	// pencari jasa tidak boleh mengubah status pesanan yang bukan haknya
	if st := a.tanya(t, `SELECT status FROM orders WHERE id=$1`, orderID); st != "completed" {
		t.Fatalf("status order berubah tanpa hak: %q", st)
	}

	// --- pencari jasa mengulas; rating penyedia terhitung ---
	harusStatus(t, seeker.post("/pesanan/"+orderID+"/ulasan",
		url.Values{"rating": {"5"}, "comment": {"Kerja rapi dan cepat."}, "conversation_id": {percakapan}}, false),
		http.StatusSeeOther, "tulis ulasan")
	if n := a.tanya(t, `SELECT count(*)::text FROM reviews WHERE order_id=$1`, orderID); n != "1" {
		t.Fatalf("ulasan tersimpan %s kali", n)
	}
	if r := a.tanya(t, `SELECT avg_rating::text FROM provider_profiles p JOIN users u ON u.id=p.user_id WHERE u.phone='6281311100002'`); !strings.HasPrefix(r, "5") {
		t.Errorf("rating penyedia %s, harusnya 5 setelah satu ulasan bintang 5", r)
	}
	if _, isi := seeker.get("/penyedia/penyedia-uji"); !strings.Contains(isi, "Kerja rapi dan cepat.") {
		t.Error("ulasan tidak tampil di halaman publik penyedia")
	}

	// --- menu utama (tombol melayang) mengikuti peran di sisi web ---
	if _, isi := a.klienBaru(t).get("/"); !strings.Contains(isi, "Daftar akun baru") || strings.Contains(isi, `action="/keluar"`) {
		t.Error("menu tamu: harus menawarkan daftar, tanpa tombol keluar")
	}
	if _, isi := seeker.get("/"); !strings.Contains(isi, "Jadi penyedia jasa") || strings.Contains(isi, "Sorot jasa") || !strings.Contains(isi, `action="/keluar"`) {
		t.Error("menu pencari: ajakan jadi penyedia, tanpa item penyedia, dengan keluar")
	}
	if _, isi := penyedia.get("/"); !strings.Contains(isi, "Sorot jasa") || !strings.Contains(isi, `href="/jasa/baru" class="sheet-item is-utama"`) || strings.Contains(isi, "Jadi penyedia jasa") {
		t.Error("menu penyedia: sorot jasa & pasang jasa ditonjolkan, tanpa ajakan jadi penyedia")
	}
	if _, isi := admin.get("/"); !strings.Contains(isi, "Panel admin") {
		t.Error("menu admin tanpa tautan panel admin")
	}

	// --- kontrak API: tanpa nomor WhatsApp, dengan penjaga CSRF ---
	kode, isi := seeker.get("/api/v1/services/" + jasaID)
	if kode != http.StatusOK {
		t.Fatalf("API detail jasa = %d", kode)
	}
	var amplop map[string]json.RawMessage
	if err := json.Unmarshal([]byte(isi), &amplop); err != nil || amplop["data"] == nil {
		t.Fatalf("API detail jasa bukan amplop {data}: %s", isi[:min(len(isi), 120)])
	}
	if strings.Contains(isi, `"whatsapp_number"`) {
		t.Error("API publik membocorkan whatsapp_number")
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"identifier":"x","password":"y"}`))
	req.Header.Set("Content-Type", "application/json")
	if res := seeker.kirim(req); res.StatusCode != http.StatusForbidden {
		t.Errorf("POST API tanpa X-Requested-With = %d, harusnya 403", res.StatusCode)
	}
}

// TestIsProviderSelaluSinkron: users.is_provider hanya ditulis trigger
// database, dari keberadaan baris provider_profiles — dua arah.
func TestIsProviderSelaluSinkron(t *testing.T) {
	a := siapkan(t)
	k := a.klienBaru(t)
	daftar(t, k, "Calon Penyedia", "081311100004")
	ctx := context.Background()

	if v := a.tanya(t, `SELECT is_provider::text FROM users WHERE phone='6281311100004'`); v != "false" {
		t.Fatalf("pengguna baru is_provider=%s", v)
	}
	if _, err := a.Pool.Exec(ctx, `INSERT INTO provider_profiles (user_id, slug, whatsapp_number)
		SELECT id, 'calon-penyedia', '' FROM users WHERE phone='6281311100004'`); err != nil {
		t.Fatal(err)
	}
	if v := a.tanya(t, `SELECT is_provider::text FROM users WHERE phone='6281311100004'`); v != "true" {
		t.Errorf("setelah profil dibuat, is_provider=%s", v)
	}
	if _, err := a.Pool.Exec(ctx, `DELETE FROM provider_profiles WHERE slug='calon-penyedia'`); err != nil {
		t.Fatal(err)
	}
	if v := a.tanya(t, `SELECT is_provider::text FROM users WHERE phone='6281311100004'`); v != "false" {
		t.Errorf("setelah profil dihapus, is_provider=%s", v)
	}
	// Invarian menyeluruh: tidak satu pun pengguna yang menyimpang.
	if n := a.tanya(t, `SELECT count(*)::text FROM users u
		WHERE u.is_provider <> EXISTS (SELECT 1 FROM provider_profiles p WHERE p.user_id = u.id)`); n != "0" {
		t.Errorf("%s pengguna dengan is_provider yang tidak sesuai profilnya", n)
	}
}

// TestPencarianLentur: kata kunci tidak harus persis. Data seed punya jasa
// "Cuci AC split rumah" di kategori "Servis AC".
func TestPencarianLentur(t *testing.T) {
	a := siapkan(t)
	k := a.klienBaru(t)

	total := func(q string) (int, string) {
		t.Helper()
		kode, isi := k.get("/api/v1/services?q=" + url.QueryEscape(q))
		if kode != http.StatusOK {
			t.Fatalf("q=%q: status %d", q, kode)
		}
		var res struct {
			Data struct {
				Total int `json:"total"`
				Items []struct {
					Title string `json:"title"`
				} `json:"items"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(isi), &res); err != nil {
			t.Fatalf("q=%q: %v", q, err)
		}
		pertama := ""
		if len(res.Data.Items) > 0 {
			pertama = res.Data.Items[0].Title
		}
		return res.Data.Total, pertama
	}

	if n, _ := total("servis ac"); n == 0 {
		t.Fatal("pencarian persis pun gagal; data seed tidak seperti yang diharapkan")
	}
	// ejaan lain untuk kata yang sama
	if n, _ := total("service ac"); n == 0 {
		t.Error("\"service ac\" tidak menemukan jasa di kategori Servis AC")
	}
	if n, _ := total("servise"); n == 0 {
		t.Error("salah ketik \"servise\" tidak menemukan apa pun")
	}
	// urutan kata bebas
	if n, _ := total("ac servis"); n == 0 {
		t.Error("urutan kata yang dibalik tidak menemukan apa pun")
	}
	// relevansi: yang judulnya memuat kata kunci naik ke atas
	if _, pertama := total("cuci ac"); !strings.Contains(strings.ToLower(pertama), "cuci ac") {
		t.Errorf("hasil pertama untuk \"cuci ac\" adalah %q, bukan jasa cuci AC", pertama)
	}
	// yang tidak berhubungan tetap tidak muncul
	if n, _ := total("notaris"); n != 0 {
		t.Errorf("kata yang tidak ada di data menemukan %d hasil", n)
	}
}

// TestPaketPromosiDanTrigger: admin mengelola paket lewat form, dan
// featured_until listing/penyedia adalah turunan promosi aktif yang dijaga
// trigger — diuji lewat SQL langsung, jalur yang melewati kode aplikasi.
func TestPaketPromosiDanTrigger(t *testing.T) {
	a := siapkan(t)
	admin := a.klienBaru(t)
	ctx := context.Background()

	if _, _, err := a.Services.Admin.CreateAdmin(ctx, "Admin Paket", "081311100008", "rahasia-e2e-123"); err != nil {
		t.Fatal(err)
	}
	harusStatus(t, admin.post("/masuk", url.Values{"identifier": {"081311100008"}, "password": {"rahasia-e2e-123"}}, false),
		http.StatusSeeOther, "masuk admin")

	// --- paket bawaan dari seeder tampil ---
	kode, isi := admin.get("/admin/paket")
	if kode != http.StatusOK || !strings.Contains(isi, "Utama 14 hari") {
		t.Fatalf("/admin/paket = %d; paket seed tidak tampil", kode)
	}

	// --- tambah paket iklan lewat form; slot dikirim sebagai beberapa nilai ---
	harusStatus(t, admin.post("/admin/paket", url.Values{
		"jenis": {"iklan"}, "nama": {"Uji e2e"}, "durasi_hari": {"3"}, "harga": {"25000"},
		"bobot": {"2"}, "slot_iklan": {"cari_atas", "detail_samping"}, "aktif": {"1"},
	}, false), http.StatusSeeOther, "tambah paket")
	if n := a.tanya(t, `SELECT cardinality(slot_iklan)::text FROM paket_promosi WHERE nama='Uji e2e'`); n != "2" {
		t.Errorf("slot tersimpan %s, harusnya 2", n)
	}
	// nama kembar per jenis ditolak
	res := admin.post("/admin/paket", url.Values{
		"jenis": {"iklan"}, "nama": {"uji E2E"}, "durasi_hari": {"3"}, "bobot": {"1"}, "slot_iklan": {"cari_atas"},
	}, false)
	if res.StatusCode != http.StatusConflict {
		t.Errorf("paket dengan nama kembar = %d, harusnya 409", res.StatusCode)
	}

	// --- trigger: promosi aktif ⇒ featured_until; berhenti ⇒ NULL ---
	paketID := a.tanya(t, `SELECT id::text FROM paket_promosi WHERE jenis='sorotan_jasa' ORDER BY id LIMIT 1`)
	if _, err := a.Pool.Exec(ctx, `INSERT INTO promosi (jenis, user_id, provider_id, service_id, paket_id, status, sumber, mulai_at, selesai_at)
		SELECT 'sorotan_jasa', p.user_id, s.provider_id, s.id, $1, 'aktif', 'admin', NOW(), NOW() + interval '7 days'
		  FROM services s JOIN provider_profiles p ON p.id = s.provider_id WHERE s.id = 1`, paketID); err != nil {
		t.Fatal(err)
	}
	if v := a.tanya(t, `SELECT (featured_until > NOW())::text FROM services WHERE id=1`); v != "true" {
		t.Errorf("setelah promosi aktif, featured_until listing = %s", v)
	}
	// pengajuan kedua yang masih hidup untuk listing yang sama ditolak DB
	if _, err := a.Pool.Exec(ctx, `INSERT INTO promosi (jenis, user_id, provider_id, service_id, paket_id, status)
		SELECT 'sorotan_jasa', p.user_id, s.provider_id, s.id, $1, 'menunggu'
		  FROM services s JOIN provider_profiles p ON p.id = s.provider_id WHERE s.id = 1`, paketID); err == nil {
		t.Error("dua pengajuan hidup untuk satu listing lolos; indeks unik parsial tidak bekerja")
	}
	if _, err := a.Pool.Exec(ctx, `UPDATE promosi SET status='dihentikan' WHERE service_id=1 AND status='aktif'`); err != nil {
		t.Fatal(err)
	}
	if v := a.tanya(t, `SELECT COALESCE(featured_until::text, 'NULL') FROM services WHERE id=1`); v != "NULL" {
		t.Errorf("setelah dihentikan, featured_until listing = %s, harusnya NULL", v)
	}
	// ditolak wajib beralasan — dijaga CHECK, bukan hanya validasi Go
	if _, err := a.Pool.Exec(ctx, `UPDATE promosi SET status='ditolak' WHERE service_id=1`); err == nil {
		t.Error("penolakan tanpa catatan lolos; CHECK constraint tidak bekerja")
	}
}

// TestPengajuanPromosi: pengguna mengajukan sorotan dan iklan lewat form,
// batasnya dijaga, admin menyetujui (uji coba gratis → langsung tayang),
// featured_until ikut, pemohon dinotifikasi, dan orang lain tidak bisa mengintip.
func TestPengajuanPromosi(t *testing.T) {
	a := siapkan(t)
	ctx := context.Background()
	penyedia := a.klienBaru(t)
	orangLain := a.klienBaru(t)

	// penyedia dengan satu jasa yang sudah tayang (disetujui lewat service)
	daftar(t, penyedia, "Penyedia Promosi", "081311100011")
	harusStatus(t, penyedia.postMultipart("/provider/daftar", map[string]string{
		"bio": "Tukang cat rumah dan ruko, rapi dan tepat waktu, wilayah Bengkalis dan Bantan.",
	}), http.StatusSeeOther, "jadi penyedia")
	kategori := a.tanya(t, `SELECT id::text FROM categories WHERE parent_id IS NOT NULL ORDER BY id LIMIT 1`)
	res := penyedia.postMultipart("/jasa/baru", map[string]string{
		"title": "Cat rumah uji promosi", "category_id": kategori, "price_type": "fixed",
		"price_min": "500000", "price_max": "500000",
		"description": "Pengecatan interior dan eksterior termasuk dempul dan plamir, cat disediakan pemilik rumah.",
	})
	harusStatus(t, res, http.StatusSeeOther, "pasang jasa")
	jasaID := regexp.MustCompile(`/jasa/(\d+)`).FindStringSubmatch(lokasi(res))[1]
	adminUser, _, err := a.Services.Admin.CreateAdmin(ctx, "Admin Promosi", "081311100019", "rahasia-e2e-123")
	if err != nil {
		t.Fatal(err)
	}
	jasaInt, _ := strconv.ParseInt(jasaID, 10, 64)
	if err := a.Services.Admin.SetujuiListing(ctx, adminUser.ID, jasaInt); err != nil {
		t.Fatalf("setujui listing: %v", err)
	}

	// --- form sorotan menampilkan paket dan jasa milik sendiri ---
	kode, isi := penyedia.get("/promosi/baru?jenis=sorotan_jasa&service_id=" + jasaID)
	if kode != http.StatusOK || !strings.Contains(isi, "Cat rumah uji promosi") || !strings.Contains(isi, "30 hari") {
		t.Fatalf("form sorotan = %d; jasa/paket tidak tampil", kode)
	}
	paketSorotan := a.tanya(t, `SELECT id::text FROM paket_promosi WHERE jenis='sorotan_jasa' ORDER BY id LIMIT 1`)

	// --- ajukan sorotan; yang kedua ditolak selama yang pertama hidup ---
	res = penyedia.post("/promosi", url.Values{"jenis": {"sorotan_jasa"}, "paket_id": {paketSorotan}, "service_id": {jasaID}}, false)
	harusStatus(t, res, http.StatusSeeOther, "ajukan sorotan")
	sorotanID := regexp.MustCompile(`/promosi/(\d+)`).FindStringSubmatch(lokasi(res))[1]
	if st := a.tanya(t, `SELECT status FROM promosi WHERE id=$1`, sorotanID); st != "menunggu" {
		t.Fatalf("pengajuan baru berstatus %q", st)
	}
	if res := penyedia.post("/promosi", url.Values{"jenis": {"sorotan_jasa"}, "paket_id": {paketSorotan}, "service_id": {jasaID}}, false); res.StatusCode != http.StatusConflict {
		t.Errorf("pengajuan kedua untuk jasa yang sama = %d, harusnya 409", res.StatusCode)
	}

	// --- iklan: tanpa gambar ditolak; dengan gambar lolos; batas 3 ---
	paketIklan := a.tanya(t, `SELECT id::text FROM paket_promosi WHERE jenis='iklan' ORDER BY id LIMIT 1`)
	if res := penyedia.postMultipart("/promosi", map[string]string{"jenis": "iklan", "paket_id": paketIklan, "judul": "Cat murah bergaransi"}); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("iklan tanpa gambar = %d, harusnya 422", res.StatusCode)
	}
	gambar := pngUji(t)
	var iklanID string
	for i := 1; i <= 3; i++ {
		res = penyedia.postMultipartBerkas("/promosi", map[string]string{
			"jenis": "iklan", "paket_id": paketIklan, "judul": fmt.Sprintf("Cat murah bergaransi %d", i),
			"tautan_url": "https://contoh.co.id", "target_kecamatan": "Bengkalis",
		}, "gambar", "spanduk.png", gambar)
		harusStatus(t, res, http.StatusSeeOther, fmt.Sprintf("ajukan iklan ke-%d", i))
		iklanID = regexp.MustCompile(`/promosi/(\d+)`).FindStringSubmatch(lokasi(res))[1]
	}
	if res := penyedia.postMultipartBerkas("/promosi", map[string]string{"jenis": "iklan", "paket_id": paketIklan, "judul": "Iklan keempat"}, "gambar", "s.png", gambar); res.StatusCode != http.StatusConflict {
		t.Errorf("iklan ke-4 = %d, harusnya 409 (batas %d)", res.StatusCode, 3)
	}
	if v := a.tanya(t, `SELECT array_to_string(target_kecamatan, ',') FROM promosi WHERE id=$1`, iklanID); v != "Bengkalis" {
		t.Errorf("target kecamatan tersimpan %q", v)
	}

	// --- admin menyetujui sorotan: uji coba gratis → langsung tayang ---
	sorotanInt, _ := strconv.ParseInt(sorotanID, 10, 64)
	if _, err := a.Services.Promosi.Setujui(ctx, adminUser.ID, sorotanInt); err != nil {
		t.Fatalf("setujui promosi: %v", err)
	}
	if st := a.tanya(t, `SELECT status FROM promosi WHERE id=$1`, sorotanID); st != "aktif" {
		t.Errorf("setelah disetujui (uji coba gratis) status %q, harusnya aktif", st)
	}
	if v := a.tanya(t, `SELECT COALESCE(featured_until > NOW(), false)::text FROM services WHERE id=$1`, jasaID); v != "true" {
		t.Error("featured_until listing tidak ikut menyala")
	}
	if n := a.tanya(t, `SELECT count(*)::text FROM notifications WHERE type='promosi_diperbarui' AND payload->>'promosi_id'=$1`, sorotanID); n != "1" {
		t.Errorf("notifikasi ke pemohon = %s, harusnya 1", n)
	}
	if kode, isi := penyedia.get("/promosi/" + sorotanID); kode != http.StatusOK || !strings.Contains(isi, "Sedang tayang") {
		t.Errorf("detail promosi = %d; status tayang tidak tampil", kode)
	}
	if _, isi := penyedia.get("/notifikasi"); !strings.Contains(isi, "Promosi: Sedang tayang") {
		t.Error("halaman notifikasi tidak menampilkan kabar promosi")
	}
	if _, isi := penyedia.get("/promosi"); !strings.Contains(isi, "Sorotan jasa") || !strings.Contains(isi, "Iklan") {
		t.Error("daftar promosi tidak memuat kedua jenis pengajuan")
	}

	// --- batal: boleh untuk yang menunggu, tidak untuk yang tayang ---
	harusStatus(t, penyedia.post("/promosi/"+iklanID+"/batal", nil, false), http.StatusSeeOther, "batalkan iklan")
	if st := a.tanya(t, `SELECT status FROM promosi WHERE id=$1`, iklanID); st != "dibatalkan" {
		t.Errorf("iklan yang dibatalkan berstatus %q", st)
	}
	penyedia.post("/promosi/"+sorotanID+"/batal", nil, false)
	if st := a.tanya(t, `SELECT status FROM promosi WHERE id=$1`, sorotanID); st != "aktif" {
		t.Errorf("promosi tayang bisa dibatalkan pemohon: status %q", st)
	}

	// --- orang lain tidak bisa melihat ---
	daftar(t, orangLain, "Orang Lain", "081311100012")
	if kode, _ := orangLain.get("/promosi/" + sorotanID); kode != http.StatusNotFound {
		t.Errorf("pengajuan orang lain terbuka = %d, harusnya 404", kode)
	}
	// pengguna biasa diarahkan membuat profil untuk sorotan, tapi boleh pasang iklan
	if res := orangLain.kirim(httptest.NewRequest(http.MethodGet, "/promosi/baru?jenis=sorotan_jasa", nil)); res.StatusCode != http.StatusSeeOther || !strings.HasPrefix(lokasi(res), "/provider/daftar") {
		t.Errorf("pengguna biasa minta sorotan = %d → %q", res.StatusCode, lokasi(res))
	}
	if kode, _ := orangLain.get("/promosi/baru?jenis=iklan"); kode != http.StatusOK {
		t.Errorf("pengguna biasa buka form iklan = %d", kode)
	}
}

// TestAntreanPromosiAdmin: seluruh tindakan admin lewat HTTP, termasuk jalur
// pembayaran saat uji coba gratis dimatikan, dan tombol sorot sekali-klik lama
// yang kini tercatat sebagai promosi bersumber admin.
func TestAntreanPromosiAdmin(t *testing.T) {
	a := siapkan(t)
	ctx := context.Background()
	penyedia := a.klienBaru(t)
	admin := a.klienBaru(t)

	daftar(t, penyedia, "Penyedia Antrean", "081311100021")
	harusStatus(t, penyedia.postMultipart("/provider/daftar", map[string]string{
		"bio": "Servis kulkas dan mesin cuci panggilan ke rumah, wilayah Bengkalis kota dan sekitarnya.",
	}), http.StatusSeeOther, "jadi penyedia")
	kategori := a.tanya(t, `SELECT id::text FROM categories WHERE parent_id IS NOT NULL ORDER BY id LIMIT 1`)
	res := penyedia.postMultipart("/jasa/baru", map[string]string{
		"title": "Servis kulkas uji antrean", "category_id": kategori, "price_type": "fixed",
		"price_min": "150000", "price_max": "150000",
		"description": "Perbaikan kulkas tidak dingin, ganti kompresor dan isi freon, garansi servis satu bulan.",
	})
	harusStatus(t, res, http.StatusSeeOther, "pasang jasa")
	jasaID := regexp.MustCompile(`/jasa/(\d+)`).FindStringSubmatch(lokasi(res))[1]
	adminUser, _, err := a.Services.Admin.CreateAdmin(ctx, "Admin Antrean", "081311100029", "rahasia-e2e-123")
	if err != nil {
		t.Fatal(err)
	}
	jasaInt, _ := strconv.ParseInt(jasaID, 10, 64)
	if err := a.Services.Admin.SetujuiListing(ctx, adminUser.ID, jasaInt); err != nil {
		t.Fatal(err)
	}
	harusStatus(t, admin.post("/masuk", url.Values{"identifier": {"081311100029"}, "password": {"rahasia-e2e-123"}}, false),
		http.StatusSeeOther, "masuk admin")

	ajukanSorotan := func() string {
		t.Helper()
		paket := a.tanya(t, `SELECT id::text FROM paket_promosi WHERE jenis='sorotan_jasa' ORDER BY id LIMIT 1`)
		res := penyedia.post("/promosi", url.Values{"jenis": {"sorotan_jasa"}, "paket_id": {paket}, "service_id": {jasaID}}, false)
		harusStatus(t, res, http.StatusSeeOther, "ajukan sorotan")
		return regexp.MustCompile(`/promosi/(\d+)`).FindStringSubmatch(lokasi(res))[1]
	}
	status := func(id string) string { return a.tanya(t, `SELECT status FROM promosi WHERE id=$1`, id) }
	disorot := func() string {
		return a.tanya(t, `SELECT COALESCE(featured_until > NOW(), false)::text FROM services WHERE id=$1`, jasaID)
	}

	// --- antrean menampilkan pengajuan; badge sidebar ikut ---
	id1 := ajukanSorotan()
	kode, isi := admin.get("/admin/promosi")
	if kode != http.StatusOK || !strings.Contains(isi, "Servis kulkas uji antrean") || !strings.Contains(isi, "Penyedia Antrean") {
		t.Fatalf("antrean admin = %d; pengajuan tidak tampil", kode)
	}

	// --- tolak tanpa alasan ditahan; dengan alasan tercatat dan tampil ke pemohon ---
	if res := admin.post("/admin/promosi/"+id1+"/tolak", url.Values{"catatan": {""}}, false); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("tolak tanpa alasan = %d, harusnya 422", res.StatusCode)
	}
	harusStatus(t, admin.post("/admin/promosi/"+id1+"/tolak", url.Values{"catatan": {"Foto jasa belum ada; lengkapi dulu."}}, false),
		http.StatusSeeOther, "tolak dengan alasan")
	if st := status(id1); st != "ditolak" {
		t.Fatalf("setelah ditolak status %q", st)
	}
	if _, isi := penyedia.get("/promosi/" + id1); !strings.Contains(isi, "Foto jasa belum ada") {
		t.Error("alasan penolakan tidak tampil ke pemohon")
	}

	// --- setujui (uji coba gratis) → aktif; jeda → featured mati; lanjutkan; hentikan ---
	id2 := ajukanSorotan()
	harusStatus(t, admin.post("/admin/promosi/"+id2+"/setujui", nil, false), http.StatusSeeOther, "setujui")
	if status(id2) != "aktif" || disorot() != "true" {
		t.Fatalf("setelah disetujui: status %s, disorot %s", status(id2), disorot())
	}
	harusStatus(t, admin.post("/admin/promosi/"+id2+"/jeda", nil, false), http.StatusSeeOther, "jeda")
	if status(id2) != "dijeda" || disorot() == "true" {
		t.Errorf("setelah dijeda: status %s, disorot %s — jeda harus mematikan sorotan", status(id2), disorot())
	}
	harusStatus(t, admin.post("/admin/promosi/"+id2+"/lanjutkan", nil, false), http.StatusSeeOther, "lanjutkan")
	if status(id2) != "aktif" || disorot() != "true" {
		t.Errorf("setelah dilanjutkan: status %s, disorot %s", status(id2), disorot())
	}
	harusStatus(t, admin.post("/admin/promosi/"+id2+"/hentikan", nil, false), http.StatusSeeOther, "hentikan")
	if status(id2) != "dihentikan" || disorot() == "true" {
		t.Errorf("setelah dihentikan: status %s, disorot %s", status(id2), disorot())
	}

	// --- jalur bayar: matikan uji coba gratis (butuh rekening), paket berbayar ---
	if res := admin.post("/admin/pengaturan/promosi", url.Values{"uji_coba_gratis": {""}}, false); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("mematikan uji coba tanpa rekening = %d, harusnya 422", res.StatusCode)
	}
	harusStatus(t, admin.post("/admin/pengaturan/promosi", url.Values{
		"uji_coba_gratis": {""}, "bank": {"BRI"}, "nomor_rekening": {"123456789"}, "atas_nama": {"Adojobs"},
	}, false), http.StatusSeeOther, "simpan rekening")
	paketIklan := a.tanya(t, `SELECT id::text FROM paket_promosi WHERE jenis='iklan' ORDER BY id LIMIT 1`)
	if _, err := a.Pool.Exec(ctx, `UPDATE paket_promosi SET harga = 25000 WHERE id = $1`, paketIklan); err != nil {
		t.Fatal(err)
	}
	res = penyedia.postMultipartBerkas("/promosi", map[string]string{
		"jenis": "iklan", "paket_id": paketIklan, "judul": "Servis kulkas panggilan",
	}, "gambar", "iklan.png", pngUji(t))
	harusStatus(t, res, http.StatusSeeOther, "ajukan iklan berbayar")
	idIklan := regexp.MustCompile(`/promosi/(\d+)`).FindStringSubmatch(lokasi(res))[1]
	harusStatus(t, admin.post("/admin/promosi/"+idIklan+"/setujui", nil, false), http.StatusSeeOther, "setujui iklan berbayar")
	if st := status(idIklan); st != "disetujui" {
		t.Fatalf("iklan berbayar setelah disetujui berstatus %q, harusnya menunggu pembayaran", st)
	}
	if _, isi := penyedia.get("/promosi/" + idIklan); !strings.Contains(isi, "123456789") || !strings.Contains(isi, "Rp") {
		teks := strings.Join(strings.Fields(regexp.MustCompile(`<[^>]*>`).ReplaceAllString(isi, " ")), " ")
		i := strings.Index(teks, "Iklan")
		if i < 0 {
			i = 0
		}
		t.Errorf("petunjuk pembayaran tidak tampil ke pemohon.\n  uji_coba_gratis di DB: %s\n  harga paket: %s\n  cuplikan: %.400s",
			a.tanya(t, `SELECT COALESCE(value->>'uji_coba_gratis','(tidak ada)') FROM app_settings WHERE key='promosi'`),
			a.tanya(t, `SELECT harga::text FROM paket_promosi WHERE id=$1`, paketIklan), teks[i:])
	}
	harusStatus(t, penyedia.postMultipartBerkas("/promosi/"+idIklan+"/bukti", nil, "bukti", "bukti.png", pngUji(t)),
		http.StatusSeeOther, "unggah bukti")
	if _, isi := admin.get("/admin/promosi?tab=bayar"); !strings.Contains(isi, "bukti bayar masuk") {
		t.Error("tab menunggu bayar tidak menandai bukti yang masuk")
	}
	// bukti transfer memuat data keuangan: pemilik & admin boleh, orang lain 404
	bukti := a.tanya(t, `SELECT bukti_bayar_url FROM promosi WHERE id=$1`, idIklan)
	if kode, _ := penyedia.get(bukti); kode != http.StatusOK {
		t.Errorf("pemilik membuka buktinya sendiri = %d", kode)
	}
	if kode, _ := admin.get(bukti); kode != http.StatusOK {
		t.Errorf("admin membuka bukti = %d", kode)
	}
	if kode, _ := a.klienBaru(t).get(bukti); kode != http.StatusNotFound {
		t.Errorf("pengunjung anonim membuka bukti = %d, harusnya 404", kode)
	}
	orangLain := a.klienBaru(t)
	daftar(t, orangLain, "Orang Lain Bukti", "081311100022")
	if kode, _ := orangLain.get(bukti); kode != http.StatusNotFound {
		t.Errorf("pengguna lain membuka bukti = %d, harusnya 404", kode)
	}
	harusStatus(t, admin.post("/admin/promosi/"+idIklan+"/konfirmasi", nil, false), http.StatusSeeOther, "konfirmasi bayar")
	if st := status(idIklan); st != "aktif" {
		t.Errorf("setelah bayar dikonfirmasi status %q, harusnya aktif", st)
	}

	// --- tombol sorot lama: tercatat sebagai promosi bersumber admin ---
	req := httptest.NewRequest(http.MethodPost, "/admin/jasa/"+jasaID+"/sorot?hari=7", nil)
	req.Header.Set("HX-Request", "true")
	if res := admin.kirim(req); res.StatusCode != http.StatusOK {
		t.Fatalf("tombol sorot lama = %d", res.StatusCode)
	}
	if n := a.tanya(t, `SELECT count(*)::text FROM promosi WHERE service_id=$1 AND sumber='admin' AND status='aktif'`, jasaID); n != "1" {
		t.Errorf("sorotan lewat tombol lama tercatat %s kali sebagai promosi admin", n)
	}
	if disorot() != "true" {
		t.Error("tombol sorot lama tidak menyalakan featured_until lewat trigger")
	}
	req = httptest.NewRequest(http.MethodPost, "/admin/jasa/"+jasaID+"/sorot?hari=0", nil)
	req.Header.Set("HX-Request", "true")
	admin.kirim(req)
	if disorot() == "true" {
		t.Error("cabut sorotan lewat tombol lama tidak mematikan featured_until")
	}
	if n := a.tanya(t, `SELECT count(*)::text FROM promosi WHERE service_id=$1 AND status='dihentikan'`, jasaID); n != "2" {
		t.Errorf("riwayat dihentikan untuk jasa ini = %s, harusnya 2", n)
	}
}

// TestPenayanganIklan: iklan yang disetujui tayang di slot paketnya saja,
// tersaring kecamatan penonton, kliknya terhitung dan dialihkan, tayangnya
// disalin dari Redis, dan yang kedaluwarsa ditutup lalu hilang dari halaman.
func TestPenayanganIklan(t *testing.T) {
	a := siapkan(t)
	ctx := context.Background()
	pengiklan := a.klienBaru(t)
	penonton := a.klienBaru(t)

	daftar(t, pengiklan, "Kedai Kopi", "081311100031")
	adminUser, _, err := a.Services.Admin.CreateAdmin(ctx, "Admin Iklan", "081311100039", "rahasia-e2e-123")
	if err != nil {
		t.Fatal(err)
	}
	// paket Dasar: slot cari_atas & detail_samping, bukan beranda_atas
	paket := a.tanya(t, `SELECT id::text FROM paket_promosi WHERE jenis='iklan' AND nama='Dasar 7 hari'`)
	res := pengiklan.postMultipartBerkas("/promosi", map[string]string{
		"jenis": "iklan", "paket_id": paket, "judul": "Kopi susu gula aren Kedai Kopi",
		"tautan_url": "https://kedaikopi.example", "target_kecamatan": "Bengkalis",
	}, "gambar", "kopi.png", pngUji(t))
	harusStatus(t, res, http.StatusSeeOther, "ajukan iklan")
	id := regexp.MustCompile(`/promosi/(\d+)`).FindStringSubmatch(lokasi(res))[1]
	idInt, _ := strconv.ParseInt(id, 10, 64)

	// belum disetujui → tidak tayang di mana pun
	if _, isi := penonton.get("/cari"); strings.Contains(isi, "Kopi susu gula aren") {
		t.Fatal("iklan yang belum disetujui sudah tayang")
	}
	if _, err := a.Services.Promosi.Setujui(ctx, adminUser.ID, idInt); err != nil {
		t.Fatal(err)
	}

	// --- tayang di slot paketnya, lewat rute klik; tidak di slot lain ---
	_, isi := penonton.get("/cari")
	if !strings.Contains(isi, "Kopi susu gula aren") || !strings.Contains(isi, "/iklan/"+id+"/klik") {
		t.Error("iklan tidak tayang di slot cari_atas dengan tautan klik")
	}
	if _, isi := penonton.get("/jasa/1"); !strings.Contains(isi, "Kopi susu gula aren") {
		t.Error("iklan tidak tayang di slot detail_samping")
	}
	if _, isi := penonton.get("/"); strings.Contains(isi, "Kopi susu gula aren") {
		t.Error("iklan tayang di beranda padahal paketnya tidak mencakup slot itu")
	}

	// --- penonton di kecamatan lain tidak melihatnya; sekecamatan melihat ---
	harusStatus(t, penonton.post("/lokasi", url.Values{"kecamatan": {"Rupat"}, "next": {"/cari"}}, false), http.StatusSeeOther, "pilih Rupat")
	if _, isi := penonton.get("/cari"); strings.Contains(isi, "Kopi susu gula aren") {
		t.Error("iklan bertarget Bengkalis tayang ke penonton Rupat")
	}
	harusStatus(t, penonton.post("/lokasi", url.Values{"kecamatan": {"Bengkalis"}, "next": {"/cari"}}, false), http.StatusSeeOther, "pilih Bengkalis")
	if _, isi := penonton.get("/cari"); !strings.Contains(isi, "Kopi susu gula aren") {
		t.Error("iklan bertarget Bengkalis tidak tayang ke penonton Bengkalis")
	}

	// --- klik terhitung dan dialihkan ---
	klik := penonton.kirim(httptest.NewRequest(http.MethodGet, "/iklan/"+id+"/klik", nil))
	if klik.StatusCode != http.StatusFound || lokasi(klik) != "https://kedaikopi.example" {
		t.Errorf("klik = %d → %q", klik.StatusCode, lokasi(klik))
	}
	if n := a.tanya(t, `SELECT klik::text FROM promosi WHERE id=$1`, id); n != "1" {
		t.Errorf("klik tercatat %s, harusnya 1", n)
	}

	// --- tayang disalin dari Redis ke database ---
	if n := a.Services.Promosi.SalinPenghitungTayang(ctx); n != 1 {
		t.Errorf("penyalinan menyentuh %d promosi, harusnya 1", n)
	}
	tayang, _ := strconv.Atoi(a.tanya(t, `SELECT tayang::text FROM promosi WHERE id=$1`, id))
	if tayang < 3 {
		t.Errorf("tayang = %d, harusnya ≥ 3 (cari ×2, detail ×1 yang menampilkannya)", tayang)
	}
	if _, isi := pengiklan.get("/promosi/" + id); !strings.Contains(isi, strconv.Itoa(tayang)+" / 1") {
		t.Error("halaman pengiklan tidak menampilkan tayang / klik yang benar")
	}

	// --- kedaluwarsa: ditutup ticker, hilang dari halaman ---
	if _, err := a.Pool.Exec(ctx, `UPDATE promosi SET selesai_at = NOW() - interval '1 minute' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	ditutup, err := a.Services.Promosi.TandaiKedaluwarsa(ctx)
	if err != nil || len(ditutup) != 1 {
		t.Fatalf("TandaiKedaluwarsa: %v, ditutup %d", err, len(ditutup))
	}
	if st := a.tanya(t, `SELECT status FROM promosi WHERE id=$1`, id); st != "selesai" {
		t.Errorf("status setelah kedaluwarsa %q", st)
	}
	if _, isi := penonton.get("/cari"); strings.Contains(isi, "Kopi susu gula aren") {
		t.Error("iklan yang sudah selesai masih tayang")
	}
	if klik := penonton.kirim(httptest.NewRequest(http.MethodGet, "/iklan/"+id+"/klik", nil)); klik.StatusCode != http.StatusNotFound {
		t.Errorf("klik iklan yang sudah selesai = %d, harusnya 404", klik.StatusCode)
	}
}

// TestStatistikDanPengingat: dasbor admin merangkum promosi, dan pemohon
// diingatkan tepat sekali sehari sebelum masa tayang habis.
func TestStatistikDanPengingat(t *testing.T) {
	a := siapkan(t)
	ctx := context.Background()
	pengiklan := a.klienBaru(t)
	admin := a.klienBaru(t)

	daftar(t, pengiklan, "Toko Bangunan", "081311100041")
	adminUser, _, err := a.Services.Admin.CreateAdmin(ctx, "Admin Statistik", "081311100049", "rahasia-e2e-123")
	if err != nil {
		t.Fatal(err)
	}
	harusStatus(t, admin.post("/masuk", url.Values{"identifier": {"081311100049"}, "password": {"rahasia-e2e-123"}}, false),
		http.StatusSeeOther, "masuk admin")

	paket := a.tanya(t, `SELECT id::text FROM paket_promosi WHERE jenis='iklan' ORDER BY id LIMIT 1`)
	var ids []string
	for i := 1; i <= 2; i++ {
		res := pengiklan.postMultipartBerkas("/promosi", map[string]string{
			"jenis": "iklan", "paket_id": paket, "judul": fmt.Sprintf("Semen murah %d", i),
		}, "gambar", "s.png", pngUji(t))
		harusStatus(t, res, http.StatusSeeOther, "ajukan iklan")
		ids = append(ids, regexp.MustCompile(`/promosi/(\d+)`).FindStringSubmatch(lokasi(res))[1])
	}
	// satu disetujui (tayang), satu dibiarkan menunggu
	id0, _ := strconv.ParseInt(ids[0], 10, 64)
	if _, err := a.Services.Promosi.Setujui(ctx, adminUser.ID, id0); err != nil {
		t.Fatal(err)
	}

	// --- dasbor admin ---
	kode, isi := admin.get("/admin")
	if kode != http.StatusOK {
		t.Fatalf("/admin = %d", kode)
	}
	for _, mau := range []string{"1 iklan · 0 sorotan · 0 penyedia", "1 ditinjau · 0 bayar", "Pengajuan promosi"} {
		if !strings.Contains(isi, mau) {
			t.Errorf("dasbor admin tidak memuat %q", mau)
		}
	}

	// --- pengingat H-1: sekali, dan hanya untuk yang akan berakhir ---
	if n := a.Services.Promosi.IngatkanAkanSelesai(ctx); n != 0 {
		t.Errorf("pengingat terkirim %d padahal belum ada yang akan berakhir", n)
	}
	if _, err := a.Pool.Exec(ctx, `UPDATE promosi SET selesai_at = NOW() + interval '6 hours' WHERE id = $1`, ids[0]); err != nil {
		t.Fatal(err)
	}
	if n := a.Services.Promosi.IngatkanAkanSelesai(ctx); n != 1 {
		t.Errorf("pengingat terkirim %d, harusnya 1", n)
	}
	if n := a.Services.Promosi.IngatkanAkanSelesai(ctx); n != 0 {
		t.Errorf("pengingat terkirim lagi %d kali; harus tepat sekali", n)
	}
	if _, isi := pengiklan.get("/notifikasi"); !strings.Contains(isi, "Promosi berakhir besok") {
		t.Error("pengingat tidak tampil di notifikasi pemohon")
	}

	// --- halaman pengiklan: rasio klik & rata-rata per hari ---
	if _, isi := pengiklan.get("/promosi/" + ids[0]); !strings.Contains(isi, "Rasio klik") || !strings.Contains(isi, "Rata-rata tayang / hari") {
		t.Error("statistik iklan tidak tampil di halaman pengiklan")
	}
}

// TestAPIUntukAndroid meniru aplikasi native: tanpa cookie sama sekali —
// hanya bearer token — menelusuri konfigurasi, pencarian berlokasi, profil
// penyedia by slug, obrolan (termasuk siaran SSE di lapisan service),
// pesanan, ulasan, notifikasi, promosi, iklan, perangkat, dan cabut sesi.
func TestAPIUntukAndroid(t *testing.T) {
	a := siapkan(t)
	ctx := context.Background()
	pencari := a.klienNative(t)
	penyedia := a.klienNative(t)

	// --- daftar & masuk mengembalikan token; /auth/me memakai bearer ---
	kode, res := pencari.postJSON("/api/v1/auth/register", map[string]any{
		"full_name": "Pencari Native", "phone": "081311100051", "kecamatan": "Bengkalis",
		"password": "rahasia-e2e-123", "password_confirm": "rahasia-e2e-123",
	})
	if kode != http.StatusCreated || data(t, res)["token"] == "" {
		t.Fatalf("register API = %d %v", kode, res)
	}
	pencari.token, _ = data(t, res)["token"].(string)
	if kode, res := pencari.getJSON("/api/v1/auth/me"); kode != http.StatusOK || data(t, res)["user"] == nil {
		t.Fatalf("/auth/me dengan bearer = %d %v", kode, res)
	}
	// pengguna biasa (bukan penyedia): rute akun di bawah /me terbuka,
	// rute penyedia di bawah /me tertutup — keduanya berawalan sama.
	if kode, _ := pencari.getJSON("/api/v1/me/unread"); kode != http.StatusOK {
		t.Errorf("/me/unread bagi bukan penyedia = %d, harusnya 200", kode)
	}
	if kode, _ := pencari.getJSON("/api/v1/me/services"); kode != http.StatusForbidden {
		t.Errorf("/me/services bagi bukan penyedia = %d, harusnya 403", kode)
	}
	// tanpa token → 401
	if kode, _ := a.klienNative(t).getJSON("/api/v1/auth/me"); kode != http.StatusUnauthorized {
		t.Errorf("/auth/me tanpa token = %d, harusnya 401", kode)
	}

	// --- penyedia lewat API: profil, lokasi, jasa, disetujui admin ---
	kode, res = penyedia.postJSON("/api/v1/auth/register", map[string]any{
		"full_name": "Penyedia Native", "phone": "081311100052", "kecamatan": "Bengkalis",
		"password": "rahasia-e2e-123", "password_confirm": "rahasia-e2e-123",
	})
	if kode != http.StatusCreated {
		t.Fatalf("register penyedia = %d %v", kode, res)
	}
	penyedia.token, _ = data(t, res)["token"].(string)
	if kode, res := penyedia.postJSON("/api/v1/providers", map[string]any{"bio": "Teknisi listrik rumah dan instalasi baru, bergaransi, wilayah Bengkalis."}); kode != http.StatusCreated {
		t.Fatalf("buat profil penyedia = %d %v", kode, res)
	}
	if kode, res := penyedia.patchJSON("/api/v1/me/provider/location", map[string]any{
		"latitude": 1.4667, "longitude": 102.1, "address_label": "Jl. Hangtuah", "service_radius_km": 20,
	}); kode != http.StatusOK || data(t, res)["latitude"] == nil {
		t.Fatalf("lokasi penyedia = %d %v", kode, res)
	}
	kategori := a.tanya(t, `SELECT id::text FROM categories WHERE parent_id IS NOT NULL ORDER BY id LIMIT 1`)
	kategoriInt, _ := strconv.ParseInt(kategori, 10, 64)
	kode, res = penyedia.postJSON("/api/v1/me/services", map[string]any{
		"category_id": kategoriInt, "title": "Instalasi listrik rumah native", "price_type": "fixed",
		"price_min": "750000", "price_max": "750000",
		"description": "Instalasi listrik rumah baru termasuk MCB dan grounding, dikerjakan teknisi bersertifikat.",
	})
	if kode != http.StatusCreated {
		t.Fatalf("buat jasa = %d %v", kode, res)
	}
	jasaID := angka(data(t, res)["id"])
	adminUser, _, err := a.Services.Admin.CreateAdmin(ctx, "Admin Native", "081311100059", "rahasia-e2e-123")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Services.Admin.SetujuiListing(ctx, adminUser.ID, jasaID); err != nil {
		t.Fatal(err)
	}

	// --- config: satu panggilan saat aplikasi dibuka ---
	kode, res = pencari.getJSON("/api/v1/config")
	cfg := data(t, res)
	if kode != http.StatusOK || cfg["base_url"] == "" || cfg["kategori"] == nil || cfg["lokasi"] == nil || cfg["menu"] == nil {
		t.Fatalf("/config = %d, kunci utama hilang: %v", kode, cfg)
	}
	// menu mengikuti pemanggil: bearer pengguna → ada Keluar; tamu → ada Masuk
	if m := labelMenuJSON(cfg["menu"]); !strings.Contains(m, "Keluar") || strings.Contains(m, "Masuk") {
		t.Errorf("menu /config untuk pengguna ber-bearer salah: %s", m)
	}
	_, resTamu := a.klienNative(t).getJSON("/api/v1/config")
	if m := labelMenuJSON(data(t, resTamu)["menu"]); !strings.Contains(m, "Masuk") || strings.Contains(m, "Keluar") {
		t.Errorf("menu /config untuk tamu salah: %s", m)
	}

	// --- pencarian sadar-lokasi & profil by slug ---
	kode, res = pencari.getJSON("/api/v1/services?q=listrik&lat=1.4667&lng=102.1&urut=terdekat")
	items, _ := data(t, res)["items"].([]any)
	if kode != http.StatusOK || len(items) == 0 {
		t.Fatalf("pencarian berlokasi = %d, %d hasil", kode, len(items))
	}
	if pertama, _ := items[0].(map[string]any); pertama["jarak_km"] == nil {
		t.Error("hasil pencarian berlokasi tidak membawa jarak_km")
	}
	kode, res = pencari.getJSON("/api/v1/providers/slug/penyedia-native")
	if kode != http.StatusOK || data(t, res)["area_layanan"] == nil {
		t.Fatalf("profil by slug = %d %v", kode, res)
	}
	if bg, _ := data(t, res)["bagikan"].(map[string]any); bg == nil || !strings.HasSuffix(fmt.Sprint(bg["url"]), "/penyedia/penyedia-native") {
		t.Errorf("profil by slug tanpa data bagikan: %v", data(t, res)["bagikan"])
	}
	if kode, res := pencari.getJSON(fmt.Sprintf("/api/v1/services/%d", jasaID)); kode != http.StatusOK {
		t.Fatalf("detail jasa API = %d", kode)
	} else if bg, _ := data(t, res)["bagikan"].(map[string]any); bg == nil || len(bg["target"].([]any)) != 5 || data(t, res)["title"] == nil {
		t.Errorf("detail jasa API: bagikan/target/title hilang: %v", res)
	}
	if _, ada := data(t, res)["provider"].(map[string]any)["whatsapp_number"]; ada {
		t.Error("whatsapp_number bocor lewat profil by slug")
	}

	// --- obrolan: mulai, siaran SSE di lapisan service, kirim, ambil ---
	kode, res = pencari.postJSON(fmt.Sprintf("/api/v1/services/%d/conversations", jasaID), nil)
	if kode != http.StatusCreated {
		t.Fatalf("mulai obrolan = %d %v", kode, res)
	}
	convID := angka(data(t, res)["id"])
	saluran, batal := a.Services.Chat.Langgan(convID)
	defer batal()
	kode, res = pencari.postJSON(fmt.Sprintf("/api/v1/conversations/%d/messages", convID), map[string]any{"body": "Halo, bisa survei besok?"})
	if kode != http.StatusCreated {
		t.Fatalf("kirim pesan = %d %v", kode, res)
	}
	select {
	case m := <-saluran:
		if m.Body != "Halo, bisa survei besok?" {
			t.Errorf("siaran membawa pesan lain: %q", m.Body)
		}
	case <-time.After(2 * time.Second):
		t.Error("pesan yang dikirim lewat API tidak disiarkan ke pendengar SSE")
	}
	kode, res = penyedia.getJSON(fmt.Sprintf("/api/v1/conversations/%d/messages?since=0", convID))
	pesan, _ := data(t, res)["messages"].([]any)
	if kode != http.StatusOK || len(pesan) != 1 {
		t.Fatalf("ambil pesan = %d, %d pesan", kode, len(pesan))
	}
	if kode, res := penyedia.getJSON("/api/v1/me/unread"); kode != http.StatusOK || data(t, res)["notifikasi"] == nil {
		t.Errorf("unread = %d %v", kode, res)
	}

	// --- pesanan → status → ulasan ---
	kode, res = pencari.postJSON(fmt.Sprintf("/api/v1/services/%d/orders", jasaID), map[string]any{"notes": "Rumah baru 2 lantai, 12 titik lampu."})
	if kode != http.StatusCreated {
		t.Fatalf("buat pesanan = %d %v", kode, res)
	}
	orderID := angka(data(t, res)["order"].(map[string]any)["id"])
	for _, st := range []string{"accepted", "completed"} {
		if kode, res := penyedia.postJSON(fmt.Sprintf("/api/v1/orders/%d/status", orderID), map[string]any{"status": st}); kode != http.StatusOK || data(t, res)["status"] != st {
			t.Fatalf("ubah status %s = %d %v", st, kode, res)
		}
	}
	if kode, res := pencari.postJSON(fmt.Sprintf("/api/v1/orders/%d/review", orderID), map[string]any{"rating": 5, "comment": "Rapi dan cepat."}); kode != http.StatusCreated {
		t.Fatalf("ulasan = %d %v", kode, res)
	}

	// --- notifikasi: judul disusun server ---
	kode, res = penyedia.getJSON("/api/v1/notifications")
	notif, _ := res["data"].([]any)
	if kode != http.StatusOK || len(notif) == 0 {
		t.Fatalf("notifikasi = %d, %d item", kode, len(notif))
	}
	if n0, _ := notif[0].(map[string]any); n0["judul"] == nil || n0["tautan"] == nil {
		t.Error("notifikasi API tidak membawa judul/tautan yang sudah disusun")
	}
	if kode, _ := penyedia.postJSON("/api/v1/notifications/read", nil); kode != http.StatusOK {
		t.Error("tandai terbaca gagal")
	}

	// --- promosi: paket dari config, ajukan iklan (multipart + bearer), iklan tayang ---
	paketList, _ := cfg["promosi"].(map[string]any)["paket"].([]any)
	var paketIklan int64
	for _, p := range paketList {
		pm, _ := p.(map[string]any)
		if pm["jenis"] == "iklan" {
			paketIklan = angka(pm["id"])
			break
		}
	}
	res2 := pencari.postMultipartBerkas("/api/v1/me/promotions", map[string]string{
		"jenis": "iklan", "paket_id": strconv.FormatInt(paketIklan, 10), "judul": "Toko listrik native",
		"tautan_url": "https://toko.example", "target_kecamatan": "Bengkalis",
	}, "gambar", "iklan.png", pngUji(t))
	if res2.StatusCode != http.StatusCreated {
		t.Fatalf("ajukan iklan via API = %d: %.200s", res2.StatusCode, baca(res2))
	}
	promoID := angka(uraiJSON(t, res2)["data"].(map[string]any)["id"])
	if _, err := a.Services.Promosi.Setujui(ctx, adminUser.ID, promoID); err != nil {
		t.Fatal(err)
	}
	kode, res = pencari.getJSON("/api/v1/ads?slot=cari_atas&kecamatan=Bengkalis")
	if kode != http.StatusOK || data(t, res)["sumber"] != "promosi" || data(t, res)["klik_url"] == "" {
		t.Errorf("iklan API = %d %v", kode, res)
	}
	if kode, res := pencari.getJSON("/api/v1/ads?slot=cari_atas&kecamatan=Rupat"); kode != http.StatusOK || res["data"] != nil {
		t.Errorf("iklan bertarget Bengkalis tampil ke Rupat: %v", res)
	}
	if kode, res := pencari.getJSON(fmt.Sprintf("/api/v1/me/promotions/%d", promoID)); kode != http.StatusOK || data(t, res)["label_status"] != "Sedang tayang" {
		t.Errorf("detail promosi API = %d %v", kode, res)
	}

	// --- perangkat: didaftarkan meski push mati ---
	if kode, res := pencari.postJSON("/api/v1/me/devices", map[string]any{"token": strings.Repeat("t", 40), "platform": "android"}); kode != http.StatusOK || data(t, res)["push_aktif"] != false {
		t.Errorf("daftar perangkat = %d %v", kode, res)
	}
	if n := a.tanya(t, `SELECT count(*)::text FROM device_tokens`); n != "1" {
		t.Errorf("token perangkat tersimpan %s", n)
	}

	// --- cabut semua sesi: token lama mati ---
	if kode, _ := pencari.postJSON("/api/v1/auth/logout-all", nil); kode != http.StatusOK {
		t.Fatal("logout-all gagal")
	}
	if kode, _ := pencari.getJSON("/api/v1/auth/me"); kode != http.StatusUnauthorized {
		t.Errorf("token setelah logout-all masih diterima: %d", kode)
	}
}

// TestSeedDemo: konten peragaan lengkap terbentuk di atas seed dasar, tampil
// di halaman publik, dan aman diulang.
func TestSeedDemo(t *testing.T) {
	a := siapkan(t)
	ctx := context.Background()
	t.Setenv("DEMO_PASSWORD", "demo-e2e-123456")
	if err := seed.Demo(ctx, a.Repos, a.Services.Upload); err != nil {
		t.Fatalf("seed demo: %v", err)
	}
	for tabel, minimal := range map[string]int{"users": 10, "service_images": 10, "portfolios": 6, "orders": 6, "reviews": 4, "messages": 10} {
		n := a.tanya(t, "SELECT count(*)::text FROM "+tabel)
		if v, _ := strconv.Atoi(n); v < minimal {
			t.Errorf("%s = %s, minimal %d", tabel, n, minimal)
		}
	}
	if n := a.tanya(t, `SELECT count(*)::text FROM services WHERE status='active' AND total_kunjungan = 0`); n != "0" {
		t.Errorf("%s jasa aktif tanpa riwayat kunjungan demo", n)
	}
	if n := a.tanya(t, `SELECT count(DISTINCT tanggal)::text FROM kunjungan_jasa`); n != "29" {
		t.Errorf("riwayat kunjungan demo mencakup %s hari, harusnya 29", n)
	}
	if n := a.tanya(t, `SELECT count(*)::text FROM promosi WHERE status='aktif'`); n != "5" {
		t.Errorf("promosi aktif = %s, harusnya 5 (2 iklan, 1 sorotan, 2 penyedia pilihan)", n)
	}
	if n := a.tanya(t, `SELECT count(*)::text FROM provider_profiles WHERE featured_until > now()`); n != "2" {
		t.Errorf("trigger featured dari promosi: %s penyedia, harusnya 2", n)
	}
	if r := a.tanya(t, `SELECT avg_rating::text FROM provider_profiles p JOIN users u ON u.id=p.user_id WHERE u.phone='628117512001'`); !strings.HasPrefix(r, "5") {
		t.Errorf("rating penyedia pertama %s, harusnya 5", r)
	}
	k := a.klienBaru(t)
	if _, isi := k.get("/"); !strings.Contains(isi, "Servis AC panggilan, garansi 30 hari") {
		t.Error("iklan demo tidak tayang di beranda")
	} else if strings.Count(isi, `href="/penyedia/`) < 2 {
		t.Error("kartu penyedia pilihan di beranda tidak bertautan ke halaman penyedia")
	}
	if _, isi := k.get("/penyedia/rizal-teknik-ac"); !strings.Contains(isi, "Datang tepat waktu") || !strings.Contains(isi, "/uploads/") {
		t.Error("ulasan atau foto demo tidak tampil di halaman penyedia")
	} else if !strings.Contains(isi, `data-bagikan`) || !strings.Contains(isi, `property="og:image" content="http://localhost:3000/uploads/`) {
		t.Error("halaman penyedia: tombol bagikan atau og:image avatar hilang")
	}
	// Halaman jasa: tombol bagikan, tautan WhatsApp ter-escape, OG memakai foto jasa.
	if _, isi := k.get("/jasa/1"); !strings.Contains(isi, `https://wa.me/?text=`) || !strings.Contains(isi, `data-salin-tautan="http://localhost:3000/jasa/1"`) || !strings.Contains(isi, `property="og:image" content="http://localhost:3000/uploads/`) || !strings.Contains(isi, `property="og:url" content="http://localhost:3000/jasa/1"`) {
		t.Error("halaman jasa: sheet bagikan atau meta Open Graph tidak lengkap")
	}
	if _, isi := k.get("/"); !strings.Contains(isi, `property="og:image" content="http://localhost:3000/static/img/og-default.png"`) {
		t.Error("beranda tanpa og:image bawaan")
	}
	harusStatus(t, k.post("/masuk", url.Values{"identifier": {"628117512011"}, "password": {"demo-e2e-123456"}}, false), http.StatusSeeOther, "masuk akun demo")
	if err := seed.Demo(ctx, a.Repos, a.Services.Upload); err != nil {
		t.Fatalf("seed demo ulang: %v", err)
	}
	if n := a.tanya(t, `SELECT count(*)::text FROM users WHERE phone='628117512011'`); n != "1" {
		t.Errorf("seed demo diulang membuat akun ganda: %s", n)
	}
}

// TestStatistikKunjungan: kunjungan dihitung di Redis lalu disalin ke tabel
// harian; pemilik, admin, dan bot tidak dihitung; angka tampil di kartu,
// halaman detail, dan API; pemilik melihat statistiknya.
func TestStatistikKunjungan(t *testing.T) {
	a := siapkan(t)
	ctx := context.Background()
	ua := func(k *klien, agen string) func(string) (int, string) {
		return func(path string) (int, string) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("User-Agent", agen)
			res := k.kirim(req)
			return res.StatusCode, baca(res)
		}
	}
	pengunjungA := ua(a.klienBaru(t), "Mozilla/5.0 (Linux; Android 14) Chrome/128 Mobile Safari/537.36")
	pengunjungB := ua(a.klienBaru(t), "Mozilla/5.0 (Macintosh) Safari/605.1.15")
	bot := ua(a.klienBaru(t), "facebookexternalhit/1.1")
	pengunjungA("/jasa/1")
	pengunjungA("/jasa/1") // orang yang sama dua kali: 2 kunjungan, 1 unik
	pengunjungB("/jasa/1")
	bot("/jasa/1") // pratinjau tautan: tidak dihitung

	// pemilik (seed: 628117512001) melihat jasanya sendiri: tidak dihitung
	pemilik := a.klienBaru(t)
	harusStatus(t, pemilik.post("/masuk", url.Values{"identifier": {"628117512001"}, "password": {"rahasia123"}}, false), http.StatusSeeOther, "masuk pemilik")
	req := httptest.NewRequest(http.MethodGet, "/jasa/1", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 14) Chrome/128")
	if res := pemilik.kirim(req); res.StatusCode != http.StatusOK {
		t.Fatalf("pemilik buka jasa = %d", res.StatusCode)
	}

	if n := a.Services.Kunjungan.Salin(ctx); n != 1 {
		t.Fatalf("salin kunjungan memperbarui %d baris, harusnya 1 (satu jasa, satu hari)", n)
	}
	if got := a.tanya(t, `SELECT jumlah::text || '/' || unik::text FROM kunjungan_jasa WHERE service_id=1`); got != "3/2" {
		t.Errorf("kunjungan hari ini = %s, harusnya 3/2 (jumlah/unik)", got)
	}
	if got := a.tanya(t, `SELECT total_kunjungan::text FROM services WHERE id=1`); got != "3" {
		t.Errorf("trigger total_kunjungan = %s, harusnya 3", got)
	}
	// tampil di halaman detail, kartu pencarian, dan API — dibaca lewat UA bot
	// supaya pembacaan ini sendiri tidak menambah hitungan
	if _, isi := bot("/jasa/1"); !strings.Contains(isi, "Dilihat 3 kali") {
		t.Error("halaman detail tidak menampilkan 'Dilihat 3 kali'")
	}
	if _, isi := bot("/cari?q=cuci+ac"); !strings.Contains(isi, `title="3 kali dilihat"`) {
		t.Error("kartu listing tidak menampilkan jumlah dilihat")
	}
	if kode, res := a.klienNative(t).getJSON("/api/v1/services/1"); kode != http.StatusOK || angka(data(t, res)["total_kunjungan"]) != 3 {
		t.Errorf("API detail total_kunjungan = %v", data(t, res)["total_kunjungan"])
	}
	// panel statistik publik: tamu (bot UA, agar tidak menambah hitungan) melihatnya di web dan lewat API
	if _, isi := bot("/jasa/1"); !strings.Contains(isi, `id="judul-statistik"`) || !strings.Contains(isi, "2 pengunjung berbeda") {
		t.Error("tamu tidak melihat panel statistik kunjungan")
	}
	if kode, res := a.klienNative(t).getJSON("/api/v1/services/1/stats"); kode != http.StatusOK || angka(data(t, res)["hari_7"]) != 3 || angka(data(t, res)["unik_30"]) != 2 || len(data(t, res)["harian"].([]any)) != 30 {
		t.Errorf("API statistik publik = %d %v", kode, res)
	}
	// statistik di-cache 60 detik: kunjungan baru belum tampak sebelum cache dibersihkan
	a.Rdb.FlushDB(ctx)
	// setelah pengunjung lain datang lagi, kunjungan tercatat menambah (bukan menimpa)
	pengunjungA("/jasa/1")
	a.Services.Kunjungan.Salin(ctx)
	if got := a.tanya(t, `SELECT total_kunjungan::text FROM services WHERE id=1`); got != "4" {
		t.Errorf("total setelah salinan kedua = %s, harusnya 4", got)
	}
}

// TestHalamanPublik memastikan halaman tanpa login terender, bukan 500.
func TestHalamanPublik(t *testing.T) {
	a := siapkan(t)
	k := a.klienBaru(t)
	if kode, isi := k.get("/"); kode != http.StatusOK || !strings.Contains(isi, `<dialog id="menu-utama"`) || !strings.Contains(isi, `rel="manifest"`) {
		t.Errorf("beranda = %d; menu utama atau tautan manifest hilang", kode)
	}
	if kode, isi := k.get("/tentang"); kode != http.StatusOK || !strings.Contains(isi, `id="cara-kerja"`) || !strings.Contains(isi, `<details class="faq">`) || !strings.Contains(isi, "jasa aktif") {
		t.Errorf("tentang = %d; anchor, FAQ, atau angka hidup hilang", kode)
	}
	if res := k.kirim(httptest.NewRequest(http.MethodGet, "/bantuan", nil)); res.StatusCode != http.StatusMovedPermanently || !strings.HasPrefix(lokasi(res), "/tentang#") {
		t.Errorf("/bantuan = %d → %q, harusnya 301 ke /tentang#…", res.StatusCode, lokasi(res))
	}
	if kode, res := a.klienNative(t).getJSON("/api/v1/help"); kode != http.StatusOK || data(t, res)["bagian"] == nil || data(t, res)["ringkasan"] == nil || data(t, res)["paket"] == nil {
		t.Errorf("/api/v1/help = %d %v", kode, res)
	}
	manifest := k.kirim(httptest.NewRequest(http.MethodGet, "/manifest.webmanifest", nil))
	if ct := manifest.Header.Get("Content-Type"); manifest.StatusCode != http.StatusOK || !strings.HasPrefix(ct, "application/manifest+json") {
		t.Errorf("manifest = %d %s", manifest.StatusCode, ct)
	}
	if isi := baca(manifest); !strings.Contains(isi, `"display":"standalone"`) || !strings.Contains(isi, `"background_color":"#ffffff"`) || !strings.Contains(isi, "ikon-512.png") {
		t.Errorf("isi manifest tidak lengkap: %.300s", isi)
	}
	for _, path := range []string{"/", "/cari", "/cari?q=ac", "/penyedia", "/jasa/1", "/penyedia/rizal-teknik-ac", "/tentang", "/masuk", "/daftar", "/sehat"} {
		if kode, _ := k.get(path); kode != http.StatusOK {
			t.Errorf("%s = %d", path, kode)
		}
	}
	if kode, _ := k.get("/dasbor"); kode != http.StatusSeeOther {
		t.Errorf("/dasbor tanpa login = %d, harusnya dialihkan ke /masuk", kode)
	}
	// Panel admin: pengunjung anonim diperlakukan seperti halaman terlindung
	// lain (dialihkan ke /masuk), sedangkan pengguna yang sudah masuk tetapi
	// bukan admin mendapat 404 — keberadaan panel tidak dikonfirmasi kepada
	// orang yang tidak berkepentingan.
	res := k.kirim(httptest.NewRequest(http.MethodGet, "/admin", nil))
	if res.StatusCode != http.StatusSeeOther || !strings.HasPrefix(lokasi(res), "/masuk") {
		t.Errorf("/admin tanpa login = %d → %q, harusnya dialihkan ke /masuk", res.StatusCode, lokasi(res))
	}
	daftar(t, k, "Bukan Admin", "081311100003")
	if kode, _ := k.get("/admin"); kode != http.StatusNotFound {
		t.Errorf("/admin oleh pengguna biasa = %d, harusnya 404", kode)
	}
}
