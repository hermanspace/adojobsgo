// Package service memuat seluruh aturan bisnis. Handler API (JSON) dan handler web
// (templ) memakai service yang sama persis, sehingga perilaku web dan calon
// aplikasi mobile tidak pernah menyimpang.
package service

import (
	"errors"
	"fmt"

	"github.com/hermansyah/adojobsid/internal/config"
	"github.com/hermansyah/adojobsid/internal/database"
	"github.com/hermansyah/adojobsid/internal/repository"
	"github.com/hermansyah/adojobsid/internal/session"
	"github.com/hermansyah/adojobsid/internal/validator"
)

// Kode error domain. Handler memetakan kode ini ke status HTTP.
const (
	CodeInvalidInput = "invalid_input"
	CodeNotFound     = "not_found"
	CodeUnauthorized = "unauthorized"
	CodeForbidden    = "forbidden"
	CodeConflict     = "conflict"
	CodeInternal     = "internal"
)

// Error adalah error domain yang membawa kode, pesan siap tampil, dan
// opsional kesalahan per-field untuk ditampilkan di form.
type Error struct {
	Code    string
	Message string
	Fields  validator.Errors
	cause   error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.cause }

func Invalid(fields validator.Errors) *Error {
	return &Error{Code: CodeInvalidInput, Message: "Periksa kembali isian Anda.", Fields: fields}
}

func InvalidMsg(msg string) *Error {
	return &Error{Code: CodeInvalidInput, Message: msg}
}

func NotFound(msg string) *Error     { return &Error{Code: CodeNotFound, Message: msg} }
func Unauthorized(msg string) *Error { return &Error{Code: CodeUnauthorized, Message: msg} }
func Forbidden(msg string) *Error    { return &Error{Code: CodeForbidden, Message: msg} }
func Conflict(msg string) *Error     { return &Error{Code: CodeConflict, Message: msg} }

func Internal(cause error) *Error {
	return &Error{
		Code:    CodeInternal,
		Message: "Terjadi kesalahan di sistem kami. Silakan coba lagi.",
		cause:   cause,
	}
}

// AsError mengambil *Error dari rantai error, bila ada.
func AsError(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}

// Services adalah kumpulan seluruh service aplikasi.
type Services struct {
	Auth      *AuthService
	Provider  *ProviderService
	Catalog   *CatalogService
	Listing   *ListingService
	Upload    *UploadService
	Admin     *AdminService
	Settings  *SettingsService
	Location  *LocationService
	Chat      *ChatService
	Order     *OrderService
	Notif     *NotificationService
	Portfolio *PortfolioService
	Review    *ReviewService
	Paket     *PaketService
	Promosi   *PromosiService
	Push      *PushService
	Kunjungan *KunjunganService
}

// New merangkai seluruh service. Session store dibutuhkan AdminService untuk
// mencabut sesi pengguna yang ditangguhkan; boleh nil pada perintah CLI yang
// tidak menyentuh sesi.
func New(repos *repository.Repositories, cache *database.Cache, cfg *config.Config, sessions *session.Store) *Services {
	upload := NewUploadService(cfg.Upload)
	settings := &SettingsService{repos: repos, cache: cache, upload: upload}
	location := &LocationService{repos: repos, cache: cache, settings: settings}
	promosi := &PromosiService{repos: repos, cache: cache, settings: settings, upload: upload}
	return &Services{
		Auth:      &AuthService{repos: repos},
		Provider:  &ProviderService{repos: repos, cache: cache, upload: upload},
		Catalog:   &CatalogService{repos: repos, cache: cache},
		Listing:   &ListingService{repos: repos, cache: cache, upload: upload, cfg: cfg.Upload},
		Upload:    upload,
		Admin:     &AdminService{repos: repos, cache: cache, sessions: sessions, settings: settings, promosi: promosi},
		Settings:  settings,
		Location:  location,
		Chat:      &ChatService{repos: repos, siaran: NewSiaran()},
		Order:     &OrderService{repos: repos},
		Notif:     &NotificationService{repos: repos},
		Paket:     &PaketService{repos: repos},
		Promosi:   promosi,
		Kunjungan: &KunjunganService{repos: repos, cache: cache},
		Push:      NewPushService(repos, cfg.Push),
		Portfolio: &PortfolioService{repos: repos, cache: cache, upload: upload},
		Review:    &ReviewService{repos: repos, cache: cache},
	}
}

// invalidateSearchCache dipanggil setiap kali listing berubah agar hasil
// pencarian yang tersimpan di Redis tidak menampilkan data basi.
func invalidateSearchCache(cache *database.Cache) {
	if cache == nil {
		return
	}
	ctx := contextBackground()
	cache.Forget(ctx, "search:*")
	cache.Forget(ctx, "filter:*")
}
