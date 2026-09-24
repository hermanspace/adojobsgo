// Package config memuat seluruh konfigurasi aplikasi dari environment variable.
// Tidak ada nilai kredensial yang di-hardcode; lihat .env.example untuk daftar lengkap.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	App      App
	Postgres Postgres
	Redis    Redis
	Session  Session
	Upload   Upload
	Push     Push
}

type App struct {
	Env            string // local | prod
	Name           string
	Port           string
	BaseURL        string
	Debug          bool
	TrustedProxies []string
	// ProxyHeader adalah header yang membawa IP klien asli dari proxy
	// tepercaya. Di belakang Cloudflare pakai CF-Connecting-IP: Cloudflare
	// hanya MENAMBAHKAN IP klien ke X-Forwarded-For, sehingga entri pertama
	// header itu bisa dipalsukan klien; CF-Connecting-IP ditulis ulang
	// Cloudflare sendiri. Hanya dibaca bila pengirimnya ada di TrustedProxies.
	ProxyHeader string
}

type Postgres struct {
	Host     string
	Port     string
	User     string
	Password string
	DB       string
	SSLMode  string
	MaxConns int32
}

type Redis struct {
	Host     string
	Port     string
	Password string
	DB       int
	CacheTTL time.Duration
}

type Session struct {
	CookieName string
	Secret     string
	TTL        time.Duration
	Secure     bool
}

type Upload struct {
	Dir           string
	MaxSizeMB     int64
	MaxPerListing int
}

// DSN mengembalikan connection string Postgres.
// Push adalah konfigurasi FCM; kosong berarti push dimatikan.
type Push struct {
	ProjectID          string
	ServiceAccountFile string
}

func (p Push) Aktif() bool { return p.ProjectID != "" && p.ServiceAccountFile != "" }

func (p Postgres) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		p.User, p.Password, p.Host, p.Port, p.DB, p.SSLMode)
}

func (r Redis) Addr() string { return r.Host + ":" + r.Port }

// Load membaca konfigurasi dari environment. Nilai wajib yang kosong menghasilkan error
// supaya aplikasi gagal cepat saat start, bukan saat request pertama masuk.
func Load() (*Config, error) {
	cfg := &Config{
		App: App{
			Env:            env("APP_ENV", "local"),
			Name:           env("APP_NAME", "AdoJobs"),
			Port:           env("APP_PORT", "3000"),
			BaseURL:        env("APP_BASE_URL", "http://localhost:3000"),
			Debug:          envBool("APP_DEBUG", true),
			TrustedProxies: envList("APP_TRUSTED_PROXIES", nil),
			ProxyHeader:    env("APP_PROXY_HEADER", "X-Forwarded-For"),
		},
		Postgres: Postgres{
			Host:     env("POSTGRES_HOST", "postgres"),
			Port:     env("POSTGRES_PORT", "5432"),
			User:     env("POSTGRES_USER", ""),
			Password: env("POSTGRES_PASSWORD", ""),
			DB:       env("POSTGRES_DB", ""),
			SSLMode:  env("POSTGRES_SSLMODE", "disable"),
			MaxConns: int32(envInt("POSTGRES_MAX_CONNS", 10)),
		},
		Redis: Redis{
			Host:     env("REDIS_HOST", "redis"),
			Port:     env("REDIS_PORT", "6379"),
			Password: env("REDIS_PASSWORD", ""),
			DB:       envInt("REDIS_DB", 0),
			CacheTTL: time.Duration(envInt("REDIS_CACHE_TTL_SECONDS", 300)) * time.Second,
		},
		Session: Session{
			CookieName: env("SESSION_COOKIE_NAME", "adojobs_session"),
			Secret:     env("SESSION_SECRET", ""),
			TTL:        time.Duration(envInt("SESSION_TTL_HOURS", 720)) * time.Hour,
			Secure:     envBool("SESSION_COOKIE_SECURE", false),
		},
		Push: Push{
			ProjectID:          env("FCM_PROJECT_ID", ""),
			ServiceAccountFile: env("FCM_SERVICE_ACCOUNT_FILE", ""),
		},
		Upload: Upload{
			Dir:           env("UPLOAD_DIR", "/app/storage/uploads"),
			MaxSizeMB:     int64(envInt("UPLOAD_MAX_SIZE_MB", 5)),
			MaxPerListing: envInt("UPLOAD_MAX_PER_LISTING", 6),
		},
	}

	var missing []string
	for k, v := range map[string]string{
		"POSTGRES_USER":     cfg.Postgres.User,
		"POSTGRES_PASSWORD": cfg.Postgres.Password,
		"POSTGRES_DB":       cfg.Postgres.DB,
		"SESSION_SECRET":    cfg.Session.Secret,
	} {
		if strings.TrimSpace(v) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("environment variable wajib belum diisi: %s", strings.Join(missing, ", "))
	}
	if len(cfg.Session.Secret) < 32 {
		return nil, fmt.Errorf("SESSION_SECRET minimal 32 karakter (saat ini %d)", len(cfg.Session.Secret))
	}
	// Salah konfigurasi produksi lebih baik menghentikan aplikasi di detik
	// pertama daripada diam-diam mengirim cookie sesi lewat HTTP.
	if cfg.IsProd() {
		if !cfg.Session.Secure {
			return nil, errors.New("APP_ENV=prod tetapi SESSION_COOKIE_SECURE=false: cookie sesi akan bocor lewat HTTP")
		}
		if cfg.Session.Secret == RahasiaContoh {
			return nil, errors.New("SESSION_SECRET masih memakai nilai contoh dari .env.example")
		}
	}
	return cfg, nil
}

// RahasiaContoh adalah nilai SESSION_SECRET di .env.example; produksi tidak
// boleh memakainya.
const RahasiaContoh = "ganti_dengan_string_acak_minimal_32_karakter"

func (c *Config) IsProd() bool { return c.App.Env == "prod" || c.App.Env == "production" }

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func envList(key string, fallback []string) []string {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return fallback
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
