package repository

import "context"

type DeviceRepository struct{ db DBTX }

// Upsert mendaftarkan token untuk pengguna; token yang sudah ada dipindahkan
// ke pengguna ini.
func (r *DeviceRepository) Upsert(ctx context.Context, userID int64, token, platform string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO device_tokens (user_id, token, platform)
		VALUES ($1, $2, $3)
		ON CONFLICT (token) DO UPDATE SET user_id = EXCLUDED.user_id,
		    platform = EXCLUDED.platform, last_seen_at = NOW()`, userID, token, platform)
	return err
}

func (r *DeviceRepository) Delete(ctx context.Context, userID int64, token string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM device_tokens WHERE user_id = $1 AND token = $2`, userID, token)
	return err
}

// DeleteToken membuang token yang ditolak FCM (perangkat sudah mencabutnya).
func (r *DeviceRepository) DeleteToken(ctx context.Context, token string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM device_tokens WHERE token = $1`, token)
	return err
}

func (r *DeviceRepository) ListByUser(ctx context.Context, userID int64) ([]string, error) {
	rows, err := r.db.Query(ctx, `SELECT token FROM device_tokens WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
