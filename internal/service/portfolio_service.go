package service

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"strings"
	"time"

	"github.com/hermansyah/adojobsid/internal/database"
	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/repository"
	"github.com/hermansyah/adojobsid/internal/validator"
)

// ProfilPortofolio memakai ukuran tampilan lebih kecil daripada foto listing:
// karya portofolio dilihat dalam galeri, bukan sebagai gambar utama halaman.
var ProfilPortofolio = ProfilGambar{
	Subdir: "portofolio", MaksSisi: 1200, Kualitas: 80,
	ThumbSisi: 480, ThumbKualitas: 75,
}

// PortfolioService mengelola karya yang dipamerkan penyedia jasa.
type PortfolioService struct {
	repos  *repository.Repositories
	cache  *database.Cache
	upload *UploadService
}

// maksPortofolio membatasi jumlah karya per penyedia. Batas ini menjaga
// halaman publik tetap ringkas sekaligus membatasi pemakaian penyimpanan.
const maksPortofolio = 24

type PortofolioInput struct {
	Judul       string
	CompletedAt string
}

func (s *PortfolioService) Daftar(ctx context.Context, providerID int64, limit int) ([]model.Portfolio, error) {
	items, err := s.repos.Portfolio.ListByProvider(ctx, providerID, limit)
	if err != nil {
		return nil, Internal(err)
	}
	return items, nil
}

// Tambah menyimpan satu karya portofolio beserta gambarnya.
func (s *PortfolioService) Tambah(ctx context.Context, providerID int64, in PortofolioInput, fh *multipart.FileHeader) (*model.Portfolio, error) {
	errs := validator.New()

	judul := errs.Required("title", "Judul pekerjaan", in.Judul)
	errs.Length("title", "Judul pekerjaan", judul, 3, 140)

	var selesai *time.Time
	if v := strings.TrimSpace(in.CompletedAt); v != "" {
		t, err := time.Parse("2006-01-02", v)
		switch {
		case err != nil:
			errs.Add("completed_at", "Format tanggal tidak valid.")
		case t.After(time.Now().AddDate(0, 0, 1)):
			errs.Add("completed_at", "Tanggal pengerjaan tidak boleh di masa depan.")
		default:
			selesai = &t
		}
	}
	if fh == nil {
		errs.Add("gambar", "Pilih foto hasil pekerjaan.")
	}
	if errs.Any() {
		return nil, Invalid(errs)
	}

	jumlah, err := s.repos.Portfolio.CountByProvider(ctx, providerID)
	if err != nil {
		return nil, Internal(err)
	}
	if jumlah >= maksPortofolio {
		return nil, InvalidMsg(fmt.Sprintf(
			"Maksimal %d karya portofolio. Hapus salah satu sebelum menambah yang baru.", maksPortofolio))
	}

	g, err := s.upload.SaveImage(fh, ProfilPortofolio)
	if err != nil {
		return nil, err
	}

	item := &model.Portfolio{
		ProviderID:  providerID,
		Title:       judul,
		ImageURL:    g.URL,
		Width:       &g.Lebar,
		Height:      &g.Tinggi,
		Bytes:       &g.Bytes,
		CompletedAt: selesai,
	}
	if g.ThumbURL != "" {
		thumb := g.ThumbURL
		item.ThumbURL = &thumb
	}

	if err := s.repos.Portfolio.Create(ctx, item); err != nil {
		s.upload.Delete(g.URL)
		if g.ThumbURL != "" {
			s.upload.Delete(g.ThumbURL)
		}
		return nil, Internal(err)
	}
	s.invalidate()
	return item, nil
}

// Hapus membuang satu karya beserta berkas gambarnya.
func (s *PortfolioService) Hapus(ctx context.Context, providerID, portfolioID int64) error {
	item, err := s.repos.Portfolio.GetByID(ctx, portfolioID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return NotFound("Karya portofolio tidak ditemukan.")
		}
		return Internal(err)
	}
	if item.ProviderID != providerID {
		return Forbidden("Karya ini bukan milik akun Anda.")
	}

	if err := s.repos.Portfolio.Delete(ctx, portfolioID); err != nil {
		return Internal(err)
	}
	s.upload.Delete(item.ImageURL)
	if item.ThumbURL != nil {
		s.upload.Delete(*item.ThumbURL)
	}
	s.invalidate()
	return nil
}

// MaksKarya mengembalikan batas jumlah karya portofolio.
func (s *PortfolioService) MaksKarya() int { return maksPortofolio }

func (s *PortfolioService) invalidate() {
	if s.cache == nil {
		return
	}
	s.cache.Forget(contextBackground(), "penyedia:*")
}
