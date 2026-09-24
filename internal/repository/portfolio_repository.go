package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/hermansyah/adojobsid/internal/model"
)

type PortfolioRepository struct{ db DBTX }

const portfolioColumns = `id, provider_id, title, image_url, thumb_url, width, height,
	bytes, completed_at, created_at`

func scanPortfolio(row pgx.Row) (*model.Portfolio, error) {
	var p model.Portfolio
	err := row.Scan(&p.ID, &p.ProviderID, &p.Title, &p.ImageURL, &p.ThumbURL,
		&p.Width, &p.Height, &p.Bytes, &p.CompletedAt, &p.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *PortfolioRepository) Create(ctx context.Context, p *model.Portfolio) error {
	const q = `
		INSERT INTO portfolios (provider_id, title, image_url, thumb_url, width, height, bytes, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at`
	return r.db.QueryRow(ctx, q, p.ProviderID, p.Title, p.ImageURL, p.ThumbURL,
		p.Width, p.Height, p.Bytes, p.CompletedAt).Scan(&p.ID, &p.CreatedAt)
}

// ListByProvider mengembalikan portofolio terurut dari pekerjaan terbaru.
// Karya tanpa tanggal penyelesaian diletakkan setelah yang bertanggal.
func (r *PortfolioRepository) ListByProvider(ctx context.Context, providerID int64, limit int) ([]model.Portfolio, error) {
	q := `
		SELECT ` + portfolioColumns + `
		  FROM portfolios
		 WHERE provider_id = $1
		 ORDER BY completed_at DESC NULLS LAST, created_at DESC`
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

	out := make([]model.Portfolio, 0, 12)
	for rows.Next() {
		var p model.Portfolio
		if err := rows.Scan(&p.ID, &p.ProviderID, &p.Title, &p.ImageURL, &p.ThumbURL,
			&p.Width, &p.Height, &p.Bytes, &p.CompletedAt, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *PortfolioRepository) GetByID(ctx context.Context, id int64) (*model.Portfolio, error) {
	return scanPortfolio(r.db.QueryRow(ctx,
		`SELECT `+portfolioColumns+` FROM portfolios WHERE id = $1`, id))
}

func (r *PortfolioRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM portfolios WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PortfolioRepository) CountByProvider(ctx context.Context, providerID int64) (int, error) {
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM portfolios WHERE provider_id = $1`, providerID).Scan(&n)
	return n, err
}
