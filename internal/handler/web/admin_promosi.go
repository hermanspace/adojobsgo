package web

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/hermansyah/adojobsid/internal/middleware"
	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/repository"
	"github.com/hermansyah/adojobsid/internal/service"
	"github.com/hermansyah/adojobsid/internal/view"
	"github.com/hermansyah/adojobsid/web/templates/pages"
)

const perHalamanPromosi = 30

// AdminPromosi adalah antrean pengajuan promosi.
func (h *Handler) AdminPromosi(c *fiber.Ctx) error {
	tab := strings.TrimSpace(c.Query("tab"))
	if tab == "" {
		tab = "menunggu"
	}
	jenis := strings.TrimSpace(c.Query("jenis"))
	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}

	items, total, err := h.svc.Promosi.DaftarAdmin(ctx(c), repository.PromosiFilter{
		Status: service.TabPromosi(tab), Jenis: jenis,
		Limit: perHalamanPromosi, Offset: (page - 1) * perHalamanPromosi,
	})
	if err != nil {
		return h.errorPage(c, err)
	}
	return h.render(c, fiber.StatusOK, pages.AdminPromosi(view.AdminPromosiData{
		AdminBase:  h.adminBase(c, "Promosi", "Pengajuan iklan, sorotan jasa, dan penyedia pilihan", "promosi"),
		Tab:        tab,
		Jenis:      jenis,
		Items:      items,
		Total:      total,
		Page:       page,
		TotalPages: (total + perHalamanPromosi - 1) / perHalamanPromosi,
	}))
}

func (h *Handler) dataPromosiAdmin(c *fiber.Ctx, p *model.Promosi, form view.Form) view.AdminPromosiDetailData {
	data := view.AdminPromosiDetailData{
		AdminBase: h.adminBase(c, model.LabelJenisPromosi(p.Jenis)+" #"+strconv.FormatInt(p.ID, 10),
			"Tinjau dan putuskan pengajuan ini", "promosi"),
		Promosi: *p,
		Form:    form,
	}
	if p.PaketID != nil {
		if paket, err := h.svc.Paket.Ambil(ctx(c), *p.PaketID); err == nil {
			data.Paket = paket
		}
	}
	if u, err := h.svc.Auth.GetUser(ctx(c), p.UserID); err == nil {
		data.Pemohon = u
	}
	if p.ServiceID != nil {
		if d, err := h.svc.Listing.GetDetail(ctx(c), *p.ServiceID); err == nil {
			data.JudulJasa = d.Title
		}
	}
	if p.ProviderID != nil {
		if d, err := h.svc.Provider.GetDetail(ctx(c), *p.ProviderID); err == nil {
			data.NamaPenyedia, data.SlugPenyedia = d.FullName, d.Slug
		}
	}
	return data
}

// AdminPromosiDetail menampilkan satu pengajuan beserta tindakan yang sah.
func (h *Handler) AdminPromosiDetail(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}
	p, err := h.svc.Promosi.Ambil(ctx(c), id)
	if err != nil {
		return h.errorPage(c, err)
	}
	return h.render(c, fiber.StatusOK, pages.AdminPromosiDetail(h.dataPromosiAdmin(c, p, view.NewForm())))
}

// tindakanPromosi menjalankan satu perubahan status lalu kembali ke detail.
func (h *Handler) tindakanPromosi(c *fiber.Ctx, sukses string, jalankan func(id int64) error) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}
	tujuan := "/admin/promosi/" + strconv.FormatInt(id, 10)
	if err := jalankan(id); err != nil {
		return h.redirectWithFlash(c, tujuan, "galat", pesanKegagalan(err))
	}
	return h.redirectWithFlash(c, tujuan, "sukses", sukses)
}

func (h *Handler) AdminPromosiSetujui(c *fiber.Ctx) error {
	admin := middleware.CurrentUser(c)
	return h.tindakanPromosi(c, "Pengajuan disetujui.", func(id int64) error {
		_, err := h.svc.Promosi.Setujui(ctx(c), admin.ID, id)
		return err
	})
}

// AdminPromosiTolak merender ulang detail saat alasannya kosong, supaya
// isian lain tidak hilang dan galatnya tampil tepat di kolomnya.
func (h *Handler) AdminPromosiTolak(c *fiber.Ctx) error {
	admin := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}
	catatan := c.FormValue("catatan")
	if _, err := h.svc.Promosi.Tolak(ctx(c), admin.ID, id, catatan); err != nil {
		if e, ok := service.AsError(err); ok && e.Code == service.CodeInvalidInput {
			p, perr := h.svc.Promosi.Ambil(ctx(c), id)
			if perr != nil {
				return h.errorPage(c, perr)
			}
			form := view.NewForm()
			form.Set("catatan", catatan)
			data := h.dataPromosiAdmin(c, p, form)
			data.Form.Errors = e.Fields
			return h.render(c, fiber.StatusUnprocessableEntity, pages.AdminPromosiDetail(data))
		}
		return h.redirectWithFlash(c, "/admin/promosi/"+strconv.FormatInt(id, 10), "galat", pesanKegagalan(err))
	}
	return h.redirectWithFlash(c, "/admin/promosi/"+strconv.FormatInt(id, 10), "sukses", "Pengajuan ditolak; pemohon diberi tahu beserta alasannya.")
}

func (h *Handler) AdminPromosiKonfirmasi(c *fiber.Ctx) error {
	admin := middleware.CurrentUser(c)
	return h.tindakanPromosi(c, "Pembayaran dikonfirmasi; promosi mulai tayang.", func(id int64) error {
		_, err := h.svc.Promosi.KonfirmasiBayar(ctx(c), admin.ID, id)
		return err
	})
}

func (h *Handler) AdminPromosiJeda(c *fiber.Ctx) error {
	return h.tindakanPromosi(c, "Promosi dijeda.", func(id int64) error {
		_, err := h.svc.Promosi.Jeda(ctx(c), id)
		return err
	})
}

func (h *Handler) AdminPromosiLanjutkan(c *fiber.Ctx) error {
	return h.tindakanPromosi(c, "Promosi dilanjutkan.", func(id int64) error {
		_, err := h.svc.Promosi.Lanjutkan(ctx(c), id)
		return err
	})
}

func (h *Handler) AdminPromosiHentikan(c *fiber.Ctx) error {
	return h.tindakanPromosi(c, "Promosi dihentikan.", func(id int64) error {
		_, err := h.svc.Promosi.Hentikan(ctx(c), id)
		return err
	})
}
