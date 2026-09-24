// Package app merakit seluruh aplikasi — koneksi, service, middleware, rute —
// menjadi satu nilai yang bisa dijalankan main maupun dibangun di dalam uji.
// Sebelumnya perakitan ini hidup di dalam runServer, sehingga alur end-to-end
// tidak bisa diuji tanpa menjalankan binary sungguhan.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/hermansyah/adojobsid/internal/config"
	"github.com/hermansyah/adojobsid/internal/database"
	"github.com/hermansyah/adojobsid/internal/handler"
	"github.com/hermansyah/adojobsid/internal/handler/api"
	"github.com/hermansyah/adojobsid/internal/handler/web"
	"github.com/hermansyah/adojobsid/internal/middleware"
	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/repository"
	"github.com/hermansyah/adojobsid/internal/service"
	"github.com/hermansyah/adojobsid/internal/session"
	"github.com/hermansyah/adojobsid/internal/view"
)

// App adalah aplikasi yang sudah dirakit lengkap tetapi belum mendengarkan.
type App struct {
	Fiber    *fiber.App
	Pool     *pgxpool.Pool
	Rdb      *redis.Client
	Repos    *repository.Repositories
	Services *service.Services

	hentikanTicker context.CancelFunc
}

// New membuka koneksi dan merakit aplikasi. versi dipakai sebagai versi
// cadangan aset statis yang tidak sempat dihitung hash-nya.
func New(ctx context.Context, cfg *config.Config, versi string) (*App, error) {
	pool, err := database.NewPostgres(ctx, cfg.Postgres)
	if err != nil {
		return nil, err
	}
	rdb, err := database.NewRedis(ctx, cfg.Redis)
	if err != nil {
		pool.Close()
		return nil, err
	}
	if err := os.MkdirAll(cfg.Upload.Dir, 0o755); err != nil {
		pool.Close()
		_ = rdb.Close()
		return nil, fmt.Errorf("siapkan folder upload: %w", err)
	}

	repos := repository.New(pool)
	cache := database.NewCache(rdb, cfg.Redis.CacheTTL)
	sessions := session.NewStore(rdb, cfg.Session.TTL)
	services := service.New(repos, cache, cfg, sessions)
	flashes := session.NewFlashStore(rdb)
	auth := middleware.NewAuth(sessions, cfg.Session)

	f := fiber.New(fiber.Config{
		AppName:               cfg.App.Name,
		DisableStartupMessage: true,
		// Batas ukuran body mengikuti batas unggah foto, dengan ruang lebih
		// untuk beberapa berkas sekaligus dalam satu permintaan.
		BodyLimit:               int(cfg.Upload.MaxSizeMB) * 1024 * 1024 * (cfg.Upload.MaxPerListing + 1),
		ReadTimeout:             30 * time.Second,
		WriteTimeout:            60 * time.Second,
		IdleTimeout:             90 * time.Second,
		ProxyHeader:             cfg.App.ProxyHeader,
		EnableTrustedProxyCheck: len(cfg.App.TrustedProxies) > 0,
		TrustedProxies:          cfg.App.TrustedProxies,
	})

	f.Use(recover.New())
	f.Use(requestid.New())
	f.Use(helmet.New(helmet.Config{
		// CSP dibuat ketat: seluruh skrip, gaya, dan font dilayani sendiri.
		// Satu-satunya pengecualian adalah gambar tile peta: Leaflet
		// di-self-host, tapi petaknya diambil peramban dari server
		// OpenStreetMap. Pelonggaran dibatasi tepat pada img-src untuk domain
		// itu saja — script-src tetap 'self' tanpa 'unsafe-eval'.
		ContentSecurityPolicy: "default-src 'self'; " +
			"script-src 'self'; " +
			"style-src 'self' 'unsafe-inline'; " +
			"img-src 'self' data: https://*.tile.openstreetmap.org; " +
			"font-src 'self'; " +
			"connect-src 'self'; " +
			"form-action 'self' https://wa.me; " +
			"frame-ancestors 'none'; " +
			"base-uri 'self'",
		ReferrerPolicy: "strict-origin-when-cross-origin",
		// Bawaan helmet adalah require-corp, yang memblokir sumber daya lintas
		// asal tanpa header CORP — termasuk tile peta OpenStreetMap. Aplikasi
		// ini tidak memakai SharedArrayBuffer, jadi COEP tidak dibutuhkan dan
		// dimatikan agar peta bisa dimuat.
		CrossOriginEmbedderPolicy: "unsafe-none",
		// HSTS hanya di produksi — di lokal tanpa HTTPS ia justru mengunci
		// peramban pengembang keluar dari localhost.
		HSTSMaxAge: hstsProd(cfg),
	}))
	f.Use(compress.New(compress.Config{Level: compress.LevelDefault}))

	// Pembatasan laju untuk endpoint autentikasi, guna meredam percobaan
	// tebak kata sandi. Rute unggah punya pembatasnya sendiri di router.
	f.Use("/masuk", limiter.New(limiter.Config{
		Max:        20,
		Expiration: time.Minute,
		Next:       func(c *fiber.Ctx) bool { return c.Method() == fiber.MethodGet },
	}))
	f.Use("/api/v1/auth", limiter.New(limiter.Config{Max: 30, Expiration: time.Minute}))
	// Pendaftaran: satu IP tidak boleh menciptakan akun beruntun. Angkanya
	// longgar untuk kantor/NAT yang berbagi IP, tetapi menutup skrip.
	f.Use("/daftar", limiter.New(limiter.Config{
		Max:        10,
		Expiration: time.Minute,
		Next:       func(c *fiber.Ctx) bool { return c.Method() == fiber.MethodGet },
	}))
	f.Use("/api/v1/auth/register", limiter.New(limiter.Config{Max: 10, Expiration: time.Minute}))

	// Aset statis boleh di-cache lama karena setiap URL membawa hash isinya;
	// begitu berkas berubah, URL-nya ikut berubah dan cache lama terlewati.
	assets := view.NewAssets("web/static", versi)
	f.Static("/static", "web/static", fiber.Static{
		Compress:      true,
		CacheDuration: 10 * time.Minute,
		MaxAge:        int((365 * 24 * time.Hour).Seconds()),
	})
	// Bukti transfer memuat data keuangan pribadi: hanya pemiliknya dan admin
	// yang boleh membukanya. Diperiksa sebelum handler statis, karena handler
	// statis tidak tahu apa-apa soal sesi.
	f.Use("/uploads/bukti", auth.Load, func(c *fiber.Ctx) error {
		user := middleware.CurrentUser(c)
		if user == nil {
			return fiber.ErrNotFound
		}
		if user.IsAdmin {
			return c.Next()
		}
		pemilik, err := repos.Promosi.PemilikBukti(c.Context(), c.Path())
		if err != nil || pemilik != user.ID {
			return fiber.ErrNotFound
		}
		return c.Next()
	})
	// Foto unggahan dilayani dari volume terpisah, tanpa daftar isi folder.
	// CacheDuration dimatikan: berkas di sini bisa dihapus penyedia kapan saja,
	// dan cache handler Fiber akan tetap menyajikan berkas yang sudah dihapus.
	f.Static("/uploads", cfg.Upload.Dir, fiber.Static{
		Compress:      false,
		Browse:        false,
		CacheDuration: -1,
		MaxAge:        int((30 * 24 * time.Hour).Seconds()),
	})

	f.Get("/sehat", func(c *fiber.Ctx) error {
		if err := pool.Ping(c.Context()); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "postgres tidak siap"})
		}
		if err := rdb.Ping(c.Context()).Err(); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "redis tidak siap"})
		}
		return c.JSON(fiber.Map{"status": "sehat"})
	})

	// Setiap notifikasi yang tersimpan diteruskan ke ponsel pengguna. Judul
	// dan isinya disusun oleh view.IsiNotifikasi yang sama dengan halaman web,
	// jadi kedua jalur selalu berbunyi sama. Berjalan di goroutine sendiri —
	// pengiriman HTTP ke FCM tidak boleh memperlambat permintaan pengguna.
	if services.Push.Aktif() {
		repos.Notif.SetelahBuat = func(userID int64, jenis string, payload []byte) {
			go func() {
				n := model.Notification{Type: jenis, Payload: payload}
				judul, isi, tautan := view.IsiNotifikasi(n)
				ctx, batal := context.WithTimeout(context.Background(), 15*time.Second)
				defer batal()
				services.Push.Kirim(ctx, userID, judul, isi, map[string]string{"type": jenis, "tautan": tautan})
			}()
		}
	}

	webHandler := web.New(services, sessions, flashes, auth, cfg, assets)
	apiHandler := api.New(services, sessions, auth, cfg)
	handler.Register(f, webHandler, apiHandler, auth)

	// Satu goroutine latar untuk dua pekerjaan berkala; tanpa container cron.
	tctx, batal := context.WithCancel(context.Background())
	go jalankanBerkala(tctx, services)

	return &App{Fiber: f, Pool: pool, Rdb: rdb, Repos: repos, Services: services, hentikanTicker: batal}, nil
}

// jalankanBerkala menyalin penghitung tayang tiap menit dan menutup promosi
// yang masa tayangnya habis tiap sepuluh menit. Keduanya idempoten, jadi
// jeda yang terlewat saat restart tidak merusak apa pun.
func jalankanBerkala(ctx context.Context, services *service.Services) {
	salin := time.NewTicker(time.Minute)
	tutup := time.NewTicker(10 * time.Minute)
	defer salin.Stop()
	defer tutup.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-salin.C:
			services.Promosi.SalinPenghitungTayang(ctx)
		case <-tutup.C:
			if _, err := services.Promosi.TandaiKedaluwarsa(ctx); err != nil {
				slog.Warn("menutup promosi kedaluwarsa", "error", err)
			}
			services.Promosi.IngatkanAkanSelesai(ctx)
		}
	}
}

// hstsProd mengembalikan umur HSTS (detik) hanya untuk produksi.
func hstsProd(cfg *config.Config) int {
	if cfg.IsProd() {
		return 15552000 // 180 hari
	}
	return 0
}

// Close menutup koneksi. Server HTTP-nya dihentikan pemanggil lewat Fiber.
func (a *App) Close() {
	if a.hentikanTicker != nil {
		a.hentikanTicker()
	}
	a.Pool.Close()
	_ = a.Rdb.Close()
}
