package layout

import (
	"context"
	"strings"
	"testing"

	"github.com/hermansyah/adojobsid/internal/view"
)

func render(t *testing.T, b view.Base) string {
	t.Helper()
	var sb strings.Builder
	if err := MenuUtama(b).Render(context.Background(), &sb); err != nil {
		t.Fatal(err)
	}
	return sb.String()
}

func TestMenuUtamaMerenderSesuaiPeran(t *testing.T) {
	situs := view.SitusRingkas{Nama: "Adojobs", AplikasiTampil: true}

	tamu := render(t, view.Base{Situs: situs})
	for _, harus := range []string{`<dialog id="menu-utama"`, `data-menu-buka`, `data-menu-tutup`, `href="/masuk"`, `data-pasang-pwa`, `data-theme-toggle`} {
		if !strings.Contains(tamu, harus) {
			t.Errorf("tamu: %s tidak ada", harus)
		}
	}
	if strings.Contains(tamu, `action="/keluar"`) {
		t.Error("tamu mendapat tombol keluar")
	}

	penyedia := render(t, view.Base{Situs: situs, BelumDibaca: 2, User: &view.CurrentUser{ID: 1, FullName: "Budi", IsProvider: true, ProviderID: 3}})
	for _, harus := range []string{`action="/keluar" method="post"`, `href="/jasa/baru"`, `is-utama`, `Penyedia jasa`, `2 belum dibaca`, `href="/promosi/baru?jenis=sorotan_jasa"`} {
		if !strings.Contains(penyedia, harus) {
			t.Errorf("penyedia: %s tidak ada", harus)
		}
	}

	toko := render(t, view.Base{Situs: view.SitusRingkas{Nama: "Adojobs", AplikasiTampil: true, PlaystoreURL: "https://play.google.com/store/apps/details?id=id.adojobs"}})
	if !strings.Contains(toko, `href="https://play.google.com/store/apps/details?id=id.adojobs" class="sheet-item" target="_blank" rel="noopener"`) {
		t.Error("tautan Play Store tidak dirender sebagai tautan luar ber-noopener")
	}
	if strings.Contains(toko, "data-pasang-pwa") {
		t.Error("tombol PWA masih tampil padahal tautan toko sudah ada")
	}
}

func TestTampilkanMenu(t *testing.T) {
	for path, harap := range map[string]bool{"/": true, "/pesan": true, "/pesan/12": false, "/pesan/12/baru": false, "/admin/promosi": false, "/cari": true} {
		if got := TampilkanMenu(path); got != harap {
			t.Errorf("TampilkanMenu(%q) = %v, harusnya %v", path, got, harap)
		}
	}
}
