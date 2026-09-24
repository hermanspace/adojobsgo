package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestTokenBearer(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	var dapat string
	var lewat bool
	app.Get("/", func(c *fiber.Ctx) error {
		dapat, lewat = tokenBearer(c)
		return nil
	})
	kasus := map[string]struct {
		token string
		lewat bool
	}{
		"Bearer abc123": {"abc123", true},
		"bearer abc123": {"abc123", true}, // skema tidak peduli huruf besar
		"Basic abc123":  {"", false},
		"":              {"", false},
		"Bearer":        {"", false},
	}
	for header, mau := range kasus {
		req := httptest.NewRequest("GET", "/", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		if _, err := app.Test(req); err != nil {
			t.Fatal(err)
		}
		if dapat != mau.token || lewat != mau.lewat {
			t.Errorf("%q → (%q, %v), harusnya (%q, %v)", header, dapat, lewat, mau.token, mau.lewat)
		}
	}
}

// Permintaan ber-Authorization tidak memakai cookie, jadi penjaga CSRF
// tidak boleh menghalanginya.
func TestTolakLintasSitusAPIMelewatkanBearer(t *testing.T) {
	app := appCSRF()
	req := httptest.NewRequest("POST", "/api/ubah", nil)
	req.Header.Set("Authorization", "Bearer sesi-apa-pun")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != fiber.StatusOK {
		t.Errorf("POST dengan bearer tanpa X-Requested-With = %d, harusnya 200", res.StatusCode)
	}
}
