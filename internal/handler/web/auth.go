package web

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/hermansyah/adojobsid/internal/middleware"
	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/service"
	"github.com/hermansyah/adojobsid/internal/session"
	"github.com/hermansyah/adojobsid/internal/view"
	"github.com/hermansyah/adojobsid/web/templates/pages"
)

// ShowMasuk menampilkan halaman login.
func (h *Handler) ShowMasuk(c *fiber.Ctx) error {
	if middleware.CurrentUser(c) != nil {
		return c.Redirect("/dasbor", fiber.StatusSeeOther)
	}
	data := view.AuthData{
		Base: h.base(c, "Masuk", "Masuk ke akun Adojobs Bengkalis.", "akun"),
		Form: view.NewForm(),
		Next: safeNext(c.Query("next")),
	}
	return h.render(c, fiber.StatusOK, pages.Masuk(data))
}

// Masuk memproses form login.
func (h *Handler) Masuk(c *fiber.Ctx) error {
	form := view.NewForm()
	form.Set("identifier", strings.TrimSpace(c.FormValue("identifier")))
	next := safeNext(c.FormValue("next"))

	user, err := h.svc.Auth.Login(ctx(c), service.LoginInput{
		Identifier: c.FormValue("identifier"),
		Password:   c.FormValue("password"),
	})
	if err != nil {
		data := view.AuthData{
			Base: h.base(c, "Masuk", "Masuk ke akun Adojobs Bengkalis.", "akun"),
			Form: form,
			Next: next,
		}
		if e, ok := service.AsError(err); ok {
			if e.Fields != nil {
				data.Form.Errors = e.Fields
			}
			data.Flash = &view.Flash{Kind: "galat", Message: e.Message}
			return h.render(c, statusFor(e.Code), pages.Masuk(data))
		}
		data.Flash = &view.Flash{Kind: "galat", Message: "Terjadi kesalahan. Silakan coba lagi."}
		return h.render(c, fiber.StatusInternalServerError, pages.Masuk(data))
	}

	if err := h.mulaiSesi(c, user); err != nil {
		return err
	}
	if next == "" {
		// Admin langsung diarahkan ke panelnya, bukan ke dasbor pengguna biasa.
		if user.IsAdmin() {
			next = "/admin"
		} else {
			next = "/dasbor"
		}
	}
	return h.redirectWithFlash(c, next, "sukses", "Selamat datang kembali, "+user.FullName+".")
}

// ShowDaftar menampilkan halaman registrasi.
func (h *Handler) ShowDaftar(c *fiber.Ctx) error {
	if middleware.CurrentUser(c) != nil {
		return c.Redirect("/dasbor", fiber.StatusSeeOther)
	}
	data := view.AuthData{
		Base: h.base(c, "Daftar", "Buat akun Adojobs untuk mencari atau menawarkan jasa di Bengkalis.", "akun"),
		Form: view.NewForm(),
	}
	return h.render(c, fiber.StatusOK, pages.Daftar(data))
}

// Daftar memproses registrasi lalu langsung memulai sesi,
// supaya pengguna tidak perlu login ulang setelah mendaftar.
func (h *Handler) Daftar(c *fiber.Ctx) error {
	in := service.RegisterInput{
		FullName:        c.FormValue("full_name"),
		Phone:           c.FormValue("phone"),
		Email:           c.FormValue("email"),
		Password:        c.FormValue("password"),
		PasswordConfirm: c.FormValue("password_confirm"),
		Kecamatan:       c.FormValue("kecamatan"),
	}

	user, err := h.svc.Auth.Register(ctx(c), in)
	if err != nil {
		form := view.NewForm()
		form.Set("full_name", in.FullName)
		form.Set("phone", in.Phone)
		form.Set("email", in.Email)
		form.Set("kecamatan", in.Kecamatan)

		data := view.AuthData{
			Base: h.base(c, "Daftar", "Buat akun Adojobs.", "akun"),
			Form: form,
		}
		if e, ok := service.AsError(err); ok {
			if e.Fields != nil {
				data.Form.Errors = e.Fields
			}
			if e.Code != service.CodeInvalidInput {
				data.Flash = &view.Flash{Kind: "galat", Message: e.Message}
			}
			return h.render(c, statusFor(e.Code), pages.Daftar(data))
		}
		data.Flash = &view.Flash{Kind: "galat", Message: "Terjadi kesalahan. Silakan coba lagi."}
		return h.render(c, fiber.StatusInternalServerError, pages.Daftar(data))
	}

	if err := h.mulaiSesi(c, user); err != nil {
		return err
	}
	return h.redirectWithFlash(c, "/dasbor", "sukses",
		"Akun Anda sudah aktif. Lengkapi profil penyedia bila ingin menawarkan jasa.")
}

// Keluar menghapus sesi di Redis sekaligus cookie-nya.
func (h *Handler) Keluar(c *fiber.Ctx) error {
	if sid := middleware.SessionID(c); sid != "" {
		_ = h.sessions.Destroy(ctx(c), sid)
	}
	h.auth.ClearCookie(c)
	return h.redirectWithFlash(c, "/", "info", "Anda sudah keluar.")
}

// mulaiSesi membuat sesi baru, melengkapi provider_id bila pengguna
// sudah punya profil penyedia.
func (h *Handler) mulaiSesi(c *fiber.Ctx, user *model.User) error {
	data := session.Data{
		UserID:     user.ID,
		FullName:   user.FullName,
		IsProvider: user.IsProvider,
		IsAdmin:    user.IsAdmin(),
		AvatarURL:  derefStr(user.AvatarURL),
	}
	if user.IsProvider {
		if p, err := h.svc.Provider.GetByUserID(ctx(c), user.ID); err == nil {
			data.ProviderID = p.ID
		}
	}
	return h.login(c, data)
}

// safeNext hanya menerima path internal, mencegah pengalihan terbuka
// ke situs luar lewat parameter next.
func safeNext(next string) string {
	next = strings.TrimSpace(next)
	if next == "" {
		return ""
	}
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return ""
	}
	return next
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// statusFor memetakan kode error domain ke status HTTP.
func statusFor(code string) int {
	switch code {
	case service.CodeInvalidInput:
		return fiber.StatusUnprocessableEntity
	case service.CodeNotFound:
		return fiber.StatusNotFound
	case service.CodeUnauthorized:
		return fiber.StatusUnauthorized
	case service.CodeForbidden:
		return fiber.StatusForbidden
	case service.CodeConflict:
		return fiber.StatusConflict
	default:
		return fiber.StatusInternalServerError
	}
}
