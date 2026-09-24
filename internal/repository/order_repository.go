package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/hermansyah/adojobsid/internal/model"
)

type OrderRepository struct{ db DBTX }

const orderColumns = `id, seeker_id, provider_id, service_id, status, scheduled_date,
	notes, agreed_price, created_at`

func scanOrder(row pgx.Row) (*model.Order, error) {
	var o model.Order
	err := row.Scan(&o.ID, &o.SeekerID, &o.ProviderID, &o.ServiceID, &o.Status,
		&o.ScheduledDate, &o.Notes, &o.AgreedPrice, &o.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &o, nil
}

func (r *OrderRepository) Create(ctx context.Context, o *model.Order) error {
	const q = `
		INSERT INTO orders (seeker_id, provider_id, service_id, status, scheduled_date, notes, agreed_price)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`
	return r.db.QueryRow(ctx, q, o.SeekerID, o.ProviderID, o.ServiceID, o.Status,
		o.ScheduledDate, o.Notes, o.AgreedPrice).Scan(&o.ID, &o.CreatedAt)
}

func (r *OrderRepository) GetByID(ctx context.Context, id int64) (*model.Order, error) {
	return scanOrder(r.db.QueryRow(ctx, `SELECT `+orderColumns+` FROM orders WHERE id = $1`, id))
}

// SetStatus mengubah status pesanan.
func (r *OrderRepository) SetStatus(ctx context.Context, orderID int64, status model.OrderStatus) error {
	tag, err := r.db.Exec(ctx, `UPDATE orders SET status = $2 WHERE id = $1`, orderID, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetAgreedPrice menyimpan harga yang disepakati saat penyedia menerima pesanan.
func (r *OrderRepository) SetAgreedPrice(ctx context.Context, orderID int64, price *float64) error {
	tag, err := r.db.Exec(ctx, `UPDATE orders SET agreed_price = $2 WHERE id = $1`, orderID, price)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
