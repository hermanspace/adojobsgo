package web

import (
	"github.com/gofiber/fiber/v2"

	"github.com/hermansyah/adojobsid/internal/middleware"
	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/service"
	"github.com/hermansyah/adojobsid/internal/session"
	"github.com/hermansyah/adojobsid/internal/view"
	"github.com/hermansyah/adojobsid/web/templates/pages"
)

// Dasbor adalah halaman akun. Untuk penyedia jasa halaman ini juga menampilkan
// seluruh listing miliknya, termasuk yang nonaktif.
func (h *Handler) Dasbor(c *fiber.Ctx) error {
	current := middleware.CurrentUser(c)

	user, err := h.svc.Auth.GetUser(ctx(c), current.ID)
	if err != nil {
		return h.errorPage(c, err)
	}

	data := view.DashboardData{
		Base: h.base(c, "Akun saya", "Kelola akun dan listing jasa Anda.", "akun"),
		User: user,
	}

	if user.IsProvider {
		profile, err := h.svc.Provider.GetByUserID(ctx(c), user.ID)
		if err == nil {
			data.Provider = profile
			listings, err := h.svc.Listing.ListByProvider(ctx(c), profile.ID, true)
			if err != nil {
				return h.errorPage(c, err)
			}
			data.Listings = listings
		}
	}
	return h.render(c, fiber.StatusOK, pages.Dashboard(data))
}

// ShowAkunProfil menampilkan form data akun.
func (h *Handler) ShowAkunProfil(c *fiber.Ctx) error {
	current := middleware.CurrentUser(c)
	user, err := h.svc.Auth.GetUser(ctx(c), current.ID)
	if err != nil {
		return h.errorPage(c, err)
	}

	form := view.NewForm()
	form.Set("full_name", user.FullName)
	form.Set("phone", user.Phone)
	form.Set("email", view.Deref(user.Email))
	form.Set("city", view.Deref(user.City))
	form.Set("kecamatan", view.Deref(user.Kecamatan))

	data := view.AuthData{
		Base: h.base(c, "Data akun", "Ubah data akun Anda.", "akun"),
		Form: form,
	}
	return h.render(c, fiber.StatusOK, pages.AccountForm(data))
}

// AkunProfil memproses perubahan data akun.
func (h *Handler) AkunProfil(c *fiber.Ctx) error {
	current := middleware.CurrentUser(c)
	in := service.UpdateProfileInput{
		FullName:  c.FormValue("full_name"),
		Email:     c.FormValue("email"),
		City:      c.FormValue("city"),
		Kecamatan: c.FormValue("kecamatan"),
	}

	user, err := h.svc.Auth.UpdateProfile(ctx(c), current.ID, in)
	if err != nil {
		form := view.NewForm()
		form.Set("full_name", in.FullName)
		form.Set("email", in.Email)
		form.Set("city", in.City)
		form.Set("kecamatan", in.Kecamatan)
		if u, gerr := h.svc.Auth.GetUser(ctx(c), current.ID); gerr == nil {
			form.Set("phone", u.Phone)
		}

		data := view.AuthData{
			Base: h.base(c, "Data akun", "Ubah data akun Anda.", "akun"),
			Form: form,
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
		return h.render(c, status, pages.AccountForm(data))
	}

	h.refreshSession(c, func(d *session.Data) { d.FullName = user.FullName })
	return h.redirectWithFlash(c, "/dasbor", "sukses", "Data akun berhasil diperbarui.")
}

// Tentang adalah halaman statis singkat tentang platform.
// Tentang memakai identitas dari pengaturan admin — nama situs dan deskripsi
// beranda — supaya tidak ada salinan nama yang harus diingat di kode ketika
// admin mengubahnya.
func (h *Handler) Tentang(c *fiber.Ctx) error {
	pengaturan := h.svc.Settings.Get(ctx(c))
	umum := pengaturan.Umum
	// Angka hidup dan paket tidak boleh menggagalkan halaman: bila gagal,
	// bagian itu sekadar kosong.
	ringkasan, _ := h.svc.Catalog.Ringkasan(ctx(c))
	paket, _ := h.svc.Paket.Daftar(ctx(c), true)
	kontak := ""
	if umum.WhatsappAktif {
		kontak = umum.KontakWA
	}
	data := view.TentangData{
		Base:      h.base(c, "Tentang & panduan", umum.NamaSitus+": apa itu, cara kerja, fitur, promosi, dan pertanyaan umum.", ""),
		Bagian:    view.Panduan(umum.NamaSitus),
		Paket:     paket,
		JasaAktif: ringkasan.JasaAktif,
		Penyedia:  ringkasan.Penyedia,
		Kecamatan: ringkasan.Kecamatan,
		KontakWA:  kontak,

		UjiCobaGratis: pengaturan.Promosi.UjiCobaGratis,
	}
	return h.render(c, fiber.StatusOK, pages.Tentang(data))
}

// Bantuan adalah alias /tentang#panduan supaya tautan "bantuan" dari mana
// pun (menu, notifikasi, aplikasi) punya alamat yang stabil.
func (h *Handler) Bantuan(c *fiber.Ctx) error {
	return c.Redirect("/tentang#cara-kerja", fiber.StatusMovedPermanently)
}

// Pesanan adalah penampung rute /pesanan. Fitur order dikerjakan pada tahap 2;
// untuk saat ini halaman memberi tahu status tersebut dengan jujur.
func (h *Handler) Pesanan(c *fiber.Ctx) error {
	_ = model.OrderPending
	data := view.ErrorData{
		Base:    h.base(c, "Pesanan", "Daftar pesanan jasa Anda.", "pesanan"),
		Code:    0,
		Heading: "Pesanan belum tersedia",
		Message: "Permintaan order dan riwayat pesanan akan aktif pada tahap berikutnya. Untuk sekarang, hubungi penyedia lewat halaman detail jasa.",
	}
	return h.render(c, fiber.StatusOK, pages.ErrorPage(data))
}
