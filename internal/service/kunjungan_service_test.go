package service

import (
	"testing"
	"time"
)

func TestAdalahBot(t *testing.T) {
	bot := []string{"", "Mozilla/5.0 (compatible; Googlebot/2.1)", "facebookexternalhit/1.1", "WhatsApp/2.23.20", "curl/8.4.0", "python-requests/2.31", "TelegramBot (like TwitterBot)"}
	for _, ua := range bot {
		if !AdalahBot(ua) {
			t.Errorf("%q harus dianggap bot", ua)
		}
	}
	manusia := []string{"Mozilla/5.0 (Linux; Android 14; SM-A546B) AppleWebKit/537.36 Chrome/128 Mobile Safari/537.36", "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_5) Safari/605.1.15", "adojobs-android/1.0"}
	for _, ua := range manusia {
		if AdalahBot(ua) {
			t.Errorf("%q bukan bot", ua)
		}
	}
}

func TestSidikPengunjungBerbedaPerHari(t *testing.T) {
	h1 := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	a := SidikPengunjung("1.2.3.4", "UA", h1)
	if a != SidikPengunjung("1.2.3.4", "UA", h1) || len(a) != 16 {
		t.Error("sidik harus stabil dalam hari yang sama, 16 heksa")
	}
	if a == SidikPengunjung("1.2.3.4", "UA", h1.AddDate(0, 0, 1)) || a == SidikPengunjung("1.2.3.5", "UA", h1) {
		t.Error("sidik harus berbeda lintas hari dan lintas IP")
	}
}
