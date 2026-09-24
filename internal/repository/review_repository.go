package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/hermansyah/adojobsid/internal/model"
)

type ReviewRepository struct{ db DBTX }

// Create menyimpan ulasan. Indeks unik pada order_id yang menjamin satu
// pesanan hanya menghasilkan satu ulasan, sehingga dua permintaan bersamaan
// tetap tidak bisa menggandakannya.
func (r *ReviewRepository) Create(ctx context.Context, rv *model.Review) error {
	const q = `
		INSERT INTO reviews (order_id, rating, comment)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`
	err := r.db.QueryRow(ctx, q, rv.OrderID, rv.Rating, rv.Comment).
		Scan(&rv.ID, &rv.CreatedAt)
	if err != nil && isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

func (r *ReviewRepository) GetByOrder(ctx context.Context, orderID int64) (*model.Review, error) {
	const q = `SELECT id, order_id, rating, comment, created_at FROM reviews WHERE order_id = $1`

	var rv model.Review
	err := r.db.QueryRow(ctx, q, orderID).
		Scan(&rv.ID, &rv.OrderID, &rv.Rating, &rv.Comment, &rv.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &rv, nil
}

// ListByProvider mengembalikan ulasan untuk satu penyedia, terbaru lebih dulu,
// lengkap dengan nama penulis dan jasa yang diulas.
func (r *ReviewRepository) ListByProvider(ctx context.Context, providerID int64, limit int) ([]model.ReviewRow, error) {
	q := `
		SELECT rv.id, rv.order_id, rv.rating, rv.comment, rv.created_at,
		       u.full_name, u.avatar_url, s.id, s.title
		  FROM reviews rv
		  JOIN orders o   ON o.id = rv.order_id
		  JOIN users u    ON u.id = o.seeker_id
		  JOIN services s ON s.id = o.service_id
		 WHERE o.provider_id = $1
		 ORDER BY rv.created_at DESC`
	args := []any{providerID}
	if limit > 0 {
		q += ` LIMIT $2`
		args = append(args, limit)
	}

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.ReviewRow, 0, 12)
	for rows.Next() {
		var rv model.ReviewRow
		if err := rows.Scan(&rv.ID, &rv.OrderID, &rv.Rating, &rv.Comment, &rv.CreatedAt,
			&rv.PenulisNama, &rv.PenulisAvatar, &rv.ServiceID, &rv.ServiceTitle); err != nil {
			return nil, err
		}
		out = append(out, rv)
	}
	return out, rows.Err()
}

// SebaranBintang menghitung jumlah ulasan per nilai bintang, dipakai
// ringkasan rating di halaman penyedia.
func (r *ReviewRepository) SebaranBintang(ctx context.Context, providerID int64) (map[int]int, error) {
	const q = `
		SELECT rv.rating, COUNT(*)
		  FROM reviews rv
		  JOIN orders o ON o.id = rv.order_id
		 WHERE o.provider_id = $1
		 GROUP BY rv.rating`

	rows, err := r.db.Query(ctx, q, providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int]int{}
	for rows.Next() {
		var bintang, jumlah int
		if err := rows.Scan(&bintang, &jumlah); err != nil {
			return nil, err
		}
		out[bintang] = jumlah
	}
	return out, rows.Err()
}
