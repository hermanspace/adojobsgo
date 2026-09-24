package api

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/hermansyah/adojobsid/internal/middleware"
	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/service"
	"github.com/hermansyah/adojobsid/internal/session"
)

// ---------- autentikasi ----------

type registerRequest struct {
	FullName        string `json:"full_name"`
	Phone           string `json:"phone"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	PasswordConfirm string `json:"password_confirm"`
	City            string `json:"city"`
	Kecamatan       string `json:"kecamatan"`
}

func (h *Handler) Register(c *fiber.Ctx) error {
	var req registerRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, service.InvalidMsg("Format permintaan tidak valid."))
	}

	user, err := h.svc.Auth.Register(c.Context(), service.RegisterInput(req))
	if err != nil {
		return fail(c, err)
	}
	sid, err := h.mulaiSesi(c, user, 0)
	if err != nil {
		return fail(c, service.Internal(err))
	}
	return created(c, fiber.Map{"user": user, "provider_id": 0, "token": sid, "expires_in": int(h.sessions.TTL().Seconds())})
}

type loginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

func (h *Handler) Login(c *fiber.Ctx) error {
	var req loginRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, service.InvalidMsg("Format permintaan tidak valid."))
	}

	user, err := h.svc.Auth.Login(c.Context(), service.LoginInput(req))
	if err != nil {
		return fail(c, err)
	}

	var providerID int64
	if user.IsProvider {
		if p, perr := h.svc.Provider.GetByUserID(c.Context(), user.ID); perr == nil {
			providerID = p.ID
		}
	}
	sid, err := h.mulaiSesi(c, user, providerID)
	if err != nil {
		return fail(c, service.Internal(err))
	}
	// Token adalah ID sesi yang sama dengan cookie; aplikasi native
	// mengirimnya kembali sebagai Authorization: Bearer <token>.
	return ok(c, fiber.Map{
		"user":        user,
		"provider_id": providerID,
		"token":       sid,
		"expires_in":  int(h.sessions.TTL().Seconds()),
	})
}

func (h *Handler) Logout(c *fiber.Ctx) error {
	if sid := middleware.SessionID(c); sid != "" {
		_ = h.sessions.Destroy(c.Context(), sid)
	}
	h.auth.ClearCookie(c)
	return ok(c, fiber.Map{"message": "Berhasil keluar."})
}

// LogoutSemua mencabut seluruh sesi pengguna di semua perangkat — untuk
// ponsel hilang, atau setelah ganti kata sandi.
func (h *Handler) LogoutSemua(c *fiber.Ctx) error {
	current := middleware.CurrentUser(c)
	if err := h.sessions.DestroyAllForUser(c.Context(), current.ID); err != nil {
		return fail(c, service.Internal(err))
	}
	h.auth.ClearCookie(c)
	return ok(c, fiber.Map{"message": "Semua sesi dicabut."})
}

// Me mengembalikan data pengguna yang sedang masuk.
func (h *Handler) Me(c *fiber.Ctx) error {
	current := middleware.CurrentUser(c)
	user, err := h.svc.Auth.GetUser(c.Context(), current.ID)
	if err != nil {
		return fail(c, err)
	}
	payload := fiber.Map{"user": user}
	if user.IsProvider {
		if p, perr := h.svc.Provider.GetByUserID(c.Context(), user.ID); perr == nil {
			payload["provider"] = p
		}
	}
	return ok(c, payload)
}

// ---------- katalog ----------

func (h *Handler) Categories(c *fiber.Ctx) error {
	tree, err := h.svc.Catalog.ListCategoryTree(c.Context())
	if err != nil {
		return fail(c, err)
	}
	return ok(c, tree)
}

func (h *Handler) SearchServices(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "0"))

	lat, lng := koordinatQuery(c)
	radius, _ := strconv.Atoi(c.Query("radius"))
	result, err := h.svc.Catalog.Search(c.Context(), service.SearchInput{
		Query:         strings.TrimSpace(c.Query("q")),
		CategorySlug:  strings.TrimSpace(c.Query("kategori")),
		City:          strings.TrimSpace(c.Query("kota")),
		Kecamatan:     strings.TrimSpace(c.Query("kecamatan")),
		PriceType:     strings.TrimSpace(c.Query("harga")),
		Sort:          strings.TrimSpace(c.Query("urut")),
		Page:          page,
		PerPage:       perPage,
		Latitude:      lat,
		Longitude:     lng,
		RadiusKm:      radius,
		OnlyReachable: c.Query("terjangkau") == "1",
	})
	if err != nil {
		return fail(c, err)
	}
	return ok(c, result)
}

func (h *Handler) ServiceDetail(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fail(c, service.NotFound("Jasa tidak ditemukan."))
	}
	detail, err := h.svc.Listing.GetDetail(c.Context(), id)
	if err != nil {
		return fail(c, err)
	}
	sembunyikanKontak(h.svc.Settings.Get(c.Context()).Umum.WhatsappAktif, &detail.Provider)
	return ok(c, detail)
}

func (h *Handler) KecamatanOptions(c *fiber.Ctx) error {
	list, err := h.svc.Catalog.KecamatanOptions(c.Context())
	if err != nil {
		return fail(c, err)
	}
	return ok(c, list)
}

// ---------- provider ----------

type providerRequest struct {
	Bio            string `json:"bio"`
	WhatsappNumber string `json:"whatsapp_number"`
}

func (h *Handler) CreateProvider(c *fiber.Ctx) error {
	var req providerRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, service.InvalidMsg("Format permintaan tidak valid."))
	}
	current := middleware.CurrentUser(c)

	profile, err := h.svc.Provider.Create(c.Context(), current.ID, service.ProviderProfileInput(req))
	if err != nil {
		return fail(c, err)
	}
	h.perbaruiSesi(c, func(d *session.Data) {
		d.IsProvider = true
		d.ProviderID = profile.ID
	})
	return created(c, profile)
}

func (h *Handler) UpdateProvider(c *fiber.Ctx) error {
	var req providerRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, service.InvalidMsg("Format permintaan tidak valid."))
	}
	current := middleware.CurrentUser(c)

	profile, err := h.svc.Provider.Update(c.Context(), current.ProviderID, service.ProviderProfileInput(req))
	if err != nil {
		return fail(c, err)
	}
	return ok(c, profile)
}

func (h *Handler) ProviderDetail(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fail(c, service.NotFound("Penyedia jasa tidak ditemukan."))
	}
	detail, err := h.svc.Provider.GetDetail(c.Context(), id)
	if err != nil {
		return fail(c, err)
	}
	listings, err := h.svc.Listing.ListByProvider(c.Context(), id, false)
	if err != nil {
		return fail(c, err)
	}
	sembunyikanKontak(h.svc.Settings.Get(c.Context()).Umum.WhatsappAktif, detail)
	return ok(c, fiber.Map{"provider": detail, "services": listings})
}

// sembunyikanKontak mengosongkan nomor WhatsApp pada data penyedia yang
// dikirim ke publik selama fitur WhatsApp dimatikan. Halaman HTML sudah
// menggerbang tombolnya, tetapi JSON memuat seluruh struct apa adanya — tanpa
// ini, nomor pribadi penyedia tetap terbaca siapa pun yang membuka endpoint.
// Endpoint milik penyedia sendiri (buat/ubah profil) tidak melewati ini,
// karena pemiliknya memang berhak melihat nomornya.
func sembunyikanKontak(whatsappAktif bool, p *model.ProviderDetail) {
	if p == nil || whatsappAktif {
		return
	}
	p.WhatsappNumber = ""
}

// ---------- listing ----------

type listingRequest struct {
	CategoryID  int64  `json:"category_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	PriceType   string `json:"price_type"`
	PriceMin    string `json:"price_min"`
	PriceMax    string `json:"price_max"`
	Status      string `json:"status"`
}

func (h *Handler) CreateListing(c *fiber.Ctx) error {
	var req listingRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, service.InvalidMsg("Format permintaan tidak valid."))
	}
	current := middleware.CurrentUser(c)

	svc, err := h.svc.Listing.Create(c.Context(), current.ProviderID, service.ListingInput(req))
	if err != nil {
		return fail(c, err)
	}
	return created(c, svc)
}

func (h *Handler) UpdateListing(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fail(c, service.NotFound("Jasa tidak ditemukan."))
	}
	var req listingRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, service.InvalidMsg("Format permintaan tidak valid."))
	}
	current := middleware.CurrentUser(c)

	svc, err := h.svc.Listing.Update(c.Context(), current.ProviderID, id, service.ListingInput(req))
	if err != nil {
		return fail(c, err)
	}
	return ok(c, svc)
}

func (h *Handler) DeleteListing(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fail(c, service.NotFound("Jasa tidak ditemukan."))
	}
	current := middleware.CurrentUser(c)
	if err := h.svc.Listing.Delete(c.Context(), current.ProviderID, id); err != nil {
		return fail(c, err)
	}
	return ok(c, fiber.Map{"message": "Listing dihapus."})
}

// MyListings mengembalikan seluruh listing milik penyedia yang sedang masuk,
// termasuk yang nonaktif.
func (h *Handler) MyListings(c *fiber.Ctx) error {
	current := middleware.CurrentUser(c)
	items, err := h.svc.Listing.ListByProvider(c.Context(), current.ProviderID, true)
	if err != nil {
		return fail(c, err)
	}
	return ok(c, items)
}

// UploadListingImages menerima foto lewat multipart/form-data.
func (h *Handler) UploadListingImages(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fail(c, service.NotFound("Jasa tidak ditemukan."))
	}
	form, err := c.MultipartForm()
	if err != nil {
		return fail(c, service.InvalidMsg("Berkas tidak terbaca."))
	}
	current := middleware.CurrentUser(c)

	images, err := h.svc.Listing.AddImages(c.Context(), current.ProviderID, id, form.File["foto"], false)
	if err != nil {
		return fail(c, err)
	}
	return created(c, images)
}

func (h *Handler) DeleteListingImage(c *fiber.Ctx) error {
	imageID, err := strconv.ParseInt(c.Params("imageID"), 10, 64)
	if err != nil {
		return fail(c, service.NotFound("Foto tidak ditemukan."))
	}
	current := middleware.CurrentUser(c)
	if err := h.svc.Listing.DeleteImage(c.Context(), current.ProviderID, imageID, false); err != nil {
		return fail(c, err)
	}
	return ok(c, fiber.Map{"message": "Foto dihapus."})
}

// ---------- sesi ----------

func (h *Handler) mulaiSesi(c *fiber.Ctx, user *model.User, providerID int64) (string, error) {
	sid, err := h.sessions.Create(c.Context(), session.Data{
		UserID:     user.ID,
		FullName:   user.FullName,
		IsProvider: user.IsProvider,
		IsAdmin:    user.IsAdmin(),
		ProviderID: providerID,
	})
	if err != nil {
		return "", err
	}
	h.auth.SetCookie(c, sid)
	return sid, nil
}

// sessionData adalah alias lokal supaya berkas lain di paket ini tidak perlu
// mengimpor session hanya untuk closure pembaruan.
type sessionData = session.Data

func (h *Handler) perbaruiSesi(c *fiber.Ctx, mutate func(*session.Data)) {
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
}
