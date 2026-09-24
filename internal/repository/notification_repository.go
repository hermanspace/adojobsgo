package repository

import (
	"context"
	"encoding/json"

	"github.com/hermansyah/adojobsid/internal/model"
)

type NotificationRepository struct {
	db DBTX
	// SetelahBuat, bila disetel, dipanggil setiap notifikasi tersimpan —
	// jalan masuk push ke ponsel tanpa mengubah satu pun pemanggil Create.
	SetelahBuat func(userID int64, jenis string, payload []byte)
}

// Jenis notifikasi yang dikenali aplikasi.
const (
	NotifPesanBaru        = "pesan_baru"
	NotifOrderBaru        = "order_baru"
	NotifOrderDiperbarui  = "order_diperbarui"
	NotifListingDisetujui = "listing_disetujui"
	NotifListingDitolak   = "listing_ditolak"
	NotifUlasanBaru       = "ulasan_baru"
	// Satu jenis untuk seluruh perubahan status promosi; statusnya di payload.
	NotifPromosiDiperbarui = "promosi_diperbarui"
)

// Create menyimpan satu notifikasi untuk seorang pengguna.
func (r *NotificationRepository) Create(ctx context.Context, userID int64, jenis string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx,
		`INSERT INTO notifications (user_id, type, payload) VALUES ($1, $2, $3)`,
		userID, jenis, raw)
	if err == nil && r.SetelahBuat != nil {
		r.SetelahBuat(userID, jenis, raw)
	}
	return err
}

// List mengembalikan notifikasi terbaru milik satu pengguna.
func (r *NotificationRepository) List(ctx context.Context, userID int64, limit int) ([]model.Notification, error) {
	const q = `
		SELECT id, user_id, type, payload, read_at, created_at
		  FROM notifications
		 WHERE user_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2`

	rows, err := r.db.Query(ctx, q, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.Notification, 0, limit)
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Payload, &n.ReadAt, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *NotificationRepository) CountUnread(ctx context.Context, userID int64) (int, error) {
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL`, userID).Scan(&n)
	return n, err
}

// MarkAllRead menandai seluruh notifikasi pengguna sebagai sudah dibaca.
func (r *NotificationRepository) MarkAllRead(ctx context.Context, userID int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE notifications SET read_at = NOW() WHERE user_id = $1 AND read_at IS NULL`, userID)
	return err
}
