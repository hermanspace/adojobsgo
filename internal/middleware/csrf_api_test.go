package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/hermansyah/adojobsid/internal/view"
)

func appCSRF() *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	g := app.Group("/api", TolakLintasSitusAPI)
	g.Get("/baca", func(c *fiber.Ctx) error { return c.SendString("ok") })
	g.Post("/ubah", func(c *fiber.Ctx) error { return c.SendString("ok") })
	g.Delete("/hapus", func(c *fiber.Ctx) error { return c.SendString("ok") })
	return app
}

// Permintaan yang mengubah data tanpa header kustom persis seperti yang bisa
// dikirim form HTML dari situs lain — itulah yang harus ditolak.
func TestTolakLintasSitusAPITanpaHeader(t *testing.T) {
	app := appCSRF()
	for _, m := range []string{"POST", "DELETE"} {
		path := map[string]string{"POST": "/api/ubah", "DELETE": "/api/hapus"}[m]
		res, err := app.Test(httptest.NewRequest(m, path, nil))
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != fiber.StatusForbidden {
			t.Errorf("%s tanpa header = %d, harusnya 403", m, res.StatusCode)
		}
	}
}

func TestTolakLintasSitusAPIDenganHeader(t *testing.T) {
	app := appCSRF()
	req := httptest.NewRequest("POST", "/api/ubah", nil)
	req.Header.Set(HeaderPermintaanAPI, "XMLHttpRequest")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != fiber.StatusOK {
		t.Errorf("POST dengan header = %d, harusnya 200", res.StatusCode)
	}
}

// GET tidak mengubah apa pun, jadi tidak boleh ikut ditolak — kalau ikut,
// klien biasa yang cuma membaca akan terhalang tanpa alasan.
func TestTolakLintasSitusAPIMelewatkanGET(t *testing.T) {
	res, err := appCSRF().Test(httptest.NewRequest("GET", "/api/baca", nil))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != fiber.StatusOK {
		t.Errorf("GET tanpa header = %d, harusnya 200", res.StatusCode)
	}
}

// Kunci pembatas mengikuti akun; IP hanya untuk pengunjung tanpa akun.
// Pengguna di balik satu NAT tidak boleh saling menghabiskan kuota.
func TestKunciPembatasPengguna(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	var kunci string
	app.Get("/", func(c *fiber.Ctx) error {
		if c.Query("login") == "1" {
			c.Locals(LocalUser, &view.CurrentUser{ID: 42})
		}
		kunci = KunciPembatasPengguna(c)
		return c.SendString(kunci)
	})

	if _, err := app.Test(httptest.NewRequest("GET", "/?login=1", nil)); err != nil {
		t.Fatal(err)
	}
	if kunci != "u:42" {
		t.Errorf("pengguna masuk dikunci sebagai %q, harusnya u:42", kunci)
	}

	if _, err := app.Test(httptest.NewRequest("GET", "/", nil)); err != nil {
		t.Fatal(err)
	}
	if len(kunci) < 4 || kunci[:3] != "ip:" {
		t.Errorf("pengunjung tanpa akun dikunci sebagai %q, harusnya berawalan ip:", kunci)
	}
}
