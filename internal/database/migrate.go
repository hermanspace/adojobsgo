package database

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq" // driver database/sql yang dibutuhkan golang-migrate

	"database/sql"

	"github.com/hermansyah/adojobsid/internal/config"
	"github.com/hermansyah/adojobsid/migrations"
)

// newMigrator menyiapkan migrator dengan berkas SQL yang tertanam di binary.
func newMigrator(cfg config.Postgres) (*migrate.Migrate, *sql.DB, error) {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return nil, nil, fmt.Errorf("baca berkas migrasi: %w", err)
	}

	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, nil, fmt.Errorf("buka koneksi migrasi: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("ping database untuk migrasi: %w", err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("siapkan driver migrasi: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("buat migrator: %w", err)
	}
	return m, db, nil
}

// MigrateUp menjalankan seluruh migrasi yang belum diterapkan.
func MigrateUp(cfg config.Postgres) error {
	m, db, err := newMigrator(cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			slog.Info("migrasi: sudah pada versi terbaru")
			return nil
		}
		return fmt.Errorf("jalankan migrasi: %w", err)
	}
	version, dirty, _ := m.Version()
	slog.Info("migrasi selesai", "versi", version, "dirty", dirty)
	return nil
}

// MigrateDown membatalkan satu langkah migrasi terakhir.
func MigrateDown(cfg config.Postgres) error {
	m, db, err := newMigrator(cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := m.Steps(-1); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			slog.Info("migrasi: tidak ada yang bisa dibatalkan")
			return nil
		}
		return fmt.Errorf("rollback migrasi: %w", err)
	}
	version, dirty, _ := m.Version()
	slog.Info("rollback selesai", "versi", version, "dirty", dirty)
	return nil
}

// MigrateVersion menampilkan versi migrasi yang sedang aktif.
func MigrateVersion(cfg config.Postgres) error {
	m, db, err := newMigrator(cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	version, dirty, err := m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			slog.Info("belum ada migrasi yang dijalankan")
			return nil
		}
		return err
	}
	slog.Info("versi migrasi", "versi", version, "dirty", dirty)
	return nil
}
