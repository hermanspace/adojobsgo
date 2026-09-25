// Package handler merangkai seluruh rute aplikasi.
package handler

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"

	"github.com/hermansyah/adojobsid/internal/handler/api"
	"github.com/hermansyah/adojobsid/internal/handler/web"
	"github.com/hermansyah/adojobsid/internal/middleware"
)

// Register memasang seluruh rute: halaman web dan endpoint JSON.
// Keduanya berbagi middleware sesi yang sama.
func Register(app *fiber.App, w *web.Handler, a *api.Handler, auth *middleware.Auth) {
	// Sesi dibaca untuk setiap request, termasuk halaman publik, agar navigasi
	// bisa menampilkan status login tanpa query tambahan.
	app.Use(auth.Load)

	// Satu pembatas untuk semua rute unggah, dikunci per akun: kuota
	// berkasnya berlaku menyeluruh, bukan per rute. Tanpa ini, satu akun
	// bisa membanjiri disk dan CPU pemroses gambar tanpa hambatan — hanya
	// endpoint masuk yang selama ini dibatasi.
	batasUnggah := limiter.New(limiter.Config{
		Max:          40,
		Expiration:   time.Minute,
		KeyGenerator: middleware.KunciPembatasPengguna,
		LimitReached: func(c *fiber.Ctx) error {
			if strings.HasPrefix(c.Path(), "/api/v1/") {
				return a.TerlaluBanyak(c)
			}
			return w.TerlaluBanyakUnggahan(c)
		},
	})

	registerAPI(app, a, auth, batasUnggah)
	registerWeb(app, w, auth, batasUnggah)
	registerAdmin(app, w, auth, batasUnggah)

	// Fallback: rute /api/v1 menjawab JSON, selebihnya halaman 404.
	app.Use(func(c *fiber.Ctx) error {
		if len(c.Path()) >= 8 && c.Path()[:8] == "/api/v1/" {
			return a.NotFound(c)
		}
		return w.NotFoundHandler(c)
	})
}

func registerAPI(app *fiber.App, a *api.Handler, auth *middleware.Auth, batasUnggah fiber.Handler) {
	// API memakai cookie sesi yang sama dengan web, jadi ia perlu penjaga
	// CSRF-nya sendiri: form HTML tidak punya token untuk endpoint JSON.
	v1 := app.Group("/api/v1", middleware.TolakLintasSitusAPI)

	v1.Get("/config", a.Config)
	v1.Get("/help", a.Help)
	v1.Get("/ads", a.Ad)
	v1.Get("/providers", a.ProviderDirectory)
	v1.Get("/providers/featured", a.FeaturedProviders)
	v1.Get("/providers/slug/:slug", a.ProviderBySlug)
	v1.Post("/auth/logout-all", auth.RequireAPI, a.LogoutSemua)
	v1.Post("/auth/register", a.Register)
	v1.Post("/auth/login", a.Login)
	v1.Post("/auth/logout", a.Logout)
	v1.Get("/auth/me", auth.RequireAPI, a.Me)

	v1.Get("/categories", a.Categories)
	v1.Get("/kecamatan", a.KecamatanOptions)
	v1.Get("/services", a.SearchServices)
	v1.Get("/services/:id", a.ServiceDetail)
	v1.Get("/services/:id/stats", a.ServiceStats)
	v1.Get("/providers/:id", a.ProviderDetail)

	v1.Post("/providers", auth.RequireAPI, a.CreateProvider)
	v1.Patch("/providers/me", auth.RequireProviderAPI, a.UpdateProvider)

	// Rute khusus penyedia. Penjaganya dipasang per rute, BUKAN di grup:
	// middleware grup di Fiber cocok berdasarkan awalan, sehingga
	// Group("/me", RequireProviderAPI) ikut memblokir /me/unread,
	// /me/promotions, dan /me/devices bagi pengguna biasa.
	me := v1.Group("/me")
	penyedia := auth.RequireProviderAPI
	me.Get("/services", penyedia, a.MyListings)
	me.Post("/services", penyedia, a.CreateListing)
	me.Patch("/services/:id", penyedia, a.UpdateListing)
	me.Delete("/services/:id", penyedia, a.DeleteListing)
	me.Post("/services/:id/images", penyedia, batasUnggah, a.UploadListingImages)
	me.Delete("/services/:id/images/:imageID", penyedia, a.DeleteListingImage)
	me.Patch("/provider/location", penyedia, a.UpdateProviderLocation)
	me.Get("/portfolio", penyedia, a.MyPortfolio)
	me.Post("/portfolio", penyedia, batasUnggah, a.AddPortfolio)
	me.Delete("/portfolio/:id", penyedia, a.DeletePortfolio)

	// Rute pengguna masuk (bukan hanya penyedia).
	akun := v1.Group("/", auth.RequireAPI)
	akun.Patch("me", a.UpdateMe)
	akun.Post("me/password", a.ChangePassword)
	akun.Get("me/unread", a.Unread)
	akun.Get("conversations", a.Conversations)
	akun.Post("services/:id/conversations", a.StartConversation)
	akun.Get("conversations/:id/messages", a.Messages)
	akun.Post("conversations/:id/messages", a.SendMessage)
	akun.Get("conversations/:id/stream", a.StreamMessages)
	akun.Post("services/:id/orders", a.CreateOrder)
	akun.Get("orders/:id", a.Order)
	akun.Post("orders/:id/status", a.UpdateOrderStatus)
	akun.Post("orders/:id/review", a.WriteReview)
	akun.Get("notifications", a.Notifications)
	akun.Post("notifications/read", a.MarkNotificationsRead)
	akun.Get("me/promotions", a.MyPromotions)
	akun.Post("me/promotions", batasUnggah, a.SubmitPromotion)
	akun.Get("me/promotions/:id", a.MyPromotion)
	akun.Post("me/promotions/:id/proof", batasUnggah, a.UploadPromotionProof)
	akun.Post("me/promotions/:id/cancel", a.CancelPromotion)
	akun.Post("me/devices", a.RegisterDevice)
	akun.Delete("me/devices", a.UnregisterDevice)
}

// registerAdmin memasang seluruh rute panel admin.
// Setiap rute dijaga RequireAdminWeb, yang menjawab 404 (bukan 403) bagi
// pengguna tanpa hak, sehingga keberadaan panel tidak terkonfirmasi.
func registerAdmin(app *fiber.App, w *web.Handler, auth *middleware.Auth, batasUnggah fiber.Handler) {
	admin := app.Group("/admin", auth.RequireAdminWeb)

	admin.Get("/", w.AdminDasbor)

	admin.Get("/pengguna", w.AdminPengguna)
	admin.Get("/pengguna/tabel", w.AdminPenggunaTabel)
	admin.Post("/pengguna/:id<int>/tangguhkan", w.AdminTangguhkan)
	admin.Post("/pengguna/:id<int>/aktifkan", w.AdminAktifkan)
	admin.Post("/pengguna/:id<int>/peran", w.AdminUbahPeran)
	admin.Get("/pengguna/:id<int>/ubah", w.AdminUbahPenggunaForm)
	admin.Post("/pengguna/:id<int>/ubah", w.AdminUbahPengguna)

	admin.Post("/penyedia/:id<int>/verifikasi", w.AdminVerifikasi)
	admin.Post("/penyedia/:id<int>/sorot", w.AdminSorotPenyedia)

	admin.Get("/jasa", w.AdminJasa)
	admin.Get("/jasa/tabel", w.AdminJasaTabel)
	admin.Post("/jasa/:id<int>/status", w.AdminStatusJasa)
	admin.Post("/jasa/:id<int>/sorot", w.AdminSorotJasa)

	admin.Get("/antrean", w.AdminAntrean)
	admin.Post("/antrean/:id<int>/setujui", w.AdminSetujuiListing)
	admin.Post("/antrean/:id<int>/tolak", w.AdminTolakListing)

	admin.Get("/pengaturan", w.AdminPengaturan)
	admin.Post("/pengaturan/umum", w.AdminPengaturanUmum)
	admin.Post("/pengaturan/logo", batasUnggah, w.AdminPengaturanLogo)
	admin.Post("/pengaturan/logo/hapus", w.AdminPengaturanLogoHapus)
	admin.Post("/pengaturan/lokasi", w.AdminPengaturanLokasi)
	admin.Post("/pengaturan/tampilan", w.AdminPengaturanTampilan)
	admin.Post("/pengaturan/hero", batasUnggah, w.AdminPengaturanHero)
	admin.Post("/pengaturan/hero/hapus", w.AdminPengaturanHeroHapus)
	admin.Post("/pengaturan/iklan/:kunci", batasUnggah, w.AdminSlotIklan)
	admin.Post("/pengaturan/iklan/:kunci/gambar/hapus", w.AdminIklanGambarHapus)

	admin.Get("/kategori", w.AdminKategori)
	admin.Post("/kategori", w.AdminKategoriBuat)
	admin.Post("/kategori/:id<int>", w.AdminKategoriUbah)
	admin.Post("/kategori/:id<int>/hapus", w.AdminKategoriHapus)
	admin.Get("/promosi", w.AdminPromosi)
	admin.Get("/promosi/:id<int>", w.AdminPromosiDetail)
	admin.Post("/promosi/:id<int>/setujui", w.AdminPromosiSetujui)
	admin.Post("/promosi/:id<int>/tolak", w.AdminPromosiTolak)
	admin.Post("/promosi/:id<int>/konfirmasi", w.AdminPromosiKonfirmasi)
	admin.Post("/promosi/:id<int>/jeda", w.AdminPromosiJeda)
	admin.Post("/promosi/:id<int>/lanjutkan", w.AdminPromosiLanjutkan)
	admin.Post("/promosi/:id<int>/hentikan", w.AdminPromosiHentikan)
	admin.Post("/pengaturan/promosi", w.AdminPengaturanPromosi)
	admin.Get("/paket", w.AdminPaket)
	admin.Post("/paket", w.AdminPaketBuat)
	admin.Post("/paket/:id<int>", w.AdminPaketUbah)
	admin.Post("/paket/:id<int>/hapus", w.AdminPaketHapus)
}

func registerWeb(app *fiber.App, w *web.Handler, auth *middleware.Auth, batasUnggah fiber.Handler) {
	// Halaman publik.
	app.Get("/", w.Beranda)
	app.Get("/cari", w.Cari)
	app.Get("/cari/hasil", w.HasilCari)
	app.Get("/jasa/:id<int>", w.DetailJasa)
	// Direktori penyedia, untuk pengunjung yang mencari orangnya lebih dulu
	// ketimbang jasanya.
	app.Get("/penyedia", w.DaftarPenyedia)
	// Halaman publik penyedia memakai slug agar tautannya terbaca saat
	// dibagikan dan lebih baik untuk mesin pencari.
	app.Get("/penyedia/:slug", w.Penyedia)
	app.Get("/tentang", w.Tentang)
	app.Get("/bantuan", w.Bantuan)
	app.Get("/manifest.webmanifest", w.Manifest)
	app.Get("/iklan/:id<int>/klik", w.KlikIklan)

	// Lokasi acuan pencari jasa.
	app.Post("/lokasi", w.SetLokasi)
	app.Post("/lokasi/hapus", w.HapusLokasi)

	// Autentikasi.
	app.Get("/masuk", w.ShowMasuk)
	app.Post("/masuk", w.Masuk)
	app.Get("/daftar", w.ShowDaftar)
	app.Post("/daftar", w.Daftar)
	app.Post("/keluar", w.Keluar)

	// Butuh login.
	app.Get("/dasbor", auth.RequireWeb, w.Dasbor)
	app.Get("/notifikasi", auth.RequireWeb, w.Notifikasi)

	// Percakapan dan pesanan.
	app.Get("/pesan", auth.RequireWeb, w.Pesan)
	app.Get("/pesan/badge", auth.RequireWeb, w.BadgePesan)
	app.Get("/pesan/:id<int>", auth.RequireWeb, w.RuangPesan)
	app.Get("/pesan/:id<int>/baru", auth.RequireWeb, w.PesanBaru)
	app.Post("/pesan/:id<int>", auth.RequireWeb, w.KirimPesan)
	app.Post("/jasa/:id<int>/pesan", auth.RequireWeb, w.MulaiPesan)
	app.Post("/jasa/:id<int>/order", auth.RequireWeb, w.BuatOrder)
	app.Post("/pesanan/:id<int>/status", auth.RequireWeb, w.UbahStatusOrder)
	app.Post("/pesanan/:id<int>/ulasan", auth.RequireWeb, w.TulisUlasan)
	app.Get("/akun/profil", auth.RequireWeb, w.ShowAkunProfil)
	app.Post("/akun/profil", auth.RequireWeb, w.AkunProfil)
	app.Get("/provider/daftar", auth.RequireWeb, w.ShowProviderDaftar)
	app.Post("/provider/daftar", auth.RequireWeb, w.ProviderDaftar)

	// Butuh profil penyedia jasa.
	// Middleware dipasang per rute, bukan lewat app.Group("", …): grup dengan
	// prefiks kosong akan memasang guard ini pada SEMUA path yang tidak cocok
	// rute mana pun, sehingga halaman 404 bagi pengunjung anonim berubah
	// menjadi pengalihan ke halaman masuk.
	butuhProvider := auth.RequireProviderWeb
	app.Get("/provider/profil", butuhProvider, w.ShowProviderProfil)
	app.Post("/provider/profil", butuhProvider, w.ProviderProfil)
	app.Get("/provider/lokasi", butuhProvider, w.ShowProviderLokasi)
	app.Post("/provider/lokasi", butuhProvider, w.ProviderLokasi)
	app.Get("/provider/portofolio", butuhProvider, w.ShowPortofolio)
	app.Post("/provider/portofolio", butuhProvider, batasUnggah, w.TambahPortofolio)
	app.Post("/provider/portofolio/:id<int>/hapus", butuhProvider, w.HapusPortofolio)
	// Promosi: iklan untuk semua pengguna; sorotan & penyedia pilihan diperiksa
	// di handler karena butuh pesan pengalihan yang berbeda, bukan sekadar 403.
	app.Get("/promosi", auth.RequireWeb, w.Promosi)
	app.Get("/promosi/baru", auth.RequireWeb, w.PromosiBaru)
	app.Post("/promosi", auth.RequireWeb, batasUnggah, w.PromosiAjukan)
	app.Get("/promosi/:id<int>", auth.RequireWeb, w.PromosiDetail)
	app.Post("/promosi/:id<int>/bukti", auth.RequireWeb, batasUnggah, w.PromosiBukti)
	app.Post("/promosi/:id<int>/batal", auth.RequireWeb, w.PromosiBatal)
	app.Get("/jasa/baru", butuhProvider, w.ShowJasaBaru)
	app.Post("/jasa/baru", butuhProvider, w.JasaBaru)
	// Admin ikut diizinkan menyunting listing milik siapa pun; pemeriksaan
	// kepemilikan dilakukan di handler.
	app.Get("/jasa/:id<int>/ubah", auth.RequireWeb, w.ShowJasaUbah)
	app.Post("/jasa/:id<int>/ubah", auth.RequireWeb, w.JasaUbah)
	// Pengelolaan foto dan penghapusan listing mengikuti aturan yang sama
	// dengan form ubah: admin boleh atas listing siapa pun, provider hanya
	// atas miliknya. Pemeriksaannya di handler, bukan di middleware, karena
	// admin bukan provider dan akan tertolak butuhProvider.
	app.Post("/jasa/:id<int>/hapus", auth.RequireWeb, w.JasaHapus)
	app.Post("/jasa/:id<int>/foto", auth.RequireWeb, batasUnggah, w.FotoUnggah)
	app.Delete("/jasa/:id<int>/foto/:imageID<int>", auth.RequireWeb, w.FotoHapus)
}
