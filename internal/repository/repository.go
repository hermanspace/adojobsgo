// Package repository membungkus seluruh akses SQL. Tidak ada query SQL di luar package ini.
package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("data tidak ditemukan")
	ErrConflict = errors.New("data sudah ada")
)

// DBTX menyamakan pgxpool.Pool dan pgx.Tx sehingga query yang sama
// bisa dijalankan di dalam maupun di luar transaksi.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Repositories struct {
	pool *pgxpool.Pool

	User      *UserRepository
	Provider  *ProviderRepository
	Category  *CategoryRepository
	Service   *ServiceRepository
	Admin     *AdminRepository
	Settings  *SettingsRepository
	Order     *OrderRepository
	Chat      *ChatRepository
	Notif     *NotificationRepository
	Portfolio *PortfolioRepository
	Review    *ReviewRepository
	Paket     *PaketRepository
	Promosi   *PromosiRepository
	Device    *DeviceRepository
}

func New(pool *pgxpool.Pool) *Repositories {
	return &Repositories{
		pool:      pool,
		User:      &UserRepository{db: pool},
		Provider:  &ProviderRepository{db: pool},
		Category:  &CategoryRepository{db: pool},
		Service:   &ServiceRepository{db: pool},
		Admin:     &AdminRepository{db: pool},
		Settings:  &SettingsRepository{db: pool},
		Order:     &OrderRepository{db: pool},
		Chat:      &ChatRepository{db: pool},
		Notif:     &NotificationRepository{db: pool},
		Portfolio: &PortfolioRepository{db: pool},
		Review:    &ReviewRepository{db: pool},
		Paket:     &PaketRepository{db: pool},
		Promosi:   &PromosiRepository{db: pool},
		Device:    &DeviceRepository{db: pool},
	}
}

// WithTx menjalankan fn dalam satu transaksi. Repositories yang diberikan ke fn
// memakai transaksi tersebut, sehingga operasi lintas tabel tetap atomik.
func (r *Repositories) WithTx(ctx context.Context, fn func(*Repositories) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	scoped := &Repositories{
		pool:      r.pool,
		User:      &UserRepository{db: tx},
		Provider:  &ProviderRepository{db: tx},
		Category:  &CategoryRepository{db: tx},
		Service:   &ServiceRepository{db: tx},
		Admin:     &AdminRepository{db: tx},
		Settings:  &SettingsRepository{db: tx},
		Order:     &OrderRepository{db: tx},
		Chat:      &ChatRepository{db: tx},
		Notif:     &NotificationRepository{db: tx},
		Portfolio: &PortfolioRepository{db: tx},
		Review:    &ReviewRepository{db: tx},
	}
	if err := fn(scoped); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// isUniqueViolation mendeteksi pelanggaran unique constraint Postgres (kode 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func constraintName(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.ConstraintName
	}
	return ""
}
