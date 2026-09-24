package web

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"

	"github.com/hermansyah/adojobsid/internal/service"
	"github.com/hermansyah/adojobsid/internal/view"
	"github.com/hermansyah/adojobsid/web/templates/pages"
)

// errorPage merender halaman error yang sesuai dengan jenis kegagalan.
func (h *Handler) errorPage(c *fiber.Ctx, err error) error {
	e, ok := service.AsError(err)
	if !ok {
		slog.Error("kesalahan tak terduga", "path", c.Path(), "error", err)
		return h.serverError(c)
	}
	switch e.Code {
	case service.CodeNotFound:
		return h.renderError(c, fiber.StatusNotFound, "Halaman tidak ditemukan", e.Message)
	case service.CodeForbidden:
		return h.renderError(c, fiber.StatusForbidden, "Akses ditolak", e.Message)
	case service.CodeUnauthorized:
		return c.Redirect("/masuk?next="+c.Path(), fiber.StatusSeeOther)
	default:
		slog.Error("kesalahan internal", "path", c.Path(), "error", e)
		return h.serverError(c)
	}
}

func (h *Handler) notFound(c *fiber.Ctx) error {
	return h.renderError(c, fiber.StatusNotFound,
		"Halaman tidak ditemukan",
		"Tautan yang Anda buka mungkin salah, atau listing-nya sudah dihapus penyedia.")
}

func (h *Handler) serverError(c *fiber.Ctx) error {
	return h.renderError(c, fiber.StatusInternalServerError,
		"Ada gangguan di sistem kami",
		"Silakan muat ulang halaman ini beberapa saat lagi.")
}

// NotFoundHandler dipasang sebagai handler terakhir di router.
func (h *Handler) NotFoundHandler(c *fiber.Ctx) error { return h.notFound(c) }

// TerlaluBanyakUnggahan dipanggil pembatas laju saat kuota unggah terlampaui.
func (h *Handler) TerlaluBanyakUnggahan(c *fiber.Ctx) error {
	return h.renderError(c, fiber.StatusTooManyRequests, "Terlalu banyak unggahan",
		"Anda mengunggah terlalu banyak berkas dalam waktu singkat. Tunggu sebentar, lalu coba lagi.")
}

func (h *Handler) renderError(c *fiber.Ctx, code int, heading, message string) error {
	data := view.ErrorData{
		Base:    h.base(c, heading, message, ""),
		Code:    code,
		Heading: heading,
		Message: message,
	}
	return h.render(c, code, pages.ErrorPage(data))
}
