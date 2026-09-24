package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/hermansyah/adojobsid/internal/config"
	"github.com/hermansyah/adojobsid/internal/view"
)

// appUji membangun satu rute yang dijaga penjaga tertentu, dengan pengguna
// yang sudah ditentukan lebih dulu di Locals.
func appUji(penjaga fiber.Handler, user *view.CurrentUser) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(func(c *fiber.Ctx) error {
		if user != nil {
			c.Locals(LocalUser, user)
		}
		return c.Next()
	})
	app.Get("/dijaga", penjaga, func(c *fiber.Ctx) error {
		return c.SendString("lolos")
	})
	return app
}

func auth() *Auth {
	return NewAuth(nil, config.Session{CookieName: "uji"})
}

// Permintaan HTMX tidak boleh dijawab pengalihan 303.
//
// HTMX mengikuti pengalihan itu, mendapat HTML halaman tujuan, lalu
// menyuntikkannya ke elemen sasaran — pernah membuat seluruh halaman
// "Jadi penyedia jasa" muncul di dalam daftar foto saat admin menghapus foto.
// Header HX-Redirect membuat peramban benar-benar berpindah halaman.
func TestRequireProviderWebTidakMengalihkan303SaatHTMX(t *testing.T) {
	a := auth()
	bukanProvider := &view.CurrentUser{ID: 1, FullName: "Admin", IsAdmin: true}

	// Permintaan biasa: pengalihan 303 wajar.
	req := httptest.NewRequest(fiber.MethodGet, "/dijaga", nil)
	resp, err := appUji(a.RequireProviderWeb, bukanProvider).Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusSeeOther {
		t.Errorf("permintaan biasa = %d, harusnya 303", resp.StatusCode)
	}

	// Permintaan HTMX: 204 beserta HX-Redirect, bukan 303.
	reqHTMX := httptest.NewRequest(fiber.MethodGet, "/dijaga", nil)
	reqHTMX.Header.Set("HX-Request", "true")
	respHTMX, err := appUji(a.RequireProviderWeb, bukanProvider).Test(reqHTMX)
	if err != nil {
		t.Fatal(err)
	}
	if respHTMX.StatusCode == fiber.StatusSeeOther {
		t.Error("permintaan HTMX dijawab 303 — HTMX akan menyuntikkan halaman tujuan ke elemen sasaran")
	}
	if respHTMX.StatusCode != fiber.StatusNoContent {
		t.Errorf("permintaan HTMX = %d, harusnya 204", respHTMX.StatusCode)
	}
	if got := respHTMX.Header.Get("HX-Redirect"); got != "/provider/daftar" {
		t.Errorf("HX-Redirect = %q, harusnya /provider/daftar", got)
	}
}

// Panel admin menjawab 404 bagi yang tidak berhak. Untuk permintaan HTMX,
// jawabannya harus berupa status saja — mengirim HTML halaman 404 hanya akan
// disuntikkan ke elemen sasaran.
func TestRequireAdminWebTidakMengirimHalamanSaatHTMX(t *testing.T) {
	a := auth()
	bukanAdmin := &view.CurrentUser{ID: 2, FullName: "Penyedia", IsProvider: true, ProviderID: 5}

	req := httptest.NewRequest(fiber.MethodGet, "/dijaga", nil)
	req.Header.Set("HX-Request", "true")
	resp, err := appUji(a.RequireAdminWeb, bukanAdmin).Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("status = %d, harusnya 404", resp.StatusCode)
	}
	if resp.ContentLength > 0 {
		t.Errorf("jawaban HTMX seharusnya tanpa badan HTML, dapat %d byte", resp.ContentLength)
	}
}

func TestPenjagaMeloloskanYangBerhak(t *testing.T) {
	a := auth()

	kasus := []struct {
		nama    string
		penjaga fiber.Handler
		user    *view.CurrentUser
	}{
		{"provider atas rute provider", a.RequireProviderWeb,
			&view.CurrentUser{ID: 3, IsProvider: true, ProviderID: 9}},
		{"admin atas rute admin", a.RequireAdminWeb,
			&view.CurrentUser{ID: 4, IsAdmin: true}},
		{"pengguna masuk atas rute berlogin", a.RequireWeb,
			&view.CurrentUser{ID: 5}},
	}

	for _, k := range kasus {
		req := httptest.NewRequest(fiber.MethodGet, "/dijaga", nil)
		resp, err := appUji(k.penjaga, k.user).Test(req)
		if err != nil {
			t.Fatalf("%s: %v", k.nama, err)
		}
		if resp.StatusCode != fiber.StatusOK {
			t.Errorf("%s: status = %d, harusnya 200", k.nama, resp.StatusCode)
		}
	}
}

// Pengunjung anonim pada rute berlogin dijawab 401 saat HTMX, yang ditangani
// app.js dengan mengarahkan ke halaman masuk.
func TestRequireWebAnonimSaatHTMX(t *testing.T) {
	req := httptest.NewRequest(fiber.MethodGet, "/dijaga", nil)
	req.Header.Set("HX-Request", "true")

	resp, err := appUji(auth().RequireWeb, nil).Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("status = %d, harusnya 401", resp.StatusCode)
	}
}
