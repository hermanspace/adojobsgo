// Package web berisi handler yang merender HTML dengan templ.
// Seluruh aturan bisnis diambil dari package service — sama persis dengan
// yang dipakai handler API — sehingga perilaku web dan API tidak berbeda.
package web

import (
	"context"
	"strings"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"

	"github.com/hermansyah/adojobsid/internal/config"
	"github.com/hermansyah/adojobsid/internal/middleware"
	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/service"
	"github.com/hermansyah/adojobsid/internal/session"
	"github.com/hermansyah/adojobsid/internal/view"
)

const flashCookie = "adojobs_flash"

type Handler struct {
	svc      *service.Services
	sessions *session.Store
	flashes  *session.FlashStore
	auth     *middleware.Auth
	cfg      *config.Config
	assets   *view.Assets
}

func New(
	svc *service.Services,
	sessions *session.Store,
	flashes *session.FlashStore,
	auth *middleware.Auth,
	cfg *config.Config,
	assets *view.Assets,
) *Handler {
	return &Handler{
		svc:      svc,
		sessions: sessions,
		flashes:  flashes,
		auth:     auth,
		cfg:      cfg,
		assets:   assets,
	}
}

// base menyusun data yang tersedia di setiap halaman, termasuk flash message
// yang langsung dihapus setelah dibaca.
func (h *Handler) base(c *fiber.Ctx, title, description, activeNav string) view.Base {
	pengaturan := h.svc.Settings.Get(ctx(c))
	lokasi := h.lokasiAktif(c)

	b := view.Base{
		Title:       title,
		Description: description,
		User:        middleware.CurrentUser(c),
		ActiveNav:   activeNav,
		Path:        c.Path(),
		URL:         strings.TrimRight(h.cfg.App.BaseURL, "/") + c.Path(),
		Assets:      h.assets,
		Lokasi: view.LokasiRingkas{
			Label:     lokasi.Label,
			Latitude:  lokasi.Latitude,
			Longitude: lokasi.Longitude,
			Tepat:     lokasi.Tepat,
			Sumber:    lokasi.Sumber,
		},
		Situs: view.SitusRingkas{
			Nama:       pengaturan.Umum.NamaSitus,
			LogoURL:    pengaturan.Umum.LogoURL,
			TeksFooter: pengaturan.Umum.TeksFooter,

			WhatsappAktif: pengaturan.Umum.WhatsappAktif,
			HeroGambarURL: pengaturan.Tampilan.HeroGambarURL,
			HeroOverlay:   pengaturan.Tampilan.HeroOverlay,

			AplikasiTampil:     pengaturan.Tampilan.AplikasiTampil,
			PlaystoreURL:       pengaturan.Tampilan.PlaystoreURL,
			PlaystoreTeksAtas:  pengaturan.Tampilan.PlaystoreTeksAtas,
			PlaystoreTeksBawah: pengaturan.Tampilan.PlaystoreTeksBawah,
		},
		Iklan: pengaturan.Iklan,
		PilihIklan: func(kunci string) *model.SlotIklan {
			ik := h.svc.Promosi.PilihUntukSlot(ctx(c), kunci, kecamatanPenonton(lokasi))
			if ik == nil {
				return nil
			}
			h.svc.Promosi.CatatTayang(ctx(c), ik.ID)
			slot := model.SlotDariIklan(*ik, kunci)
			return &slot
		},
	}
	if u := middleware.CurrentUser(c); u != nil {
		b.BelumDibaca = h.svc.Chat.BelumDibaca(ctx(c), u.ID)
	}
	if id := c.Cookies(flashCookie); id != "" {
		if p := h.flashes.Take(c.Context(), id); p != nil {
			b.Flash = &view.Flash{Kind: p.Kind, Message: p.Message}
		}
		h.clearFlashCookie(c)
	}
	return b
}

// render mengirim komponen templ sebagai respons HTML.
func (h *Handler) render(c *fiber.Ctx, status int, component templ.Component) error {
	c.Status(status).Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return component.Render(c.Context(), c.Response().BodyWriter())
}

// redirectWithFlash menyimpan pesan lalu mengalihkan ke halaman tujuan.
// Untuk request HTMX dipakai header HX-Redirect agar peramban benar-benar pindah.
func (h *Handler) redirectWithFlash(c *fiber.Ctx, to, kind, message string) error {
	h.setFlash(c, kind, message)
	return h.redirect(c, to)
}

func (h *Handler) redirect(c *fiber.Ctx, to string) error {
	if middleware.IsHTMX(c) {
		c.Set("HX-Redirect", to)
		return c.SendStatus(fiber.StatusNoContent)
	}
	return c.Redirect(to, fiber.StatusSeeOther)
}

func (h *Handler) setFlash(c *fiber.Ctx, kind, message string) {
	id, err := h.flashes.Put(c.Context(), kind, message)
	if err != nil {
		return
	}
	c.Cookie(&fiber.Cookie{
		Name:     flashCookie,
		Value:    id,
		Path:     "/",
		MaxAge:   120,
		HTTPOnly: true,
		Secure:   h.cfg.Session.Secure,
		SameSite: "Lax",
	})
}

func (h *Handler) clearFlashCookie(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     flashCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HTTPOnly: true,
		Secure:   h.cfg.Session.Secure,
		SameSite: "Lax",
	})
}

// login membuat sesi baru dan memasang cookie-nya.
func (h *Handler) login(c *fiber.Ctx, data session.Data) error {
	sid, err := h.sessions.Create(c.Context(), data)
	if err != nil {
		return err
	}
	h.auth.SetCookie(c, sid)
	return nil
}

// refreshSession memperbarui isi sesi, misalnya setelah pengguna menjadi provider
// atau mengganti nama, agar navigasi langsung menampilkan data terbaru.
func (h *Handler) refreshSession(c *fiber.Ctx, mutate func(*session.Data)) {
	sid := middleware.SessionID(c)
	if sid == "" {
		return
	}
	data, err := h.sessions.Get(c.Context(), sid)
	if err != nil {
		return
	}
	mutate(data)
	_ = h.sessions.Save(c.Context(), sid, *data)

	if u := middleware.CurrentUser(c); u != nil {
		u.FullName = data.FullName
		u.IsProvider = data.IsProvider
		u.ProviderID = data.ProviderID
		u.AvatarURL = data.AvatarURL
	}
}

// ctx memakai konteks request agar query database ikut dibatalkan
// saat klien memutus koneksi.
func ctx(c *fiber.Ctx) context.Context { return c.Context() }

// kecamatanPenonton menurunkan kecamatan penonton dari lokasi aktifnya untuk
// penyaringan iklan. Lokasi bawaan (belum memilih apa pun) tidak menyaring:
// penonton yang tidak diketahui posisinya melihat semua iklan.
func kecamatanPenonton(l service.LokasiAktif) string {
	if l.Sumber == "bawaan" {
		return ""
	}
	k, _ := model.KecamatanTerdekat(l.Latitude, l.Longitude)
	return k.Nama
}
