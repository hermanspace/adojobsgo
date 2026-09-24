package service

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"strings"

	"github.com/hermansyah/adojobsid/internal/database"
	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/repository"
	"github.com/hermansyah/adojobsid/internal/validator"
)

type ProviderService struct {
	repos  *repository.Repositories
	cache  *database.Cache
	upload *UploadService
}

type ProviderProfileInput struct {
	Bio            string
	WhatsappNumber string
}

func (s *ProviderService) validate(in ProviderProfileInput) (validator.Errors, string) {
	errs := validator.New()

	// Nomor WhatsApp opsional: percakapan berlangsung di dalam aplikasi, dan
	// data wajib yang tidak dipakai hanya jadi beban pendaftaran. Bila diisi,
	// formatnya tetap diperiksa supaya tombol WhatsApp — kalau nanti
	// dinyalakan — tidak menuju nomor yang salah.
	wa := ""
	if strings.TrimSpace(in.WhatsappNumber) != "" {
		wa = validator.NormalizePhone(in.WhatsappNumber)
		if !validator.ValidPhone(wa) {
			errs.Add("whatsapp_number", "Format nomor WhatsApp tidak valid. Contoh: 0812xxxxxxx.")
		}
	}

	errs.Length("bio", "Deskripsi", in.Bio, 20, 1000)
	return errs, wa
}

// Create membuat profil provider. Penanda users.is_provider tidak disentuh di
// sini: trigger di database (migrasi 000020) yang menyalakannya saat baris
// profil masuk, sehingga hanya ada satu penulis dan keduanya tidak bisa
// menyimpang.
func (s *ProviderService) Create(ctx context.Context, userID int64, in ProviderProfileInput) (*model.ProviderProfile, error) {
	errs, wa := s.validate(in)
	if errs.Any() {
		return nil, Invalid(errs)
	}

	// Slug diturunkan dari nama pengguna dan ditetapkan sekali di sini.
	// Nama yang berubah kemudian tidak menggeser slug, sehingga tautan
	// yang sudah dibagikan tetap berlaku.
	slug, err := s.slugUnik(ctx, userID)
	if err != nil {
		return nil, err
	}

	profile := &model.ProviderProfile{
		UserID:         userID,
		Slug:           slug,
		Bio:            strPtrOrNil(in.Bio),
		WhatsappNumber: wa,
	}

	if err := s.repos.Provider.Create(ctx, profile); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return nil, Conflict("Anda sudah memiliki profil penyedia jasa.")
		}
		return nil, Internal(err)
	}
	return profile, nil
}

func (s *ProviderService) Update(ctx context.Context, providerID int64, in ProviderProfileInput) (*model.ProviderProfile, error) {
	errs, wa := s.validate(in)
	if errs.Any() {
		return nil, Invalid(errs)
	}

	profile, err := s.GetByID(ctx, providerID)
	if err != nil {
		return nil, err
	}
	profile.Bio = strPtrOrNil(in.Bio)
	profile.WhatsappNumber = wa

	if err := s.repos.Provider.Update(ctx, profile); err != nil {
		return nil, Internal(err)
	}
	// Nama & area provider ikut tampil di kartu listing, jadi cache pencarian disegarkan.
	invalidateSearchCache(s.cache)
	return profile, nil
}

func (s *ProviderService) GetByID(ctx context.Context, id int64) (*model.ProviderProfile, error) {
	p, err := s.repos.Provider.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, NotFound("Penyedia jasa tidak ditemukan.")
		}
		return nil, Internal(err)
	}
	return p, nil
}

func (s *ProviderService) GetByUserID(ctx context.Context, userID int64) (*model.ProviderProfile, error) {
	p, err := s.repos.Provider.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, NotFound("Anda belum membuat profil penyedia jasa.")
		}
		return nil, Internal(err)
	}
	return p, nil
}

// GetDetail dipakai halaman publik detail provider.
func (s *ProviderService) GetDetail(ctx context.Context, id int64) (*model.ProviderDetail, error) {
	d, err := s.repos.Provider.GetDetailByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, NotFound("Penyedia jasa tidak ditemukan.")
		}
		return nil, Internal(err)
	}
	return d, nil
}

// UpdateAvatar menyimpan foto profil baru dan menghapus berkas lama.
func (s *ProviderService) UpdateAvatar(ctx context.Context, userID int64, fh *multipart.FileHeader) (string, error) {
	user, err := s.repos.User.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", NotFound("Akun tidak ditemukan.")
		}
		return "", Internal(err)
	}

	g, err := s.upload.SaveImage(fh, ProfilAvatar)
	if err != nil {
		return "", err
	}

	old := user.AvatarURL
	user.AvatarURL = &g.URL
	if err := s.repos.User.Update(ctx, user); err != nil {
		s.upload.Delete(g.URL)
		return "", Internal(err)
	}
	if old != nil && *old != "" {
		s.upload.Delete(*old)
	}
	invalidateSearchCache(s.cache)
	return g.URL, nil
}

// WhatsappLink membentuk tautan wa.me lengkap dengan pesan pembuka yang sudah terisi.
func WhatsappLink(number, message string) string {
	n := validator.NormalizePhone(number)
	if n == "" {
		return ""
	}
	return "https://wa.me/" + n + "?text=" + urlQueryEscape(message)
}

// slugUnik menyusun alamat publik penyedia dari namanya, dan menambahkan
// akhiran angka bila slug itu sudah dipakai penyedia lain.
func (s *ProviderService) slugUnik(ctx context.Context, userID int64) (string, error) {
	user, err := s.repos.User.GetByID(ctx, userID)
	if err != nil {
		return "", Internal(err)
	}

	pangkal := validator.Slugify(user.FullName)
	if pangkal == "" {
		pangkal = "penyedia"
	}
	if len(pangkal) > 120 {
		pangkal = strings.Trim(pangkal[:120], "-")
	}

	kandidat := pangkal
	for i := 2; i < 50; i++ {
		dipakai, err := s.repos.Provider.SlugDipakai(ctx, kandidat, 0)
		if err != nil {
			return "", Internal(err)
		}
		if !dipakai {
			return kandidat, nil
		}
		kandidat = fmt.Sprintf("%s-%d", pangkal, i)
	}
	// Sangat tidak mungkin tercapai; dipakai sebagai pengaman terakhir.
	return fmt.Sprintf("%s-%d", pangkal, userID), nil
}

// GetDetailBySlug mengambil penyedia dari alamat publiknya.
func (s *ProviderService) GetDetailBySlug(ctx context.Context, slug string) (*model.ProviderDetail, error) {
	d, err := s.repos.Provider.GetDetailBySlug(ctx, strings.TrimSpace(slug))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, NotFound("Penyedia jasa tidak ditemukan.")
		}
		return nil, Internal(err)
	}
	return d, nil
}

// ---------- direktori penyedia ----------

// CariPenyediaInput adalah parameter halaman daftar penyedia.
type CariPenyediaInput struct {
	Query              string
	Kecamatan          string
	HanyaTerverifikasi bool
	HanyaBerjasa       bool
	Sort               string
	Page               int

	Latitude  *float64
	Longitude *float64
	RadiusKm  int
}

// HasilPenyedia adalah satu halaman hasil direktori penyedia.
type HasilPenyedia struct {
	Items      []repository.ProviderCard `json:"items"`
	Total      int                       `json:"total"`
	Page       int                       `json:"page"`
	PerPage    int                       `json:"per_page"`
	TotalPages int                       `json:"total_pages"`
}

const penyediaPerHalaman = 12

// Cari mengembalikan daftar penyedia sesuai filter.
// Hasilnya di-cache di Redis seperti pencarian jasa, karena halaman ini
// sama-sama sering dibuka dan datanya jarang berubah.
func (s *ProviderService) Cari(ctx context.Context, in CariPenyediaInput) (*HasilPenyedia, error) {
	page := in.Page
	if page < 1 {
		page = 1
	}

	filter := repository.ProviderFilter{
		Query:              strings.TrimSpace(in.Query),
		Kecamatan:          strings.TrimSpace(in.Kecamatan),
		HanyaTerverifikasi: in.HanyaTerverifikasi,
		HanyaBerjasa:       in.HanyaBerjasa,
		Sort:               in.Sort,
		Limit:              penyediaPerHalaman,
		Offset:             (page - 1) * penyediaPerHalaman,
		Latitude:           in.Latitude,
		Longitude:          in.Longitude,
		RadiusKm:           in.RadiusKm,
	}

	items, err := s.repos.Provider.Search(ctx, filter)
	if err != nil {
		return nil, Internal(err)
	}
	total, err := s.repos.Provider.CountSearch(ctx, filter)
	if err != nil {
		return nil, Internal(err)
	}

	return &HasilPenyedia{
		Items:      items,
		Total:      total,
		Page:       page,
		PerPage:    penyediaPerHalaman,
		TotalPages: (total + penyediaPerHalaman - 1) / penyediaPerHalaman,
	}, nil
}

// KecamatanPenyedia menyediakan pilihan lokasi pada filter direktori.
func (s *ProviderService) KecamatanPenyedia(ctx context.Context) ([]string, error) {
	return database.Remember(ctx, s.cache, "filter:kecamatan-penyedia", 0,
		func() ([]string, error) {
			dariDB, err := s.repos.Provider.DistinctKecamatanPenyedia(ctx)
			if err != nil {
				return nil, err
			}
			seen := make(map[string]bool, len(model.KecamatanBengkalis))
			out := make([]string, 0, len(model.KecamatanBengkalis)+len(dariDB))
			for _, k := range model.KecamatanBengkalis {
				seen[strings.ToLower(k)] = true
				out = append(out, k)
			}
			for _, k := range dariDB {
				if !seen[strings.ToLower(k)] {
					out = append(out, k)
				}
			}
			return out, nil
		})
}
