package web

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/hermansyah/adojobsid/internal/middleware"
	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/service"
	"github.com/hermansyah/adojobsid/internal/view"
	"github.com/hermansyah/adojobsid/web/templates/pages"
)

// Pesan menampilkan daftar percakapan milik pengguna.
func (h *Handler) Pesan(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)

	rows, err := h.svc.Chat.Daftar(ctx(c), user.ID)
	if err != nil {
		return h.errorPage(c, err)
	}
	return h.render(c, fiber.StatusOK, pages.DaftarPesan(view.PesanData{
		Base:  h.base(c, "Pesan", "Percakapan Anda dengan penyedia dan pelanggan.", "pesan"),
		Items: rows,
	}))
}

// RuangPesan menampilkan satu percakapan beserta riwayatnya.
func (h *Handler) RuangPesan(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}

	data, err := h.dataRuangPesan(c, user.ID, id)
	if err != nil {
		return h.errorPage(c, err)
	}
	return h.render(c, fiber.StatusOK, pages.RuangPesan(*data))
}

func (h *Handler) dataRuangPesan(c *fiber.Ctx, userID, convID int64) (*view.RuangPesanData, error) {
	row, err := h.svc.Chat.Akses(ctx(c), convID, userID)
	if err != nil {
		return nil, err
	}
	msgs, err := h.svc.Chat.Pesan(ctx(c), convID, userID, 0)
	if err != nil {
		return nil, err
	}

	data := &view.RuangPesanData{
		Base:     h.base(c, row.LawanNama, "Percakapan mengenai "+row.ServiceTitle, "pesan"),
		Conv:     row,
		Messages: msgs,
		UserID:   userID,
		Form:     view.NewForm(),
	}

	// Penyedia dan pencari jasa melihat tombol tindakan pesanan yang berbeda.
	if prov, perr := h.svc.Provider.GetByID(ctx(c), row.ProviderID); perr == nil {
		data.SebagaiPenyedia = prov.UserID == userID
	}
	if row.OrderID != nil {
		if order, oerr := h.svc.Order.Ambil(ctx(c), *row.OrderID, userID); oerr == nil {
			data.Order = order
			data.Ulasan = h.svc.Review.UlasanPesanan(ctx(c), order.ID)
			data.DapatDiulas = h.svc.Review.DapatDiulas(ctx(c), order, userID)
		}
	}
	return data, nil
}

// PesanBaru mengembalikan hanya pesan yang lebih baru dari sinceID.
// Dipakai polling HTMX supaya ruang obrolan tidak mengunduh ulang riwayat.
func (h *Handler) PesanBaru(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}
	since, _ := strconv.ParseInt(c.Query("sejak", "0"), 10, 64)

	msgs, err := h.svc.Chat.Pesan(ctx(c), id, user.ID, since)
	if err != nil {
		return h.errorPage(c, err)
	}
	return h.render(c, fiber.StatusOK, pages.GelembungPesan(msgs, user.ID))
}

// KirimPesan menyimpan pesan lalu mengembalikan gelembung pesannya saja.
func (h *Handler) KirimPesan(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}

	if _, err := h.svc.Chat.Kirim(ctx(c), id, user.ID, c.FormValue("body")); err != nil {
		if e, ok := service.AsError(err); ok && e.Code == service.CodeInvalidInput {
			// Pesan kosong cukup diabaikan tanpa mengubah tampilan.
			return c.SendStatus(fiber.StatusNoContent)
		}
		return h.errorPage(c, err)
	}

	since, _ := strconv.ParseInt(c.FormValue("sejak"), 10, 64)
	msgs, err := h.svc.Chat.Pesan(ctx(c), id, user.ID, since)
	if err != nil {
		return h.errorPage(c, err)
	}
	return h.render(c, fiber.StatusOK, pages.GelembungPesan(msgs, user.ID))
}

// MulaiPesan membuka percakapan dari halaman detail jasa.
func (h *Handler) MulaiPesan(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	serviceID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}

	conv, err := h.svc.Chat.Mulai(ctx(c), serviceID, user.ID)
	if err != nil {
		return h.errorPage(c, err)
	}
	return h.redirect(c, "/pesan/"+strconv.FormatInt(conv.ID, 10))
}

// ---------- pesanan ----------

// BuatOrder membuat permintaan order dan mengarahkan ke ruang obrolannya.
func (h *Handler) BuatOrder(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	serviceID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}

	in := service.OrderInput{
		ServiceID:     serviceID,
		ScheduledDate: c.FormValue("scheduled_date"),
		Notes:         c.FormValue("notes"),
	}

	_, conv, err := h.svc.Order.Buat(ctx(c), user.ID, in)
	if err != nil {
		return h.redirectWithFlash(c, "/jasa/"+strconv.FormatInt(serviceID, 10),
			"galat", pesanKegagalan(err))
	}
	return h.redirectWithFlash(c, "/pesan/"+strconv.FormatInt(conv.ID, 10), "sukses",
		"Permintaan terkirim. Lanjutkan pembahasannya di sini.")
}

// UbahStatusOrder dipakai kedua pihak untuk menggerakkan status pesanan.
func (h *Handler) UbahStatusOrder(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	orderID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return h.notFound(c)
	}
	convID, _ := strconv.ParseInt(c.FormValue("conversation_id"), 10, 64)
	status := model.OrderStatus(c.FormValue("status"))

	tujuan := "/pesan"
	if convID > 0 {
		tujuan = "/pesan/" + strconv.FormatInt(convID, 10)
	}

	if err := h.svc.Order.UbahStatus(ctx(c), orderID, user.ID, status); err != nil {
		return h.redirectWithFlash(c, tujuan, "galat", pesanKegagalan(err))
	}
	return h.redirectWithFlash(c, tujuan, "sukses",
		"Status pesanan diperbarui menjadi "+status.Label()+".")
}

// ---------- notifikasi ----------

func (h *Handler) Notifikasi(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)

	items, err := h.svc.Notif.Daftar(ctx(c), user.ID)
	if err != nil {
		return h.errorPage(c, err)
	}
	// Dibuka berarti dibaca; badge langsung kembali ke nol.
	_ = h.svc.Notif.TandaiTerbaca(ctx(c), user.ID)

	return h.render(c, fiber.StatusOK, pages.Notifikasi(view.NotifikasiData{
		Base:  h.base(c, "Notifikasi", "Pemberitahuan terbaru untuk Anda.", "akun"),
		Items: items,
	}))
}

// BadgePesan mengembalikan penghitung pesan belum dibaca untuk navigasi.
// Dipanggil berkala oleh HTMX dan sengaja dibuat sangat ringan.
func (h *Handler) BadgePesan(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	if user == nil {
		return c.SendString("")
	}
	return h.render(c, fiber.StatusOK,
		pages.BadgeBelumDibaca(h.svc.Chat.BelumDibaca(ctx(c), user.ID)))
}
