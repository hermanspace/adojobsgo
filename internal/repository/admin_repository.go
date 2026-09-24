package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hermansyah/adojobsid/internal/model"
)

type AdminRepository struct{ db DBTX }

// UserFilter adalah parameter daftar pengguna di panel admin.
type UserFilter struct {
	Query     string // nama, nomor HP, atau email
	Role      string // user | admin
	Status    string // aktif | ditangguhkan
	OnlyType  string // provider | pencari
	Kecamatan string
	Sort      string // terbaru | nama | rating
	Limit     int
	Offset    int
}

// AdminUserRow adalah satu baris pada tabel pengguna, sudah digabung dengan
// data provider bila akun tersebut penyedia jasa.
type AdminUserRow struct {
	model.User
	ProviderID    *int64     `json:"provider_id,omitempty"`
	IsVerified    bool       `json:"is_verified"`
	AvgRating     float64    `json:"avg_rating"`
	TotalReviews  int        `json:"total_reviews"`
	FeaturedUntil *time.Time `json:"featured_until,omitempty"`
	TotalListing  int        `json:"total_listing"`
}

// IsFeatured menandai penyedia pilihan yang masa tayangnya masih berlaku.
func (r AdminUserRow) IsFeatured() bool {
	return r.FeaturedUntil != nil && r.FeaturedUntil.After(time.Now())
}

const adminUserSelect = `
	SELECT u.id, u.full_name, u.phone, u.email, '' AS password_hash, u.avatar_url,
	       u.city, u.kecamatan, u.is_provider, u.role, u.suspended_at, u.suspended_reason,
	       u.created_at,
	       p.id, COALESCE(p.is_verified, FALSE), COALESCE(p.avg_rating, 0),
	       COALESCE(p.total_reviews, 0), p.featured_until,
	       COALESCE(s.total, 0)
	  FROM users u
	  LEFT JOIN provider_profiles p ON p.user_id = u.id
	  LEFT JOIN LATERAL (
	        SELECT COUNT(*) AS total FROM services sv WHERE sv.provider_id = p.id
	  ) s ON TRUE`

// buildUserFilter menyusun klausa WHERE dinamis untuk daftar pengguna.
func buildUserFilter(f UserFilter) (string, []any) {
	var (
		conds []string
		args  []any
	)
	add := func(cond string, val any) {
		args = append(args, val)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}

	if q := strings.TrimSpace(f.Query); q != "" {
		args = append(args, "%"+q+"%")
		n := len(args)
		conds = append(conds, fmt.Sprintf(
			"(u.full_name ILIKE $%d OR u.phone ILIKE $%[1]d OR COALESCE(u.email, '') ILIKE $%[1]d)", n))
	}
	if v := strings.TrimSpace(f.Role); v != "" {
		add("u.role = $%d", v)
	}
	switch f.Status {
	case "ditangguhkan":
		conds = append(conds, "u.suspended_at IS NOT NULL")
	case "aktif":
		conds = append(conds, "u.suspended_at IS NULL")
	}
	switch f.OnlyType {
	case "provider":
		conds = append(conds, "u.is_provider = TRUE")
	case "pencari":
		conds = append(conds, "u.is_provider = FALSE")
	}
	if v := strings.TrimSpace(f.Kecamatan); v != "" {
		add("u.kecamatan ILIKE $%d", v)
	}

	if len(conds) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conds, " AND "), args
}

func userOrderClause(sort string) string {
	switch sort {
	case "nama":
		return "ORDER BY u.full_name ASC"
	case "rating":
		return "ORDER BY COALESCE(p.avg_rating, 0) DESC, COALESCE(p.total_reviews, 0) DESC"
	case "listing":
		return "ORDER BY COALESCE(s.total, 0) DESC, u.created_at DESC"
	default:
		return "ORDER BY u.created_at DESC"
	}
}

func (r *AdminRepository) ListUsers(ctx context.Context, f UserFilter) ([]AdminUserRow, error) {
	where, args := buildUserFilter(f)
	args = append(args, f.Limit, f.Offset)
	q := fmt.Sprintf("%s %s %s LIMIT $%d OFFSET $%d",
		adminUserSelect, where, userOrderClause(f.Sort), len(args)-1, len(args))

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]AdminUserRow, 0, 25)
	for rows.Next() {
		var u AdminUserRow
		err := rows.Scan(&u.ID, &u.FullName, &u.Phone, &u.Email, &u.PasswordHash,
			&u.AvatarURL, &u.City, &u.Kecamatan, &u.IsProvider, &u.Role,
			&u.SuspendedAt, &u.SuspendedReason, &u.CreatedAt,
			&u.ProviderID, &u.IsVerified, &u.AvgRating, &u.TotalReviews,
			&u.FeaturedUntil, &u.TotalListing)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (r *AdminRepository) CountUsers(ctx context.Context, f UserFilter) (int, error) {
	where, args := buildUserFilter(f)
	q := `
		SELECT COUNT(*)
		  FROM users u
		  LEFT JOIN provider_profiles p ON p.user_id = u.id ` + where
	var n int
	err := r.db.QueryRow(ctx, q, args...).Scan(&n)
	return n, err
}

// GetUserRow mengambil satu baris pengguna lengkap dengan data providernya.
func (r *AdminRepository) GetUserRow(ctx context.Context, userID int64) (*AdminUserRow, error) {
	var u AdminUserRow
	err := r.db.QueryRow(ctx, adminUserSelect+" WHERE u.id = $1", userID).Scan(&u.ID, &u.FullName, &u.Phone, &u.Email,
		&u.PasswordHash, &u.AvatarURL, &u.City, &u.Kecamatan, &u.IsProvider, &u.Role,
		&u.SuspendedAt, &u.SuspendedReason, &u.CreatedAt,
		&u.ProviderID, &u.IsVerified, &u.AvgRating, &u.TotalReviews,
		&u.FeaturedUntil, &u.TotalListing)
	if err != nil {
		return nil, ErrNotFound
	}
	return &u, nil
}

// ---------- statistik dasbor ----------

// Stats adalah ringkasan angka untuk dasbor admin.
type Stats struct {
	TotalUser         int `json:"total_user"`
	UserBaru7Hari     int `json:"user_baru_7_hari"`
	TotalProvider     int `json:"total_provider"`
	ProviderTerverif  int `json:"provider_terverifikasi"`
	ProviderPilihan   int `json:"provider_pilihan"`
	UserDitangguhkan  int `json:"user_ditangguhkan"`
	TotalListing      int `json:"total_listing"`
	ListingAktif      int `json:"listing_aktif"`
	ListingNonaktif   int `json:"listing_nonaktif"`
	ListingPilihan    int `json:"listing_pilihan"`
	ListingBaru7Hari  int `json:"listing_baru_7_hari"`
	ListingTanpaFoto  int `json:"listing_tanpa_foto"`
	TotalKategori     int `json:"total_kategori"`
	KategoriTanpaJasa int `json:"kategori_tanpa_jasa"`
}

// Stats menghitung seluruh angka dasbor dalam satu perjalanan ke database,
// bukan belasan query terpisah.
func (r *AdminRepository) Stats(ctx context.Context) (*Stats, error) {
	const q = `
		SELECT
		  (SELECT COUNT(*) FROM users),
		  (SELECT COUNT(*) FROM users WHERE created_at >= NOW() - INTERVAL '7 days'),
		  (SELECT COUNT(*) FROM provider_profiles),
		  (SELECT COUNT(*) FROM provider_profiles WHERE is_verified),
		  (SELECT COUNT(*) FROM provider_profiles WHERE featured_until > NOW()),
		  (SELECT COUNT(*) FROM users WHERE suspended_at IS NOT NULL),
		  (SELECT COUNT(*) FROM services),
		  (SELECT COUNT(*) FROM services WHERE status = 'active'),
		  (SELECT COUNT(*) FROM services WHERE status = 'inactive'),
		  (SELECT COUNT(*) FROM services WHERE featured_until > NOW()),
		  (SELECT COUNT(*) FROM services WHERE created_at >= NOW() - INTERVAL '7 days'),
		  (SELECT COUNT(*) FROM services s
		    WHERE NOT EXISTS (SELECT 1 FROM service_images si WHERE si.service_id = s.id)),
		  (SELECT COUNT(*) FROM categories),
		  (SELECT COUNT(*) FROM categories c
		    WHERE c.parent_id IS NOT NULL
		      AND NOT EXISTS (SELECT 1 FROM services s WHERE s.category_id = c.id))`

	var st Stats
	err := r.db.QueryRow(ctx, q).Scan(
		&st.TotalUser, &st.UserBaru7Hari, &st.TotalProvider, &st.ProviderTerverif,
		&st.ProviderPilihan, &st.UserDitangguhkan, &st.TotalListing, &st.ListingAktif,
		&st.ListingNonaktif, &st.ListingPilihan, &st.ListingBaru7Hari,
		&st.ListingTanpaFoto, &st.TotalKategori, &st.KategoriTanpaJasa)
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// KategoriTeratas adalah jumlah listing aktif per kategori induk,
// dipakai sebagai grafik batang sederhana di dasbor.
type KategoriTeratas struct {
	Nama   string `json:"nama"`
	Slug   string `json:"slug"`
	Jumlah int    `json:"jumlah"`
}

func (r *AdminRepository) KategoriTeratas(ctx context.Context, limit int) ([]KategoriTeratas, error) {
	// Listing dihitung ke kategori induknya, sehingga grafik tetap terbaca
	// walau jasa tersebar di banyak sub-kategori.
	const q = `
		SELECT COALESCE(pc.name, c.name) AS nama,
		       COALESCE(pc.slug, c.slug) AS slug,
		       COUNT(*)                  AS jumlah
		  FROM services s
		  JOIN categories c       ON c.id = s.category_id
		  LEFT JOIN categories pc ON pc.id = c.parent_id
		 WHERE s.status = 'active'
		 GROUP BY 1, 2
		 ORDER BY jumlah DESC, nama
		 LIMIT $1`

	rows, err := r.db.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]KategoriTeratas, 0, limit)
	for rows.Next() {
		var k KategoriTeratas
		if err := rows.Scan(&k.Nama, &k.Slug, &k.Jumlah); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}
