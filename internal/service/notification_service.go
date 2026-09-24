package service

import (
	"context"

	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/repository"
)

// NotificationService melayani daftar dan penghitung notifikasi pengguna.
type NotificationService struct {
	repos *repository.Repositories
}

func (s *NotificationService) Daftar(ctx context.Context, userID int64) ([]model.Notification, error) {
	items, err := s.repos.Notif.List(ctx, userID, 30)
	if err != nil {
		return nil, Internal(err)
	}
	return items, nil
}

// BelumDibaca tidak pernah menggagalkan permintaan: badge yang gagal dihitung
// cukup ditampilkan sebagai nol, bukan membuat seluruh halaman gagal.
func (s *NotificationService) BelumDibaca(ctx context.Context, userID int64) int {
	n, err := s.repos.Notif.CountUnread(ctx, userID)
	if err != nil {
		return 0
	}
	return n
}

func (s *NotificationService) TandaiTerbaca(ctx context.Context, userID int64) error {
	if err := s.repos.Notif.MarkAllRead(ctx, userID); err != nil {
		return Internal(err)
	}
	return nil
}
