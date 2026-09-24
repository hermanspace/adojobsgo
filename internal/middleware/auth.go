// Package middleware berisi middleware Fiber yang dipakai bersama oleh
// rute web maupun rute API.
package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/hermansyah/adojobsid/internal/config"
	"github.com/hermansyah/adojobsid/internal/session"
	"github.com/hermansyah/adojobsid/internal/view"
)

// Kunci penyimpanan di c.Locals.
const (
	LocalUser      = "user"
	LocalSessionID = "session_id"
)

type Auth struct {
	store *session.Store
	cfg   config.Session
}

func NewAuth(store *session.Store, cfg config.Session) *Auth {
	return &Auth{store: store, cfg: cfg}
}

// Load membaca cookie sesi dan menaruh data pengguna di Locals bila sesi valid.
// Middleware ini tidak pernah menolak request — hanya mengisi konteks.
func (a *Auth) Load(c *fiber.Ctx) error {
	// Aplikasi native mengirim sesi lewat header Authorization: Bearer <id>;
	// peramban lewat cookie. Keduanya menunjuk sesi yang sama di Redis, jadi
	// pencabutan (logout-semua, penangguhan) berlaku untuk keduanya sekaligus.
	sid, lewatBearer := tokenBearer(c)
	if sid == "" {
		sid = c.Cookies(a.cfg.CookieName)
	}
	if sid == "" {
		return c.Next()
	}
	data, err := a.store.Get(c.Context(), sid)
	if err != nil {
		// Sesi kedaluwarsa atau tidak dikenal: bersihkan cookie-nya.
		if !lewatBearer {
			a.ClearCookie(c)
		}
		return c.Next()
	}
	c.Locals(LocalBearer, lewatBearer)
	c.Locals(LocalSessionID, sid)
	c.Locals(LocalUser, &view.CurrentUser{
		ID:         data.UserID,
		FullName:   data.FullName,
		IsProvider: data.IsProvider,
		IsAdmin:    data.IsAdmin,
		ProviderID: data.ProviderID,
		AvatarURL:  data.AvatarURL,
	})
	return c.Next()
}

// RequireWeb melindungi halaman web: pengunjung anonim diarahkan ke halaman masuk
// dengan parameter next agar kembali ke halaman yang dituju setelah login.
func (a *Auth) RequireWeb(c *fiber.Ctx) error {
	if CurrentUser(c) != nil {
		return c.Next()
	}
	if IsHTMX(c) {
		// Ditangani app.js: status 401 memicu pengalihan di sisi klien.
		return c.Status(fiber.StatusUnauthorized).Send(nil)
	}
	return c.Redirect("/masuk?next="+c.Path(), fiber.StatusSeeOther)
}

// RequireProviderWeb memastikan pengguna sudah punya profil penyedia jasa.
func (a *Auth) RequireProviderWeb(c *fiber.Ctx) error {
	user := CurrentUser(c)
	if user == nil {
		return a.RequireWeb(c)
	}
	if !user.IsProvider || user.ProviderID == 0 {
		return redirectAman(c, "/provider/daftar")
	}
	return c.Next()
}

// redirectAman mengalihkan peramban tanpa merusak tampilan saat permintaannya
// datang dari HTMX.
//
// Pengalihan 303 biasa akan diikuti HTMX, yang lalu menyuntikkan HTML halaman
// tujuan ke dalam elemen sasaran — misalnya seluruh halaman "Jadi penyedia
// jasa" masuk ke dalam daftar foto. Header HX-Redirect membuat HTMX
// benar-benar berpindah halaman.
func redirectAman(c *fiber.Ctx, tujuan string) error {
	if IsHTMX(c) {
		c.Set("HX-Redirect", tujuan)
		return c.Status(fiber.StatusNoContent).Send(nil)
	}
	return c.Redirect(tujuan, fiber.StatusSeeOther)
}

// LocalBearer menandai permintaan yang sesinya datang lewat header, bukan cookie.
const LocalBearer = "sesi_bearer"

// tokenBearer membaca sesi dari header Authorization.
func tokenBearer(c *fiber.Ctx) (string, bool) {
	h := c.Get(fiber.HeaderAuthorization)
	const awalan = "Bearer "
	if len(h) > len(awalan) && strings.EqualFold(h[:len(awalan)], awalan) {
		return strings.TrimSpace(h[len(awalan):]), true
	}
	return "", false
}

// LewatBearer menjawab apakah sesi permintaan ini datang lewat header.
func LewatBearer(c *fiber.Ctx) bool {
	v, _ := c.Locals(LocalBearer).(bool)
	return v
}

// RequireAPI melindungi endpoint JSON.
func (a *Auth) RequireAPI(c *fiber.Ctx) error {
	if CurrentUser(c) == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fiber.Map{"code": "unauthorized", "message": "Silakan masuk terlebih dahulu."},
		})
	}
	return c.Next()
}

// RequireProviderAPI memastikan pemanggil API adalah penyedia jasa.
func (a *Auth) RequireProviderAPI(c *fiber.Ctx) error {
	user := CurrentUser(c)
	if user == nil {
		return a.RequireAPI(c)
	}
	if !user.IsProvider || user.ProviderID == 0 {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": fiber.Map{"code": "forbidden", "message": "Akun Anda belum terdaftar sebagai penyedia jasa."},
		})
	}
	return c.Next()
}

// SetCookie memasang cookie sesi. HttpOnly dan SameSite=Lax mencegah
// pembacaan lewat JavaScript dan sebagian besar serangan CSRF lintas situs.
func (a *Auth) SetCookie(c *fiber.Ctx, sid string) {
	c.Cookie(&fiber.Cookie{
		Name:     a.cfg.CookieName,
		Value:    sid,
		Path:     "/",
		MaxAge:   int(a.cfg.TTL.Seconds()),
		HTTPOnly: true,
		Secure:   a.cfg.Secure,
		SameSite: "Lax",
	})
}

func (a *Auth) ClearCookie(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     a.cfg.CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HTTPOnly: true,
		Secure:   a.cfg.Secure,
		SameSite: "Lax",
	})
}

// CurrentUser mengambil pengguna yang sedang masuk dari Locals, atau nil.
func CurrentUser(c *fiber.Ctx) *view.CurrentUser {
	if u, ok := c.Locals(LocalUser).(*view.CurrentUser); ok {
		return u
	}
	return nil
}

// SessionID mengembalikan ID sesi aktif.
func SessionID(c *fiber.Ctx) string {
	if s, ok := c.Locals(LocalSessionID).(string); ok {
		return s
	}
	return ""
}

// IsHTMX menandai request yang berasal dari HTMX, sehingga handler bisa
// mengirim fragmen HTML alih-alih halaman penuh.
func IsHTMX(c *fiber.Ctx) bool {
	return c.Get("HX-Request") == "true"
}

// RequireAdminWeb melindungi panel admin.
// Pengunjung yang tidak berhak mendapat 404, bukan 403: keberadaan panel
// admin tidak perlu dikonfirmasi ke orang yang tidak berkepentingan.
func (a *Auth) RequireAdminWeb(c *fiber.Ctx) error {
	user := CurrentUser(c)
	if user == nil {
		return redirectAman(c, "/masuk?next="+c.Path())
	}
	if !user.IsAdmin {
		// Permintaan HTMX dijawab status tanpa badan sama sekali; HTML
		// halaman 404 hanya akan disuntikkan ke elemen sasaran.
		if IsHTMX(c) {
			return c.Status(fiber.StatusNotFound).Send(nil)
		}
		return fiber.ErrNotFound
	}
	return c.Next()
}

// RequireAdminAPI melindungi endpoint admin pada API JSON.
func (a *Auth) RequireAdminAPI(c *fiber.Ctx) error {
	user := CurrentUser(c)
	if user == nil {
		return a.RequireAPI(c)
	}
	if !user.IsAdmin {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": fiber.Map{"code": "not_found", "message": "Endpoint tidak ditemukan."},
		})
	}
	return c.Next()
}
