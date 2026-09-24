package api

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/hermansyah/adojobsid/internal/middleware"
	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/service"
	"github.com/hermansyah/adojobsid/internal/view"
)

// Endpoint untuk aplikasi native. Semuanya memanggil service yang sama
// dengan handler web — tidak ada logika bisnis di sini, hanya pembacaan
// permintaan dan pembentukan respons.
//
// URL gambar di seluruh respons bersifat relatif (/uploads/...); klien
// menyambungnya dengan base_url dari GET /config.

// koordinatQuery membaca lat/lng dari query; keduanya harus ada dan sah.
func koordinatQuery(c *fiber.Ctx) (*float64, *float64) {
	lat, err1 := strconv.ParseFloat(c.Query("lat"), 64)
	lng, err2 := strconv.ParseFloat(c.Query("lng"), 64)
	if err1 != nil || err2 != nil || !model.KoordinatValid(lat, lng) {
		return nil, nil
	}
	return &lat, &lng
}

// kecamatanQuery membaca kecamatan penonton: dari parameter langsung, atau
// diturunkan dari lat/lng bila ada.
func kecamatanQuery(c *fiber.Ctx) string {
	if k := strings.TrimSpace(c.Query("kecamatan")); k != "" {
		return k
	}
	if lat, lng := koordinatQuery(c); lat != nil {
		k, _ := model.KecamatanTerdekat(*lat, *lng)
		return k.Nama
	}
	return ""
}

func idParam(c *fiber.Ctx, pesan string) (int64, error) {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, service.NotFound(pesan)
	}
	return id, nil
}

// ---------- konfigurasi awal ----------

// Config adalah satu panggilan saat aplikasi dibuka: semua yang dibutuhkan
// untuk merender layar pertama tanpa rangkaian permintaan terpisah.
func (h *Handler) Config(c *fiber.Ctx) error {
	ctx := c.Context()
	pengaturan := h.svc.Settings.Get(ctx)
	kategori, err := h.svc.Catalog.ListCategoryTree(ctx)
	if err != nil {
		return fail(c, err)
	}
	paket, err := h.svc.Paket.Daftar(ctx, true)
	if err != nil {
		return fail(c, err)
	}
	// Menu utama disusun untuk pemanggil ini: dengan bearer, isinya sesuai
	// peran; tanpa bearer, versi tamu. Sumbernya sama dengan sheet di web.
	menu := view.MenuUtama(view.MenuKonteks{
		User:        middleware.CurrentUser(c),
		BelumDibaca: belumDibacaUntuk(c, h),
		Situs: view.SitusRingkas{
			Nama:           pengaturan.Umum.NamaSitus,
			AplikasiTampil: pengaturan.Tampilan.AplikasiTampil,
			PlaystoreURL:   pengaturan.Tampilan.PlaystoreURL,
		},
	})
	return ok(c, fiber.Map{
		"base_url": h.cfg.App.BaseURL,
		"menu":     menu,
		"situs": fiber.Map{
			"nama":           pengaturan.Umum.NamaSitus,
			"deskripsi":      pengaturan.Umum.Tagline,
			"logo_url":       pengaturan.Umum.LogoURL,
			"whatsapp_aktif": pengaturan.Umum.WhatsappAktif,
			"kontak_wa":      pengaturan.Umum.KontakWA,
		},
		"hero": fiber.Map{
			"gambar_url": pengaturan.Tampilan.HeroGambarURL,
			"overlay":    pengaturan.Tampilan.HeroOverlay,
		},
		"lokasi": fiber.Map{
			"pusat":             fiber.Map{"lat": pengaturan.Lokasi.PusatLatitude, "lng": pengaturan.Lokasi.PusatLongitude},
			"radius_default_km": pengaturan.Lokasi.RadiusDefaultKm,
			"radius_maks_km":    pengaturan.Lokasi.RadiusMaksKm,
			"kecamatan":         model.KecamatanBengkalisKoordinat,
		},
		"kategori": kategori,
		"promosi": fiber.Map{
			"paket":           paket,
			"uji_coba_gratis": pengaturan.Promosi.UjiCobaGratis,
			"pembayaran": fiber.Map{
				"bank": pengaturan.Promosi.Bank, "nomor_rekening": pengaturan.Promosi.NomorRekening,
				"atas_nama": pengaturan.Promosi.AtasNama, "petunjuk": pengaturan.Promosi.PetunjukBayar,
			},
			"maks_iklan_hidup": service.MaksIklanHidup,
		},
		"slot_iklan": model.SlotIklanBawaan(),
	})
}

// belumDibacaUntuk mengembalikan hitungan pesan belum dibaca pemanggil, 0
// bagi tamu — dipakai badge menu.
func belumDibacaUntuk(c *fiber.Ctx, h *Handler) int {
	current := middleware.CurrentUser(c)
	if current == nil {
		return 0
	}
	return h.svc.Chat.BelumDibaca(c.Context(), current.ID)
}

// Help mengirim isi halaman Tentang & Panduan dalam bentuk data — struktur
// yang sama dengan yang dirender web — beserta angka hidup dan paket.
func (h *Handler) Help(c *fiber.Ctx) error {
	ctx := c.Context()
	pengaturan := h.svc.Settings.Get(ctx)
	ringkasan, _ := h.svc.Catalog.Ringkasan(ctx)
	paket, err := h.svc.Paket.Daftar(ctx, true)
	if err != nil {
		return fail(c, err)
	}
	kontak := ""
	if pengaturan.Umum.WhatsappAktif {
		kontak = pengaturan.Umum.KontakWA
	}
	return ok(c, fiber.Map{
		"bagian":    view.Panduan(pengaturan.Umum.NamaSitus),
		"ringkasan": ringkasan,
		"paket":     paket,
		"kontak_wa": kontak,
	})
}

// ServiceStats mengembalikan statistik kunjungan 30 hari untuk jasa milik
// penyedia yang masuk (admin juga boleh).
func (h *Handler) ServiceStats(c *fiber.Ctx) error {
	id, err := idParam(c, "Jasa tidak ditemukan.")
	if err != nil {
		return fail(c, err)
	}
	current := middleware.CurrentUser(c)
	jasa, err := h.svc.Listing.GetDetail(c.Context(), id)
	if err != nil {
		return fail(c, err)
	}
	if jasa.ProviderID != current.ProviderID && !current.IsAdmin {
		return fail(c, service.NotFound("Jasa tidak ditemukan."))
	}
	st, err := h.svc.Kunjungan.Statistik(c.Context(), id)
	if err != nil {
		return fail(c, err)
	}
	return ok(c, st)
}

// ---------- penyedia ----------

func (h *Handler) ProviderDirectory(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	radius, _ := strconv.Atoi(c.Query("radius"))
	lat, lng := koordinatQuery(c)
	hasil, err := h.svc.Provider.Cari(c.Context(), service.CariPenyediaInput{
		Query:              strings.TrimSpace(c.Query("q")),
		Kecamatan:          strings.TrimSpace(c.Query("kecamatan")),
		HanyaTerverifikasi: c.Query("terverifikasi") == "1",
		HanyaBerjasa:       c.Query("berjasa") == "1",
		Sort:               strings.TrimSpace(c.Query("urut")),
		Page:               page,
		Latitude:           lat,
		Longitude:          lng,
		RadiusKm:           radius,
	})
	if err != nil {
		return fail(c, err)
	}
	return ok(c, hasil)
}

// ProviderBySlug adalah halaman publik penyedia dalam satu respons: profil,
// jasa, portofolio, ringkasan ulasan — sesuai tautan yang beredar.
func (h *Handler) ProviderBySlug(c *fiber.Ctx) error {
	ctx := c.Context()
	p, err := h.svc.Provider.GetDetailBySlug(ctx, strings.TrimSpace(c.Params("slug")))
	if err != nil {
		return fail(c, err)
	}
	current := middleware.CurrentUser(c)
	pemilik := current != nil && (current.ProviderID == p.ID || current.IsAdmin)
	if p.IsSuspended() && !pemilik {
		return fail(c, service.NotFound("Penyedia tidak ditemukan."))
	}
	listings, err := h.svc.Listing.ListByProvider(ctx, p.ID, pemilik)
	if err != nil {
		return fail(c, err)
	}
	portofolio, err := h.svc.Portfolio.Daftar(ctx, p.ID, 0)
	if err != nil {
		return fail(c, err)
	}
	ulasan, err := h.svc.Review.Ringkasan(ctx, p, 20)
	if err != nil {
		return fail(c, err)
	}
	sembunyikanKontak(h.svc.Settings.Get(ctx).Umum.WhatsappAktif, p)
	return ok(c, fiber.Map{
		"bagikan":      view.BagikanPenyedia(h.cfg.App.BaseURL, p),
		"provider":     p,
		"area_layanan": p.AreaLayanan(view.Deref(p.Kecamatan)),
		"services":     listings,
		"portfolio":    portofolio,
		"reviews":      ulasan,
	})
}

// FeaturedProviders adalah blok penyedia pilihan beranda, digilir dan
// mengutamakan kecamatan penonton.
func (h *Handler) FeaturedProviders(c *fiber.Ctx) error {
	semua, err := h.svc.Admin.ListFeaturedProviders(c.Context(), 12)
	if err != nil {
		return fail(c, err)
	}
	n, _ := strconv.Atoi(c.Query("n", "4"))
	if n < 1 || n > 12 {
		n = 4
	}
	return ok(c, h.svc.Promosi.PenyediaPilihan(semua, kecamatanQuery(c), n))
}

// ---------- akun ----------

type profilRequest struct {
	FullName  string `json:"full_name"`
	Email     string `json:"email"`
	City      string `json:"city"`
	Kecamatan string `json:"kecamatan"`
}

func (h *Handler) UpdateMe(c *fiber.Ctx) error {
	var req profilRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, service.InvalidMsg("Format permintaan tidak valid."))
	}
	current := middleware.CurrentUser(c)
	user, err := h.svc.Auth.UpdateProfile(c.Context(), current.ID, service.UpdateProfileInput(req))
	if err != nil {
		return fail(c, err)
	}
	h.perbaruiSesi(c, func(d *sessionData) { d.FullName = user.FullName })
	return ok(c, user)
}

type sandiRequest struct {
	Current string `json:"current_password"`
	Next    string `json:"new_password"`
	Confirm string `json:"new_password_confirm"`
}

func (h *Handler) ChangePassword(c *fiber.Ctx) error {
	var req sandiRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, service.InvalidMsg("Format permintaan tidak valid."))
	}
	current := middleware.CurrentUser(c)
	if err := h.svc.Auth.ChangePassword(c.Context(), current.ID, req.Current, req.Next, req.Confirm); err != nil {
		return fail(c, err)
	}
	return ok(c, fiber.Map{"message": "Kata sandi diganti."})
}

// Unread adalah satu panggilan ringan untuk badge: pesan dan notifikasi.
func (h *Handler) Unread(c *fiber.Ctx) error {
	current := middleware.CurrentUser(c)
	return ok(c, fiber.Map{
		"pesan":      h.svc.Chat.BelumDibaca(c.Context(), current.ID),
		"notifikasi": h.svc.Notif.BelumDibaca(c.Context(), current.ID),
	})
}

// ---------- lokasi & portofolio penyedia ----------

type lokasiRequest struct {
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	AddressLabel string  `json:"address_label"`
	RadiusKm     int     `json:"service_radius_km"`
	Hapus        bool    `json:"hapus"`
}

func (h *Handler) UpdateProviderLocation(c *fiber.Ctx) error {
	var req lokasiRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, service.InvalidMsg("Format permintaan tidak valid."))
	}
	current := middleware.CurrentUser(c)
	in := service.LokasiProviderInput{
		Latitude:     strconv.FormatFloat(req.Latitude, 'f', -1, 64),
		Longitude:    strconv.FormatFloat(req.Longitude, 'f', -1, 64),
		AddressLabel: req.AddressLabel,
		RadiusKm:     strconv.Itoa(req.RadiusKm),
		Hapus:        req.Hapus,
	}
	if err := h.svc.Location.SimpanLokasiProvider(c.Context(), current.ProviderID, in); err != nil {
		return fail(c, err)
	}
	p, err := h.svc.Provider.GetByID(c.Context(), current.ProviderID)
	if err != nil {
		return fail(c, err)
	}
	return ok(c, p)
}

func (h *Handler) MyPortfolio(c *fiber.Ctx) error {
	current := middleware.CurrentUser(c)
	items, err := h.svc.Portfolio.Daftar(c.Context(), current.ProviderID, 0)
	if err != nil {
		return fail(c, err)
	}
	return ok(c, fiber.Map{"items": items, "maks": h.svc.Portfolio.MaksKarya()})
}

func (h *Handler) AddPortfolio(c *fiber.Ctx) error {
	current := middleware.CurrentUser(c)
	fh, _ := c.FormFile("gambar")
	item, err := h.svc.Portfolio.Tambah(c.Context(), current.ProviderID, service.PortofolioInput{
		Judul: c.FormValue("title"), CompletedAt: c.FormValue("completed_at"),
	}, fh)
	if err != nil {
		return fail(c, err)
	}
	return created(c, item)
}

func (h *Handler) DeletePortfolio(c *fiber.Ctx) error {
	id, err := idParam(c, "Karya tidak ditemukan.")
	if err != nil {
		return fail(c, err)
	}
	current := middleware.CurrentUser(c)
	if err := h.svc.Portfolio.Hapus(c.Context(), current.ProviderID, id); err != nil {
		return fail(c, err)
	}
	return ok(c, fiber.Map{"message": "Karya dihapus."})
}

// ---------- pesan ----------

func (h *Handler) Conversations(c *fiber.Ctx) error {
	current := middleware.CurrentUser(c)
	rows, err := h.svc.Chat.Daftar(c.Context(), current.ID)
	if err != nil {
		return fail(c, err)
	}
	return ok(c, rows)
}

// StartConversation membuka (atau menemukan) ruang obrolan untuk satu jasa.
func (h *Handler) StartConversation(c *fiber.Ctx) error {
	serviceID, err := idParam(c, "Jasa tidak ditemukan.")
	if err != nil {
		return fail(c, err)
	}
	current := middleware.CurrentUser(c)
	conv, err := h.svc.Chat.Mulai(c.Context(), serviceID, current.ID)
	if err != nil {
		return fail(c, err)
	}
	return created(c, conv)
}

// Messages mengembalikan riwayat, atau hanya yang lebih baru dari since.
func (h *Handler) Messages(c *fiber.Ctx) error {
	convID, err := idParam(c, "Percakapan tidak ditemukan.")
	if err != nil {
		return fail(c, err)
	}
	current := middleware.CurrentUser(c)
	since, _ := strconv.ParseInt(c.Query("since", "0"), 10, 64)
	conv, err := h.svc.Chat.Akses(c.Context(), convID, current.ID)
	if err != nil {
		return fail(c, err)
	}
	msgs, err := h.svc.Chat.Pesan(c.Context(), convID, current.ID, since)
	if err != nil {
		return fail(c, err)
	}
	payload := fiber.Map{"conversation": conv, "messages": msgs}
	if conv.OrderID != nil {
		if order, oerr := h.svc.Order.Ambil(c.Context(), *conv.OrderID, current.ID); oerr == nil {
			sebagaiPenyedia := current.ProviderID > 0 && conv.ProviderID == current.ProviderID
			payload["order"] = order
			payload["tindakan_order"] = h.svc.Order.TindakanTersedia(order, sebagaiPenyedia)
			payload["dapat_diulas"] = h.svc.Review.DapatDiulas(c.Context(), order, current.ID)
			payload["ulasan"] = h.svc.Review.UlasanPesanan(c.Context(), order.ID)
		}
	}
	return ok(c, payload)
}

type pesanRequest struct {
	Body string `json:"body"`
}

func (h *Handler) SendMessage(c *fiber.Ctx) error {
	convID, err := idParam(c, "Percakapan tidak ditemukan.")
	if err != nil {
		return fail(c, err)
	}
	var req pesanRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, service.InvalidMsg("Format permintaan tidak valid."))
	}
	current := middleware.CurrentUser(c)
	msg, err := h.svc.Chat.Kirim(c.Context(), convID, current.ID, req.Body)
	if err != nil {
		return fail(c, err)
	}
	return created(c, msg)
}

// ---------- pesanan & ulasan ----------

type orderRequest struct {
	ScheduledDate string `json:"scheduled_date"`
	Notes         string `json:"notes"`
}

func (h *Handler) CreateOrder(c *fiber.Ctx) error {
	serviceID, err := idParam(c, "Jasa tidak ditemukan.")
	if err != nil {
		return fail(c, err)
	}
	var req orderRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, service.InvalidMsg("Format permintaan tidak valid."))
	}
	current := middleware.CurrentUser(c)
	order, conv, err := h.svc.Order.Buat(c.Context(), current.ID, service.OrderInput{
		ServiceID: serviceID, ScheduledDate: req.ScheduledDate, Notes: req.Notes,
	})
	if err != nil {
		return fail(c, err)
	}
	return created(c, fiber.Map{"order": order, "conversation": conv})
}

func (h *Handler) Order(c *fiber.Ctx) error {
	id, err := idParam(c, "Pesanan tidak ditemukan.")
	if err != nil {
		return fail(c, err)
	}
	current := middleware.CurrentUser(c)
	order, err := h.svc.Order.Ambil(c.Context(), id, current.ID)
	if err != nil {
		return fail(c, err)
	}
	return ok(c, order)
}

type statusRequest struct {
	Status string `json:"status"`
}

func (h *Handler) UpdateOrderStatus(c *fiber.Ctx) error {
	id, err := idParam(c, "Pesanan tidak ditemukan.")
	if err != nil {
		return fail(c, err)
	}
	var req statusRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, service.InvalidMsg("Format permintaan tidak valid."))
	}
	current := middleware.CurrentUser(c)
	if err := h.svc.Order.UbahStatus(c.Context(), id, current.ID, model.OrderStatus(req.Status)); err != nil {
		return fail(c, err)
	}
	order, err := h.svc.Order.Ambil(c.Context(), id, current.ID)
	if err != nil {
		return fail(c, err)
	}
	return ok(c, order)
}

type ulasanRequest struct {
	Rating   int    `json:"rating"`
	Komentar string `json:"comment"`
}

func (h *Handler) WriteReview(c *fiber.Ctx) error {
	id, err := idParam(c, "Pesanan tidak ditemukan.")
	if err != nil {
		return fail(c, err)
	}
	var req ulasanRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, service.InvalidMsg("Format permintaan tidak valid."))
	}
	current := middleware.CurrentUser(c)
	review, err := h.svc.Review.Tulis(c.Context(), current.ID, service.UlasanInput{
		OrderID: id, Rating: req.Rating, Komentar: req.Komentar,
	})
	if err != nil {
		return fail(c, err)
	}
	return created(c, review)
}

// ---------- notifikasi ----------

func (h *Handler) Notifications(c *fiber.Ctx) error {
	current := middleware.CurrentUser(c)
	items, err := h.svc.Notif.Daftar(c.Context(), current.ID)
	if err != nil {
		return fail(c, err)
	}
	// Judul, keterangan, dan tautan disusun di server — aplikasi tidak perlu
	// tahu bentuk payload tiap jenis notifikasi.
	out := make([]fiber.Map, 0, len(items))
	for _, n := range items {
		judul, keterangan, tautan := view.IsiNotifikasi(n)
		out = append(out, fiber.Map{
			"id": n.ID, "type": n.Type, "judul": judul, "keterangan": keterangan,
			"tautan": tautan, "read_at": n.ReadAt, "created_at": n.CreatedAt, "payload": n.Payload,
		})
	}
	return ok(c, out)
}

func (h *Handler) MarkNotificationsRead(c *fiber.Ctx) error {
	current := middleware.CurrentUser(c)
	if err := h.svc.Notif.TandaiTerbaca(c.Context(), current.ID); err != nil {
		return fail(c, err)
	}
	return ok(c, fiber.Map{"message": "Semua notifikasi ditandai terbaca."})
}

// ---------- promosi ----------

func (h *Handler) MyPromotions(c *fiber.Ctx) error {
	current := middleware.CurrentUser(c)
	items, err := h.svc.Promosi.DaftarPengguna(c.Context(), current.ID)
	if err != nil {
		return fail(c, err)
	}
	return ok(c, items)
}

func (h *Handler) SubmitPromotion(c *fiber.Ctx) error {
	current := middleware.CurrentUser(c)
	paketID, _ := strconv.ParseInt(c.FormValue("paket_id"), 10, 64)
	serviceID, _ := strconv.ParseInt(c.FormValue("service_id"), 10, 64)
	var target []string
	if mf, err := c.MultipartForm(); err == nil && mf != nil {
		target = mf.Value["target_kecamatan"]
	}
	gambar, _ := c.FormFile("gambar")
	p, err := h.svc.Promosi.Ajukan(c.Context(), current.ID, current.ProviderID, service.AjukanInput{
		Jenis: c.FormValue("jenis"), PaketID: paketID, ServiceID: serviceID,
		Alasan: c.FormValue("alasan"), Judul: c.FormValue("judul"), Deskripsi: c.FormValue("deskripsi"),
		TautanURL: c.FormValue("tautan_url"), TargetKecamatan: target,
	}, gambar)
	if err != nil {
		return fail(c, err)
	}
	return created(c, p)
}

func (h *Handler) MyPromotion(c *fiber.Ctx) error {
	id, err := idParam(c, "Pengajuan tidak ditemukan.")
	if err != nil {
		return fail(c, err)
	}
	current := middleware.CurrentUser(c)
	p, err := h.svc.Promosi.AmbilMilik(c.Context(), current.ID, id)
	if err != nil {
		return fail(c, err)
	}
	payload := fiber.Map{"promosi": p, "label_status": model.LabelStatusPromosi(p.Status)}
	if p.PaketID != nil {
		if paket, perr := h.svc.Paket.Ambil(c.Context(), *p.PaketID); perr == nil {
			payload["paket"] = paket
		}
	}
	return ok(c, payload)
}

func (h *Handler) UploadPromotionProof(c *fiber.Ctx) error {
	id, err := idParam(c, "Pengajuan tidak ditemukan.")
	if err != nil {
		return fail(c, err)
	}
	current := middleware.CurrentUser(c)
	fh, ferr := c.FormFile("bukti")
	if ferr != nil {
		return fail(c, service.InvalidMsg("Berkas bukti tidak terbaca."))
	}
	p, err := h.svc.Promosi.UnggahBukti(c.Context(), current.ID, id, fh)
	if err != nil {
		return fail(c, err)
	}
	return ok(c, p)
}

func (h *Handler) CancelPromotion(c *fiber.Ctx) error {
	id, err := idParam(c, "Pengajuan tidak ditemukan.")
	if err != nil {
		return fail(c, err)
	}
	current := middleware.CurrentUser(c)
	p, err := h.svc.Promosi.Batalkan(c.Context(), current.ID, id)
	if err != nil {
		return fail(c, err)
	}
	return ok(c, p)
}

// ---------- iklan ----------

// Ad memilih satu iklan untuk satu slot — aturan yang sama dengan web:
// paket → kecamatan penonton → rotasi berbobot, jatuh ke materi bawaan.
// Tayangan dicatat di sini karena aplikasi memang akan menampilkannya.
func (h *Handler) Ad(c *fiber.Ctx) error {
	slot := strings.TrimSpace(c.Query("slot"))
	if slot == "" {
		return fail(c, service.InvalidMsg("Parameter slot wajib diisi."))
	}
	if ik := h.svc.Promosi.PilihUntukSlot(c.Context(), slot, kecamatanQuery(c)); ik != nil {
		h.svc.Promosi.CatatTayang(c.Context(), ik.ID)
		s := model.SlotDariIklan(*ik, slot)
		return ok(c, fiber.Map{"sumber": "promosi", "id": ik.ID, "judul": s.Teks, "gambar_url": s.GambarURL,
			"deskripsi": view.Deref(ik.Deskripsi), "klik_url": s.TautanURL})
	}
	if s := h.svc.Settings.Get(c.Context()).Iklan.Cari(slot); s != nil {
		return ok(c, fiber.Map{"sumber": "bawaan", "judul": s.Teks, "gambar_url": s.GambarURL, "klik_url": s.TautanURL})
	}
	return ok(c, nil)
}

// ---------- perangkat (push) ----------

type perangkatRequest struct {
	Token    string `json:"token"`
	Platform string `json:"platform"`
}

func (h *Handler) RegisterDevice(c *fiber.Ctx) error {
	var req perangkatRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, service.InvalidMsg("Format permintaan tidak valid."))
	}
	current := middleware.CurrentUser(c)
	if err := h.svc.Push.DaftarkanPerangkat(c.Context(), current.ID, req.Token, req.Platform); err != nil {
		return fail(c, err)
	}
	return ok(c, fiber.Map{"push_aktif": h.svc.Push.Aktif()})
}

func (h *Handler) UnregisterDevice(c *fiber.Ctx) error {
	var req perangkatRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, service.InvalidMsg("Format permintaan tidak valid."))
	}
	current := middleware.CurrentUser(c)
	if err := h.svc.Push.CabutPerangkat(c.Context(), current.ID, req.Token); err != nil {
		return fail(c, err)
	}
	return ok(c, fiber.Map{"message": "Perangkat dicabut."})
}
