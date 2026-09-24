package database

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hermansyah/adojobsid/internal/config"
)

// NewPostgres membuka connection pool dan memastikan database benar-benar siap
// sebelum aplikasi menerima request.
func NewPostgres(ctx context.Context, cfg config.Postgres) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse dsn postgres: %w", err)
	}
	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MaxConnLifetime = time.Hour
	poolCfg.MaxConnIdleTime = 30 * time.Minute
	poolCfg.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("buka pool postgres: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	if err := periksaKapasitas(pingCtx, pool, cfg.MaxConns); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// periksaKapasitas membandingkan ukuran pool dengan batas koneksi server.
// Ketidakselarasan di sini tidak terasa saat sepi dan baru meledak saat ramai
// — sebagai "too many clients" di tengah puncak — jadi lebih baik ketahuan
// saat aplikasi dinyalakan.
func periksaKapasitas(ctx context.Context, pool *pgxpool.Pool, maksPool int32) error {
	var maks, cadangan string
	if err := pool.QueryRow(ctx, `SHOW max_connections`).Scan(&maks); err != nil {
		return fmt.Errorf("baca max_connections: %w", err)
	}
	if err := pool.QueryRow(ctx, `SHOW superuser_reserved_connections`).Scan(&cadangan); err != nil {
		return fmt.Errorf("baca superuser_reserved_connections: %w", err)
	}
	m, _ := strconv.Atoi(maks)
	c, _ := strconv.Atoi(cadangan)

	tersedia := int32(m - c)
	switch nilaiKapasitas(maksPool, tersedia) {
	case kapasitasMelebihi:
		return fmt.Errorf("POSTGRES_MAX_CONNS=%d melebihi koneksi yang tersedia di server (%d = max_connections %d − cadangan superuser %d)",
			maksPool, tersedia, m, c)
	case kapasitasSempit:
		slog.Warn("pool postgres memakai lebih dari separuh koneksi server; migrasi, alat admin, dan instance kedua akan berebut sisanya",
			"pool", maksPool, "tersedia", tersedia)
	default:
		slog.Info("pool postgres", "maks", maksPool, "tersedia_di_server", tersedia)
	}
	return nil
}

type kapasitas int

const (
	kapasitasLonggar kapasitas = iota
	kapasitasSempit
	kapasitasMelebihi
)

// nilaiKapasitas dipisah sebagai fungsi murni supaya aturannya bisa diuji
// tanpa server: melebihi = galat; lebih dari separuh = peringatan, karena
// migrasi, alat admin, dan instance kedua semuanya memakai server yang sama.
func nilaiKapasitas(pool, tersedia int32) kapasitas {
	switch {
	case pool > tersedia:
		return kapasitasMelebihi
	case pool > tersedia/2:
		return kapasitasSempit
	default:
		return kapasitasLonggar
	}
}
