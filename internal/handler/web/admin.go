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

// paramFilter membaca satu nilai filter dari query string, lalu dari body form.
// Tindakan moderasi dikirim sebagai POST dengan hx-include, sehingga nilai
// filternya ada di body — bukan di query string seperti pada permintaan GET.
// Tanpa ini, tabel akan kembali menampilkan seluruh data setiap kali admin
// menjalankan satu tindakan.
func paramFilter(c *fiber.Ctx, key string) string {
	if v := strings.TrimSpace(c.Query(key)); v != "" {
		return v
	}
	return strings.TrimSpace(c.FormValue(key))
}

// adminBase menyusun data bersama seluruh halaman panel admin.
func (h *Handler) adminBase(c *fiber.Ctx, title, description, menu string) view.AdminBase {
	return view.AdminBase{
		Base:           h.base(c, title, description, "akun"),
		ActiveMenu:     menu,
		Antrean:        h.svc.Admin.JumlahAntrean(ctx(c)),
		AntreanPromosi: h.svc.Promosi.JumlahPerluTindakan(ctx(c)),
	}
}

// ---------- dasbor ----------

func (h *Handler) AdminDasbor(c *fiber.Ctx) error {
	data, err := h.svc.Admin.Dashboard(ctx(c))
	if err != nil {
		return h.errorPage(c, err)
	}
	return h.render(c, fiber.StatusOK, pages.AdminDashboard(view.AdminDashboardData{
		AdminBase: h.adminBase(c, "Ringkasan", "Keadaan platform hari ini", "dasbor"),
		Stats:     data.Stats,
		Kategori:  data.Kategori,
		Terbaru:   data.Terbaru,
		Promosi:   data.Promosi,
	}))
}

// ---------- pengguna ----------

// dataPengguna membaca filter dari query string lalu menjalankan pencarian.
func (h *Handler) dataPengguna(c *fiber.Ctx) (*view.AdminUsersData, error) {
	page, _ := strconv.Atoi(orDefault(paramFilter(c, "page"), "1"))
	filter := repository.UserFilter{
		Query:     paramFilter(c, "q"),
		Role:      paramFilter(c, "peran"),
		Status:    paramFilter(c, "status"),
		OnlyType:  paramFilter(c, "tipe"),
		Kecamatan: paramFilter(c, "kecamatan"),
		Sort:      paramFilter(c, "urut"),
	}

	hasil, err := h.svc.Admin.ListUsers(ctx(c), filter, page)
	if err != nil {
		return nil, err
	}
	kecamatan, err := h.svc.Catalog.KecamatanOptions(ctx(c))
	if err != nil {
		return nil, err
	}

	return &view.AdminUsersData{
		AdminBase:  h.adminBase(c, "Pengguna", "Kelola akun pencari jasa, penyedia, dan admin", "pengguna"),
		Filter:     filter,
		Items:      hasil.Items,
		Total:      hasil.Total,
		Page:       hasil.Page,
		TotalPages: hasil.TotalPages,
		Kecamatan:  kecamatan,
		Durasi:     view.DurasiSorotOptions(),
		AdminID:    middleware.CurrentUser(c).ID,
	}, nil
}

func (h *Handler) AdminPengguna(c *fiber.Ctx) error {
	data, err := h.dataPengguna(c)
	if err != nil {
		return h.errorPage(c, err)
	}
	return h.render(c, fiber.StatusOK, pages.AdminUsers(*data))
}

// AdminPenggunaTabel mengembalikan fragmen tabel saja, dipakai HTMX.
func (h *Handler) AdminPenggunaTabel(c *fiber.Ctx) error {
	data, err := h.dataPengguna(c)
	if err != nil {
		return h.errorPage(c, err)
	}
	if !middleware.IsHTMX(c) {
		return h.render(c, fiber.StatusOK, pages.AdminUsers(*data))
	}
	return h.render(c, fiber.StatusOK, pages.AdminUsersTabel(*data))
}

// aksiPengguna menjalankan satu tindakan lalu mengembalikan tabel yang sudah
// disegarkan, sehingga admin langsung melihat hasilnya tanpa memuat ulang halaman.
func (h *Handler) aksiPengguna(c *fiber.Ctx, jalankan func(userID int64) error) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}

	var pesan string
	if err := jalankan(id); err != nil {
		pesan = pesanKegagalan(err)
	}

	data, derr := h.dataPengguna(c)
	if derr != nil {
		return h.errorPage(c, derr)
	}
	if pesan != "" {
		data.Flash = &view.Flash{Kind: "galat", Message: pesan}
		// Pesan kegagalan disisipkan di atas tabel, karena fragmen yang ditukar
		// hanya bagian tabel dan tidak memuat area flash halaman.
		return h.render(c, fiber.StatusOK, pages.AdminAksiGagal(pesan, pages.AdminUsersTabel(*data)))
	}
	return h.render(c, fiber.StatusOK, pages.AdminUsersTabel(*data))
}

func (h *Handler) AdminTangguhkan(c *fiber.Ctx) error {
	admin := middleware.CurrentUser(c)
	return h.aksiPengguna(c, func(userID int64) error {
		return h.svc.Admin.Suspend(ctx(c), admin.ID, userID, c.FormValue("reason"))
	})
}

func (h *Handler) AdminAktifkan(c *fiber.Ctx) error {
	return h.aksiPengguna(c, func(userID int64) error {
		return h.svc.Admin.Reactivate(ctx(c), userID)
	})
}

func (h *Handler) AdminUbahPeran(c *fiber.Ctx) error {
	admin := middleware.CurrentUser(c)
	role := model.Role(paramFilter(c, "role"))
	return h.aksiPengguna(c, func(userID int64) error {
		return h.svc.Admin.SetRole(ctx(c), admin.ID, userID, role)
	})
}

// ---------- penyedia ----------

func (h *Handler) AdminVerifikasi(c *fiber.Ctx) error {
	// Nilainya datang lewat query string: hx-vals tidak bisa dipakai karena
	// htmx mem-parsingnya dengan Function(), yang diblokir CSP.
	verified := paramFilter(c, "verified") == "true"
	return h.aksiPengguna(c, func(providerID int64) error {
		return h.svc.Admin.SetVerified(ctx(c), providerID, verified)
	})
}

func (h *Handler) AdminSorotPenyedia(c *fiber.Ctx) error {
	hari, _ := strconv.Atoi(paramFilter(c, "hari"))
	admin := middleware.CurrentUser(c)
	return h.aksiPengguna(c, func(providerID int64) error {
		return h.svc.Admin.SetProviderFeatured(ctx(c), admin.ID, providerID, hari)
	})
}

// ---------- listing ----------

func (h *Handler) dataJasaAdmin(c *fiber.Ctx) (*view.AdminServicesData, error) {
	page, _ := strconv.Atoi(orDefault(paramFilter(c, "page"), "1"))
	categoryID, _ := strconv.ParseInt(paramFilter(c, "kategori"), 10, 64)

	filter := repository.AdminServiceFilter{
		Query:      paramFilter(c, "q"),
		Status:     paramFilter(c, "status"),
		Featured:   paramFilter(c, "sorot"),
		TanpaFoto:  paramFilter(c, "tanpa_foto") == "1",
		CategoryID: categoryID,
		Sort:       paramFilter(c, "urut"),
	}

	hasil, err := h.svc.Admin.ListServices(ctx(c), filter, page)
	if err != nil {
		return nil, err
	}
	kategori, err := h.svc.Catalog.ListRootCategories(ctx(c))
	if err != nil {
		return nil, err
	}

	return &view.AdminServicesData{
		AdminBase:  h.adminBase(c, "Listing jasa", "Moderasi seluruh listing lintas penyedia", "jasa"),
		Filter:     filter,
		Items:      hasil.Items,
		Total:      hasil.Total,
		Page:       hasil.Page,
		TotalPages: hasil.TotalPages,
		Kategori:   kategori,
		Durasi:     view.DurasiSorotOptions(),
	}, nil
}

func (h *Handler) AdminJasa(c *fiber.Ctx) error {
	data, err := h.dataJasaAdmin(c)
	if err != nil {
		return h.errorPage(c, err)
	}
	return h.render(c, fiber.StatusOK, pages.AdminServices(*data))
}

func (h *Handler) AdminJasaTabel(c *fiber.Ctx) error {
	data, err := h.dataJasaAdmin(c)
	if err != nil {
		return h.errorPage(c, err)
	}
	if !middleware.IsHTMX(c) {
		return h.render(c, fiber.StatusOK, pages.AdminServices(*data))
	}
	return h.render(c, fiber.StatusOK, pages.AdminServicesTabel(*data))
}

func (h *Handler) aksiJasa(c *fiber.Ctx, jalankan func(serviceID int64) error) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}

	var pesan string
	if err := jalankan(id); err != nil {
		pesan = pesanKegagalan(err)
	}

	data, derr := h.dataJasaAdmin(c)
	if derr != nil {
		return h.errorPage(c, derr)
	}
	if pesan != "" {
		return h.render(c, fiber.StatusOK, pages.AdminAksiGagal(pesan, pages.AdminServicesTabel(*data)))
	}
	return h.render(c, fiber.StatusOK, pages.AdminServicesTabel(*data))
}

func (h *Handler) AdminStatusJasa(c *fiber.Ctx) error {
	status := model.ServiceStatus(paramFilter(c, "status"))
	return h.aksiJasa(c, func(serviceID int64) error {
		return h.svc.Admin.SetServiceStatus(ctx(c), serviceID, status)
	})
}

func (h *Handler) AdminSorotJasa(c *fiber.Ctx) error {
	hari, _ := strconv.Atoi(paramFilter(c, "hari"))
	admin := middleware.CurrentUser(c)
	return h.aksiJasa(c, func(serviceID int64) error {
		return h.svc.Admin.SetServiceFeatured(ctx(c), admin.ID, serviceID, hari)
	})
}

// ---------- kategori ----------

func (h *Handler) dataKategori(c *fiber.Ctx, form view.Form, editID int64) (*view.AdminCategoriesData, error) {
	items, err := h.svc.Admin.ListCategories(ctx(c))
	if err != nil {
		return nil, err
	}
	induk, err := h.svc.Catalog.ListRootCategories(ctx(c))
	if err != nil {
		return nil, err
	}

	return &view.AdminCategoriesData{
		AdminBase: h.adminBase(c, "Kategori", "Susun kategori dan sub-kategori jasa", "kategori"),
		Items:     items,
		Induk:     induk,
		Ikon:      view.IkonOptions(),
		Form:      form,
		EditID:    editID,
		IsEdit:    editID > 0,
	}, nil
}

func (h *Handler) AdminKategori(c *fiber.Ctx) error {
	editID, _ := strconv.ParseInt(c.Query("edit"), 10, 64)

	form := view.NewForm()
	form.Set("icon", "tambah")
	if editID > 0 {
		// Form diisi nilai kategori yang sedang disunting.
		kategori, err := h.svc.Catalog.GetCategoryByID(ctx(c), editID)
		if err != nil {
			return h.errorPage(c, err)
		}
		form.Set("nama", kategori.Name)
		form.Set("slug", kategori.Slug)
		form.Set("icon", view.Deref(kategori.Icon))
		if kategori.ParentID != nil {
			form.Set("parent_id", strconv.FormatInt(*kategori.ParentID, 10))
		}
	}

	data, err := h.dataKategori(c, form, editID)
	if err != nil {
		return h.errorPage(c, err)
	}
	return h.render(c, fiber.StatusOK, pages.AdminCategories(*data))
}

func bacaKategoriInput(c *fiber.Ctx) service.CategoryInput {
	parentID, _ := strconv.ParseInt(c.FormValue("parent_id"), 10, 64)
	return service.CategoryInput{
		Nama:     c.FormValue("nama"),
		Slug:     c.FormValue("slug"),
		ParentID: parentID,
		Icon:     c.FormValue("icon"),
	}
}

func (h *Handler) AdminKategoriBuat(c *fiber.Ctx) error {
	in := bacaKategoriInput(c)
	if _, err := h.svc.Admin.CreateCategory(ctx(c), in); err != nil {
		return h.renderKategoriGagal(c, in, 0, err)
	}
	return h.redirectWithFlash(c, "/admin/kategori", "sukses", "Kategori ditambahkan.")
}

func (h *Handler) AdminKategoriUbah(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}
	in := bacaKategoriInput(c)
	if _, err := h.svc.Admin.UpdateCategory(ctx(c), id, in); err != nil {
		return h.renderKategoriGagal(c, in, id, err)
	}
	return h.redirectWithFlash(c, "/admin/kategori", "sukses", "Kategori diperbarui.")
}

func (h *Handler) AdminKategoriHapus(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}
	if err := h.svc.Admin.DeleteCategory(ctx(c), id); err != nil {
		pesan := "Kategori gagal dihapus."
		if e, ok := service.AsError(err); ok {
			pesan = e.Message
		}
		return h.redirectWithFlash(c, "/admin/kategori", "galat", pesan)
	}
	return h.redirectWithFlash(c, "/admin/kategori", "sukses", "Kategori dihapus.")
}

// renderKategoriGagal merender ulang halaman kategori beserta pesan kesalahan,
// tanpa menghilangkan isian yang sudah diketik admin.
func (h *Handler) renderKategoriGagal(c *fiber.Ctx, in service.CategoryInput, editID int64, err error) error {
	form := view.NewForm()
	form.Set("nama", in.Nama)
	form.Set("slug", in.Slug)
	form.Set("icon", in.Icon)
	if in.ParentID > 0 {
		form.Set("parent_id", strconv.FormatInt(in.ParentID, 10))
	}

	data, derr := h.dataKategori(c, form, editID)
	if derr != nil {
		return h.errorPage(c, derr)
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
	return h.render(c, status, pages.AdminCategories(*data))
}

// pesanKegagalan memilih pesan yang paling berguna bagi admin.
// Untuk kesalahan validasi, pesan per-field lebih menjelaskan daripada
// kalimat umum "Periksa kembali isian Anda", karena tindakan di panel ini
// dijalankan lewat tombol, bukan lewat form yang bisa menandai field-nya.
func pesanKegagalan(err error) string {
	e, ok := service.AsError(err)
	if !ok {
		return "Tindakan gagal dijalankan."
	}
	for _, field := range []string{"reason", "nama", "slug", "parent_id", "icon"} {
		if msg := e.Fields[field]; msg != "" {
			return msg
		}
	}
	for _, msg := range e.Fields {
		return msg
	}
	return e.Message
}

func orDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
