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

// DaftarPenyedia menampilkan direktori penyedia jasa.
func (h *Handler) DaftarPenyedia(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	pengaturan := h.svc.Settings.Get(ctx(c))
	lokasi := h.lokasiAktif(c)
	lat, lng := lokasi.Koordinat()

	radius, _ := strconv.Atoi(c.Query("radius"))
	if radius < 0 || radius > pengaturan.Lokasi.RadiusMaksKm {
		radius = 0
	}

	in := service.CariPenyediaInput{
		Query:              strings.TrimSpace(c.Query("q")),
		Kecamatan:          strings.TrimSpace(c.Query("kecamatan")),
		HanyaTerverifikasi: c.Query("terverifikasi") == "1",
		HanyaBerjasa:       c.Query("berjasa") == "1",
		Sort:               strings.TrimSpace(c.Query("urut")),
		Page:               page,
		Latitude:           lat,
		Longitude:          lng,
		RadiusKm:           radius,
	}

	hasil, err := h.svc.Provider.Cari(ctx(c), in)
	if err != nil {
		return h.errorPage(c, err)
	}
	kecamatan, err := h.svc.Provider.KecamatanPenyedia(ctx(c))
	if err != nil {
		return h.errorPage(c, err)
	}

	return h.render(c, fiber.StatusOK, pages.DaftarPenyedia(view.DaftarPenyediaData{
		Base: h.base(c, "Penyedia jasa",
			"Daftar penyedia jasa di Kabupaten Bengkalis: tukang, teknisi, kebersihan, dekorasi, dan dokumentasi.",
			"cari"),
		Query:              in.Query,
		Kecamatan:          in.Kecamatan,
		HanyaTerverifikasi: in.HanyaTerverifikasi,
		HanyaBerjasa:       in.HanyaBerjasa,
		Sort:               in.Sort,
		RadiusKm:           in.RadiusKm,
		RadiusMaks:         pengaturan.Lokasi.RadiusMaksKm,
		Items:              hasil.Items,
		Total:              hasil.Total,
		Page:               hasil.Page,
		TotalPages:         hasil.TotalPages,
		KecamatanOps:       kecamatan,
	}))
}

// Penyedia menampilkan halaman publik satu penyedia jasa.
func (h *Handler) Penyedia(c *fiber.Ctx) error {
	slug := strings.TrimSpace(c.Params("slug"))
	if slug == "" {
		return h.notFound(c)
	}

	provider, err := h.svc.Provider.GetDetailBySlug(ctx(c), slug)
	if err != nil {
		return h.errorPage(c, err)
	}

	user := middleware.CurrentUser(c)
	isOwner := user != nil && user.ProviderID == provider.ID
	isAdmin := user != nil && user.IsAdmin

	// Penyedia yang ditangguhkan hilang dari halaman publik, sama seperti
	// listing-listingnya di hasil pencarian.
	if provider.IsSuspended() && !isOwner && !isAdmin {
		return h.notFound(c)
	}

	// Pemilik dan admin melihat seluruh listing termasuk yang belum tayang,
	// supaya halaman ini bisa dipakai memeriksa keadaan sebenarnya.
	listings, err := h.svc.Listing.ListByProvider(ctx(c), provider.ID, isOwner || isAdmin)
	if err != nil {
		return h.errorPage(c, err)
	}

	portofolio, err := h.svc.Portfolio.Daftar(ctx(c), provider.ID, 0)
	if err != nil {
		return h.errorPage(c, err)
	}
	ulasan, err := h.svc.Review.Ringkasan(ctx(c), provider, 20)
	if err != nil {
		return h.errorPage(c, err)
	}

	// Jarak dihitung di aplikasi karena hanya satu titik.
	lokasi := h.lokasiAktif(c)
	var jarak *float64
	if provider.PunyaLokasi() {
		km := model.JarakKm(lokasi.Latitude, lokasi.Longitude,
			*provider.Latitude, *provider.Longitude)
		jarak = &km
	}

	keterangan := view.Potong(view.Deref(provider.Bio), 155)
	if keterangan == "" {
		keterangan = "Penyedia jasa di " + view.Lokasi(provider.Kecamatan, provider.City) + "."
	}

	return h.render(c, fiber.StatusOK, pages.Penyedia(view.PenyediaData{
		Base:       h.base(c, provider.FullName, keterangan, "cari").DenganPeta(),
		Provider:   provider,
		Listings:   listings,
		Portofolio: portofolio,
		Ulasan:     ulasan,
		JarakKm:    jarak,
		IsOwner:    isOwner,
		IsAdmin:    isAdmin,
	}))
}

// ---------- kelola portofolio ----------

func (h *Handler) dataPortofolio(c *fiber.Ctx, form view.Form) (*view.PortofolioData, error) {
	user := middleware.CurrentUser(c)

	items, err := h.svc.Portfolio.Daftar(ctx(c), user.ProviderID, 0)
	if err != nil {
		return nil, err
	}
	return &view.PortofolioData{
		Base: h.base(c, "Portofolio",
			"Susun foto hasil pekerjaan yang pernah Anda selesaikan.", "akun"),
		Items:      items,
		Form:       form,
		MaksKarya:  h.svc.Portfolio.MaksKarya(),
		MaksSizeMB: h.cfg.Upload.MaxSizeMB,
	}, nil
}

func (h *Handler) ShowPortofolio(c *fiber.Ctx) error {
	data, err := h.dataPortofolio(c, view.NewForm())
	if err != nil {
		return h.errorPage(c, err)
	}
	return h.render(c, fiber.StatusOK, pages.KelolaPortofolio(*data))
}

func (h *Handler) TambahPortofolio(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	in := service.PortofolioInput{
		Judul:       c.FormValue("title"),
		CompletedAt: c.FormValue("completed_at"),
	}

	// Berkas yang tidak terbaca ditangani service layer sebagai isian kosong,
	// supaya pesan kesalahannya muncul di field yang tepat.
	fh, _ := c.FormFile("gambar")

	if _, err := h.svc.Portfolio.Tambah(ctx(c), user.ProviderID, in, fh); err != nil {
		form := view.NewForm()
		form.Set("title", in.Judul)
		form.Set("completed_at", in.CompletedAt)

		data, derr := h.dataPortofolio(c, form)
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
		return h.render(c, status, pages.KelolaPortofolio(*data))
	}
	return h.redirectWithFlash(c, "/provider/portofolio", "sukses", "Karya ditambahkan ke portofolio.")
}

func (h *Handler) HapusPortofolio(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}

	if err := h.svc.Portfolio.Hapus(ctx(c), user.ProviderID, id); err != nil {
		return h.redirectWithFlash(c, "/provider/portofolio", "galat", pesanKegagalan(err))
	}
	return h.redirectWithFlash(c, "/provider/portofolio", "sukses", "Karya dihapus.")
}

// ---------- ulasan ----------

// TulisUlasan menyimpan penilaian pemesan atas pekerjaan yang sudah selesai.
func (h *Handler) TulisUlasan(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	orderID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}
	rating, _ := strconv.Atoi(c.FormValue("rating"))
	convID, _ := strconv.ParseInt(c.FormValue("conversation_id"), 10, 64)

	tujuan := "/pesan"
	if convID > 0 {
		tujuan = "/pesan/" + strconv.FormatInt(convID, 10)
	}

	_, err = h.svc.Review.Tulis(ctx(c), user.ID, service.UlasanInput{
		OrderID:  orderID,
		Rating:   rating,
		Komentar: c.FormValue("comment"),
	})
	if err != nil {
		return h.redirectWithFlash(c, tujuan, "galat", pesanKegagalan(err))
	}
	return h.redirectWithFlash(c, tujuan, "sukses",
		"Terima kasih. Ulasan Anda membantu pengguna lain memilih.")
}
