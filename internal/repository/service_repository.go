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

type ServiceRepository struct{ db DBTX }

const serviceColumns = `id, provider_id, category_id, title, description, price_type,
	price_min, price_max, status, featured_until, approved_at, approved_by,
	rejection_reason, submitted_at, created_at, total_kunjungan`

// ServiceFilter adalah parameter halaman pencarian. Nilai kosong berarti filter tidak aktif.
type ServiceFilter struct {
	Query       string
	CategoryIDs []int64
	City        string
	Kecamatan   string
	PriceType   string
	Sort        string // terbaru | rating | termurah | termahal | terdekat
	Limit       int
	Offset      int

	// Titik acuan pencari jasa. Bila terisi, jarak ikut dihitung dan
	// hasil bisa disaring berdasarkan radius.
	Latitude  *float64
	Longitude *float64
	// RadiusKm membatasi hasil ke penyedia dalam radius tersebut. 0 = tanpa batas.
	RadiusKm int
	// OnlyReachable menyaring penyedia yang radius layanannya sendiri
	// benar-benar mencakup titik pencari jasa.
	OnlyReachable bool
}

// PunyaLokasi menandai filter yang membawa titik acuan pencari jasa.
func (f ServiceFilter) PunyaLokasi() bool { return f.Latitude != nil && f.Longitude != nil }

// CacheKey menghasilkan kunci cache Redis yang stabil untuk kombinasi filter ini.
func (f ServiceFilter) CacheKey() string {
	ids := make([]string, 0, len(f.CategoryIDs))
	for _, id := range f.CategoryIDs {
		ids = append(ids, fmt.Sprint(id))
	}
	lokasi := "-"
	if f.PunyaLokasi() {
		// Koordinatnya sudah dibulatkan ke sel ~1 km oleh service layer
		// (lihat service.BulatkanSel); di sini cukup ditulis apa adanya.
		lokasi = fmt.Sprintf("%.2f,%.2f,r%d,j%t", *f.Latitude, *f.Longitude, f.RadiusKm, f.OnlyReachable)
	}
	return fmt.Sprintf("search:q=%s|cat=%s|city=%s|kec=%s|pt=%s|sort=%s|loc=%s|l=%d|o=%d",
		strings.Join(pecahKata(f.Query), " "), strings.Join(ids, ","), strings.ToLower(f.City),
		strings.ToLower(f.Kecamatan), f.PriceType, f.Sort, lokasi, f.Limit, f.Offset)
}

func scanServiceRow(row pgx.Row) (*model.Service, error) {
	var s model.Service
	err := row.Scan(&s.ID, &s.ProviderID, &s.CategoryID, &s.Title, &s.Description,
		&s.PriceType, &s.PriceMin, &s.PriceMax, &s.Status, &s.FeaturedUntil,
		&s.ApprovedAt, &s.ApprovedBy, &s.RejectionReason, &s.SubmittedAt, &s.CreatedAt, &s.TotalKunjungan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *ServiceRepository) Create(ctx context.Context, s *model.Service) error {
	const q = `
		INSERT INTO services (provider_id, category_id, title, description, price_type,
		                      price_min, price_max, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, submitted_at, created_at`
	return r.db.QueryRow(ctx, q, s.ProviderID, s.CategoryID, s.Title, s.Description,
		s.PriceType, s.PriceMin, s.PriceMax, s.Status).
		Scan(&s.ID, &s.SubmittedAt, &s.CreatedAt)
}

func (r *ServiceRepository) Update(ctx context.Context, s *model.Service) error {
	const q = `
		UPDATE services
		   SET category_id = $2, title = $3, description = $4, price_type = $5,
		       price_min = $6, price_max = $7, status = $8
		 WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, s.ID, s.CategoryID, s.Title, s.Description,
		s.PriceType, s.PriceMin, s.PriceMax, s.Status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ServiceRepository) GetByID(ctx context.Context, id int64) (*model.Service, error) {
	return scanServiceRow(r.db.QueryRow(ctx, `SELECT `+serviceColumns+` FROM services WHERE id = $1`, id))
}

func (r *ServiceRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM services WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// cardSelect mengambil seluruh data kartu listing dalam satu query.
// Foto cover diambil lewat LATERAL supaya tidak menimbulkan N+1 query.
// distExpr disisipkan sebagai kolom jarak: berisi perhitungan bila pencari
// jasa membawa titik lokasi, atau NULL bila tidak.
func cardSelect(distExpr string) string {
	return `
	SELECT s.id, s.provider_id, s.category_id, s.title, s.description, s.price_type,
	       s.price_min, s.price_max, s.status, s.featured_until, s.approved_at,
	       s.approved_by, s.rejection_reason, s.submitted_at, s.created_at, s.total_kunjungan,
	       img.image_url, c.name, c.slug,
	       u.full_name, u.city, u.kecamatan,
	       p.avg_rating, p.total_reviews, p.is_verified, p.featured_until,
	       p.latitude, p.longitude, p.service_radius_km,
	       ` + distExpr + ` AS jarak_km
	  FROM services s
	  JOIN categories c         ON c.id = s.category_id
	  LEFT JOIN categories pc   ON pc.id = c.parent_id
	  JOIN provider_profiles p  ON p.id = s.provider_id
	  JOIN users u              ON u.id = p.user_id
	  LEFT JOIN LATERAL (
	        -- Kartu memakai thumbnail; gambar yang diunggah sebelum pemrosesan
	        -- otomatis ada belum punya thumbnail, jadi jatuh kembali ke aslinya.
	        SELECT COALESCE(si.thumb_url, si.image_url) AS image_url
	          FROM service_images si
	         WHERE si.service_id = s.id
	         ORDER BY si.sort_order, si.id
	         LIMIT 1
	  ) img ON TRUE`
}

// tanpaJarak dipakai saat pencari jasa tidak membawa titik lokasi.
const tanpaJarak = "NULL::double precision"

// ekspresiJarak menghitung jarak dalam kilometer dari titik acuan ke penyedia.
// earth_distance mengembalikan meter, karena itu dibagi seribu.
func ekspresiJarak(idxLat, idxLng int) string {
	return fmt.Sprintf(
		"CASE WHEN p.latitude IS NULL THEN NULL::double precision"+
			" ELSE earth_distance(ll_to_earth($%d, $%d), ll_to_earth(p.latitude, p.longitude)) / 1000.0 END",
		idxLat, idxLng)
}

func (r *ServiceRepository) scanCards(ctx context.Context, q string, args ...any) ([]model.ServiceCard, error) {
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.ServiceCard, 0, 20)
	for rows.Next() {
		var c model.ServiceCard
		err := rows.Scan(&c.ID, &c.ProviderID, &c.CategoryID, &c.Title, &c.Description,
			&c.PriceType, &c.PriceMin, &c.PriceMax, &c.Status, &c.FeaturedUntil,
			&c.ApprovedAt, &c.ApprovedBy, &c.RejectionReason, &c.SubmittedAt, &c.CreatedAt, &c.TotalKunjungan,
			&c.CoverImage, &c.CategoryName, &c.CategorySlug,
			&c.ProviderName, &c.ProviderCity, &c.ProviderKec,
			&c.AvgRating, &c.TotalReviews, &c.IsVerified, &c.ProviderFeaturedUntil,
			&c.ProviderLat, &c.ProviderLng, &c.ProviderRadiusKm, &c.JarakKm)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// buildFilter menyusun klausa WHERE dinamis beserta argumennya.
// args yang masuk sudah berisi argumen sebelumnya (lintang/bujur untuk kolom
// jarak), sehingga penomoran placeholder tetap berurutan.
func buildFilter(f ServiceFilter, args []any) (where string, outArgs []any) {
	outArgs = args
	// Hanya listing yang benar-benar tayang dan penyedianya tidak ditangguhkan
	// yang boleh muncul di halaman publik.
	conds := []string{"s.status = 'active'", "u.suspended_at IS NULL"}
	add := func(cond string, val any) {
		outArgs = append(outArgs, val)
		conds = append(conds, fmt.Sprintf(cond, len(outArgs)))
	}

	// Kata kunci diletakkan paling awal supaya nomor argumennya bisa dihitung
	// ulang oleh relevansiPencarian untuk ORDER BY (lihat katakunci.go).
	if kata := pecahKata(f.Query); len(kata) > 0 {
		var cond string
		cond, outArgs = kondisiKataKunci(kata, teksListing, outArgs)
		conds = append(conds, cond)
	}
	if len(f.CategoryIDs) > 0 {
		add("s.category_id = ANY($%d)", f.CategoryIDs)
	}
	if v := strings.TrimSpace(f.City); v != "" {
		add("u.city ILIKE $%d", v)
	}
	if v := strings.TrimSpace(f.Kecamatan); v != "" {
		add("u.kecamatan ILIKE $%d", v)
	}
	if v := strings.TrimSpace(f.PriceType); v != "" {
		add("s.price_type = $%d", v)
	}

	if f.PunyaLokasi() {
		// Lintang dan bujur selalu menempati $1 dan $2 saat lokasi tersedia.
		if f.RadiusKm > 0 {
			outArgs = append(outArgs, float64(f.RadiusKm)*1000)
			n := len(outArgs)
			// earth_box memanfaatkan indeks GiST untuk membuang kandidat jauh
			// lebih dulu; earth_distance kemudian menyaring sudut kotaknya.
			conds = append(conds, fmt.Sprintf(
				"p.latitude IS NOT NULL"+
					" AND earth_box(ll_to_earth($1, $2), $%d) @> ll_to_earth(p.latitude, p.longitude)"+
					" AND earth_distance(ll_to_earth($1, $2), ll_to_earth(p.latitude, p.longitude)) <= $%[1]d", n))
		}
		if f.OnlyReachable {
			// Penyedia hanya ditampilkan bila radius layanannya sendiri
			// benar-benar mencapai titik pencari jasa.
			conds = append(conds,
				"p.latitude IS NOT NULL"+
					" AND earth_distance(ll_to_earth($1, $2), ll_to_earth(p.latitude, p.longitude))"+
					" <= p.service_radius_km * 1000")
		}
	}
	return "WHERE " + strings.Join(conds, " AND "), outArgs
}

// relevansiPencarian menyusun skor relevansi untuk filter ini. mulaiArgs
// adalah panjang argumen sebelum buildFilter dipanggil — kata kunci selalu
// jadi argumen pertama yang ditambahkan buildFilter.
func relevansiPencarian(f ServiceFilter, mulaiArgs int) string {
	return ekspresiRelevansi(pecahKata(f.Query), teksRelevansiListing, mulaiArgs+1)
}

func orderClause(sort string, punyaLokasi bool, relevansi string) string {
	switch sort {
	case "terdekat":
		if punyaLokasi {
			// Penyedia tanpa titik lokasi diletakkan paling belakang.
			return "ORDER BY jarak_km ASC NULLS LAST, s.created_at DESC"
		}
		return "ORDER BY s.created_at DESC"
	case "rating":
		return "ORDER BY p.avg_rating DESC, p.total_reviews DESC, s.created_at DESC"
	case "termurah":
		return "ORDER BY s.price_min ASC NULLS LAST, s.created_at DESC"
	case "termahal":
		return "ORDER BY COALESCE(s.price_max, s.price_min) DESC NULLS LAST, s.created_at DESC"
	default:
		// Listing berbayar (featured_until masih berlaku) selalu naik ke atas.
		// Di antara yang setara, hasil yang paling mirip kata kuncinya lebih
		// dulu; tanpa kata kunci, yang terbaru.
		if relevansi != "" {
			return "ORDER BY (s.featured_until IS NOT NULL AND s.featured_until > NOW()) DESC, " +
				relevansi + " DESC, s.created_at DESC"
		}
		return "ORDER BY (s.featured_until IS NOT NULL AND s.featured_until > NOW()) DESC, s.created_at DESC"
	}
}

// perluKoordinat menandai filter yang klausa WHERE-nya benar-benar memakai
// lintang dan bujur. Query COUNT tidak punya kolom jarak, jadi koordinatnya
// hanya boleh ikut dikirim bila ada klausa radius atau keterjangkauan —
// Postgres menolak argumen yang tidak dirujuk placeholder mana pun.
func (f ServiceFilter) perluKoordinat() bool {
	return f.PunyaLokasi() && (f.RadiusKm > 0 || f.OnlyReachable)
}

// mulaiArgs menyiapkan argumen awal dan ekspresi jarak.
// Saat lokasi tersedia, lintang dan bujur selalu menempati $1 dan $2 sehingga
// klausa radius bisa mengacu ke keduanya tanpa perlu menghitung ulang indeks.
func mulaiArgs(f ServiceFilter) ([]any, string) {
	if !f.PunyaLokasi() {
		return nil, tanpaJarak
	}
	return []any{*f.Latitude, *f.Longitude}, ekspresiJarak(1, 2)
}

// mulaiArgsHitung menyiapkan argumen untuk query COUNT, yang tidak memiliki
// kolom jarak sehingga koordinat hanya disertakan bila memang dirujuk.
func mulaiArgsHitung(f ServiceFilter) []any {
	if !f.perluKoordinat() {
		return nil
	}
	return []any{*f.Latitude, *f.Longitude}
}

// Search mengembalikan kartu listing sesuai filter, lengkap dengan jaraknya
// dari titik pencari jasa bila titik itu tersedia.
func (r *ServiceRepository) Search(ctx context.Context, f ServiceFilter) ([]model.ServiceCard, error) {
	args, distExpr := mulaiArgs(f)
	relevansi := relevansiPencarian(f, len(args))
	where, args := buildFilter(f, args)
	args = append(args, f.Limit, f.Offset)

	q := fmt.Sprintf("%s %s %s LIMIT $%d OFFSET $%d",
		cardSelect(distExpr), where, orderClause(f.Sort, f.PunyaLokasi(), relevansi), len(args)-1, len(args))
	return r.scanCards(ctx, q, args...)
}

// CountSearch menghitung total hasil untuk paginasi.
func (r *ServiceRepository) CountSearch(ctx context.Context, f ServiceFilter) (int, error) {
	// Klausa lokasi dibangun dari filter yang sama, tapi koordinatnya hanya
	// ikut bila benar-benar dipakai menyaring.
	fHitung := f
	if !f.perluKoordinat() {
		fHitung.Latitude, fHitung.Longitude = nil, nil
	}
	where, args := buildFilter(fHitung, mulaiArgsHitung(f))

	q := `
		SELECT COUNT(*)
		  FROM services s
		  JOIN categories c        ON c.id = s.category_id
		  LEFT JOIN categories pc  ON pc.id = c.parent_id
		  JOIN provider_profiles p ON p.id = s.provider_id
		  JOIN users u             ON u.id = p.user_id ` + where
	var n int
	err := r.db.QueryRow(ctx, q, args...).Scan(&n)
	return n, err
}

// ListByProvider mengembalikan listing milik satu provider.
// includeInactive dipakai di dashboard pemilik; halaman publik hanya melihat yang aktif.
func (r *ServiceRepository) ListByProvider(ctx context.Context, providerID int64, includeInactive bool) ([]model.ServiceCard, error) {
	cond := "WHERE s.provider_id = $1"
	if !includeInactive {
		cond += " AND s.status = 'active'"
	}
	return r.scanCards(ctx, cardSelect(tanpaJarak)+" "+cond+" ORDER BY s.created_at DESC", providerID)
}

// GetDetail mengambil satu listing lengkap dengan foto, kategori, dan providernya.
func (r *ServiceRepository) GetDetail(ctx context.Context, id int64) (*model.ServiceDetail, error) {
	// Blok kolom penyedia diambil dari providerDetailKolom, bukan disalin,
	// supaya penambahan kolom tidak perlu diingat di dua tempat.
	const q = `
		SELECT s.id, s.provider_id, s.category_id, s.title, s.description, s.price_type,
		       s.price_min, s.price_max, s.status, s.featured_until, s.approved_at,
		       s.approved_by, s.rejection_reason, s.submitted_at, s.created_at, s.total_kunjungan,
		       c.id, c.name, c.slug, c.parent_id, c.icon,
		       ` + providerDetailKolom + `
		  FROM services s
		  JOIN categories c        ON c.id = s.category_id
		  JOIN provider_profiles p ON p.id = s.provider_id
		  JOIN users u             ON u.id = p.user_id
		 WHERE s.id = $1`

	var d model.ServiceDetail
	target := append([]any{
		&d.ID, &d.ProviderID, &d.CategoryID, &d.Title, &d.Description, &d.PriceType,
		&d.PriceMin, &d.PriceMax, &d.Status, &d.FeaturedUntil, &d.ApprovedAt,
		&d.ApprovedBy, &d.RejectionReason, &d.SubmittedAt, &d.CreatedAt, &d.TotalKunjungan,
		&d.Category.ID, &d.Category.Name, &d.Category.Slug, &d.Category.ParentID, &d.Category.Icon,
	}, targetProviderDetail(&d.Provider)...)

	err := r.db.QueryRow(ctx, q, id).Scan(target...)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	images, err := r.ListImages(ctx, id)
	if err != nil {
		return nil, err
	}
	d.Images = images
	return &d, nil
}

// ---------- foto listing ----------

func (r *ServiceRepository) ListImages(ctx context.Context, serviceID int64) ([]model.ServiceImage, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, service_id, image_url, thumb_url, width, height, bytes, sort_order
		  FROM service_images
		 WHERE service_id = $1
		 ORDER BY sort_order, id`, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.ServiceImage, 0, 6)
	for rows.Next() {
		var im model.ServiceImage
		if err := rows.Scan(&im.ID, &im.ServiceID, &im.ImageURL, &im.ThumbURL,
			&im.Width, &im.Height, &im.Bytes, &im.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, im)
	}
	return out, rows.Err()
}

func (r *ServiceRepository) AddImage(ctx context.Context, im *model.ServiceImage) error {
	const q = `
		INSERT INTO service_images (service_id, image_url, thumb_url, width, height, bytes, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`
	return r.db.QueryRow(ctx, q, im.ServiceID, im.ImageURL, im.ThumbURL,
		im.Width, im.Height, im.Bytes, im.SortOrder).Scan(&im.ID)
}

// GetImage dipakai sebelum menghapus foto, untuk memastikan kepemilikan dan
// mendapatkan path file yang perlu ikut dihapus dari disk.
func (r *ServiceRepository) GetImage(ctx context.Context, imageID int64) (*model.ServiceImage, error) {
	var im model.ServiceImage
	err := r.db.QueryRow(ctx, `
		SELECT id, service_id, image_url, thumb_url, width, height, bytes, sort_order
		  FROM service_images WHERE id = $1`, imageID).
		Scan(&im.ID, &im.ServiceID, &im.ImageURL, &im.ThumbURL,
			&im.Width, &im.Height, &im.Bytes, &im.SortOrder)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &im, nil
}

func (r *ServiceRepository) DeleteImage(ctx context.Context, imageID int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM service_images WHERE id = $1`, imageID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ServiceRepository) CountImages(ctx context.Context, serviceID int64) (int, error) {
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM service_images WHERE service_id = $1`, serviceID).Scan(&n)
	return n, err
}

// NextImageSortOrder mengembalikan urutan berikutnya untuk foto baru.
func (r *ServiceRepository) NextImageSortOrder(ctx context.Context, serviceID int64) (int, error) {
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(MAX(sort_order) + 1, 0) FROM service_images WHERE service_id = $1`,
		serviceID).Scan(&n)
	return n, err
}

// DistinctKecamatan menyediakan pilihan lokasi pada filter pencarian,
// diambil dari lokasi provider yang benar-benar punya listing aktif.
func (r *ServiceRepository) DistinctKecamatan(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT u.kecamatan
		  FROM services s
		  JOIN provider_profiles p ON p.id = s.provider_id
		  JOIN users u             ON u.id = p.user_id
		 WHERE s.status = 'active' AND u.kecamatan IS NOT NULL AND u.kecamatan <> ''
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

// ---------- operasi admin ----------

// AdminServiceFilter adalah parameter daftar listing di panel admin.
// Berbeda dengan pencarian publik, filter ini juga bisa menampilkan listing
// nonaktif dan listing milik akun yang ditangguhkan.
type AdminServiceFilter struct {
	Query     string
	Status    string // active | inactive
	Featured  string // ya | tidak
	TanpaFoto bool
	// Antrean membatasi hasil ke listing yang menunggu peninjauan.
	Antrean    bool
	CategoryID int64
	Sort       string
	Limit      int
	Offset     int
}

func buildAdminServiceFilter(f AdminServiceFilter) (string, []any) {
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
			"(s.title ILIKE $%d OR u.full_name ILIKE $%[1]d OR c.name ILIKE $%[1]d)", n))
	}
	if v := strings.TrimSpace(f.Status); v != "" {
		add("s.status = $%d", v)
	}
	if f.Antrean {
		conds = append(conds, "s.status = 'pending'")
	}
	switch f.Featured {
	case "ya":
		conds = append(conds, "s.featured_until > NOW()")
	case "tidak":
		conds = append(conds, "(s.featured_until IS NULL OR s.featured_until <= NOW())")
	}
	if f.TanpaFoto {
		conds = append(conds, "NOT EXISTS (SELECT 1 FROM service_images si WHERE si.service_id = s.id)")
	}
	if f.CategoryID > 0 {
		add("(s.category_id = $%d OR c.parent_id = $%[1]d)", f.CategoryID)
	}

	if len(conds) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conds, " AND "), args
}

func adminServiceOrder(sort string) string {
	switch sort {
	case "antrean":
		// Antrean dikerjakan dari yang paling lama menunggu.
		return "ORDER BY s.submitted_at ASC"
	case "terlama":
		return "ORDER BY s.created_at ASC"
	case "termahal":
		return "ORDER BY COALESCE(s.price_max, s.price_min) DESC NULLS LAST"
	case "penyedia":
		return "ORDER BY u.full_name ASC, s.created_at DESC"
	default:
		return "ORDER BY s.created_at DESC"
	}
}

// AdminList mengembalikan kartu listing tanpa menyaring status maupun
// akun yang ditangguhkan, karena justru itulah yang perlu dimoderasi.
func (r *ServiceRepository) AdminList(ctx context.Context, f AdminServiceFilter) ([]model.ServiceCard, error) {
	where, args := buildAdminServiceFilter(f)
	args = append(args, f.Limit, f.Offset)
	q := fmt.Sprintf("%s %s %s LIMIT $%d OFFSET $%d",
		cardSelect(tanpaJarak), where, adminServiceOrder(f.Sort), len(args)-1, len(args))
	return r.scanCards(ctx, q, args...)
}

func (r *ServiceRepository) AdminCount(ctx context.Context, f AdminServiceFilter) (int, error) {
	where, args := buildAdminServiceFilter(f)
	q := `
		SELECT COUNT(*)
		  FROM services s
		  JOIN categories c        ON c.id = s.category_id
		  LEFT JOIN categories pc  ON pc.id = c.parent_id
		  JOIN provider_profiles p ON p.id = s.provider_id
		  JOIN users u             ON u.id = p.user_id ` + where
	var n int
	err := r.db.QueryRow(ctx, q, args...).Scan(&n)
	return n, err
}

// SetStatus dipakai admin untuk menonaktifkan atau mengaktifkan listing
// tanpa memeriksa kepemilikan.
func (r *ServiceRepository) SetStatus(ctx context.Context, serviceID int64, status model.ServiceStatus) error {
	tag, err := r.db.Exec(ctx, `UPDATE services SET status = $2 WHERE id = $1`, serviceID, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetFeatured menyorot listing sampai waktu tertentu; nil mencabut sorotan.
func (r *ServiceRepository) SetFeatured(ctx context.Context, serviceID int64, until *time.Time) error {
	tag, err := r.db.Exec(ctx, `UPDATE services SET featured_until = $2 WHERE id = $1`, serviceID, until)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------- alur persetujuan ----------

// Approve menandai listing sebagai disetujui dan menayangkannya.
func (r *ServiceRepository) Approve(ctx context.Context, serviceID, adminID int64) error {
	const q = `
		UPDATE services
		   SET status = 'active', approved_at = NOW(), approved_by = $2, rejection_reason = NULL
		 WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, serviceID, adminID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Reject menolak listing dengan alasan yang bisa dibaca provider.
func (r *ServiceRepository) Reject(ctx context.Context, serviceID, adminID int64, reason string) error {
	const q = `
		UPDATE services
		   SET status = 'rejected', approved_at = NULL, approved_by = $2, rejection_reason = $3
		 WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, serviceID, adminID, strings.TrimSpace(reason))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SubmitForReview mengembalikan listing ke antrean peninjauan.
// Dipanggil saat provider mengubah bagian penting dari listing yang sudah
// tayang, dan saat provider mengajukan ulang listing yang pernah ditolak.
func (r *ServiceRepository) SubmitForReview(ctx context.Context, serviceID int64) error {
	const q = `
		UPDATE services
		   SET status = 'pending', submitted_at = NOW(), approved_at = NULL, rejection_reason = NULL
		 WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, serviceID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CountPending menghitung isi antrean peninjauan, untuk badge di panel admin.
func (r *ServiceRepository) CountPending(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM services WHERE status = 'pending'`).Scan(&n)
	return n, err
}
