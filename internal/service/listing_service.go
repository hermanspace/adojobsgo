package service

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"strconv"
	"strings"

	"github.com/hermansyah/adojobsid/internal/config"
	"github.com/hermansyah/adojobsid/internal/database"
	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/repository"
	"github.com/hermansyah/adojobsid/internal/validator"
)

type ListingService struct {
	repos  *repository.Repositories
	cache  *database.Cache
	upload *UploadService
	cfg    config.Upload
}

// ListingInput menampung isian form buat/edit listing. Harga dikirim sebagai
// string karena datang dari input teks berformat rupiah.
type ListingInput struct {
	CategoryID  int64
	Title       string
	Description string
	PriceType   string
	PriceMin    string
	PriceMax    string
	Status      string
}

func (s *ListingService) validate(ctx context.Context, in ListingInput) (validator.Errors, *model.Service) {
	errs := validator.New()

	title := errs.Required("title", "Judul jasa", in.Title)
	errs.Length("title", "Judul jasa", title, 10, 140)

	desc := errs.Required("description", "Deskripsi", in.Description)
	errs.Length("description", "Deskripsi", desc, 30, 5000)

	if in.CategoryID <= 0 {
		errs.Add("category_id", "Pilih kategori jasa.")
	} else if _, err := s.repos.Category.GetByID(ctx, in.CategoryID); err != nil {
		errs.Add("category_id", "Kategori yang dipilih tidak tersedia.")
	}

	priceType := model.PriceType(strings.TrimSpace(in.PriceType))
	if !priceType.Valid() {
		errs.Add("price_type", "Pilih jenis harga.")
	}

	min, errMin := parseRupiah(in.PriceMin)
	max, errMax := parseRupiah(in.PriceMax)
	if errMin != nil {
		errs.Add("price_min", "Harga tidak valid. Isi angka saja, misal 150000.")
	}
	if errMax != nil {
		errs.Add("price_max", "Harga tidak valid. Isi angka saja, misal 300000.")
	}

	if priceType == model.PriceNegotiable {
		// Harga nego tidak menyimpan angka sama sekali.
		min, max = nil, nil
	} else if priceType.Valid() {
		if min == nil {
			errs.Add("price_min", "Isi harga untuk jenis harga yang dipilih.")
		}
		if min != nil && max != nil && *max < *min {
			errs.Add("price_max", "Harga tertinggi tidak boleh lebih kecil dari harga terendah.")
		}
	}

	status := model.ServiceStatus(strings.TrimSpace(in.Status))
	if !status.Valid() {
		status = model.ServiceActive
	}

	if errs.Any() {
		return errs, nil
	}
	return errs, &model.Service{
		CategoryID:  in.CategoryID,
		Title:       title,
		Description: desc,
		PriceType:   priceType,
		PriceMin:    min,
		PriceMax:    max,
		Status:      status,
	}
}

func (s *ListingService) Create(ctx context.Context, providerID int64, in ListingInput) (*model.Service, error) {
	errs, svc := s.validate(ctx, in)
	if errs.Any() {
		return nil, Invalid(errs)
	}
	svc.ProviderID = providerID
	// Listing baru selalu masuk antrean peninjauan, apa pun status yang
	// dikirim dari form. Provider tidak bisa menayangkan listing sendiri.
	svc.Status = model.ServicePending

	if err := s.repos.Service.Create(ctx, svc); err != nil {
		return nil, Internal(err)
	}
	invalidateSearchCache(s.cache)
	return svc, nil
}

// bidangPentingBerubah menentukan apakah suatu perubahan cukup berarti untuk
// dikembalikan ke antrean peninjauan.
//
// Judul, deskripsi, dan kategori adalah isi yang dinilai admin saat menyetujui.
// Mengubahnya setelah disetujui adalah pintu klasik bait-and-switch: listing
// lolos dengan konten bersih lalu diganti isinya. Perubahan harga saja
// sengaja TIDAK memicu peninjauan ulang — itu perubahan paling lazim dan
// paling tidak berisiko, dan menahannya hanya membuat provider enggan
// memperbarui harga.
func bidangPentingBerubah(lama, baru *model.Service) bool {
	return lama.Title != baru.Title ||
		lama.Description != baru.Description ||
		lama.CategoryID != baru.CategoryID
}

func (s *ListingService) Update(ctx context.Context, providerID, serviceID int64, in ListingInput) (*model.Service, error) {
	existing, err := s.getOwned(ctx, providerID, serviceID)
	if err != nil {
		return nil, err
	}

	errs, svc := s.validate(ctx, in)
	if errs.Any() {
		return nil, Invalid(errs)
	}
	svc.ID = existing.ID
	svc.ProviderID = existing.ProviderID
	svc.CreatedAt = existing.CreatedAt

	// Provider hanya boleh memilih antara menayangkan dan menyembunyikan.
	// Status menunggu dan ditolak sepenuhnya ditentukan alur peninjauan.
	if !svc.Status.DapatDipilihProvider() {
		svc.Status = model.ServiceActive
	}

	perluTinjauUlang := bidangPentingBerubah(existing, svc)
	switch {
	case existing.Status == model.ServicePending:
		// Listing yang masih di antrean tetap di antrean.
		svc.Status = model.ServicePending
	case existing.Status == model.ServiceRejected:
		// Perbaikan atas listing yang ditolak diajukan ulang.
		svc.Status = model.ServicePending
		perluTinjauUlang = true
	case perluTinjauUlang && existing.Status == model.ServiceActive:
		// Listing yang sedang tayang turun dari peredaran sampai ditinjau ulang.
		svc.Status = model.ServicePending
	}

	if err := s.repos.Service.Update(ctx, svc); err != nil {
		return nil, Internal(err)
	}
	if svc.Status == model.ServicePending && existing.Status != model.ServicePending {
		if err := s.repos.Service.SubmitForReview(ctx, svc.ID); err != nil {
			return nil, Internal(err)
		}
	}
	invalidateSearchCache(s.cache)
	return svc, nil
}

// PerluTinjauUlang memberi tahu apakah isian yang sedang diketik akan membuat
// listing turun dari peredaran bila disimpan, supaya peringatannya bisa
// ditampilkan lebih dulu di form.
func (s *ListingService) PerluTinjauUlang(lama *model.Service, in ListingInput) bool {
	return strings.TrimSpace(in.Title) != lama.Title ||
		strings.TrimSpace(in.Description) != lama.Description ||
		in.CategoryID != lama.CategoryID
}

func (s *ListingService) GetDetail(ctx context.Context, id int64) (*model.ServiceDetail, error) {
	d, err := s.repos.Service.GetDetail(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, NotFound("Jasa tidak ditemukan atau sudah dihapus.")
		}
		return nil, Internal(err)
	}
	return d, nil
}

func (s *ListingService) ListByProvider(ctx context.Context, providerID int64, includeInactive bool) ([]model.ServiceCard, error) {
	items, err := s.repos.Service.ListByProvider(ctx, providerID, includeInactive)
	if err != nil {
		return nil, Internal(err)
	}
	return items, nil
}

func (s *ListingService) Delete(ctx context.Context, providerID, serviceID int64) error {
	if _, err := s.getOwned(ctx, providerID, serviceID); err != nil {
		return err
	}
	images, err := s.repos.Service.ListImages(ctx, serviceID)
	if err != nil {
		return Internal(err)
	}
	if err := s.repos.Service.Delete(ctx, serviceID); err != nil {
		return Internal(err)
	}
	for _, im := range images {
		s.upload.Delete(im.ImageURL)
		if im.ThumbURL != nil {
			s.upload.Delete(*im.ThumbURL)
		}
	}
	invalidateSearchCache(s.cache)
	return nil
}

// ---------- foto listing ----------

// AddImages menyimpan beberapa foto sekaligus, menghormati batas jumlah per listing.
//
// olehAdmin menandai perubahan yang dilakukan pengelola. Perubahan foto oleh
// provider mengembalikan listing ke antrean peninjauan, sedangkan perubahan
// oleh admin tidak — admin sendirilah peninjaunya.
func (s *ListingService) AddImages(ctx context.Context, providerID, serviceID int64, files []*multipart.FileHeader, olehAdmin bool) ([]model.ServiceImage, error) {
	if _, err := s.getOwned(ctx, providerID, serviceID); err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}

	existing, err := s.repos.Service.CountImages(ctx, serviceID)
	if err != nil {
		return nil, Internal(err)
	}
	if existing+len(files) > s.cfg.MaxPerListing {
		return nil, InvalidMsg(fmt.Sprintf(
			"Maksimal %d foto per listing. Saat ini sudah ada %d foto.",
			s.cfg.MaxPerListing, existing))
	}

	sortOrder, err := s.repos.Service.NextImageSortOrder(ctx, serviceID)
	if err != nil {
		return nil, Internal(err)
	}

	saved := make([]model.ServiceImage, 0, len(files))
	// bersihkan membuang seluruh berkas yang sudah tersimpan bila salah satu
	// gambar gagal diproses, supaya tidak ada berkas yatim di volume.
	bersihkan := func() {
		for _, im := range saved {
			s.upload.Delete(im.ImageURL)
			if im.ThumbURL != nil {
				s.upload.Delete(*im.ThumbURL)
			}
		}
	}

	for _, fh := range files {
		g, err := s.upload.SaveImage(fh, ProfilListing)
		if err != nil {
			bersihkan()
			return nil, err
		}

		im := model.ServiceImage{
			ServiceID: serviceID,
			ImageURL:  g.URL,
			SortOrder: sortOrder,
			Width:     &g.Lebar,
			Height:    &g.Tinggi,
			Bytes:     &g.Bytes,
		}
		if g.ThumbURL != "" {
			thumb := g.ThumbURL
			im.ThumbURL = &thumb
		}

		if err := s.repos.Service.AddImage(ctx, &im); err != nil {
			s.upload.Delete(g.URL)
			if g.ThumbURL != "" {
				s.upload.Delete(g.ThumbURL)
			}
			bersihkan()
			return nil, Internal(err)
		}
		saved = append(saved, im)
		sortOrder++
	}
	// Foto termasuk isi yang dinilai admin, jadi listing yang sedang tayang
	// kembali ke antrean begitu fotonya berubah.
	if !olehAdmin {
		s.kembalikanKeAntrean(ctx, serviceID)
	}
	invalidateSearchCache(s.cache)
	return saved, nil
}

// kembalikanKeAntrean menurunkan listing yang sedang tayang ke antrean
// peninjauan. Listing yang sudah berstatus lain dibiarkan apa adanya.
func (s *ListingService) kembalikanKeAntrean(ctx context.Context, serviceID int64) {
	svc, err := s.repos.Service.GetByID(ctx, serviceID)
	if err != nil || svc.Status != model.ServiceActive {
		return
	}
	_ = s.repos.Service.SubmitForReview(ctx, serviceID)
}

func (s *ListingService) DeleteImage(ctx context.Context, providerID, imageID int64, olehAdmin bool) error {
	im, err := s.repos.Service.GetImage(ctx, imageID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return NotFound("Foto tidak ditemukan.")
		}
		return Internal(err)
	}
	if _, err := s.getOwned(ctx, providerID, im.ServiceID); err != nil {
		return err
	}
	if err := s.repos.Service.DeleteImage(ctx, imageID); err != nil {
		return Internal(err)
	}
	s.upload.Delete(im.ImageURL)
	if im.ThumbURL != nil {
		s.upload.Delete(*im.ThumbURL)
	}
	if !olehAdmin {
		s.kembalikanKeAntrean(ctx, im.ServiceID)
	}
	invalidateSearchCache(s.cache)
	return nil
}

func (s *ListingService) ListImages(ctx context.Context, serviceID int64) ([]model.ServiceImage, error) {
	images, err := s.repos.Service.ListImages(ctx, serviceID)
	if err != nil {
		return nil, Internal(err)
	}
	return images, nil
}

// getOwned memastikan listing benar-benar milik provider yang sedang login.
func (s *ListingService) getOwned(ctx context.Context, providerID, serviceID int64) (*model.Service, error) {
	svc, err := s.repos.Service.GetByID(ctx, serviceID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, NotFound("Jasa tidak ditemukan.")
		}
		return nil, Internal(err)
	}
	if svc.ProviderID != providerID {
		return nil, Forbidden("Anda tidak berhak mengubah listing ini.")
	}
	return svc, nil
}

// parseRupiah menerima "150.000", "150000", "Rp 150.000" maupun string kosong.
func parseRupiah(raw string) (*float64, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, nil
	}
	s = strings.TrimPrefix(strings.ToLower(s), "rp")
	replacer := strings.NewReplacer(".", "", ",", "", " ", "", " ", "")
	s = replacer.Replace(s)
	if s == "" {
		return nil, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 {
		return nil, fmt.Errorf("harga tidak valid: %q", raw)
	}
	return &v, nil
}
