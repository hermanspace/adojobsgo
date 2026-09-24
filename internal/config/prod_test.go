package config

import (
	"strings"
	"testing"
)

func setEnvDasar(t *testing.T) {
	t.Helper()
	t.Setenv("POSTGRES_USER", "u")
	t.Setenv("POSTGRES_PASSWORD", "p")
	t.Setenv("POSTGRES_DB", "d")
	t.Setenv("SESSION_SECRET", strings.Repeat("x", 40))
}

// Produksi dengan cookie tidak-secure atau rahasia contoh harus gagal menyala.
func TestProdMenolakKonfigurasiBerbahaya(t *testing.T) {
	setEnvDasar(t)
	t.Setenv("APP_ENV", "prod")

	t.Setenv("SESSION_COOKIE_SECURE", "false")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "SESSION_COOKIE_SECURE") {
		t.Errorf("prod dengan cookie tidak secure lolos: %v", err)
	}

	t.Setenv("SESSION_COOKIE_SECURE", "true")
	t.Setenv("SESSION_SECRET", RahasiaContoh)
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "contoh") {
		t.Errorf("prod dengan rahasia contoh lolos: %v", err)
	}

	t.Setenv("SESSION_SECRET", strings.Repeat("y", 40))
	if _, err := Load(); err != nil {
		t.Errorf("prod yang benar ditolak: %v", err)
	}
}

// Lokal boleh longgar: pengembangan tanpa HTTPS tidak boleh dihalangi.
func TestLokalBolehTanpaSecure(t *testing.T) {
	setEnvDasar(t)
	t.Setenv("APP_ENV", "local")
	t.Setenv("SESSION_COOKIE_SECURE", "false")
	if _, err := Load(); err != nil {
		t.Errorf("lokal ditolak: %v", err)
	}
}
