// Command server adalah satu-satunya binary aplikasi.
// Selain menjalankan server HTTP, binary ini juga menjalankan migrasi dan
// seeding lewat subcommand — sehingga image Docker tidak perlu tool tambahan
// dan perintah Makefile berjalan identik di lokal maupun di server.
//
//	server            menjalankan server HTTP
//	server migrate up menjalankan migrasi ke versi terbaru
//	server migrate down membatalkan satu migrasi
//	server migrate version menampilkan versi migrasi aktif
//	server seed       mengisi data awal untuk pengembangan
//	server seed:dasar mengisi kategori & paket promosi saja (produksi)
//	server seed:demo  mengisi konten peragaan lengkap (kata sandi dari DEMO_PASSWORD atau acak, dicetak di log)
//	server admin:create  membuat atau menaikkan satu akun menjadi admin
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/hermansyah/adojobsid/internal/app"
	"github.com/hermansyah/adojobsid/internal/config"
	"github.com/hermansyah/adojobsid/internal/database"
	"github.com/hermansyah/adojobsid/internal/repository"
	"github.com/hermansyah/adojobsid/internal/seed"
	"github.com/hermansyah/adojobsid/internal/service"
	"github.com/hermansyah/adojobsid/internal/session"
)

func main() {
	// .env dibaca bila ada. Di dalam Docker variabel biasanya sudah
	// disuntikkan compose, sehingga berkas ini tidak wajib.
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		fatal(err)
	}
	setupLogger(cfg)

	args := os.Args[1:]
	if len(args) == 0 {
		if err := runServer(cfg); err != nil {
			fatal(err)
		}
		return
	}

	switch args[0] {
	case "migrate":
		if err := runMigrate(cfg, args[1:]); err != nil {
			fatal(err)
		}
	case "seed":
		if err := runSeed(cfg, seed.Run); err != nil {
			fatal(err)
		}
	case "seed:dasar":
		// Produksi: kategori & paket promosi saja, tanpa data contoh.
		if err := runSeed(cfg, seed.Dasar); err != nil {
			fatal(err)
		}
	case "seed:demo":
		// Konten peragaan lengkap (akun, jasa berfoto, pesanan, ulasan, promosi).
		upload := service.NewUploadService(cfg.Upload)
		if err := runSeed(cfg, func(ctx context.Context, repos *repository.Repositories) error {
			return seed.Demo(ctx, repos, upload)
		}); err != nil {
			fatal(err)
		}
	case "admin:create":
		if err := runCreateAdmin(cfg); err != nil {
			fatal(err)
		}
	case "serve":
		if err := runServer(cfg); err != nil {
			fatal(err)
		}
	default:
		fatal(fmt.Errorf("perintah tidak dikenal: %q (pilihan: serve, migrate, seed, seed:dasar, seed:demo, admin:create)", args[0]))
	}
}

// runServer merangkai seluruh dependensi lalu menjalankan HTTP server.
func runServer(cfg *config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := app.New(ctx, cfg, versiBuild())
	if err != nil {
		return err
	}
	defer a.Close()

	// Matikan server dengan rapi saat menerima sinyal dari Docker.
	go func() {
		<-ctx.Done()
		slog.Info("menghentikan server…")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := a.Fiber.ShutdownWithContext(shutdownCtx); err != nil {
			slog.Error("gagal menghentikan server", "error", err)
		}
	}()

	addr := ":" + cfg.App.Port
	slog.Info("server berjalan", "alamat", addr, "env", cfg.App.Env)
	if err := a.Fiber.Listen(addr); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	slog.Info("server berhenti")
	return nil
}

func runMigrate(cfg *config.Config, args []string) error {
	arah := "up"
	if len(args) > 0 {
		arah = args[0]
	}
	switch arah {
	case "up":
		return database.MigrateUp(cfg.Postgres)
	case "down":
		return database.MigrateDown(cfg.Postgres)
	case "version":
		return database.MigrateVersion(cfg.Postgres)
	default:
		return fmt.Errorf("arah migrasi tidak dikenal: %q (pilihan: up, down, version)", arah)
	}
}

func runSeed(cfg *config.Config, jalankan func(context.Context, *repository.Repositories) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := database.NewPostgres(ctx, cfg.Postgres)
	if err != nil {
		return err
	}
	defer pool.Close()

	return jalankan(ctx, repository.New(pool))
}

// runCreateAdmin membuat admin pertama dari variabel environment.
// Sengaja lewat CLI, bukan antarmuka web: admin pertama harus bisa dibuat
// saat belum ada admin mana pun yang dapat masuk ke panel.
func runCreateAdmin(cfg *config.Config) error {
	phone := os.Getenv("ADMIN_PHONE")
	password := os.Getenv("ADMIN_PASSWORD")
	nama := os.Getenv("ADMIN_NAME")

	if strings.TrimSpace(phone) == "" || strings.TrimSpace(password) == "" {
		return errors.New("isi ADMIN_PHONE dan ADMIN_PASSWORD di .env terlebih dahulu")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	pool, err := database.NewPostgres(ctx, cfg.Postgres)
	if err != nil {
		return err
	}
	defer pool.Close()

	rdb, err := database.NewRedis(ctx, cfg.Redis)
	if err != nil {
		return err
	}
	defer rdb.Close()

	repos := repository.New(pool)
	cache := database.NewCache(rdb, cfg.Redis.CacheTTL)
	sessions := session.NewStore(rdb, cfg.Session.TTL)
	services := service.New(repos, cache, cfg, sessions)

	user, dibuat, err := services.Admin.CreateAdmin(ctx, nama, phone, password)
	if err != nil {
		return err
	}
	if dibuat {
		slog.Info("akun admin dibuat", "nama", user.FullName, "phone", user.Phone)
	} else {
		slog.Info("akun yang sudah ada dinaikkan menjadi admin",
			"nama", user.FullName, "phone", user.Phone)
	}
	slog.Info("masuk lewat /masuk memakai nomor tersebut, lalu buka /admin")
	return nil
}

// versiBuild dipakai sebagai versi cadangan untuk aset yang tidak sempat
// dihitung hash-nya. Nilainya diambil dari versi binary bila tersedia,
// sehingga tetap berubah setiap kali aplikasi dibangun ulang.
func versiBuild() string {
	if version != "" && version != "dev" {
		return version
	}
	sum := sha256.Sum256([]byte(fmt.Sprint(time.Now().Unix())))
	return hex.EncodeToString(sum[:])[:12]
}

// version diisi saat kompilasi lewat -ldflags "-X main.version=…".
var version = "dev"

func setupLogger(cfg *config.Config) {
	level := slog.LevelInfo
	if cfg.App.Debug {
		level = slog.LevelDebug
	}
	var h slog.Handler
	if cfg.IsProd() {
		h = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		h = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}
	slog.SetDefault(slog.New(h))
}

func fatal(err error) {
	slog.Error("aplikasi berhenti", "error", err)
	os.Exit(1)
}
