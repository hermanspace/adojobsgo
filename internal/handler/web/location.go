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

const lokasiCookie = "adojobs_lokasi"

// lokasiAktif menentukan titik acuan pencari jasa untuk permintaan ini.
func (h *Handler) lokasiAktif(c *fiber.Ctx) service.LokasiAktif {
	var userID int64
	if u := middleware.CurrentUser(c); u != nil {
		userID = u.ID
	}
	return h.svc.Location.Resolve(ctx(c), c.Cookies(lokasiCookie), userID)
}

// simpanLokasiCookie menyimpan titik lokasi pilihan pengguna.
// Cookie sengaja tidak HttpOnly karena JavaScript perlu menuliskannya setelah
// peramban memberi koordinat GPS; isinya hanya koordinat, bukan rahasia.
func (h *Handler) simpanLokasiCookie(c *fiber.Ctx, nilai string) {
	c.Cookie(&fiber.Cookie{
		Name:     lokasiCookie,
		Value:    nilai,
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 90,
		HTTPOnly: false,
		Secure:   h.cfg.Session.Secure,
		SameSite: "Lax",
	})
}

// SetLokasi menerima titik lokasi dari peramban atau pilihan kecamatan.
func (h *Handler) SetLokasi(c *fiber.Ctx) error {
	kembali := safeNext(c.FormValue("next"))
	if kembali == "" {
		kembali = "/"
	}

	if kec := strings.TrimSpace(c.FormValue("kecamatan")); kec != "" {
		nilai, ok := service.DariKecamatan(kec)
		if !ok {
			return h.redirectWithFlash(c, kembali, "galat", "Kecamatan tidak dikenal.")
		}
		h.simpanLokasiCookie(c, nilai)
		return h.redirect(c, kembali)
	}

	lat, errLat := strconv.ParseFloat(strings.TrimSpace(c.FormValue("latitude")), 64)
	lng, errLng := strconv.ParseFloat(strings.TrimSpace(c.FormValue("longitude")), 64)
	if errLat != nil || errLng != nil || !model.KoordinatValid(lat, lng) {
		return h.redirectWithFlash(c, kembali, "galat", "Titik lokasi tidak terbaca.")
	}

	// Label ditulis ulang dari daftar kecamatan aplikasi, bukan dari kiriman
	// klien, supaya teks yang tampil di antarmuka tidak bisa disisipi.
	kec, _ := model.KecamatanTerdekat(lat, lng)
	h.simpanLokasiCookie(c, service.FormatKoordinat(lat, lng, kec.Nama))
	return h.redirect(c, kembali)
}

// HapusLokasi mengembalikan acuan ke lokasi bawaan.
func (h *Handler) HapusLokasi(c *fiber.Ctx) error {
	c.Cookie(&fiber.Cookie{
		Name:     lokasiCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HTTPOnly: false,
		Secure:   h.cfg.Session.Secure,
		SameSite: "Lax",
	})
	kembali := safeNext(c.FormValue("next"))
	if kembali == "" {
		kembali = "/"
	}
	return h.redirect(c, kembali)
}

// ---------- lokasi penyedia ----------

func (h *Handler) dataProviderLokasi(c *fiber.Ctx, form view.Form) (*view.ProviderLokasiData, error) {
	user := middleware.CurrentUser(c)
	profile, err := h.svc.Provider.GetByUserID(ctx(c), user.ID)
	if err != nil {
		return nil, err
	}
	pengaturan := h.svc.Settings.Get(ctx(c))

	return &view.ProviderLokasiData{
		Base:       h.base(c, "Lokasi layanan", "Tentukan titik lokasi dan radius layanan Anda.", "akun").DenganPeta(),
		Form:       form,
		Profile:    profile,
		PusatLat:   pengaturan.Lokasi.PusatLatitude,
		PusatLng:   pengaturan.Lokasi.PusatLongitude,
		RadiusMaks: pengaturan.Lokasi.RadiusMaksKm,
		Kecamatan:  model.KecamatanBengkalisKoordinat,
	}, nil
}

func (h *Handler) ShowProviderLokasi(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	profile, err := h.svc.Provider.GetByUserID(ctx(c), user.ID)
	if err != nil {
		return h.errorPage(c, err)
	}

	form := view.NewForm()
	form.Set("service_radius_km", strconv.Itoa(profile.ServiceRadiusKm))
	form.Set("address_label", view.Deref(profile.AddressLabel))
	if profile.PunyaLokasi() {
		form.Set("latitude", strconv.FormatFloat(*profile.Latitude, 'f', 6, 64))
		form.Set("longitude", strconv.FormatFloat(*profile.Longitude, 'f', 6, 64))
	}

	data, err := h.dataProviderLokasi(c, form)
	if err != nil {
		return h.errorPage(c, err)
	}
	return h.render(c, fiber.StatusOK, pages.ProviderLokasi(*data))
}

func (h *Handler) ProviderLokasi(c *fiber.Ctx) error {
	user := middleware.CurrentUser(c)
	in := service.LokasiProviderInput{
		Latitude:     c.FormValue("latitude"),
		Longitude:    c.FormValue("longitude"),
		AddressLabel: c.FormValue("address_label"),
		RadiusKm:     c.FormValue("service_radius_km"),
		Hapus:        c.FormValue("hapus") == "1",
	}

	if err := h.svc.Location.SimpanLokasiProvider(ctx(c), user.ProviderID, in); err != nil {
		form := view.NewForm()
		form.Set("latitude", in.Latitude)
		form.Set("longitude", in.Longitude)
		form.Set("address_label", in.AddressLabel)
		form.Set("service_radius_km", in.RadiusKm)

		data, derr := h.dataProviderLokasi(c, form)
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
		}
		return h.render(c, status, pages.ProviderLokasi(*data))
	}
	return h.redirectWithFlash(c, "/dasbor", "sukses", "Lokasi layanan tersimpan.")
}
