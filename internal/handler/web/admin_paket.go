package web

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/service"
	"github.com/hermansyah/adojobsid/internal/view"
	"github.com/hermansyah/adojobsid/web/templates/pages"
)

// AdminPaket menampilkan daftar paket promosi beserta form tambah/ubah.
func (h *Handler) AdminPaket(c *fiber.Ctx) error {
	editID, _ := strconv.ParseInt(c.Query("edit"), 10, 64)

	form := view.NewForm()
	form.Set("jenis", model.PromosiSorotan)
	form.Set("bobot", "1")
	form.Set("aktif", "1")
	if editID > 0 {
		p, err := h.svc.Paket.Ambil(ctx(c), editID)
		if err != nil {
			return h.errorPage(c, err)
		}
		isiFormPaket(form, service.PaketInput{
			Jenis: p.Jenis, Nama: p.Nama, Deskripsi: p.Deskripsi, DurasiHari: p.DurasiHari,
			Harga: p.Harga, Bobot: p.Bobot, SlotIklan: p.SlotIklan, Aktif: p.Aktif, Urutan: p.Urutan,
		})
	}
	data, err := h.dataPaket(c, form, editID)
	if err != nil {
		return h.errorPage(c, err)
	}
	return h.render(c, fiber.StatusOK, pages.AdminPaket(*data))
}

func (h *Handler) dataPaket(c *fiber.Ctx, form view.Form, editID int64) (*view.AdminPaketData, error) {
	items, err := h.svc.Paket.Daftar(ctx(c), false)
	if err != nil {
		return nil, err
	}
	return &view.AdminPaketData{
		AdminBase: h.adminBase(c, "Paket promosi",
			"Durasi, harga, dan penempatan yang ditawarkan ke pengiklan", "paket"),
		Items:  items,
		Slot:   model.SlotIklanBawaan(),
		Jenis:  model.JenisPromosi,
		Form:   form,
		EditID: editID,
		IsEdit: editID > 0,
	}, nil
}

func isiFormPaket(form view.Form, in service.PaketInput) {
	form.Set("jenis", in.Jenis)
	form.Set("nama", in.Nama)
	form.Set("deskripsi", in.Deskripsi)
	form.Set("durasi_hari", strconv.Itoa(in.DurasiHari))
	form.Set("harga", strconv.FormatInt(in.Harga, 10))
	form.Set("bobot", strconv.Itoa(in.Bobot))
	form.Set("slot_iklan", strings.Join(in.SlotIklan, ","))
	form.Set("urutan", strconv.Itoa(in.Urutan))
	if in.Aktif {
		form.Set("aktif", "1")
	} else {
		form.Set("aktif", "")
	}
}

func bacaPaketInput(c *fiber.Ctx) service.PaketInput {
	durasi, _ := strconv.Atoi(c.FormValue("durasi_hari"))
	harga, _ := strconv.ParseInt(strings.ReplaceAll(c.FormValue("harga"), ".", ""), 10, 64)
	bobot, _ := strconv.Atoi(c.FormValue("bobot"))
	urutan, _ := strconv.Atoi(c.FormValue("urutan"))

	// Kotak centang slot dikirim sebagai beberapa nilai dengan nama yang sama.
	var slot []string
	for _, v := range c.Context().PostArgs().PeekMulti("slot_iklan") {
		if s := strings.TrimSpace(string(v)); s != "" {
			slot = append(slot, s)
		}
	}
	return service.PaketInput{
		Jenis:      c.FormValue("jenis"),
		Nama:       c.FormValue("nama"),
		Deskripsi:  c.FormValue("deskripsi"),
		DurasiHari: durasi,
		Harga:      harga,
		Bobot:      bobot,
		SlotIklan:  slot,
		Aktif:      c.FormValue("aktif") == "1",
		Urutan:     urutan,
	}
}

func (h *Handler) AdminPaketBuat(c *fiber.Ctx) error {
	in := bacaPaketInput(c)
	if _, err := h.svc.Paket.Buat(ctx(c), in); err != nil {
		return h.renderPaketGagal(c, in, 0, err)
	}
	return h.redirectWithFlash(c, "/admin/paket", "sukses", "Paket ditambahkan.")
}

func (h *Handler) AdminPaketUbah(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}
	in := bacaPaketInput(c)
	if _, err := h.svc.Paket.Ubah(ctx(c), id, in); err != nil {
		return h.renderPaketGagal(c, in, id, err)
	}
	return h.redirectWithFlash(c, "/admin/paket", "sukses", "Paket diperbarui.")
}

func (h *Handler) AdminPaketHapus(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}
	if err := h.svc.Paket.Hapus(ctx(c), id); err != nil {
		return h.redirectWithFlash(c, "/admin/paket", "galat", pesanKegagalan(err))
	}
	return h.redirectWithFlash(c, "/admin/paket", "sukses", "Paket dihapus.")
}

// renderPaketGagal merender ulang halaman beserta pesan kesalahan, tanpa
// menghilangkan isian yang sudah diketik admin.
func (h *Handler) renderPaketGagal(c *fiber.Ctx, in service.PaketInput, editID int64, err error) error {
	form := view.NewForm()
	isiFormPaket(form, in)
	data, derr := h.dataPaket(c, form, editID)
	if derr != nil {
		return h.errorPage(c, derr)
	}
	status := fiber.StatusInternalServerError
	if e, ok := service.AsError(err); ok {
		status = statusFor(e.Code)
		if e.Fields != nil {
			data.Form.Errors = e.Fields
		}
		if e.Code != service.CodeInvalidInput || len(e.Fields) == 0 {
			data.Flash = &view.Flash{Kind: "galat", Message: e.Message}
		}
	} else {
		data.Flash = &view.Flash{Kind: "galat", Message: "Terjadi kesalahan. Silakan coba lagi."}
	}
	return h.render(c, status, pages.AdminPaket(*data))
}
