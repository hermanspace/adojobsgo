package service

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/hermansyah/adojobsid/internal/database"
	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/repository"
)

// CatalogService melayani kategori dan pencarian jasa — bagian aplikasi yang
// paling sering diakses, karena itu hasilnya di-cache di Redis.
type CatalogService struct {
	repos *repository.Repositories
	cache *database.Cache
}

// SearchInput adalah parameter pencarian yang datang dari query string.
type SearchInput struct {
	Query        string
	CategorySlug string
	City         string
	Kecamatan    string
	PriceType    string
	Sort         string
	Page         int
	PerPage      int

	// Titik acuan pencari jasa beserta batas radiusnya.
	Latitude      *float64
	Longitude     *float64
	RadiusKm      int
	OnlyReachable bool
}

type SearchResult struct {
	Items      []model.ServiceCard `json:"items"`
	Total      int                 `json:"total"`
	Page       int                 `json:"page"`
	PerPage    int                 `json:"per_page"`
	TotalPages int                 `json:"total_pages"`
	Category   *model.Category     `json:"category,omitempty"`
}

// HasNextPage dipakai tombol "Muat lebih banyak" berbasis HTMX.
func (r SearchResult) HasNextPage() bool { return r.Page < r.TotalPages }
func (r SearchResult) NextPage() int     { return r.Page + 1 }

const (
	defaultPerPage = 12
	maxPerPage     = 48
)

func (s *CatalogService) ListRootCategories(ctx context.Context) ([]model.Category, error) {
	return database.Remember(ctx, s.cache, "filter:categories:root", 0,
		func() ([]model.Category, error) {
			return s.repos.Category.ListRoots(ctx)
		})
}

// ListCategoryTree mengembalikan kategori induk lengkap dengan anak-anaknya,
// dipakai di form buat listing dan panel filter.
func (s *CatalogService) ListCategoryTree(ctx context.Context) ([]model.Category, error) {
	return database.Remember(ctx, s.cache, "filter:categories:tree", 0,
		func() ([]model.Category, error) {
			all, err := s.repos.Category.ListAll(ctx)
			if err != nil {
				return nil, err
			}
			byID := make(map[int64]*model.Category, len(all))
			roots := make([]model.Category, 0, 12)
			for i := range all {
				if all[i].ParentID == nil {
					roots = append(roots, all[i])
				}
			}
			for i := range roots {
				byID[roots[i].ID] = &roots[i]
			}
			for _, c := range all {
				if c.ParentID == nil {
					continue
				}
				if parent, ok := byID[*c.ParentID]; ok {
					parent.Children = append(parent.Children, c)
				}
			}
			return roots, nil
		})
}

func (s *CatalogService) GetCategoryBySlug(ctx context.Context, slug string) (*model.Category, error) {
	c, err := s.repos.Category.GetBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, NotFound("Kategori tidak ditemukan.")
		}
		return nil, Internal(err)
	}
	return c, nil
}

// GetCategoryByID dipakai panel admin untuk memuat kategori yang disunting.
func (s *CatalogService) GetCategoryByID(ctx context.Context, id int64) (*model.Category, error) {
	c, err := s.repos.Category.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, NotFound("Kategori tidak ditemukan.")
		}
		return nil, Internal(err)
	}
	return c, nil
}

// KecamatanOptions menggabungkan daftar kecamatan resmi Kabupaten Bengkalis
// dengan kecamatan lain yang sudah punya listing aktif.
func (s *CatalogService) KecamatanOptions(ctx context.Context) ([]string, error) {
	return database.Remember(ctx, s.cache, "filter:kecamatan", 0, func() ([]string, error) {
		fromDB, err := s.repos.Service.DistinctKecamatan(ctx)
		if err != nil {
			return nil, err
		}
		seen := make(map[string]bool, len(model.KecamatanBengkalis)+len(fromDB))
		out := make([]string, 0, len(model.KecamatanBengkalis)+len(fromDB))
		for _, k := range model.KecamatanBengkalis {
			seen[strings.ToLower(k)] = true
			out = append(out, k)
		}
		for _, k := range fromDB {
			if !seen[strings.ToLower(k)] {
				seen[strings.ToLower(k)] = true
				out = append(out, k)
			}
		}
		return out, nil
	})
}

// Search menjalankan pencarian dengan filter kategori dan lokasi.
// Filter kategori induk otomatis mencakup seluruh sub-kategorinya.
func (s *CatalogService) Search(ctx context.Context, in SearchInput) (*SearchResult, error) {
	page := in.Page
	if page < 1 {
		page = 1
	}
	perPage := in.PerPage
	if perPage <= 0 {
		perPage = defaultPerPage
	}
	if perPage > maxPerPage {
		perPage = maxPerPage
	}

	filter := repository.ServiceFilter{
		Query:         strings.TrimSpace(in.Query),
		City:          strings.TrimSpace(in.City),
		Kecamatan:     strings.TrimSpace(in.Kecamatan),
		Sort:          in.Sort,
		Limit:         perPage,
		Offset:        (page - 1) * perPage,
		Latitude:      in.Latitude,
		Longitude:     in.Longitude,
		RadiusKm:      in.RadiusKm,
		OnlyReachable: in.OnlyReachable,
	}
	if pt := model.PriceType(in.PriceType); pt.Valid() {
		filter.PriceType = string(pt)
	}

	var category *model.Category
	if slug := strings.TrimSpace(in.CategorySlug); slug != "" {
		c, err := s.GetCategoryBySlug(ctx, slug)
		if err != nil {
			return nil, err
		}
		category = c
		ids, err := s.repos.Category.DescendantIDs(ctx, c.ID)
		if err != nil {
			return nil, Internal(err)
		}
		filter.CategoryIDs = ids
	}

	// Pencarian berlokasi di-cache per sel ~1 km, bukan per titik GPS.
	// Dengan pembulatan 100 m sebelumnya, tiap pengguna jatuh di selnya
	// sendiri dan cache nyaris tak pernah kena tepat di query termahal
	// (earth_box). Titik persisnya disimpan dulu: jarak tiap hasil dihitung
	// ulang darinya setelah keluar cache, jadi angka "265 m" yang dilihat
	// pengguna tetap miliknya sendiri, bukan milik orang lain di sel yang sama.
	var tepatLat, tepatLng float64
	adaLokasi := filter.PunyaLokasi()
	if adaLokasi {
		tepatLat, tepatLng = *filter.Latitude, *filter.Longitude
		selLat, selLng := BulatkanSel(tepatLat), BulatkanSel(tepatLng)
		filter.Latitude, filter.Longitude = &selLat, &selLng
	}

	result, err := database.Remember(ctx, s.cache, filter.CacheKey(), 0, func() (*SearchResult, error) {
		items, err := s.repos.Service.Search(ctx, filter)
		if err != nil {
			return nil, err
		}
		total, err := s.repos.Service.CountSearch(ctx, filter)
		if err != nil {
			return nil, err
		}
		totalPages := (total + perPage - 1) / perPage
		return &SearchResult{
			Items:      items,
			Total:      total,
			Page:       page,
			PerPage:    perPage,
			TotalPages: totalPages,
		}, nil
	})
	if err != nil {
		return nil, Internal(err)
	}
	if adaLokasi {
		HitungUlangJarak(result.Items, tepatLat, tepatLng, filter.Sort == "terdekat")
	}
	result.Category = category
	return result, nil
}

// BulatkanSel membulatkan koordinat ke dua desimal — sel sekitar 1,1 km di
// lintang Bengkalis. Sel dipakai untuk kunci cache dan pusat query; jarak yang
// ditampilkan tidak pernah memakainya. Kesalahan terbesar yang mungkin timbul
// hanya pada saringan radius di tepinya: sekitar 0,8 km pada radius 5–100 km.
func BulatkanSel(v float64) float64 { return math.Round(v*100) / 100 }

// HitungUlangJarak mengganti jarak hasil cache — yang dihitung dari pusat sel
// — dengan jarak dari titik pengguna yang sebenarnya, lalu mengurutkan ulang
// bila pengguna memang meminta urutan terdekat. Hasil tanpa koordinat
// penyedia dibiarkan tanpa jarak, seperti sebelumnya.
func HitungUlangJarak(items []model.ServiceCard, lat, lng float64, urutkan bool) {
	for i := range items {
		if items[i].ProviderLat == nil || items[i].ProviderLng == nil {
			items[i].JarakKm = nil
			continue
		}
		km := model.JarakKm(lat, lng, *items[i].ProviderLat, *items[i].ProviderLng)
		items[i].JarakKm = &km
	}
	if !urutkan {
		return
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i].JarakKm, items[j].JarakKm
		switch {
		case a == nil:
			return false
		case b == nil:
			return true
		default:
			return *a < *b
		}
	})
}

// RingkasanSitus adalah angka hidup untuk halaman Tentang dan API bantuan.
type RingkasanSitus struct {
	JasaAktif int `json:"jasa_aktif"`
	Penyedia  int `json:"penyedia"`
	Kecamatan int `json:"kecamatan"`
}

// Ringkasan menghitung jasa aktif dan penyedia dengan filter kosong — jalur
// yang sama dengan pencarian — dan disimpan 10 menit: halaman Tentang bukan
// tempat angka harus detik-akurat, tapi juga bukan tempat menulis angka mati.
func (s *CatalogService) Ringkasan(ctx context.Context) (RingkasanSitus, error) {
	return database.Remember(ctx, s.cache, "ringkasan:situs", 10*time.Minute, func() (RingkasanSitus, error) {
		jasa, err := s.repos.Service.CountSearch(ctx, repository.ServiceFilter{})
		if err != nil {
			return RingkasanSitus{}, err
		}
		penyedia, err := s.repos.Provider.CountSearch(ctx, repository.ProviderFilter{})
		if err != nil {
			return RingkasanSitus{}, err
		}
		return RingkasanSitus{JasaAktif: jasa, Penyedia: penyedia, Kecamatan: len(model.KecamatanBengkalisKoordinat)}, nil
	})
}
