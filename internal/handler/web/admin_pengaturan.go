package web

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/hermansyah/adojobsid/internal/middleware"
	"github.com/hermansyah/adojobsid/internal/service"
	"github.com/hermansyah/adojobsid/internal/view"
	"github.com/hermansyah/adojobsid/web/templates/pages"
)

// ---------- antrean peninjauan ----------

func (h *Handler) dataAntrean(c *fiber.Ctx) (*view.AdminAntreanData, error) {
	page, _ := strconv.Atoi(orDefault(paramFilter(c, "page"), "1"))

	hasil, err := h.svc.Admin.AntreanPeninjauan(ctx(c), page)
	if err != nil {
		return nil, err
	}
	return &view.AdminAntreanData{
		AdminBase: h.adminBase(c, "Antrean peninjauan",
			"Listing baru dan perubahan penting yang menunggu keputusan", "antrean"),
		Items:      hasil.Items,
		Total:      hasil.Total,
		Page:       hasil.Page,
		TotalPages: hasil.TotalPages,
	}, nil
}

func (h *Handler) AdminAntrean(c *fiber.Ctx) error {
	data, err := h.dataAntrean(c)
	if err != nil {
		return h.errorPage(c, err)
	}
	return h.render(c, fiber.StatusOK, pages.AdminAntrean(*data))
}

// aksiAntrean menjalankan satu keputusan lalu mengembalikan antrean
// yang sudah disegarkan.
func (h *Handler) aksiAntrean(c *fiber.Ctx, jalankan func(serviceID int64) error) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}

	var pesan string
	if err := jalankan(id); err != nil {
		pesan = pesanKegagalan(err)
	}

	data, derr := h.dataAntrean(c)
	if derr != nil {
		return h.errorPage(c, derr)
	}
	if pesan != "" {
		return h.render(c, fiber.StatusOK, pages.AdminAksiGagal(pesan, pages.AdminAntreanIsi(*data)))
	}
	return h.render(c, fiber.StatusOK, pages.AdminAntreanIsi(*data))
}

func (h *Handler) AdminSetujuiListing(c *fiber.Ctx) error {
	admin := middleware.CurrentUser(c)
	return h.aksiAntrean(c, func(serviceID int64) error {
		return h.svc.Admin.SetujuiListing(ctx(c), admin.ID, serviceID)
	})
}

func (h *Handler) AdminTolakListing(c *fiber.Ctx) error {
	admin := middleware.CurrentUser(c)
	alasan := c.FormValue("alasan")
	return h.aksiAntrean(c, func(serviceID int64) error {
		return h.svc.Admin.TolakListing(ctx(c), admin.ID, serviceID, alasan)
	})
}

// ---------- pengaturan aplikasi ----------

func (h *Handler) dataPengaturan(c *fiber.Ctx, form view.Form) view.AdminPengaturanData {
	return view.AdminPengaturanData{
		AdminBase: h.adminBase(c, "Pengaturan",
			"Identitas platform, lokasi, dan slot iklan", "pengaturan").DenganPeta(),
		Pengaturan: h.svc.Settings.Get(ctx(c)),
		Form:       form,
		MaksSizeMB: h.cfg.Upload.MaxSizeMB,
	}
}

func (h *Handler) AdminPengaturan(c *fiber.Ctx) error {
	return h.render(c, fiber.StatusOK, pages.AdminPengaturan(h.dataPengaturan(c, view.NewForm())))
}

// renderPengaturanGagal merender ulang halaman pengaturan dengan pesan kesalahan.
func (h *Handler) renderPengaturanGagal(c *fiber.Ctx, err error) error {
	data := h.dataPengaturan(c, view.NewForm())

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
	return h.render(c, status, pages.AdminPengaturan(data))
}

func (h *Handler) AdminPengaturanUmum(c *fiber.Ctx) error {
	admin := middleware.CurrentUser(c)
	in := service.PengaturanUmumInput{
		NamaSitus:  c.FormValue("nama_situs"),
		Tagline:    c.FormValue("tagline"),
		KontakWA:   c.FormValue("kontak_wa"),
		TeksFooter: c.FormValue("teks_footer"),
		// Checkbox yang tidak dicentang tidak dikirim browser sama sekali,
		// jadi ketiadaan nilai berarti mati.
		WhatsappAktif: c.FormValue("whatsapp_aktif") == "1",
	}
	if err := h.svc.Settings.SimpanUmum(ctx(c), admin.ID, in); err != nil {
		return h.renderPengaturanGagal(c, err)
	}
	return h.redirectWithFlash(c, "/admin/pengaturan", "sukses", "Identitas platform tersimpan.")
}

func (h *Handler) AdminPengaturanLogo(c *fiber.Ctx) error {
	admin := middleware.CurrentUser(c)

	fh, err := c.FormFile("logo")
	if err != nil {
		return h.redirectWithFlash(c, "/admin/pengaturan", "galat", "Berkas logo tidak terbaca.")
	}
	if err := h.svc.Settings.SimpanLogo(ctx(c), admin.ID, fh); err != nil {
		return h.redirectWithFlash(c, "/admin/pengaturan", "galat", pesanKegagalan(err))
	}
	return h.redirectWithFlash(c, "/admin/pengaturan", "sukses", "Logo diperbarui.")
}

func (h *Handler) AdminPengaturanLogoHapus(c *fiber.Ctx) error {
	admin := middleware.CurrentUser(c)
	if err := h.svc.Settings.HapusLogo(ctx(c), admin.ID); err != nil {
		return h.redirectWithFlash(c, "/admin/pengaturan", "galat", pesanKegagalan(err))
	}
	return h.redirectWithFlash(c, "/admin/pengaturan", "sukses", "Logo dihapus.")
}

func (h *Handler) AdminPengaturanLokasi(c *fiber.Ctx) error {
	admin := middleware.CurrentUser(c)

	lat, _ := strconv.ParseFloat(c.FormValue("pusat_latitude"), 64)
	lng, _ := strconv.ParseFloat(c.FormValue("pusat_longitude"), 64)
	def, _ := strconv.Atoi(c.FormValue("radius_default_km"))
	maks, _ := strconv.Atoi(c.FormValue("radius_maks_km"))

	in := service.PengaturanLokasiInput{
		PusatLatitude:   lat,
		PusatLongitude:  lng,
		RadiusDefaultKm: def,
		RadiusMaksKm:    maks,
	}
	if err := h.svc.Settings.SimpanLokasi(ctx(c), admin.ID, in); err != nil {
		return h.renderPengaturanGagal(c, err)
	}
	return h.redirectWithFlash(c, "/admin/pengaturan", "sukses", "Pengaturan lokasi tersimpan.")
}

// ---------- tampilan beranda ----------

func (h *Handler) AdminPengaturanTampilan(c *fiber.Ctx) error {
	admin := middleware.CurrentUser(c)
	overlay, _ := strconv.Atoi(c.FormValue("hero_overlay"))

	in := service.PengaturanTampilanInput{
		HeroOverlay:        overlay,
		AplikasiTampil:     c.FormValue("aplikasi_tampil") == "1",
		PlaystoreURL:       c.FormValue("playstore_url"),
		PlaystoreTeksAtas:  c.FormValue("playstore_teks_atas"),
		PlaystoreTeksBawah: c.FormValue("playstore_teks_bawah"),
	}
	if err := h.svc.Settings.SimpanTampilan(ctx(c), admin.ID, in); err != nil {
		return h.renderPengaturanGagal(c, err)
	}
	return h.redirectWithFlash(c, "/admin/pengaturan", "sukses", "Tampilan beranda tersimpan.")
}

func (h *Handler) AdminPengaturanHero(c *fiber.Ctx) error {
	admin := middleware.CurrentUser(c)

	fh, err := c.FormFile("hero")
	if err != nil {
		return h.redirectWithFlash(c, "/admin/pengaturan", "galat", "Berkas gambar tidak terbaca.")
	}
	if err := h.svc.Settings.SimpanHero(ctx(c), admin.ID, fh); err != nil {
		return h.redirectWithFlash(c, "/admin/pengaturan", "galat", pesanKegagalan(err))
	}
	return h.redirectWithFlash(c, "/admin/pengaturan", "sukses", "Gambar latar beranda diperbarui.")
}

func (h *Handler) AdminPengaturanHeroHapus(c *fiber.Ctx) error {
	admin := middleware.CurrentUser(c)
	if err := h.svc.Settings.HapusHero(ctx(c), admin.ID); err != nil {
		return h.redirectWithFlash(c, "/admin/pengaturan", "galat", pesanKegagalan(err))
	}
	return h.redirectWithFlash(c, "/admin/pengaturan", "sukses", "Gambar latar beranda dihapus.")
}

// ---------- slot iklan ----------

// AdminSlotIklan menyimpan satu slot sekaligus materinya. Satu form, satu
// tombol: pemisahan teks dan berkas sebelumnya membuat admin memilih gambar
// lalu menekan tombol simpan yang salah, dan berkasnya hilang tanpa pesan.
func (h *Handler) AdminSlotIklan(c *fiber.Ctx) error {
	admin := middleware.CurrentUser(c)

	in := service.SlotIklanInput{
		Aktif:     c.FormValue("aktif") == "1",
		TautanURL: c.FormValue("tautan_url"),
		Teks:      c.FormValue("teks"),
	}

	// Berkas kosong berarti admin hanya mengubah teks; itu bukan kesalahan.
	fh, err := c.FormFile("gambar")
	if err != nil {
		fh = nil
	}

	if err := h.svc.Settings.SimpanSlotIklan(ctx(c), admin.ID, c.Params("kunci"), in, fh); err != nil {
		return h.redirectWithFlash(c, "/admin/pengaturan", "galat", pesanKegagalan(err))
	}
	return h.redirectWithFlash(c, "/admin/pengaturan", "sukses", "Slot iklan tersimpan.")
}

func (h *Handler) AdminIklanGambarHapus(c *fiber.Ctx) error {
	admin := middleware.CurrentUser(c)
	if err := h.svc.Settings.HapusGambarIklan(ctx(c), admin.ID, c.Params("kunci")); err != nil {
		return h.redirectWithFlash(c, "/admin/pengaturan", "galat", pesanKegagalan(err))
	}
	return h.redirectWithFlash(c, "/admin/pengaturan", "sukses", "Materi iklan dihapus.")
}

// ---------- pembayaran promosi ----------

func (h *Handler) AdminPengaturanPromosi(c *fiber.Ctx) error {
	admin := middleware.CurrentUser(c)
	in := service.PengaturanPromosiInput{
		UjiCobaGratis: c.FormValue("uji_coba_gratis") == "1",
		Bank:          c.FormValue("bank"),
		NomorRekening: c.FormValue("nomor_rekening"),
		AtasNama:      c.FormValue("atas_nama"),
		PetunjukBayar: c.FormValue("petunjuk_bayar"),
	}
	if err := h.svc.Settings.SimpanPromosi(ctx(c), admin.ID, in); err != nil {
		return h.renderPengaturanGagal(c, err)
	}
	return h.redirectWithFlash(c, "/admin/pengaturan", "sukses", "Pengaturan promosi tersimpan.")
}
