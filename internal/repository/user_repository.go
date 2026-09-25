package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/hermansyah/adojobsid/internal/model"
)

type UserRepository struct{ db DBTX }

const userColumns = `id, full_name, phone, email, password_hash, avatar_url, city, kecamatan,
	is_provider, role, suspended_at, suspended_reason, created_at`

func scanUser(row pgx.Row) (*model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.FullName, &u.Phone, &u.Email, &u.PasswordHash,
		&u.AvatarURL, &u.City, &u.Kecamatan, &u.IsProvider, &u.Role,
		&u.SuspendedAt, &u.SuspendedReason, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) Create(ctx context.Context, u *model.User) error {
	if u.Role == "" {
		u.Role = model.RoleUser
	}
	const q = `
		INSERT INTO users (full_name, phone, email, password_hash, avatar_url, city, kecamatan, is_provider, role)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at`
	err := r.db.QueryRow(ctx, q, u.FullName, u.Phone, u.Email, u.PasswordHash,
		u.AvatarURL, u.City, u.Kecamatan, u.IsProvider, u.Role).Scan(&u.ID, &u.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return err
	}
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*model.User, error) {
	return scanUser(r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
}

func (r *UserRepository) GetByPhone(ctx context.Context, phone string) (*model.User, error) {
	return scanUser(r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE phone = $1`, phone))
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	return scanUser(r.db.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE LOWER(email) = LOWER($1)`, strings.TrimSpace(email)))
}

// PhoneExists dipakai untuk validasi form registrasi lewat HTMX sebelum submit.
func (r *UserRepository) PhoneExists(ctx context.Context, phone string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE phone = $1)`, phone).Scan(&exists)
	return exists, err
}

func (r *UserRepository) Update(ctx context.Context, u *model.User) error {
	const q = `
		UPDATE users
		   SET full_name = $2, email = $3, avatar_url = $4, city = $5, kecamatan = $6
		 WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, u.ID, u.FullName, u.Email, u.AvatarURL, u.City, u.Kecamatan)
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

// UpdateByAdmin memperbarui data identitas termasuk nomor HP — hanya admin
// yang boleh mengubah nomor, karena nomor adalah kunci masuk akun.
func (r *UserRepository) UpdateByAdmin(ctx context.Context, u *model.User) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE users
		   SET full_name = $2, phone = $3, email = $4, city = $5, kecamatan = $6
		 WHERE id = $1`, u.ID, u.FullName, u.Phone, u.Email, u.City, u.Kecamatan)
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

func (r *UserRepository) UpdatePassword(ctx context.Context, userID int64, hash string) error {
	tag, err := r.db.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, userID, hash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------- operasi admin ----------

// Suspend menangguhkan akun. Alasan wajib diisi dan ikut tersimpan.
func (r *UserRepository) Suspend(ctx context.Context, userID int64, reason string) error {
	const q = `
		UPDATE users
		   SET suspended_at = NOW(), suspended_reason = $2
		 WHERE id = $1 AND suspended_at IS NULL`
	tag, err := r.db.Exec(ctx, q, userID, strings.TrimSpace(reason))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Reactivate mencabut penangguhan akun.
func (r *UserRepository) Reactivate(ctx context.Context, userID int64) error {
	const q = `
		UPDATE users
		   SET suspended_at = NULL, suspended_reason = NULL
		 WHERE id = $1 AND suspended_at IS NOT NULL`
	tag, err := r.db.Exec(ctx, q, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetRole mengubah peran akun.
func (r *UserRepository) SetRole(ctx context.Context, userID int64, role model.Role) error {
	tag, err := r.db.Exec(ctx, `UPDATE users SET role = $2 WHERE id = $1`, userID, role)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CountAdmins dipakai untuk mencegah admin terakhir menurunkan perannya sendiri
// atau menangguhkan dirinya sehingga tidak ada lagi yang bisa masuk panel.
func (r *UserRepository) CountAdmins(ctx context.Context, excludeID int64) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		  FROM users
		 WHERE role = 'admin' AND suspended_at IS NULL AND id <> $1`, excludeID).Scan(&n)
	return n, err
}
