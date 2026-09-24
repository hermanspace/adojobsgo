package web

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"

	"github.com/hermansyah/adojobsid/internal/view"
)

// Manifest melayani web app manifest dari pengaturan situs, sehingga nama,
// tagline, dan warna yang dilihat Android saat "Pasang di layar utama"
// selalu sama dengan halaman. Ikon PNG dihasilkan dari favicon.svg lewat
// `make ikon`.
func (h *Handler) Manifest(c *fiber.Ctx) error {
	umum := h.svc.Settings.Get(ctx(c)).Umum
	// c.JSON menimpa Content-Type menjadi application/json; manifest harus
	// bertipe application/manifest+json agar dikenali sebagai manifest.
	isi, err := json.Marshal(fiber.Map{
		"id":               "/",
		"name":             umum.NamaSitus,
		"short_name":       umum.NamaSitus,
		"description":      umum.Tagline,
		"start_url":        "/?dari=pwa",
		"scope":            "/",
		"display":          "standalone",
		"orientation":      "portrait",
		"lang":             "id",
		"background_color": view.TokenWarna("terang", "bg"),
		"theme_color":      view.TokenWarna("terang", "bg"),
		"icons": []fiber.Map{
			{"src": "/static/img/ikon-192.png", "sizes": "192x192", "type": "image/png"},
			{"src": "/static/img/ikon-512.png", "sizes": "512x512", "type": "image/png"},
			{"src": "/static/img/ikon-maskable-512.png", "sizes": "512x512", "type": "image/png", "purpose": "maskable"},
			{"src": "/static/img/favicon.svg", "sizes": "any", "type": "image/svg+xml"},
		},
	})
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, "application/manifest+json; charset=utf-8")
	c.Set(fiber.HeaderCacheControl, "public, max-age=3600")
	return c.Send(isi)
}
