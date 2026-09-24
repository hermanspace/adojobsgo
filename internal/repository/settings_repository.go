package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

// SettingsRepository menyimpan pengaturan aplikasi sebagai pasangan kunci-JSONB.
type SettingsRepository struct{ db DBTX }

// Get membaca satu kelompok pengaturan ke dalam dest.
// Mengembalikan ErrNotFound bila kuncinya belum pernah disimpan, sehingga
// pemanggil bisa memakai nilai bawaannya.
func (r *SettingsRepository) Get(ctx context.Context, key string, dest any) error {
	var raw []byte
	err := r.db.QueryRow(ctx, `SELECT value FROM app_settings WHERE key = $1`, key).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return json.Unmarshal(raw, dest)
}

// Set menyimpan satu kelompok pengaturan, menimpa nilai sebelumnya.
func (r *SettingsRepository) Set(ctx context.Context, key string, value any, updatedBy int64) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	const q = `
		INSERT INTO app_settings (key, value, updated_by, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (key) DO UPDATE
		   SET value = EXCLUDED.value,
		       updated_by = EXCLUDED.updated_by,
		       updated_at = NOW()`
	_, err = r.db.Exec(ctx, q, key, payload, updatedBy)
	return err
}
