package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/hermansyah/adojobsid/internal/model"
)

type CategoryRepository struct{ db DBTX }

const categoryColumns = `id, name, slug, parent_id, icon`

func (r *CategoryRepository) scanMany(ctx context.Context, q string, args ...any) ([]model.Category, error) {
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.Category, 0, 24)
	for rows.Next() {
		var c model.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.ParentID, &c.Icon); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListAll mengembalikan seluruh kategori dalam urutan induk lalu anak.
func (r *CategoryRepository) ListAll(ctx context.Context) ([]model.Category, error) {
	return r.scanMany(ctx, `
		SELECT `+categoryColumns+`
		  FROM categories
		 ORDER BY parent_id NULLS FIRST, name`)
}

// ListRoots mengembalikan kategori tingkat atas saja, untuk chip filter di halaman pencarian.
func (r *CategoryRepository) ListRoots(ctx context.Context) ([]model.Category, error) {
	return r.scanMany(ctx, `
		SELECT `+categoryColumns+`
		  FROM categories
		 WHERE parent_id IS NULL
		 ORDER BY name`)
}

func (r *CategoryRepository) GetBySlug(ctx context.Context, slug string) (*model.Category, error) {
	var c model.Category
	err := r.db.QueryRow(ctx,
		`SELECT `+categoryColumns+` FROM categories WHERE slug = $1`, slug).
		Scan(&c.ID, &c.Name, &c.Slug, &c.ParentID, &c.Icon)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *CategoryRepository) GetByID(ctx context.Context, id int64) (*model.Category, error) {
	var c model.Category
	err := r.db.QueryRow(ctx,
		`SELECT `+categoryColumns+` FROM categories WHERE id = $1`, id).
		Scan(&c.ID, &c.Name, &c.Slug, &c.ParentID, &c.Icon)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

// DescendantIDs mengembalikan id kategori beserta seluruh turunannya.
// Filter kategori induk di pencarian ikut menyertakan jasa pada sub-kategorinya.
func (r *CategoryRepository) DescendantIDs(ctx context.Context, rootID int64) ([]int64, error) {
	// Klausa CYCLE menghentikan penelusuran bila ada lingkaran referensi, dan
	// batas kedalaman menjaga query tetap terbatas sekalipun data rusak.
	// Tanpa keduanya, satu baris yang mereferensi dirinya sendiri membuat
	// query ini berjalan selamanya dan menggantung seluruh permintaan.
	const q = `
		WITH RECURSIVE tree (id, depth) AS (
		    SELECT id, 0 FROM categories WHERE id = $1
		    UNION ALL
		    SELECT c.id, t.depth + 1
		      FROM categories c
		      JOIN tree t ON c.parent_id = t.id
		     WHERE t.depth < 5
		) CYCLE id SET is_cycle USING path
		SELECT DISTINCT id FROM tree WHERE NOT is_cycle`
	rows, err := r.db.Query(ctx, q, rootID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Upsert dipakai oleh seeder agar `make seed` aman dijalankan berulang kali.
func (r *CategoryRepository) Upsert(ctx context.Context, c *model.Category) error {
	const q = `
		INSERT INTO categories (name, slug, parent_id, icon)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (slug) DO UPDATE
		   SET name = EXCLUDED.name, parent_id = EXCLUDED.parent_id, icon = EXCLUDED.icon
		RETURNING id`
	return r.db.QueryRow(ctx, q, c.Name, c.Slug, c.ParentID, c.Icon).Scan(&c.ID)
}

// ---------- operasi admin ----------

// CategoryRow adalah kategori beserta jumlah listing yang memakainya,
// dipakai panel admin untuk menilai kategori mana yang benar-benar terpakai.
type CategoryRow struct {
	model.Category
	ParentName   *string `json:"parent_name,omitempty"`
	TotalListing int     `json:"total_listing"`
	TotalAnak    int     `json:"total_anak"`
}

// ListWithUsage mengembalikan seluruh kategori terurut induk-lalu-anak,
// lengkap dengan jumlah pemakaiannya.
func (r *CategoryRepository) ListWithUsage(ctx context.Context) ([]CategoryRow, error) {
	const q = `
		SELECT c.id, c.name, c.slug, c.parent_id, c.icon,
		       pc.name,
		       (SELECT COUNT(*) FROM services s WHERE s.category_id = c.id),
		       (SELECT COUNT(*) FROM categories k WHERE k.parent_id = c.id)
		  FROM categories c
		  LEFT JOIN categories pc ON pc.id = c.parent_id
		 ORDER BY COALESCE(pc.name, c.name), c.parent_id NULLS FIRST, c.name`

	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]CategoryRow, 0, 48)
	for rows.Next() {
		var c CategoryRow
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.ParentID, &c.Icon,
			&c.ParentName, &c.TotalListing, &c.TotalAnak); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *CategoryRepository) Create(ctx context.Context, c *model.Category) error {
	const q = `
		INSERT INTO categories (name, slug, parent_id, icon)
		VALUES ($1, $2, $3, $4)
		RETURNING id`
	err := r.db.QueryRow(ctx, q, c.Name, c.Slug, c.ParentID, c.Icon).Scan(&c.ID)
	if err != nil && isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

func (r *CategoryRepository) Update(ctx context.Context, c *model.Category) error {
	const q = `
		UPDATE categories
		   SET name = $2, slug = $3, parent_id = $4, icon = $5
		 WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, c.ID, c.Name, c.Slug, c.ParentID, c.Icon)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete menghapus kategori. Gagal bila masih dipakai listing, karena
// services.category_id memakai ON DELETE RESTRICT.
func (r *CategoryRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM categories WHERE id = $1`, id)
	if err != nil {
		if constraintName(err) != "" {
			return ErrConflict
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CountUsage mengembalikan jumlah listing dan sub-kategori yang bergantung
// pada kategori ini, dipakai untuk menjelaskan mengapa penghapusan ditolak.
func (r *CategoryRepository) CountUsage(ctx context.Context, id int64) (listing int, anak int, err error) {
	err = r.db.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM services s WHERE s.category_id = $1),
		       (SELECT COUNT(*) FROM categories c WHERE c.parent_id = $1)`, id).
		Scan(&listing, &anak)
	return
}
