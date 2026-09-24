package web

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/hermansyah/adojobsid/internal/middleware"
	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/service"
	"github.com/hermansyah/adojobsid/internal/view"
	"github.com/hermansyah/adojobsid/web/templates/pages"
)

// Beranda menampilkan kategori dan listing terbaru.
func (h *Handler) Beranda(c *fiber.Ctx) error {
	kategori, err := h.svc.Catalog.ListRootCategories(ctx(c))
	if err != nil {
		return h.errorPage(c, err)
	}
	lokasi := h.lokasiAktif(c)
	lat, lng := lokasi.Koordinat()
	hasil, err := h.svc.Catalog.Search(ctx(c), service.SearchInput{
		Page: 1, PerPage: 8, Latitude: lat, Longitude: lng,
	})
	if err != nil {
		return h.errorPage(c, err)
	}

	// Penyedia pilihan hanya tampil bila admin memang sedang menyorot seseorang;
	// kegagalannya tidak boleh menggagalkan beranda.
	// Diambil lebih banyak dari yang ditampilkan supaya bisa digilir dan
	// diutamakan yang sekecamatan dengan penonton.
	semuaPilihan, _ := h.svc.Admin.ListFeaturedProviders(ctx(c), 12)
	pilihan := h.svc.Promosi.PenyediaPilihan(semuaPilihan, kecamatanPenonton(h.lokasiAktif(c)), 4)

	data := view.HomeData{
		// Deskripsi meta beranda diambil dari Tagline di pengaturan admin —
		// satu-satunya tempat kalimat itu hidup, bukan salinan di kode.
		Base:     h.base(c, "", h.svc.Settings.Get(ctx(c)).Umum.Tagline, "beranda"),
		Kategori: kategori,
		Terbaru:  hasil.Items,
		Total:    hasil.Total,
		Pilihan:  pilihan,
	}
	return h.render(c, fiber.StatusOK, pages.Home(data))
}

// Cari menampilkan halaman pencarian lengkap.
func (h *Handler) Cari(c *fiber.Ctx) error {
	data, err := h.dataPencarian(c)
	if err != nil {
		return h.errorPage(c, err)
	}
	return h.render(c, fiber.StatusOK, pages.Search(*data))
}

// HasilCari mengembalikan fragmen hasil saja, dipakai HTMX saat filter berubah.
func (h *Handler) HasilCari(c *fiber.Ctx) error {
	data, err := h.dataPencarian(c)
	if err != nil {
		return h.errorPage(c, err)
	}
	// Halaman penuh tetap dikirim bila request datang tanpa HTMX,
	// misalnya saat pengguna membuka URL hasil langsung dari riwayat peramban.
	if !middleware.IsHTMX(c) {
		return h.render(c, fiber.StatusOK, pages.Search(*data))
	}
	return h.render(c, fiber.StatusOK, pages.HasilPencarian(*data))
}

// dataPencarian membaca query string dan menjalankan pencarian.
func (h *Handler) dataPencarian(c *fiber.Ctx) (*view.SearchData, error) {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}
	pengaturan := h.svc.Settings.Get(ctx(c))
	lokasi := h.lokasiAktif(c)
	lat, lng := lokasi.Koordinat()

	radius, _ := strconv.Atoi(c.Query("radius"))
	if radius < 0 || radius > pengaturan.Lokasi.RadiusMaksKm {
		radius = 0
	}

	in := service.SearchInput{
		Query:         strings.TrimSpace(c.Query("q")),
		CategorySlug:  strings.TrimSpace(c.Query("kategori")),
		Kecamatan:     strings.TrimSpace(c.Query("kecamatan")),
		PriceType:     strings.TrimSpace(c.Query("harga")),
		Sort:          strings.TrimSpace(c.Query("urut")),
		Page:          page,
		Latitude:      lat,
		Longitude:     lng,
		RadiusKm:      radius,
		OnlyReachable: c.Query("menjangkau") == "1",
	}

	hasil, err := h.svc.Catalog.Search(ctx(c), in)
	if err != nil {
		return nil, err
	}
	kategori, err := h.svc.Catalog.ListRootCategories(ctx(c))
	if err != nil {
		return nil, err
	}
	kecamatan, err := h.svc.Catalog.KecamatanOptions(ctx(c))
	if err != nil {
		return nil, err
	}

	judul := "Cari jasa"
	if hasil.Category != nil {
		judul = "Jasa " + hasil.Category.Name
	} else if in.Query != "" {
		judul = "Cari: " + in.Query
	}

	return &view.SearchData{
		Base: h.base(c, judul,
			"Cari penyedia jasa terdekat di Kabupaten Bengkalis berdasarkan kategori dan kecamatan.",
			"cari"),
		Query:         in.Query,
		CategorySlug:  in.CategorySlug,
		Kecamatan:     in.Kecamatan,
		PriceType:     in.PriceType,
		Sort:          in.Sort,
		RadiusKm:      in.RadiusKm,
		RadiusMaks:    pengaturan.Lokasi.RadiusMaksKm,
		OnlyReachable: in.OnlyReachable,
		Kategori:      kategori,
		KecamatanOps:  kecamatan,
		Items:         hasil.Items,
		Total:         hasil.Total,
		Page:          hasil.Page,
		TotalPages:    hasil.TotalPages,
	}, nil
}

// DetailJasa menampilkan satu listing.
func (h *Handler) DetailJasa(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}

	detail, err := h.svc.Listing.GetDetail(ctx(c), id)
	if err != nil {
		return h.errorPage(c, err)
	}

	user := middleware.CurrentUser(c)
	isOwner := user != nil && user.ProviderID == detail.ProviderID

	isAdmin := user != nil && user.IsAdmin
	lokasi := h.lokasiAktif(c)

	// Listing nonaktif hanya boleh dilihat pemiliknya atau admin.
	if detail.Status != model.ServiceActive && !isOwner && !isAdmin {
		return h.notFound(c)
	}
	// Listing milik akun yang ditangguhkan juga hilang dari halaman publik,
	// sama seperti di hasil pencarian.
	if detail.Provider.IsSuspended() && !isOwner && !isAdmin {
		return h.notFound(c)
	}

	// Jasa serupa: kategori sama, tanpa listing ini sendiri.
	// Kegagalan di bagian ini tidak boleh menggagalkan halaman detail,
	// jadi errornya diabaikan dan daftar dibiarkan kosong.
	var serupa []model.ServiceCard
	if mirip, err := h.svc.Catalog.Search(ctx(c), service.SearchInput{
		CategorySlug: detail.Category.Slug,
		PerPage:      5,
		Page:         1,
	}); err == nil {
		serupa = make([]model.ServiceCard, 0, 4)
		for _, item := range mirip.Items {
			if item.ID == detail.ID {
				continue
			}
			serupa = append(serupa, item)
			if len(serupa) == 4 {
				break
			}
		}
	}

	// Jarak dihitung di aplikasi karena satu titik saja; penyaringan radius
	// di pencarian tetap dikerjakan database agar terindeks.
	var jarak *float64
	if detail.Provider.Latitude != nil && detail.Provider.Longitude != nil {
		km := model.JarakKm(lokasi.Latitude, lokasi.Longitude,
			*detail.Provider.Latitude, *detail.Provider.Longitude)
		jarak = &km
	}

	bagikan := view.BagikanJasa(h.cfg.App.BaseURL, detail)
	data := view.ServiceDetailData{
		Base: h.base(c, detail.Title,
			view.Potong(detail.Description, 155),
			"cari"),
		Service: detail,
		IsOwner: isOwner,
		IsAdmin: isAdmin,
		Serupa:  serupa,
		JarakKm: jarak,
		Form:    view.NewForm(),
	}
	// Hanya jasa yang tayang publik yang layak dibagikan.
	if detail.Status == model.ServiceActive {
		data.Base.Bagikan = &bagikan
	}
	// Kunjungan dihitung untuk pengunjung sungguhan: bukan pemilik, bukan
	// admin, bukan bot atau pengambil pratinjau tautan.
	if detail.Status == model.ServiceActive && !isOwner && !isAdmin && !service.AdalahBot(c.Get(fiber.HeaderUserAgent)) {
		h.svc.Kunjungan.Catat(ctx(c), detail.ID, service.SidikPengunjung(c.IP(), c.Get(fiber.HeaderUserAgent), time.Now()))
	}
	// Statistik kunjungan bersifat publik: pencari melihat seberapa ramai
	// jasa ini, pemilik melihat perkembangan jasanya.
	if detail.Status == model.ServiceActive {
		data.Statistik, _ = h.svc.Kunjungan.Statistik(ctx(c), detail.ID)
	}
	return h.render(c, fiber.StatusOK, pages.ServiceDetail(data))
}
