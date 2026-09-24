package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/hermansyah/adojobsid/internal/model"
)

type ChatRepository struct{ db DBTX }

// EnsureConversation mengembalikan percakapan yang sudah ada untuk pasangan
// listing dan pencari jasa, atau membuatnya bila belum ada. Indeks unik pada
// (service_id, seeker_id) yang menjaga agar riwayat tidak terpecah, sehingga
// dua permintaan bersamaan tetap menghasilkan satu percakapan.
func (r *ChatRepository) EnsureConversation(ctx context.Context, serviceID, seekerID, providerID int64) (*model.Conversation, error) {
	const q = `
		INSERT INTO conversations (service_id, seeker_id, provider_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (service_id, seeker_id) DO UPDATE SET service_id = EXCLUDED.service_id
		RETURNING id, service_id, seeker_id, provider_id, order_id, last_message_at, created_at`

	var c model.Conversation
	err := r.db.QueryRow(ctx, q, serviceID, seekerID, providerID).
		Scan(&c.ID, &c.ServiceID, &c.SeekerID, &c.ProviderID, &c.OrderID,
			&c.LastMessageAt, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ChatRepository) GetConversation(ctx context.Context, id int64) (*model.Conversation, error) {
	const q = `
		SELECT id, service_id, seeker_id, provider_id, order_id, last_message_at, created_at
		  FROM conversations WHERE id = $1`

	var c model.Conversation
	err := r.db.QueryRow(ctx, q, id).Scan(&c.ID, &c.ServiceID, &c.SeekerID, &c.ProviderID,
		&c.OrderID, &c.LastMessageAt, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

// conversationRowSelect menyusun satu baris daftar percakapan dari sudut
// pandang satu pengguna: lawan bicaranya selalu pihak yang lain.
const conversationRowSelect = `
	SELECT cv.id, cv.service_id, cv.seeker_id, cv.provider_id, cv.order_id,
	       cv.last_message_at, cv.created_at,
	       s.title, img.image_url,
	       CASE WHEN cv.seeker_id = $1 THEN pu.full_name  ELSE su.full_name  END,
	       CASE WHEN cv.seeker_id = $1 THEN pu.avatar_url ELSE su.avatar_url END,
	       CASE WHEN cv.seeker_id = $1 THEN pu.id         ELSE su.id         END,
	       last_msg.body,
	       COALESCE(unread.jumlah, 0),
	       o.status
	  FROM conversations cv
	  JOIN services s          ON s.id = cv.service_id
	  JOIN provider_profiles p ON p.id = cv.provider_id
	  JOIN users pu            ON pu.id = p.user_id
	  JOIN users su            ON su.id = cv.seeker_id
	  LEFT JOIN orders o       ON o.id = cv.order_id
	  LEFT JOIN LATERAL (
	        SELECT si.image_url FROM service_images si
	         WHERE si.service_id = s.id ORDER BY si.sort_order, si.id LIMIT 1
	  ) img ON TRUE
	  LEFT JOIN LATERAL (
	        SELECT m.body FROM messages m
	         WHERE m.conversation_id = cv.id ORDER BY m.created_at DESC LIMIT 1
	  ) last_msg ON TRUE
	  LEFT JOIN LATERAL (
	        SELECT COUNT(*) AS jumlah FROM messages m
	         WHERE m.conversation_id = cv.id AND m.sender_id <> $1 AND m.read_at IS NULL
	  ) unread ON TRUE`

func (r *ChatRepository) scanConversationRows(ctx context.Context, q string, args ...any) ([]model.ConversationRow, error) {
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.ConversationRow, 0, 20)
	for rows.Next() {
		var c model.ConversationRow
		err := rows.Scan(&c.ID, &c.ServiceID, &c.SeekerID, &c.ProviderID, &c.OrderID,
			&c.LastMessageAt, &c.CreatedAt, &c.ServiceTitle, &c.ServiceCover,
			&c.LawanNama, &c.LawanAvatar, &c.LawanUserID,
			&c.PesanTerakhir, &c.BelumDibaca, &c.OrderStatus)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListConversations mengembalikan seluruh percakapan milik satu pengguna,
// baik sebagai pencari jasa maupun sebagai penyedia.
func (r *ChatRepository) ListConversations(ctx context.Context, userID int64) ([]model.ConversationRow, error) {
	q := conversationRowSelect + `
		 WHERE cv.seeker_id = $1 OR p.user_id = $1
		 ORDER BY cv.last_message_at DESC NULLS LAST, cv.created_at DESC`
	return r.scanConversationRows(ctx, q, userID)
}

// GetConversationRow mengambil satu percakapan dari sudut pandang userID.
func (r *ChatRepository) GetConversationRow(ctx context.Context, convID, userID int64) (*model.ConversationRow, error) {
	q := conversationRowSelect + ` WHERE cv.id = $2 AND (cv.seeker_id = $1 OR p.user_id = $1)`
	rows, err := r.scanConversationRows(ctx, q, userID, convID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNotFound
	}
	return &rows[0], nil
}

// ---------- pesan ----------

// AddMessage menyimpan pesan sekaligus memutakhirkan waktu pesan terakhir
// pada percakapannya, supaya urutan daftar obrolan tetap benar.
func (r *ChatRepository) AddMessage(ctx context.Context, m *model.Message) error {
	const q = `
		INSERT INTO messages (conversation_id, sender_id, body)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`
	if err := r.db.QueryRow(ctx, q, m.ConversationID, m.SenderID, m.Body).
		Scan(&m.ID, &m.CreatedAt); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx,
		`UPDATE conversations SET last_message_at = $2 WHERE id = $1`, m.ConversationID, m.CreatedAt)
	return err
}

// ListMessages mengembalikan pesan terurut dari yang paling lama.
// sinceID > 0 hanya mengambil pesan yang lebih baru, dipakai polling HTMX
// agar ruang obrolan tidak perlu mengunduh ulang seluruh riwayat.
func (r *ChatRepository) ListMessages(ctx context.Context, convID int64, sinceID int64, limit int) ([]model.MessageRow, error) {
	q := `
		SELECT m.id, m.conversation_id, m.sender_id, m.body, m.read_at, m.created_at,
		       u.full_name, u.avatar_url
		  FROM messages m
		  JOIN users u ON u.id = m.sender_id
		 WHERE m.conversation_id = $1 AND m.id > $2
		 ORDER BY m.id
		 LIMIT $3`

	rows, err := r.db.Query(ctx, q, convID, sinceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.MessageRow, 0, 50)
	for rows.Next() {
		var m model.MessageRow
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.Body, &m.ReadAt,
			&m.CreatedAt, &m.SenderNama, &m.SenderAvatar); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MarkRead menandai seluruh pesan lawan bicara sebagai sudah dibaca.
func (r *ChatRepository) MarkRead(ctx context.Context, convID, readerID int64) error {
	const q = `
		UPDATE messages
		   SET read_at = NOW()
		 WHERE conversation_id = $1 AND sender_id <> $2 AND read_at IS NULL`
	_, err := r.db.Exec(ctx, q, convID, readerID)
	return err
}

// CountUnread menghitung seluruh pesan belum dibaca milik satu pengguna,
// dipakai badge notifikasi di navigasi.
func (r *ChatRepository) CountUnread(ctx context.Context, userID int64) (int, error) {
	const q = `
		SELECT COUNT(*)
		  FROM messages m
		  JOIN conversations cv   ON cv.id = m.conversation_id
		  JOIN provider_profiles p ON p.id = cv.provider_id
		 WHERE m.read_at IS NULL
		   AND m.sender_id <> $1
		   AND (cv.seeker_id = $1 OR p.user_id = $1)`
	var n int
	err := r.db.QueryRow(ctx, q, userID).Scan(&n)
	return n, err
}

// AttachOrder menautkan percakapan ke pesanan yang baru dibuat.
func (r *ChatRepository) AttachOrder(ctx context.Context, convID, orderID int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE conversations SET order_id = $2 WHERE id = $1`, convID, orderID)
	return err
}
