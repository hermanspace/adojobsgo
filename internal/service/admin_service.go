package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/hermansyah/adojobsid/internal/database"
	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/repository"
	"github.com/hermansyah/adojobsid/internal/session"
	"github.com/hermansyah/adojobsid/internal/validator"
)

// AdminService memuat seluruh operasi panel admin.
// Setiap operasi yang mengubah data juga membersihkan cache pencarian, karena
// penangguhan, penonaktifan, dan sorotan semuanya mengubah isi halaman publik.
type AdminService struct {
	repos    *repository.Repositories
	cache    *database.Cache
	sessions *session.Store
	settings *SettingsService
	promosi  *PromosiService
}

// ---------- dasbor ----------

type Dashboard struct {
	Stats    *repository.Stats            `json:"stats"`
	Kategori []repository.KategoriTeratas `json:"kategori_teratas"`
	Terbaru  []repository.AdminUserRow    `json:"pengguna_terbaru"`
	Promosi  *repository.StatistikPromosi `json:"promosi"`
}

func (s *AdminService) Dashboard(ctx context.Context) (*Dashboard, error) {
	stats, err := s.repos.Admin.Stats(ctx)
	if err != nil {
		return nil, Internal(err)
	}
	kategori, err := s.repos.Admin.KategoriTeratas(ctx, 8)
	if err != nil {
		return nil, Internal(err)
	}
	terbaru, err := s.repos.Admin.ListUsers(ctx, repository.UserFilter{Limit: 5, Sort: "terbaru"})
	if err != nil {
		return nil, Internal(err)
	}
	return &Dashboard{Stats: stats, Kategori: kategori, Terbaru: terbaru, Promosi: s.promosi.Statistik(ctx)}, nil
}

// ---------- pengguna ----------

type UserListResult struct {
	Items      []repository.AdminUserRow `json:"items"`
	Total      int                       `json:"total"`
	Page       int                       `json:"page"`
	PerPage    int                       `json:"per_page"`
	TotalPages int                       `json:"total_pages"`
}

const adminPerPage = 20

func (s *AdminService) ListUsers(ctx context.Context, f repository.UserFilter, page int) (*UserListResult, error) {
	if page < 1 {
		page = 1
	}
	f.Limit = adminPerPage
	f.Offset = (page - 1) * adminPerPage

	items, err := s.repos.Admin.ListUsers(ctx, f)
	if err != nil {
		return nil, Internal(err)
	}
	total, err := s.repos.Admin.CountUsers(ctx, f)
	if err != nil {
		return nil, Internal(err)
	}
	return &UserListResult{
		Items:      items,
		Total:      total,
		Page:       page,
		PerPage:    adminPerPage,
		TotalPages: (total + adminPerPage - 1) / adminPerPage,
	}, nil
}

func (s *AdminService) GetUser(ctx context.Context, userID int64) (*repository.AdminUserRow, error) {
	row, err := s.repos.Admin.GetUserRow(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, NotFound("Pengguna tidak ditemukan.")
		}
		return nil, Internal(err)
	}
	return row, nil
}

// Suspend menangguhkan akun dan langsung mencabut seluruh sesi aktifnya.
func (s *AdminService) Suspend(ctx context.Context, adminID, userID int64, reason string) error {
	if adminID == userID {
		return InvalidMsg("Anda tidak dapat menangguhkan akun Anda sendiri.")
	}
	reason = strings.TrimSpace(reason)
	if len([]rune(reason)) < 5 {
		errs := validator.New()
		errs.Add("reason", "Tuliskan alasan penangguhan, minimal 5 karakter.")
		return Invalid(errs)
	}

	target, err := s.GetUser(ctx, userID)
	if err != nil {
		return err
	}
	// Admin terakhir yang masih aktif tidak boleh ditangguhkan, agar panel
	// tidak menjadi tidak bisa diakses siapa pun.
	if target.Role == model.RoleAdmin {
		lain, err := s.repos.User.CountAdmins(ctx, userID)
		if err != nil {
			return Internal(err)
		}
		if lain == 0 {
			return InvalidMsg("Ini satu-satunya admin aktif. Tunjuk admin lain lebih dulu.")
		}
	}

	if err := s.repos.User.Suspend(ctx, userID, reason); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return NotFound("Pengguna tidak ditemukan atau sudah ditangguhkan.")
		}
		return Internal(err)
	}
	// Sesi yang sedang berjalan ikut dicabut; tanpa ini pengguna yang sudah
	// masuk tetap bisa memakai aplikasi sampai cookie-nya kedaluwarsa.
	if s.sessions != nil {
		_ = s.sessions.DestroyAllForUser(ctx, userID)
	}
	invalidateSearchCache(s.cache)
	return nil
}

func (s *AdminService) Reactivate(ctx context.Context, userID int64) error {
	if err := s.repos.User.Reactivate(ctx, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return NotFound("Pengguna tidak ditemukan atau sudah aktif.")
		}
		return Internal(err)
	}
	invalidateSearchCache(s.cache)
	return nil
}

// SetRole mengubah peran akun, dengan penjagaan agar selalu tersisa
// setidaknya satu admin aktif.
func (s *AdminService) SetRole(ctx context.Context, adminID, userID int64, role model.Role) error {
	if !role.Valid() {
		return InvalidMsg("Peran tidak dikenal.")
	}
	if adminID == userID && role != model.RoleAdmin {
		return InvalidMsg("Anda tidak dapat mencabut peran admin Anda sendiri.")
	}
	if role == model.RoleUser {
		lain, err := s.repos.User.CountAdmins(ctx, userID)
		if err != nil {
			return Internal(err)
		}
		if lain == 0 {
			return InvalidMsg("Ini satu-satunya admin aktif. Tunjuk admin lain lebih dulu.")
		}
	}

	target, err := s.GetUser(ctx, userID)
	if err != nil {
		return err
	}
	if role == model.RoleAdmin && target.IsSuspended() {
		return InvalidMsg("Aktifkan kembali akun ini sebelum menjadikannya admin.")
	}

	if err := s.repos.User.SetRole(ctx, userID, role); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return NotFound("Pengguna tidak ditemukan.")
		}
		return Internal(err)
	}
	// Sesi dicabut supaya perubahan peran berlaku pada permintaan berikutnya,
	// bukan menunggu pengguna keluar-masuk sendiri.
	if s.sessions != nil {
		_ = s.sessions.DestroyAllForUser(ctx, userID)
	}
	return nil
}

// ---------- penyedia ----------

func (s *AdminService) SetVerified(ctx context.Context, providerID int64, verified bool) error {
	if err := s.repos.Provider.SetVerified(ctx, providerID, verified); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return NotFound("Penyedia tidak ditemukan.")
		}
		return Internal(err)
	}
	invalidateSearchCache(s.cache)
	return nil
}

// DurasiSorot adalah pilihan lama tayang untuk penyedia dan listing pilihan.
var DurasiSorot = []struct {
	Label string
	Hari  int
}{
	{Label: "7 hari", Hari: 7},
	{Label: "14 hari", Hari: 14},
	{Label: "30 hari", Hari: 30},
	{Label: "90 hari", Hari: 90},
}

// hitungBatasSorot menerjemahkan jumlah hari menjadi waktu berakhirnya sorotan.
// Nilai 0 berarti sorotan dicabut.
func hitungBatasSorot(hari int) (*time.Time, error) {
	if hari == 0 {
		return nil, nil
	}
	for _, d := range DurasiSorot {
		if d.Hari == hari {
			t := time.Now().AddDate(0, 0, hari)
			return &t, nil
		}
	}
	return nil, fmt.Errorf("durasi %d hari tidak tersedia", hari)
}

// SetProviderFeatured kini dicatat sebagai promosi bersumber admin; durasi
// yang diterima tetap yang ada di menu (hitungBatasSorot memeriksanya).
func (s *AdminService) SetProviderFeatured(ctx context.Context, adminID, providerID int64, hari int) error {
	if _, err := hitungBatasSorot(hari); err != nil {
		return InvalidMsg("Durasi sorotan tidak tersedia.")
	}
	return s.promosi.TetapkanAdmin(ctx, adminID, model.PromosiPenyedia, providerID, hari)
}

func (s *AdminService) ListFeaturedProviders(ctx context.Context, limit int) ([]model.ProviderDetail, error) {
	return database.Remember(ctx, s.cache, fmt.Sprintf("filter:provider-pilihan:%d", limit), 0,
		func() ([]model.ProviderDetail, error) {
			return s.repos.Provider.ListFeatured(ctx, limit)
		})
}

// ---------- listing ----------

type ServiceListResult struct {
	Items      []model.ServiceCard `json:"items"`
	Total      int                 `json:"total"`
	Page       int                 `json:"page"`
	PerPage    int                 `json:"per_page"`
	TotalPages int                 `json:"total_pages"`
}

func (s *AdminService) ListServices(ctx context.Context, f repository.AdminServiceFilter, page int) (*ServiceListResult, error) {
	if page < 1 {
		page = 1
	}
	f.Limit = adminPerPage
	f.Offset = (page - 1) * adminPerPage

	items, err := s.repos.Service.AdminList(ctx, f)
	if err != nil {
		return nil, Internal(err)
	}
	total, err := s.repos.Service.AdminCount(ctx, f)
	if err != nil {
		return nil, Internal(err)
	}
	return &ServiceListResult{
		Items:      items,
		Total:      total,
		Page:       page,
		PerPage:    adminPerPage,
		TotalPages: (total + adminPerPage - 1) / adminPerPage,
	}, nil
}

func (s *AdminService) SetServiceStatus(ctx context.Context, serviceID int64, status model.ServiceStatus) error {
	if !status.Valid() {
		return InvalidMsg("Status listing tidak dikenal.")
	}
	if err := s.repos.Service.SetStatus(ctx, serviceID, status); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return NotFound("Listing tidak ditemukan.")
		}
		return Internal(err)
	}
	invalidateSearchCache(s.cache)
	return nil
}

func (s *AdminService) SetServiceFeatured(ctx context.Context, adminID, serviceID int64, hari int) error {
	if _, err := hitungBatasSorot(hari); err != nil {
		return InvalidMsg("Durasi sorotan tidak tersedia.")
	}
	return s.promosi.TetapkanAdmin(ctx, adminID, model.PromosiSorotan, serviceID, hari)
}

// ---------- kategori ----------

func (s *AdminService) ListCategories(ctx context.Context) ([]repository.CategoryRow, error) {
	rows, err := s.repos.Category.ListWithUsage(ctx)
	if err != nil {
		return nil, Internal(err)
	}
	return rows, nil
}

type CategoryInput struct {
	Nama     string
	Slug     string
	ParentID int64
	Icon     string
}

// IkonKategori adalah pilihan ikon yang tersedia untuk kategori.
// Nilainya harus cocok dengan nama ikon di components.Icon.
var IkonKategori = []struct {
	Nama  string
	Label string
}{
	{"petir", "Listrik & elektronik"},
	{"rumah-bangun", "Bangunan"},
	{"kunci-inggris", "Perbaikan & sanitasi"},
	{"sapu", "Kebersihan"},
	{"pesta", "Acara"},
	{"kamera", "Dokumentasi"},
	{"truk", "Kendaraan & angkutan"},
	{"laptop", "Digital"},
	{"gunting", "Perawatan diri"},
	{"buku", "Pendidikan"},
	{"tambah", "Umum"},
}

func ikonValid(nama string) bool {
	for _, i := range IkonKategori {
		if i.Nama == nama {
			return true
		}
	}
	return false
}

func (s *AdminService) validateCategory(ctx context.Context, in CategoryInput, currentID int64) (validator.Errors, *model.Category) {
	errs := validator.New()

	nama := errs.Required("nama", "Nama kategori", in.Nama)
	errs.Length("nama", "Nama kategori", nama, 3, 80)

	slug := strings.TrimSpace(in.Slug)
	if slug == "" {
		slug = validator.Slugify(nama)
	} else {
		slug = validator.Slugify(slug)
	}
	if slug == "" {
		errs.Add("slug", "Slug tidak boleh kosong.")
	}

	// Slug adalah kunci unik; tabrakan pernah menyebabkan satu kategori menimpa
	// induknya sendiri, jadi bentrokan ditolak terang-terangan di sini.
	if slug != "" {
		if existing, err := s.repos.Category.GetBySlug(ctx, slug); err == nil && existing.ID != currentID {
			errs.Add("slug", fmt.Sprintf("Slug %q sudah dipakai kategori %q.", slug, existing.Name))
		}
	}

	icon := strings.TrimSpace(in.Icon)
	if icon == "" {
		icon = "tambah"
	}
	if !ikonValid(icon) {
		errs.Add("icon", "Ikon tidak tersedia.")
	}

	var parentID *int64
	if in.ParentID > 0 {
		if in.ParentID == currentID {
			errs.Add("parent_id", "Kategori tidak boleh menjadi induk bagi dirinya sendiri.")
		} else {
			parent, err := s.repos.Category.GetByID(ctx, in.ParentID)
			if err != nil {
				errs.Add("parent_id", "Kategori induk tidak ditemukan.")
			} else if parent.ParentID != nil {
				// Hierarki sengaja dibatasi dua tingkat agar navigasi dan
				// penelusuran sub-kategori tetap sederhana.
				errs.Add("parent_id", "Kategori hanya boleh dua tingkat. Pilih kategori induk utama.")
			} else {
				id := parent.ID
				parentID = &id
			}
		}
	}

	if errs.Any() {
		return errs, nil
	}
	return errs, &model.Category{
		ID:       currentID,
		Name:     nama,
		Slug:     slug,
		ParentID: parentID,
		Icon:     &icon,
	}
}

func (s *AdminService) CreateCategory(ctx context.Context, in CategoryInput) (*model.Category, error) {
	errs, c := s.validateCategory(ctx, in, 0)
	if errs.Any() {
		return nil, Invalid(errs)
	}
	if err := s.repos.Category.Create(ctx, c); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			errs.Add("slug", "Slug ini sudah dipakai kategori lain.")
			return nil, Invalid(errs)
		}
		return nil, Internal(err)
	}
	s.invalidateCategoryCache()
	return c, nil
}

func (s *AdminService) UpdateCategory(ctx context.Context, id int64, in CategoryInput) (*model.Category, error) {
	errs, c := s.validateCategory(ctx, in, id)
	if errs.Any() {
		return nil, Invalid(errs)
	}
	if err := s.repos.Category.Update(ctx, c); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			errs.Add("slug", "Slug ini sudah dipakai kategori lain.")
			return nil, Invalid(errs)
		}
		if errors.Is(err, repository.ErrNotFound) {
			return nil, NotFound("Kategori tidak ditemukan.")
		}
		return nil, Internal(err)
	}
	s.invalidateCategoryCache()
	return c, nil
}

func (s *AdminService) DeleteCategory(ctx context.Context, id int64) error {
	listing, anak, err := s.repos.Category.CountUsage(ctx, id)
	if err != nil {
		return Internal(err)
	}
	if listing > 0 {
		return Conflict(fmt.Sprintf(
			"Kategori ini masih dipakai %d listing. Pindahkan listing tersebut lebih dulu.", listing))
	}
	if anak > 0 {
		return Conflict(fmt.Sprintf(
			"Kategori ini masih memiliki %d sub-kategori. Hapus atau pindahkan sub-kategorinya lebih dulu.", anak))
	}

	if err := s.repos.Category.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return NotFound("Kategori tidak ditemukan.")
		}
		if errors.Is(err, repository.ErrConflict) {
			return Conflict("Kategori masih dipakai dan tidak dapat dihapus.")
		}
		return Internal(err)
	}
	s.invalidateCategoryCache()
	return nil
}

func (s *AdminService) invalidateCategoryCache() {
	if s.cache == nil {
		return
	}
	ctx := contextBackground()
	s.cache.Forget(ctx, "filter:*")
	s.cache.Forget(ctx, "search:*")
}

// ---------- pembuatan admin pertama ----------

// CreateAdmin membuat atau menaikkan satu akun menjadi admin.
// Dipakai subcommand `server admin:create`, bukan lewat antarmuka web, supaya
// admin pertama bisa dibuat tanpa perlu ada admin sebelumnya.
func (s *AdminService) CreateAdmin(ctx context.Context, nama, phone, password string) (*model.User, bool, error) {
	errs := validator.New()

	nama = strings.TrimSpace(nama)
	if nama == "" {
		nama = "Administrator"
	}
	normalized := validator.NormalizePhone(phone)
	if !validator.ValidPhone(normalized) {
		errs.Add("phone", "Nomor HP admin tidak valid. Contoh: 0812xxxxxxx.")
	}
	if len([]rune(password)) < 12 {
		errs.Add("password", "Kata sandi admin minimal 12 karakter.")
	}
	if errs.Any() {
		return nil, false, Invalid(errs)
	}

	// Akun yang sudah ada dinaikkan perannya, bukan digandakan.
	if existing, err := s.repos.User.GetByPhone(ctx, normalized); err == nil {
		if err := s.repos.User.SetRole(ctx, existing.ID, model.RoleAdmin); err != nil {
			return nil, false, Internal(err)
		}
		existing.Role = model.RoleAdmin
		return existing, false, nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, false, Internal(err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, false, Internal(err)
	}
	kota := "Bengkalis"
	user := &model.User{
		FullName:     nama,
		Phone:        normalized,
		PasswordHash: string(hash),
		City:         &kota,
		Role:         model.RoleAdmin,
	}
	if err := s.repos.User.Create(ctx, user); err != nil {
		return nil, false, Internal(err)
	}
	return user, true, nil
}

// ---------- antrean peninjauan listing ----------

// AntreanPeninjauan mengembalikan listing yang menunggu persetujuan,
// diurutkan dari yang paling lama menunggu.
func (s *AdminService) AntreanPeninjauan(ctx context.Context, page int) (*ServiceListResult, error) {
	return s.ListServices(ctx, repository.AdminServiceFilter{
		Antrean: true,
		Sort:    "antrean",
	}, page)
}

func (s *AdminService) JumlahAntrean(ctx context.Context) int {
	n, err := s.repos.Service.CountPending(ctx)
	if err != nil {
		return 0
	}
	return n
}

// SetujuiListing menayangkan listing dan memberi tahu penyedianya.
func (s *AdminService) SetujuiListing(ctx context.Context, adminID, serviceID int64) error {
	svc, err := s.repos.Service.GetByID(ctx, serviceID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return NotFound("Listing tidak ditemukan.")
		}
		return Internal(err)
	}
	if err := s.repos.Service.Approve(ctx, serviceID, adminID); err != nil {
		return Internal(err)
	}
	s.beritahuPenyedia(ctx, svc, repository.NotifListingDisetujui, map[string]any{
		"service_id": serviceID,
		"judul":      svc.Title,
	})
	invalidateSearchCache(s.cache)
	return nil
}

// TolakListing menolak listing dengan alasan yang akan dibaca penyedianya.
func (s *AdminService) TolakListing(ctx context.Context, adminID, serviceID int64, alasan string) error {
	alasan = strings.TrimSpace(alasan)
	if len([]rune(alasan)) < 10 {
		errs := validator.New()
		errs.Add("alasan", "Tuliskan alasan penolakan, minimal 10 karakter, agar penyedia tahu apa yang harus diperbaiki.")
		return Invalid(errs)
	}

	svc, err := s.repos.Service.GetByID(ctx, serviceID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return NotFound("Listing tidak ditemukan.")
		}
		return Internal(err)
	}
	if err := s.repos.Service.Reject(ctx, serviceID, adminID, alasan); err != nil {
		return Internal(err)
	}
	s.beritahuPenyedia(ctx, svc, repository.NotifListingDitolak, map[string]any{
		"service_id": serviceID,
		"judul":      svc.Title,
		"alasan":     alasan,
	})
	invalidateSearchCache(s.cache)
	return nil
}

// beritahuPenyedia mengirim notifikasi ke pemilik listing.
// Kegagalannya tidak membatalkan keputusan admin yang sudah tersimpan.
func (s *AdminService) beritahuPenyedia(ctx context.Context, svc *model.Service, jenis string, payload map[string]any) {
	provider, err := s.repos.Provider.GetByID(ctx, svc.ProviderID)
	if err != nil {
		return
	}
	_ = s.repos.Notif.Create(ctx, provider.UserID, jenis, payload)
}

// UbahListing memungkinkan admin menyunting listing milik siapa pun.
// Berbeda dengan suntingan provider, suntingan admin tidak mengembalikan
// listing ke antrean: admin sudah menjadi peninjaunya.
func (s *AdminService) UbahListing(ctx context.Context, serviceID int64, in ListingInput, listing *ListingService) (*model.Service, error) {
	existing, err := s.repos.Service.GetByID(ctx, serviceID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, NotFound("Listing tidak ditemukan.")
		}
		return nil, Internal(err)
	}

	errs, svc := listing.validate(ctx, in)
	if errs.Any() {
		return nil, Invalid(errs)
	}
	svc.ID = existing.ID
	svc.ProviderID = existing.ProviderID
	svc.CreatedAt = existing.CreatedAt
	if !svc.Status.Valid() {
		svc.Status = existing.Status
	}

	if err := s.repos.Service.Update(ctx, svc); err != nil {
		return nil, Internal(err)
	}
	invalidateSearchCache(s.cache)
	return svc, nil
}

// AdminPenggunaInput adalah isian form admin untuk mengubah data pengguna
// dan, bila ia penyedia, profil penyedianya.
type AdminPenggunaInput struct {
	FullName  string
	Phone     string
	Email     string
	City      string
	Kecamatan string
	// Hanya dipakai bila pengguna adalah penyedia.
	Bio            string
	WhatsappNumber string
}

// UbahPengguna memperbarui identitas pengguna oleh admin. Aturannya sama
// dengan pendaftaran: nomor HP dinormalisasi dan harus unik, email opsional
// dan unik, kecamatan harus dikenal. Nomor HP yang berubah langsung
// berlaku untuk masuk; sesi yang sedang aktif tidak diputus.
func (s *AdminService) UbahPengguna(ctx context.Context, userID int64, in AdminPenggunaInput) (*model.User, error) {
	user, err := s.repos.User.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, NotFound("Pengguna tidak ditemukan.")
		}
		return nil, Internal(err)
	}

	errs := validator.New()
	name := errs.Required("full_name", "Nama lengkap", in.FullName)
	errs.Length("full_name", "Nama lengkap", name, 3, 120)

	phone := validator.NormalizePhone(in.Phone)
	switch {
	case strings.TrimSpace(in.Phone) == "":
		errs.Add("phone", "Nomor HP wajib diisi.")
	case !validator.ValidPhone(phone):
		errs.Add("phone", "Format nomor HP tidak valid. Contoh: 0812xxxxxxx.")
	case phone != user.Phone:
		if lain, err := s.repos.User.GetByPhone(ctx, phone); err == nil && lain.ID != user.ID {
			errs.Add("phone", "Nomor HP sudah dipakai akun lain.")
		}
	}

	email := strings.TrimSpace(in.Email)
	if email != "" {
		if !validator.ValidEmail(email) {
			errs.Add("email", "Format email tidak valid.")
		} else if lain, err := s.repos.User.GetByEmail(ctx, email); err == nil && lain.ID != user.ID {
			errs.Add("email", "Email sudah dipakai akun lain.")
		}
	}

	kecamatan := strings.TrimSpace(in.Kecamatan)
	if kecamatan != "" {
		if k, ok := model.CariKecamatan(kecamatan); ok {
			kecamatan = k.Nama
		} else {
			errs.Add("kecamatan", "Kecamatan tidak dikenal.")
		}
	}
	city := strings.TrimSpace(in.City)
	if city == "" {
		city = "Bengkalis"
	}

	var profil *model.ProviderProfile
	if user.IsProvider {
		profil, err = s.repos.Provider.GetByUserID(ctx, userID)
		if err != nil {
			return nil, Internal(err)
		}
		errs.Length("bio", "Deskripsi penyedia", in.Bio, 20, 1000)
		if strings.TrimSpace(in.WhatsappNumber) != "" {
			wa := validator.NormalizePhone(in.WhatsappNumber)
			if !validator.ValidPhone(wa) {
				errs.Add("whatsapp_number", "Format nomor WhatsApp tidak valid. Contoh: 0812xxxxxxx.")
			}
			in.WhatsappNumber = wa
		} else {
			in.WhatsappNumber = ""
		}
	}
	if errs.Any() {
		return nil, Invalid(errs)
	}

	user.FullName = name
	user.Phone = phone
	user.Email = strPtrOrNil(email)
	user.City = &city
	user.Kecamatan = strPtrOrNil(kecamatan)
	if err := s.repos.User.UpdateByAdmin(ctx, user); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			errs.Add("phone", "Nomor HP atau email sudah dipakai akun lain.")
			return nil, Invalid(errs)
		}
		return nil, Internal(err)
	}
	if profil != nil {
		profil.Bio = strPtrOrNil(strings.TrimSpace(in.Bio))
		profil.WhatsappNumber = in.WhatsappNumber
		if err := s.repos.Provider.Update(ctx, profil); err != nil {
			return nil, Internal(err)
		}
	}
	// Nama & kecamatan tampil di kartu jasa dan blok penyedia pilihan.
	invalidateSearchCache(s.cache)
	if s.cache != nil {
		s.cache.Forget(ctx, "filter:provider-pilihan:*")
	}
	return user, nil
}
