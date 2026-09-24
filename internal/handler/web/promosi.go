package web

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/hermansyah/adojobsid/internal/middleware"
	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/service"
	"github.com/hermansyah/adojobsid/internal/view"
	"github.com/hermansyah/adojobsid/web/templates/pages"
)

// Promosi menampilkan seluruh pengajuan milik pengguna.
func (h *Handler) Promosi(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	items, err := h.svc.Promosi.DaftarPengguna(ctx(c), user.ID)
	if err != nil {
		return h.errorPage(c, err)
	}
	semuaPaket, err := h.svc.Paket.Daftar(ctx(c), false)
	if err != nil {
		return h.errorPage(c, err)
	}
	paket := make(map[int64]model.PaketPromosi, len(semuaPaket))
	for _, p := range semuaPaket {
		paket[p.ID] = p
	}
	return h.render(c, fiber.StatusOK, pages.DaftarPromosi(view.PromosiData{
		Base:       h.base(c, "Promosi", "Iklan, sorotan jasa, dan penyedia pilihan yang Anda ajukan.", "akun"),
		Items:      items,
		Paket:      paket,
		IsProvider: user.ProviderID > 0,
	}))
}

// jenisDariQuery membaca jenis dari URL; tanpa jenis, penyedia diarahkan ke
// sorotan jasa dan pengguna biasa ke iklan — pilihan yang paling mungkin.
func jenisDariQuery(c *fiber.Ctx, user *view.CurrentUser) (string, bool) {
	jenis := strings.TrimSpace(c.Query("jenis"))
	if jenis == "" {
		if user.ProviderID > 0 {
			return model.PromosiSorotan, true
		}
		return model.PromosiIklan, true
	}
	for _, j := range model.JenisPromosi {
		if j == jenis {
			return jenis, true
		}
	}
	return "", false
}

func (h *Handler) dataPromosiBaru(c *fiber.Ctx, jenis string, form view.Form) (*view.PromosiBaruData, error) {
	user := middleware.CurrentUser(c)

	semua, err := h.svc.Paket.Daftar(ctx(c), true)
	if err != nil {
		return nil, err
	}
	paket := make([]model.PaketPromosi, 0, len(semua))
	for _, p := range semua {
		if p.Jenis == jenis {
			paket = append(paket, p)
		}
	}

	var listings []model.ServiceCard
	if jenis == model.PromosiSorotan && user.ProviderID > 0 {
		listings, err = h.svc.Listing.ListByProvider(ctx(c), user.ProviderID, false)
		if err != nil {
			return nil, err
		}
	}

	return &view.PromosiBaruData{
		Base:      h.base(c, "Ajukan "+strings.ToLower(model.LabelJenisPromosi(jenis)), "Pilih paket lalu lengkapi isiannya.", "akun"),
		Jenis:     jenis,
		Paket:     paket,
		Listings:  listings,
		Kecamatan: model.KecamatanBengkalis,
		Form:      form,
	}, nil
}

// PromosiBaru menampilkan form pengajuan sesuai jenis.
func (h *Handler) PromosiBaru(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	jenis, ok := jenisDariQuery(c, user)
	if !ok {
		return h.notFound(c)
	}
	// Sorotan dan penyedia pilihan hanya untuk penyedia; pengguna biasa
	// diarahkan membuat profil dulu, bukan disuguhi form yang pasti ditolak.
	if jenis != model.PromosiIklan && user.ProviderID == 0 {
		return h.redirectWithFlash(c, "/provider/daftar", "info",
			"Buat profil penyedia dulu untuk mengajukan "+strings.ToLower(model.LabelJenisPromosi(jenis))+".")
	}

	form := view.NewForm()
	form.Set("service_id", c.Query("service_id"))
	data, err := h.dataPromosiBaru(c, jenis, form)
	if err != nil {
		return h.errorPage(c, err)
	}
	return h.render(c, fiber.StatusOK, pages.PromosiBaru(*data))
}

func bacaAjukanInput(c *fiber.Ctx) service.AjukanInput {
	paketID, _ := strconv.ParseInt(c.FormValue("paket_id"), 10, 64)
	serviceID, _ := strconv.ParseInt(c.FormValue("service_id"), 10, 64)
	var target []string
	for _, v := range c.Context().PostArgs().PeekMulti("target_kecamatan") {
		if s := strings.TrimSpace(string(v)); s != "" {
			target = append(target, s)
		}
	}
	// Form multipart (iklan) menaruh nilainya di bagian form, bukan PostArgs.
	if len(target) == 0 {
		if mf, err := c.MultipartForm(); err == nil && mf != nil {
			target = append(target, mf.Value["target_kecamatan"]...)
		}
	}
	return service.AjukanInput{
		Jenis:           c.FormValue("jenis"),
		PaketID:         paketID,
		ServiceID:       serviceID,
		Alasan:          c.FormValue("alasan"),
		Judul:           c.FormValue("judul"),
		Deskripsi:       c.FormValue("deskripsi"),
		TautanURL:       c.FormValue("tautan_url"),
		TargetKecamatan: target,
	}
}

// PromosiAjukan memproses pengajuan.
func (h *Handler) PromosiAjukan(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	in := bacaAjukanInput(c)
	gambar, _ := c.FormFile("gambar")

	p, err := h.svc.Promosi.Ajukan(ctx(c), user.ID, user.ProviderID, in, gambar)
	if err != nil {
		jenis := in.Jenis
		if _, ok := jenisDariQuery(c, user); !ok || jenis == "" {
			jenis, _ = jenisDariQuery(c, user)
		}
		form := view.NewForm()
		form.Set("paket_id", c.FormValue("paket_id"))
		form.Set("service_id", c.FormValue("service_id"))
		form.Set("alasan", in.Alasan)
		form.Set("judul", in.Judul)
		form.Set("deskripsi", in.Deskripsi)
		form.Set("tautan_url", in.TautanURL)
		form.Set("target_kecamatan", strings.Join(in.TargetKecamatan, ","))

		data, derr := h.dataPromosiBaru(c, jenis, form)
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
		return h.render(c, status, pages.PromosiBaru(*data))
	}
	return h.redirectWithFlash(c, "/promosi/"+strconv.FormatInt(p.ID, 10), "sukses",
		"Pengajuan terkirim. Pengelola akan meninjaunya.")
}

func (h *Handler) dataPromosiDetail(c *fiber.Ctx, p *model.Promosi) (*view.PromosiDetailData, error) {
	data := &view.PromosiDetailData{
		Base:       h.base(c, model.LabelJenisPromosi(p.Jenis)+" #"+strconv.FormatInt(p.ID, 10), "Status dan rincian pengajuan promosi Anda.", "akun"),
		Promosi:    *p,
		Pembayaran: h.svc.Settings.Get(ctx(c)).Promosi,
	}
	if p.PaketID != nil {
		if paket, err := h.svc.Paket.Ambil(ctx(c), *p.PaketID); err == nil {
			data.Paket = paket
		}
	}
	if p.ServiceID != nil {
		if d, err := h.svc.Listing.GetDetail(ctx(c), *p.ServiceID); err == nil {
			data.JudulJasa = d.Title
		}
	}
	return data, nil
}

// PromosiDetail menampilkan satu pengajuan milik pengguna.
func (h *Handler) PromosiDetail(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}
	p, err := h.svc.Promosi.AmbilMilik(ctx(c), user.ID, id)
	if err != nil {
		return h.errorPage(c, err)
	}
	data, err := h.dataPromosiDetail(c, p)
	if err != nil {
		return h.errorPage(c, err)
	}
	return h.render(c, fiber.StatusOK, pages.PromosiDetail(*data))
}

// PromosiBukti menerima bukti transfer.
func (h *Handler) PromosiBukti(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}
	tujuan := "/promosi/" + strconv.FormatInt(id, 10)
	fh, err := c.FormFile("bukti")
	if err != nil {
		return h.redirectWithFlash(c, tujuan, "galat", "Berkas bukti tidak terbaca.")
	}
	if _, err := h.svc.Promosi.UnggahBukti(ctx(c), user.ID, id, fh); err != nil {
		return h.redirectWithFlash(c, tujuan, "galat", pesanKegagalan(err))
	}
	return h.redirectWithFlash(c, tujuan, "sukses", "Bukti pembayaran terkirim. Pengelola akan mengonfirmasinya.")
}

// PromosiBatal membatalkan pengajuan yang belum tayang.
func (h *Handler) PromosiBatal(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}
	tujuan := "/promosi/" + strconv.FormatInt(id, 10)
	if _, err := h.svc.Promosi.Batalkan(ctx(c), user.ID, id); err != nil {
		return h.redirectWithFlash(c, tujuan, "galat", pesanKegagalan(err))
	}
	return h.redirectWithFlash(c, tujuan, "sukses", "Pengajuan dibatalkan.")
}

// KlikIklan mencatat klik lalu mengalihkan ke tautan pengiklan.
func (h *Handler) KlikIklan(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}
	tautan, err := h.svc.Promosi.Klik(ctx(c), id)
	if err != nil {
		return h.errorPage(c, err)
	}
	return c.Redirect(tautan, fiber.StatusFound)
}
