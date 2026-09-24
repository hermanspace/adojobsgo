package web

import (
	"github.com/gofiber/fiber/v2"

	"github.com/hermansyah/adojobsid/internal/middleware"
	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/service"
	"github.com/hermansyah/adojobsid/internal/session"
	"github.com/hermansyah/adojobsid/internal/view"
	"github.com/hermansyah/adojobsid/web/templates/pages"
)

// ShowProviderDaftar menampilkan form pembuatan profil penyedia jasa.
func (h *Handler) ShowProviderDaftar(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	if user.IsProvider {
		return c.Redirect("/provider/profil", fiber.StatusSeeOther)
	}
	data := view.ProviderFormData{
		Base:      h.base(c, "Jadi penyedia jasa", "Buat profil penyedia jasa di Adojobs Bengkalis.", "jual"),
		Form:      view.NewForm(),
		Kecamatan: model.KecamatanBengkalis,
	}
	return h.render(c, fiber.StatusOK, pages.ProviderForm(data))
}

// ProviderDaftar memproses pembuatan profil penyedia.
func (h *Handler) ProviderDaftar(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	in := service.ProviderProfileInput{
		Bio:            c.FormValue("bio"),
		WhatsappNumber: c.FormValue("whatsapp_number"),
	}

	profile, err := h.svc.Provider.Create(ctx(c), user.ID, in)
	if err != nil {
		return h.renderProviderForm(c, false, 0, in, err)
	}

	h.refreshSession(c, func(d *session.Data) {
		d.IsProvider = true
		d.ProviderID = profile.ID
	})
	return h.redirectWithFlash(c, "/jasa/baru", "sukses",
		"Profil penyedia tersimpan. Sekarang pasang jasa pertama Anda.")
}

// ShowProviderProfil menampilkan form penyuntingan profil penyedia.
func (h *Handler) ShowProviderProfil(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	profile, err := h.svc.Provider.GetByUserID(ctx(c), user.ID)
	if err != nil {
		return c.Redirect("/provider/daftar", fiber.StatusSeeOther)
	}

	form := view.NewForm()
	form.Set("bio", view.Deref(profile.Bio))
	form.Set("whatsapp_number", profile.WhatsappNumber)

	data := view.ProviderFormData{
		Base:      h.base(c, "Profil penyedia", "Ubah profil penyedia jasa Anda.", "akun"),
		Form:      form,
		IsEdit:    true,
		Profile:   profile,
		Kecamatan: model.KecamatanBengkalis,
	}
	return h.render(c, fiber.StatusOK, pages.ProviderForm(data))
}

// ProviderProfil memproses penyuntingan profil penyedia.
func (h *Handler) ProviderProfil(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	in := service.ProviderProfileInput{
		Bio:            c.FormValue("bio"),
		WhatsappNumber: c.FormValue("whatsapp_number"),
	}

	if _, err := h.svc.Provider.Update(ctx(c), user.ProviderID, in); err != nil {
		return h.renderProviderForm(c, true, user.ProviderID, in, err)
	}
	return h.redirectWithFlash(c, "/dasbor", "sukses", "Profil penyedia berhasil diperbarui.")
}

// renderProviderForm merender ulang form beserta pesan kesalahannya.
func (h *Handler) renderProviderForm(c *fiber.Ctx, isEdit bool, providerID int64, in service.ProviderProfileInput, err error) error {
	form := view.NewForm()
	form.Set("bio", in.Bio)
	form.Set("whatsapp_number", in.WhatsappNumber)

	data := view.ProviderFormData{
		Base:      h.base(c, "Profil penyedia", "Profil penyedia jasa.", "jual"),
		Form:      form,
		IsEdit:    isEdit,
		Kecamatan: model.KecamatanBengkalis,
	}

	status := fiber.StatusInternalServerError
	if e, ok := service.AsError(err); ok {
		status = statusFor(e.Code)
		if e.Fields != nil {
			data.Form.Errors = e.Fields
		}
		if e.Code != service.CodeInvalidInput {
			data.Flash = &view.Flash{Kind: "galat", Message: e.Message}
		}
	} else {
		data.Flash = &view.Flash{Kind: "galat", Message: "Terjadi kesalahan. Silakan coba lagi."}
	}
	return h.render(c, status, pages.ProviderForm(data))
}
