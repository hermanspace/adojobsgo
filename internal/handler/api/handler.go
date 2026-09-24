// Package api berisi handler JSON di bawah /api/v1.
// Endpoint ini memakai service layer yang sama dengan handler web, dan
// disiapkan untuk dikonsumsi aplikasi mobile Flutter di kemudian hari.
package api

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"

	"github.com/hermansyah/adojobsid/internal/config"
	"github.com/hermansyah/adojobsid/internal/middleware"
	"github.com/hermansyah/adojobsid/internal/service"
	"github.com/hermansyah/adojobsid/internal/session"
)

type Handler struct {
	svc      *service.Services
	sessions *session.Store
	auth     *middleware.Auth
	cfg      *config.Config
}

func New(svc *service.Services, sessions *session.Store, auth *middleware.Auth, cfg *config.Config) *Handler {
	return &Handler{svc: svc, sessions: sessions, auth: auth, cfg: cfg}
}

// respons membungkus payload sukses agar bentuknya seragam.
type respons struct {
	Data any  `json:"data"`
	Meta *any `json:"meta,omitempty"`
}

func ok(c *fiber.Ctx, data any) error {
	return c.JSON(respons{Data: data})
}

func created(c *fiber.Ctx, data any) error {
	return c.Status(fiber.StatusCreated).JSON(respons{Data: data})
}

// fail menerjemahkan error domain menjadi respons JSON yang konsisten.
// Kesalahan internal tidak pernah membocorkan detail teknis ke klien.
func fail(c *fiber.Ctx, err error) error {
	e, isDomain := service.AsError(err)
	if !isDomain {
		slog.Error("kesalahan tak terduga di API", "path", c.Path(), "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{
				"code":    service.CodeInternal,
				"message": "Terjadi kesalahan di sistem kami.",
			},
		})
	}

	body := fiber.Map{"code": e.Code, "message": e.Message}
	if len(e.Fields) > 0 {
		body["fields"] = e.Fields
	}
	if e.Code == service.CodeInternal {
		slog.Error("kesalahan internal di API", "path", c.Path(), "error", e)
	}
	return c.Status(statusFor(e.Code)).JSON(fiber.Map{"error": body})
}

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

// NotFound adalah fallback untuk rute API yang tidak dikenal.
// TerlaluBanyak dipanggil pembatas laju saat kuota unggah terlampaui.
func (h *Handler) TerlaluBanyak(c *fiber.Ctx) error {
	return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
		"error": fiber.Map{"code": "rate_limited", "message": "Terlalu banyak unggahan. Tunggu sebentar, lalu coba lagi."},
	})
}

func (h *Handler) NotFound(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
		"error": fiber.Map{"code": "not_found", "message": "Endpoint tidak ditemukan."},
	})
}
