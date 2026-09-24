package web

import (
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/hermansyah/adojobsid/internal/middleware"
	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/service"
	"github.com/hermansyah/adojobsid/internal/view"
	"github.com/hermansyah/adojobsid/web/templates/pages"
)

// providerUntukAksi menentukan provider mana yang diwakili pada satu aksi.
//
// Admin boleh mengelola listing milik siapa pun, tetapi pemeriksaan
// kepemilikan di service layer tetap dipertahankan: admin cukup bertindak
// atas nama provider pemilik listingnya. Dengan begitu tidak ada jalur
// terpisah yang melewati pemeriksaan sama sekali.
func (h *Handler) providerUntukAksi(c *fiber.Ctx, serviceID int64) (int64, error) {
	user := middleware.CurrentUser(c)
	if user == nil {
		return 0, service.Unauthorized("Silakan masuk terlebih dahulu.")
	}
	if !user.IsAdmin {
		if user.ProviderID == 0 {
			return 0, service.Forbidden("Akun Anda belum terdaftar sebagai penyedia jasa.")
		}
		return user.ProviderID, nil
	}

	svc, err := h.svc.Listing.GetDetail(ctx(c), serviceID)
	if err != nil {
		return 0, err
	}
	return svc.ProviderID, nil
}

// bertindakSebagaiAdmin menandai aksi yang dijalankan admin atas listing
// milik orang lain. Admin yang kebetulan juga penyedia jasa tetap
// diperlakukan sebagai provider biasa atas listingnya sendiri.
func (h *Handler) bertindakSebagaiAdmin(c *fiber.Ctx, providerID int64) bool {
	u := middleware.CurrentUser(c)
	return u != nil && u.IsAdmin && u.ProviderID != providerID
}

// ShowJasaBaru menampilkan form pembuatan listing.
func (h *Handler) ShowJasaBaru(c *fiber.Ctx) error {
	kategori, err := h.svc.Catalog.ListCategoryTree(ctx(c))
	if err != nil {
		return h.errorPage(c, err)
	}
	data := view.ListingFormData{
		Base:      h.base(c, "Pasang jasa", "Pasang listing jasa baru di Adojobs Bengkalis.", "jual"),
		Form:      view.NewForm(),
		Kategori:  kategori,
		MaxImages: h.svc.Upload.MaxPerListing(),
		MaxSizeMB: h.cfg.Upload.MaxSizeMB,
	}
	return h.render(c, fiber.StatusOK, pages.ListingForm(data))
}

// JasaBaru memproses pembuatan listing lalu mengarahkan ke form penyuntingan,
// tempat pengguna menambahkan foto.
func (h *Handler) JasaBaru(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	in := bacaListingInput(c)

	svc, err := h.svc.Listing.Create(ctx(c), user.ProviderID, in)
	if err != nil {
		return h.renderListingForm(c, false, 0, in, nil, err)
	}
	return h.redirectWithFlash(c, fmt.Sprintf("/jasa/%d/ubah", svc.ID), "sukses",
		"Listing tersimpan. Tambahkan foto pekerjaan agar lebih meyakinkan.")
}

// ShowJasaUbah menampilkan form penyuntingan listing beserta panel foto.
func (h *Handler) ShowJasaUbah(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}

	detail, err := h.svc.Listing.GetDetail(ctx(c), id)
	if err != nil {
		return h.errorPage(c, err)
	}
	// Admin boleh menyunting listing milik siapa pun; provider hanya miliknya.
	if detail.ProviderID != user.ProviderID && !user.IsAdmin {
		return h.renderError(c, fiber.StatusForbidden, "Akses ditolak",
			"Listing ini bukan milik akun Anda.")
	}

	kategori, err := h.svc.Catalog.ListCategoryTree(ctx(c))
	if err != nil {
		return h.errorPage(c, err)
	}

	form := view.NewForm()
	form.Set("title", detail.Title)
	form.Set("description", detail.Description)
	form.Set("category_id", strconv.FormatInt(detail.CategoryID, 10))
	form.Set("price_type", string(detail.PriceType))
	form.Set("status", string(detail.Status))
	if detail.PriceMin != nil {
		form.Set("price_min", view.Rupiah(*detail.PriceMin))
	}
	if detail.PriceMax != nil {
		form.Set("price_max", view.Rupiah(*detail.PriceMax))
	}

	data := view.ListingFormData{
		Base:         h.base(c, "Ubah "+detail.Title, "Ubah listing jasa Anda.", "jual"),
		Form:         form,
		IsEdit:       true,
		SebagaiAdmin: user.IsAdmin && detail.ProviderID != user.ProviderID,
		// Peringatan hanya relevan bagi provider: suntingan admin tidak
		// mengembalikan listing ke antrean.
		StatusSaatIni: detail.Status,
		ServiceID:     detail.ID,
		Kategori:      kategori,
		Images:        detail.Images,
		MaxImages:     h.svc.Upload.MaxPerListing(),
		MaxSizeMB:     h.cfg.Upload.MaxSizeMB,
	}
	return h.render(c, fiber.StatusOK, pages.ListingForm(data))
}

// JasaUbah memproses penyuntingan listing.
func (h *Handler) JasaUbah(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}

	in := bacaListingInput(c)

	// Admin yang menyunting listing orang lain memakai jalur terpisah:
	// suntingannya tidak mengembalikan listing ke antrean peninjauan,
	// karena admin sendirilah peninjaunya.
	svcLama, gerr := h.svc.Listing.GetDetail(ctx(c), id)
	if gerr != nil {
		return h.errorPage(c, gerr)
	}
	sebagaiAdmin := user.IsAdmin && svcLama.ProviderID != user.ProviderID

	if sebagaiAdmin {
		if _, err := h.svc.Admin.UbahListing(ctx(c), id, in, h.svc.Listing); err != nil {
			images, _ := h.svc.Listing.ListImages(ctx(c), id)
			return h.renderListingForm(c, true, id, in, images, err)
		}
		return h.redirectWithFlash(c, "/admin/jasa", "sukses", "Listing diperbarui oleh admin.")
	}

	hasil, err := h.svc.Listing.Update(ctx(c), user.ProviderID, id, in)
	if err != nil {
		images, _ := h.svc.Listing.ListImages(ctx(c), id)
		return h.renderListingForm(c, true, id, in, images, err)
	}
	if hasil.Status == model.ServicePending && svcLama.Status == model.ServiceActive {
		return h.redirectWithFlash(c, "/dasbor", "info",
			"Perubahan tersimpan. Karena bagian penting listing berubah, listing kembali menunggu peninjauan pengelola.")
	}
	return h.redirectWithFlash(c, "/dasbor", "sukses", "Listing berhasil diperbarui.")
}

// JasaHapus menghapus listing beserta seluruh fotonya.
func (h *Handler) JasaHapus(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}
	providerID, err := h.providerUntukAksi(c, id)
	if err != nil {
		return h.errorPage(c, err)
	}
	if err := h.svc.Listing.Delete(ctx(c), providerID, id); err != nil {
		return h.errorPage(c, err)
	}

	tujuan := "/dasbor"
	if u := middleware.CurrentUser(c); u != nil && u.IsAdmin {
		tujuan = "/admin/jasa"
	}
	return h.redirectWithFlash(c, tujuan, "sukses", "Listing dihapus.")
}

// FotoUnggah menerima unggahan foto dan mengembalikan fragmen daftar foto.
func (h *Handler) FotoUnggah(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}
	providerID, err := h.providerUntukAksi(c, id)
	if err != nil {
		return h.errorPage(c, err)
	}
	olehAdmin := h.bertindakSebagaiAdmin(c, providerID)

	form, err := c.MultipartForm()
	if err != nil {
		return h.fragmenFoto(c, id, "Berkas tidak terbaca. Coba unggah ulang.")
	}

	var pesanGagal string
	if files := form.File["foto"]; len(files) > 0 {
		if _, err := h.svc.Listing.AddImages(ctx(c), providerID, id, files, olehAdmin); err != nil {
			if e, ok := service.AsError(err); ok {
				pesanGagal = e.Message
			} else {
				pesanGagal = "Foto gagal diunggah. Coba lagi."
			}
		}
	}
	return h.fragmenFoto(c, id, pesanGagal)
}

// FotoHapus menghapus satu foto listing.
func (h *Handler) FotoHapus(c *fiber.Ctx) error {
	serviceID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}
	imageID, err := strconv.ParseInt(c.Params("imageID"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}
	providerID, err := h.providerUntukAksi(c, serviceID)
	if err != nil {
		return h.errorPage(c, err)
	}

	var pesanGagal string
	if err := h.svc.Listing.DeleteImage(ctx(c), providerID, imageID,
		h.bertindakSebagaiAdmin(c, providerID)); err != nil {
		if e, ok := service.AsError(err); ok {
			pesanGagal = e.Message
		} else {
			pesanGagal = "Foto gagal dihapus. Coba lagi."
		}
	}
	return h.fragmenFoto(c, serviceID, pesanGagal)
}

// fragmenFoto merender ulang panel foto sebagai fragmen HTMX.
func (h *Handler) fragmenFoto(c *fiber.Ctx, serviceID int64, pesanGagal string) error {
	images, err := h.svc.Listing.ListImages(ctx(c), serviceID)
	if err != nil {
		return h.errorPage(c, err)
	}

	form := view.NewForm()
	if pesanGagal != "" {
		form.Errors.Add("foto", pesanGagal)
	}

	data := view.ListingFormData{
		Base:      h.base(c, "", "", "jual"),
		Form:      form,
		IsEdit:    true,
		ServiceID: serviceID,
		Images:    images,
		MaxImages: h.svc.Upload.MaxPerListing(),
		MaxSizeMB: h.cfg.Upload.MaxSizeMB,
	}
	return h.render(c, fiber.StatusOK, pages.DaftarFoto(data))
}

// renderListingForm merender ulang form listing lengkap dengan pesan kesalahan.
func (h *Handler) renderListingForm(
	c *fiber.Ctx,
	isEdit bool,
	serviceID int64,
	in service.ListingInput,
	images []model.ServiceImage,
	err error,
) error {
	kategori, kerr := h.svc.Catalog.ListCategoryTree(ctx(c))
	if kerr != nil {
		return h.errorPage(c, kerr)
	}

	form := view.NewForm()
	form.Set("title", in.Title)
	form.Set("description", in.Description)
	form.Set("price_type", in.PriceType)
	form.Set("price_min", in.PriceMin)
	form.Set("price_max", in.PriceMax)
	form.Set("status", in.Status)
	if in.CategoryID > 0 {
		form.Set("category_id", strconv.FormatInt(in.CategoryID, 10))
	}

	data := view.ListingFormData{
		Base:      h.base(c, "Pasang jasa", "Form listing jasa.", "jual"),
		Form:      form,
		IsEdit:    isEdit,
		ServiceID: serviceID,
		Kategori:  kategori,
		Images:    images,
		MaxImages: h.svc.Upload.MaxPerListing(),
		MaxSizeMB: h.cfg.Upload.MaxSizeMB,
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
	return h.render(c, status, pages.ListingForm(data))
}

// bacaListingInput mengambil isian form listing dari request.
func bacaListingInput(c *fiber.Ctx) service.ListingInput {
	categoryID, _ := strconv.ParseInt(c.FormValue("category_id"), 10, 64)

	// Checkbox "sembunyikan" hanya terkirim saat dicentang.
	status := string(model.ServiceActive)
	if c.FormValue("status") == string(model.ServiceInactive) {
		status = string(model.ServiceInactive)
	}

	return service.ListingInput{
		CategoryID:  categoryID,
		Title:       c.FormValue("title"),
		Description: c.FormValue("description"),
		PriceType:   c.FormValue("price_type"),
		PriceMin:    c.FormValue("price_min"),
		PriceMax:    c.FormValue("price_max"),
		Status:      status,
	}
}
