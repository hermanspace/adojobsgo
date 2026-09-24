package service

import (
	"context"
	"errors"
	"strings"

	"github.com/hermansyah/adojobsid/internal/database"
	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/repository"
	"github.com/hermansyah/adojobsid/internal/validator"
)

// ReviewService mengelola ulasan pelanggan atas pekerjaan yang sudah selesai.
type ReviewService struct {
	repos *repository.Repositories
	cache *database.Cache
}

// RingkasanUlasan adalah ringkasan penilaian satu penyedia.
type RingkasanUlasan struct {
	Rata    float64           `json:"rata"`
	Total   int               `json:"total"`
	Sebaran map[int]int       `json:"sebaran"`
	Terbaru []model.ReviewRow `json:"terbaru"`
}

// PersenBintang menghitung porsi satu nilai bintang untuk grafik batang.
func (r RingkasanUlasan) PersenBintang(bintang int) int {
	if r.Total == 0 {
		return 0
	}
	return r.Sebaran[bintang] * 100 / r.Total
}

func (r RingkasanUlasan) JumlahBintang(bintang int) int { return r.Sebaran[bintang] }

// Ringkasan mengumpulkan penilaian satu penyedia untuk halaman publiknya.
func (s *ReviewService) Ringkasan(ctx context.Context, p *model.ProviderDetail, limit int) (*RingkasanUlasan, error) {
	sebaran, err := s.repos.Review.SebaranBintang(ctx, p.ID)
	if err != nil {
		return nil, Internal(err)
	}
	terbaru, err := s.repos.Review.ListByProvider(ctx, p.ID, limit)
	if err != nil {
		return nil, Internal(err)
	}
	// avg_rating dan total_reviews dibaca dari kolom cache di provider_profiles,
	// bukan diagregasi ulang di sini — itulah gunanya kolom tersebut.
	return &RingkasanUlasan{
		Rata:    p.AvgRating,
		Total:   p.TotalReviews,
		Sebaran: sebaran,
		Terbaru: terbaru,
	}, nil
}

type UlasanInput struct {
	OrderID  int64
	Rating   int
	Komentar string
}

// Tulis menyimpan ulasan atas satu pesanan.
//
// Dua aturan ditegakkan di sini: ulasan hanya boleh ditulis pencari jasa
// pemilik pesanan, dan hanya untuk pesanan yang sudah berstatus selesai.
// Satu pesanan hanya menghasilkan satu ulasan — dijaga indeks unik pada
// order_id, sehingga dua permintaan bersamaan pun tidak bisa menggandakannya.
func (s *ReviewService) Tulis(ctx context.Context, userID int64, in UlasanInput) (*model.Review, error) {
	order, err := s.repos.Order.GetByID(ctx, in.OrderID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, NotFound("Pesanan tidak ditemukan.")
		}
		return nil, Internal(err)
	}
	if order.SeekerID != userID {
		return nil, Forbidden("Hanya pemesan yang dapat menulis ulasan.")
	}
	if order.Status != model.OrderCompleted {
		return nil, InvalidMsg("Ulasan baru bisa ditulis setelah pesanan ditandai selesai.")
	}

	errs := validator.New()
	if in.Rating < 1 || in.Rating > 5 {
		errs.Add("rating", "Beri penilaian antara 1 sampai 5 bintang.")
	}
	komentar := strings.TrimSpace(in.Komentar)
	errs.Length("comment", "Ulasan", komentar, 0, 1000)
	if errs.Any() {
		return nil, Invalid(errs)
	}

	ulasan := &model.Review{OrderID: in.OrderID, Rating: in.Rating}
	if komentar != "" {
		ulasan.Comment = &komentar
	}

	if err := s.repos.Review.Create(ctx, ulasan); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return nil, Conflict("Pesanan ini sudah pernah Anda ulas.")
		}
		return nil, Internal(err)
	}

	// Kolom cache rating disegarkan hanya di sini — saat ada ulasan baru —
	// bukan setiap kali listing muncul di pencarian.
	if err := s.repos.Provider.RecalculateRating(ctx, order.ProviderID); err != nil {
		return nil, Internal(err)
	}

	provider, err := s.repos.Provider.GetByID(ctx, order.ProviderID)
	if err == nil {
		_ = s.repos.Notif.Create(ctx, provider.UserID, repository.NotifUlasanBaru, map[string]any{
			"order_id": in.OrderID,
			"rating":   in.Rating,
		})
	}

	invalidateSearchCache(s.cache)
	return ulasan, nil
}

// UlasanPesanan mengembalikan ulasan sebuah pesanan bila sudah ditulis.
func (s *ReviewService) UlasanPesanan(ctx context.Context, orderID int64) *model.Review {
	rv, err := s.repos.Review.GetByOrder(ctx, orderID)
	if err != nil {
		return nil
	}
	return rv
}

// DapatDiulas menandai pesanan yang sudah selesai, milik pengguna ini, dan
// belum pernah diulas.
func (s *ReviewService) DapatDiulas(ctx context.Context, order *model.Order, userID int64) bool {
	if order == nil || order.SeekerID != userID || order.Status != model.OrderCompleted {
		return false
	}
	return s.UlasanPesanan(ctx, order.ID) == nil
}
