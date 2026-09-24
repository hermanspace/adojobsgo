package view

import (
	"strings"
	"testing"

	"github.com/hermansyah/adojobsid/internal/model"
)

func TestBagikanJasa(t *testing.T) {
	kec := "Bengkalis"
	s := &model.ServiceDetail{
		Service:  model.Service{ID: 42, Title: "Cuci AC & isi freon"},
		Images:   []model.ServiceImage{{ImageURL: "/uploads/listing/2026/09/a.webp"}},
		Provider: model.ProviderDetail{ProviderProfile: model.ProviderProfile{Slug: "rizal"}, FullName: "Rizal Teknik AC", Kecamatan: &kec},
	}
	b := BagikanJasa("https://adojobs.id/", s)
	if b.URL != "https://adojobs.id/jasa/42" {
		t.Errorf("URL = %q", b.URL)
	}
	if b.GambarURL != "https://adojobs.id/uploads/listing/2026/09/a.webp" {
		t.Errorf("gambar = %q", b.GambarURL)
	}
	if !strings.Contains(b.Teks, "Rizal Teknik AC (Bengkalis)") {
		t.Errorf("teks tidak menyebut penyedia & kecamatan: %q", b.Teks)
	}
	kunci := map[string]TargetBagikan{}
	for _, tg := range b.Target {
		kunci[tg.Kunci] = tg
	}
	if wa := kunci["whatsapp"].URL; !strings.HasPrefix(wa, "https://wa.me/?text=") || !strings.Contains(wa, "https%3A%2F%2Fadojobs.id%2Fjasa%2F42") || strings.Contains(wa, "&") {
		t.Errorf("WhatsApp: teks & tautan harus di-escape utuh: %q", wa)
	}
	if fb := kunci["facebook"].URL; fb != "https://www.facebook.com/sharer/sharer.php?u=https%3A%2F%2Fadojobs.id%2Fjasa%2F42" {
		t.Errorf("Facebook: %q", fb)
	}
	if kunci["salin"].URL != "" || kunci["salin"].Label == "" {
		t.Error("target salin harus tanpa URL tapi berlabel")
	}
	if len(b.Target) != 5 {
		t.Errorf("target = %d", len(b.Target))
	}
}

func TestBagikanPenyediaTanpaAvatar(t *testing.T) {
	b := BagikanPenyedia("https://adojobs.id", &model.ProviderDetail{ProviderProfile: model.ProviderProfile{Slug: "wan-dok"}, FullName: "Wan Dokumentasi"})
	if b.URL != "https://adojobs.id/penyedia/wan-dok" || b.GambarURL != "" || strings.Contains(b.Teks, "()") {
		t.Errorf("%+v", b)
	}
}
