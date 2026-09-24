package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hermansyah/adojobsid/internal/model"
)

type ProviderRepository struct{ db DBTX }

const providerColumns = `id, user_id, slug, bio, whatsapp_number, is_verified,
	avg_rating, total_reviews, featured_until, latitude, longitude, address_label,
	service_radius_km, created_at`

// providerDetailKolom adalah satu-satunya daftar kolom penyedia + user.
// ServiceRepository.GetDetail ikut memakainya karena dulu daftar ini disalin ke
// sana, lalu menyimpang diam-diam saat kolom slug ditambahkan: query-nya tetap
// jalan, hanya slug-nya kosong sehingga tautan penyedia mengarah ke /penyedia/.
const providerDetailKolom = `p.id, p.user_id, p.slug, p.bio, p.whatsapp_number,
	       p.is_verified, p.avg_rating, p.total_reviews, p.featured_until,
	       p.latitude, p.longitude, p.address_label, p.service_radius_km, p.created_at,
	       u.full_name, u.avatar_url, u.city, u.kecamatan, u.phone, u.suspended_at`

// providerDetailSelect selalu menggabungkan provider_profiles dengan users supaya
// nama, avatar dan lokasi tersedia tanpa query tambahan.
const providerDetailSelect = `
	SELECT ` + providerDetailKolom + `
	  FROM provider_profiles p
	  JOIN users u ON u.id = p.user_id`

// targetProviderDetail mengembalikan target Scan untuk providerDetailKolom dalam
// urutan yang sama persis. Pemanggil yang menyisipkan kolom lain menyambungnya
// dengan append, bukan menulis ulang daftarnya.
func targetProviderDetail(d *model.ProviderDetail) []any {
	return []any{
		&d.ID, &d.UserID, &d.Slug, &d.Bio, &d.WhatsappNumber,
		&d.IsVerified, &d.AvgRating, &d.TotalReviews, &d.FeaturedUntil,
		&d.Latitude, &d.Longitude, &d.AddressLabel, &d.ServiceRadiusKm, &d.CreatedAt,
		&d.FullName, &d.AvatarURL, &d.City, &d.Kecamatan, &d.Phone, &d.SuspendedAt,
	}
}

func scanProvider(row pgx.Row) (*model.ProviderProfile, error) {
	var p model.ProviderProfile
	err := row.Scan(&p.ID, &p.UserID, &p.Slug, &p.Bio, &p.WhatsappNumber,
		&p.IsVerified, &p.AvgRating, &p.TotalReviews, &p.FeaturedUntil,
		&p.Latitude, &p.Longitude, &p.AddressLabel, &p.ServiceRadiusKm, &p.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func scanProviderDetail(row pgx.Row) (*model.ProviderDetail, error) {
	var d model.ProviderDetail
	err := row.Scan(targetProviderDetail(&d)...)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &d, nil
}

func (r *ProviderRepository) Create(ctx context.Context, p *model.ProviderProfile) error {
	const q = `
		INSERT INTO provider_profiles (user_id, slug, bio, whatsapp_number)
		VALUES ($1, $2, $3, $4)
		RETURNING id, is_verified, avg_rating, total_reviews, featured_until,
		          service_radius_km, created_at`
	err := r.db.QueryRow(ctx, q, p.UserID, p.Slug, p.Bio, p.WhatsappNumber).
		Scan(&p.ID, &p.IsVerified, &p.AvgRating, &p.TotalReviews, &p.FeaturedUntil,
			&p.ServiceRadiusKm, &p.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return err
	}
	return nil
}

func (r *ProviderRepository) GetByID(ctx context.Context, id int64) (*model.ProviderProfile, error) {
	return scanProvider(r.db.QueryRow(ctx,
		`SELECT `+providerColumns+` FROM provider_profiles WHERE id = $1`, id))
}

func (r *ProviderRepository) GetByUserID(ctx context.Context, userID int64) (*model.ProviderProfile, error) {
	return scanProvider(r.db.QueryRow(ctx,
		`SELECT `+providerColumns+` FROM provider_profiles WHERE user_id = $1`, userID))
}

func (r *ProviderRepository) GetDetailByID(ctx context.Context, id int64) (*model.ProviderDetail, error) {
	return scanProviderDetail(r.db.QueryRow(ctx, providerDetailSelect+` WHERE p.id = $1`, id))
}

func (r *ProviderRepository) GetDetailByUserID(ctx context.Context, userID int64) (*model.ProviderDetail, error) {
	return scanProviderDetail(r.db.QueryRow(ctx, providerDetailSelect+` WHERE p.user_id = $1`, userID))
}

func (r *ProviderRepository) Update(ctx context.Context, p *model.ProviderProfile) error {
	const q = `
		UPDATE provider_profiles
		   SET bio = $2, whatsapp_number = $3
		 WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, p.ID, p.Bio, p.WhatsappNumber)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RecalculateRating menyegarkan kolom cache avg_rating dan total_reviews.
// Dipanggil hanya saat ada ulasan baru (tahap 3), bukan saat listing dirender.
func (r *ProviderRepository) RecalculateRating(ctx context.Context, providerID int64) error {
	const q = `
		UPDATE provider_profiles p
		   SET avg_rating = COALESCE(agg.avg_rating, 0),
		       total_reviews = COALESCE(agg.total_reviews, 0)
		  FROM (
		        SELECT ROUND(AVG(rv.rating)::NUMERIC, 2) AS avg_rating,
		               COUNT(*)                          AS total_reviews
		          FROM reviews rv
		          JOIN orders o ON o.id = rv.order_id
		         WHERE o.provider_id = $1
		       ) AS agg
		 WHERE p.id = $1`
	_, err := r.db.Exec(ctx, q, providerID)
	return err
}

// ---------- operasi admin ----------

// SetVerified menyetel status verifikasi penyedia.
func (r *ProviderRepository) SetVerified(ctx context.Context, providerID int64, verified bool) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE provider_profiles SET is_verified = $2 WHERE id = $1`, providerID, verified)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetFeatured menyorot penyedia sampai waktu tertentu. until bernilai nil
// untuk mencabut status pilihan.
func (r *ProviderRepository) SetFeatured(ctx context.Context, providerID int64, until *time.Time) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE provider_profiles SET featured_until = $2 WHERE id = $1`, providerID, until)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListFeatured mengembalikan penyedia pilihan yang masa tayangnya masih berlaku
// dan akunnya tidak ditangguhkan, untuk ditampilkan di beranda.
func (r *ProviderRepository) ListFeatured(ctx context.Context, limit int) ([]model.ProviderDetail, error) {
	rows, err := r.db.Query(ctx, providerDetailSelect+`
		 WHERE p.featured_until > NOW() AND u.suspended_at IS NULL
		 ORDER BY p.featured_until DESC, p.avg_rating DESC
		 LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.ProviderDetail, 0, limit)
	for rows.Next() {
		var d model.ProviderDetail
		err := rows.Scan(&d.ID, &d.UserID, &d.Slug, &d.Bio, &d.WhatsappNumber,
			&d.IsVerified, &d.AvgRating, &d.TotalReviews, &d.FeaturedUntil,
			&d.Latitude, &d.Longitude, &d.AddressLabel, &d.ServiceRadiusKm, &d.CreatedAt,
			&d.FullName, &d.AvatarURL, &d.City, &d.Kecamatan, &d.Phone, &d.SuspendedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// SetLocation menyimpan titik lokasi dan radius layanan penyedia.
// Lintang dan bujur nil berarti penyedia mencabut titik lokasinya.
func (r *ProviderRepository) SetLocation(
	ctx context.Context, providerID int64,
	lat, lng *float64, label *string, radiusKm int,
) error {
	const q = `
		UPDATE provider_profiles
		   SET latitude = $2, longitude = $3, address_label = $4, service_radius_km = $5
		 WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, providerID, lat, lng, label, radiusKm)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetDetailBySlug mengambil penyedia berdasarkan alamat publiknya.
func (r *ProviderRepository) GetDetailBySlug(ctx context.Context, slug string) (*model.ProviderDetail, error) {
	return scanProviderDetail(r.db.QueryRow(ctx, providerDetailSelect+` WHERE p.slug = $1`, slug))
}

// SlugDipakai memeriksa ketersediaan slug, mengecualikan satu profil tertentu
// agar penyimpanan ulang profil sendiri tidak dianggap bentrok.
func (r *ProviderRepository) SlugDipakai(ctx context.Context, slug string, kecualiID int64) (bool, error) {
	var ada bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM provider_profiles WHERE slug = $1 AND id <> $2)`,
		slug, kecualiID).Scan(&ada)
	return ada, err
}

// ---------- direktori penyedia ----------

// ProviderFilter adalah parameter halaman daftar penyedia.
type ProviderFilter struct {
	Query     string
	Kecamatan string
	// HanyaTerverifikasi menyaring penyedia yang identitasnya sudah diperiksa.
	HanyaTerverifikasi bool
	// HanyaBerjasa menyaring penyedia yang punya setidaknya satu listing tayang.
	HanyaBerjasa bool
	Sort         string // jasa | rating | terdekat | terbaru
	Limit        int
	Offset       int

	Latitude  *float64
	Longitude *float64
	RadiusKm  int
}

func (f ProviderFilter) PunyaLokasi() bool { return f.Latitude != nil && f.Longitude != nil }

// perluKoordinat menandai filter yang klausa WHERE-nya benar-benar merujuk
// koordinat. Query COUNT tidak punya kolom jarak, jadi koordinat hanya boleh
// ikut dikirim bila ada klausa yang memakainya.
func (f ProviderFilter) perluKoordinat() bool {
	return f.PunyaLokasi() && f.RadiusKm > 0
}

// ProviderCard adalah satu penyedia pada halaman direktori.
type ProviderCard struct {
	model.ProviderDetail
	TotalJasa int      `json:"total_jasa"`
	JarakKm   *float64 `json:"jarak_km,omitempty"`
}

// Menjangkau menandai penyedia yang radius layanannya mencakup titik pencari.
func (c ProviderCard) Menjangkau() bool {
	return c.JarakKm != nil && *c.JarakKm <= float64(c.ServiceRadiusKm)
}

// providerCardSelect mengambil seluruh data kartu penyedia dalam satu query,
// termasuk jumlah listing tayang, tanpa menimbulkan N+1.
func providerCardSelect(distExpr string) string {
	return `
	SELECT p.id, p.user_id, p.slug, p.bio, p.whatsapp_number, p.is_verified,
	       p.avg_rating, p.total_reviews, p.featured_until, p.latitude, p.longitude,
	       p.address_label, p.service_radius_km, p.created_at,
	       u.full_name, u.avatar_url, u.city, u.kecamatan, u.phone, u.suspended_at,
	       COALESCE(sv.jumlah, 0) AS total_jasa,
	       ` + distExpr + ` AS jarak_km
	  FROM provider_profiles p
	  JOIN users u ON u.id = p.user_id
	  LEFT JOIN LATERAL (
	        SELECT COUNT(*) AS jumlah
	          FROM services s
	         WHERE s.provider_id = p.id AND s.status = 'active'
	  ) sv ON TRUE`
}

func buildProviderFilter(f ProviderFilter, args []any) (string, []any) {
	// Penyedia yang ditangguhkan tidak pernah muncul di direktori publik.
	conds := []string{"u.suspended_at IS NULL"}
	add := func(cond string, val any) {
		args = append(args, val)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}

	if kata := pecahKata(f.Query); len(kata) > 0 {
		var cond string
		cond, args = kondisiKataKunci(kata, teksPenyedia, args)
		conds = append(conds, cond)
	}
	if v := strings.TrimSpace(f.Kecamatan); v != "" {
		add("u.kecamatan ILIKE $%d", v)
	}
	if f.HanyaTerverifikasi {
		conds = append(conds, "p.is_verified")
	}
	if f.HanyaBerjasa {
		conds = append(conds, "COALESCE(sv.jumlah, 0) > 0")
	}
	if f.perluKoordinat() {
		args = append(args, float64(f.RadiusKm)*1000)
		n := len(args)
		conds = append(conds, fmt.Sprintf(
			"p.latitude IS NOT NULL"+
				" AND earth_box(ll_to_earth($1, $2), $%d) @> ll_to_earth(p.latitude, p.longitude)"+
				" AND earth_distance(ll_to_earth($1, $2), ll_to_earth(p.latitude, p.longitude)) <= $%[1]d", n))
	}
	return "WHERE " + strings.Join(conds, " AND "), args
}

func providerOrderClause(sort string, punyaLokasi bool) string {
	switch sort {
	case "terdekat":
		if punyaLokasi {
			return "ORDER BY jarak_km ASC NULLS LAST, p.avg_rating DESC"
		}
		return "ORDER BY COALESCE(sv.jumlah, 0) DESC, p.created_at DESC"
	case "rating":
		return "ORDER BY p.avg_rating DESC, p.total_reviews DESC, COALESCE(sv.jumlah, 0) DESC"
	case "terbaru":
		return "ORDER BY p.created_at DESC"
	default:
		// Penyedia pilihan naik ke atas, lalu yang paling banyak menawarkan jasa.
		return "ORDER BY (p.featured_until IS NOT NULL AND p.featured_until > NOW()) DESC," +
			" COALESCE(sv.jumlah, 0) DESC, p.avg_rating DESC"
	}
}

// Search mengembalikan daftar penyedia sesuai filter.
func (r *ProviderRepository) Search(ctx context.Context, f ProviderFilter) ([]ProviderCard, error) {
	var args []any
	distExpr := tanpaJarak
	if f.PunyaLokasi() {
		args = []any{*f.Latitude, *f.Longitude}
		distExpr = ekspresiJarakProvider(1, 2)
	}

	where, args := buildProviderFilter(f, args)
	args = append(args, f.Limit, f.Offset)

	q := fmt.Sprintf("%s %s %s LIMIT $%d OFFSET $%d",
		providerCardSelect(distExpr), where,
		providerOrderClause(f.Sort, f.PunyaLokasi()), len(args)-1, len(args))

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ProviderCard, 0, 20)
	for rows.Next() {
		var c ProviderCard
		err := rows.Scan(&c.ID, &c.UserID, &c.Slug, &c.Bio, &c.WhatsappNumber,
			&c.IsVerified, &c.AvgRating, &c.TotalReviews, &c.FeaturedUntil,
			&c.Latitude, &c.Longitude, &c.AddressLabel, &c.ServiceRadiusKm, &c.CreatedAt,
			&c.FullName, &c.AvatarURL, &c.City, &c.Kecamatan, &c.Phone, &c.SuspendedAt,
			&c.TotalJasa, &c.JarakKm)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *ProviderRepository) CountSearch(ctx context.Context, f ProviderFilter) (int, error) {
	var args []any
	fHitung := f
	if f.perluKoordinat() {
		args = []any{*f.Latitude, *f.Longitude}
	} else {
		fHitung.Latitude, fHitung.Longitude = nil, nil
	}

	where, args := buildProviderFilter(fHitung, args)
	q := `
		SELECT COUNT(*)
		  FROM provider_profiles p
		  JOIN users u ON u.id = p.user_id
		  LEFT JOIN LATERAL (
		        SELECT COUNT(*) AS jumlah
		          FROM services s
		         WHERE s.provider_id = p.id AND s.status = 'active'
		  ) sv ON TRUE ` + where

	var n int
	err := r.db.QueryRow(ctx, q, args...).Scan(&n)
	return n, err
}

// ekspresiJarakProvider menghitung jarak dalam kilometer dari titik acuan
// ke penyedia. earth_distance mengembalikan meter, karena itu dibagi seribu.
func ekspresiJarakProvider(idxLat, idxLng int) string {
	return fmt.Sprintf(
		"CASE WHEN p.latitude IS NULL THEN NULL::double precision"+
			" ELSE earth_distance(ll_to_earth($%d, $%d), ll_to_earth(p.latitude, p.longitude)) / 1000.0 END",
		idxLat, idxLng)
}

// DistinctKecamatanPenyedia menyediakan pilihan lokasi pada filter direktori.
func (r *ProviderRepository) DistinctKecamatanPenyedia(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT u.kecamatan
		  FROM provider_profiles p
		  JOIN users u ON u.id = p.user_id
		 WHERE u.suspended_at IS NULL
		   AND u.kecamatan IS NOT NULL AND u.kecamatan <> ''
		 ORDER BY u.kecamatan`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
