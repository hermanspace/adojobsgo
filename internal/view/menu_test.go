package view

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// ikonTersedia membaca nama ikon dari icons.templ, supaya menu dan panduan
// tidak menunjuk ikon yang tidak ada — templ merender ikon tak dikenal
// sebagai kosong tanpa error.
func ikonTersedia(t *testing.T) map[string]bool {
	t.Helper()
	isi, err := os.ReadFile("../../web/templates/components/icons.templ")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, m := range regexp.MustCompile(`case "([a-z-]+)"`).FindAllStringSubmatch(string(isi), -1) {
		out[m[1]] = true
	}
	return out
}

func labelMenu(bagian []MenuBagian) []string {
	var out []string
	for _, b := range bagian {
		for _, it := range b.Item {
			out = append(out, it.Label)
		}
	}
	return out
}

func ada(daftar []string, cari string) bool {
	for _, d := range daftar {
		if d == cari {
			return true
		}
	}
	return false
}

func TestMenuUtamaMenurutPeran(t *testing.T) {
	situs := SitusRingkas{Nama: "Adojobs", AplikasiTampil: true}
	kasus := []struct {
		nama       string
		k          MenuKonteks
		harusAda   []string
		harusTiada []string
	}{
		{"tamu", MenuKonteks{Situs: situs},
			[]string{"Cari jasa", "Jadi penyedia jasa", "Pasang iklan", "Masuk", "Daftar akun baru", "Pasang di layar utama"},
			[]string{"Keluar", "Dasbor", "Sorot jasa", "Promosi saya", "Pasang jasa"}},
		{"pengguna", MenuKonteks{User: &CurrentUser{ID: 1}, Situs: situs, BelumDibaca: 3},
			[]string{"Pesan & pesanan", "Jadi penyedia jasa", "Pasang iklan", "Promosi saya", "Dasbor", "Keluar"},
			[]string{"Masuk", "Sorot jasa", "Pasang jasa", "Panel admin", "Portofolio"}},
		{"penyedia", MenuKonteks{User: &CurrentUser{ID: 1, IsProvider: true, ProviderID: 9}, Situs: situs},
			[]string{"Pasang jasa", "Jasa saya", "Sorot jasa", "Jadi penyedia pilihan", "Lokasi & area layanan", "Portofolio", "Keluar"},
			[]string{"Jadi penyedia jasa", "Masuk", "Panel admin"}},
		{"admin", MenuKonteks{User: &CurrentUser{ID: 1, IsAdmin: true}, Situs: situs},
			[]string{"Panel admin", "Keluar"}, []string{"Masuk"}},
	}
	ikon := ikonTersedia(t)
	for _, c := range kasus {
		t.Run(c.nama, func(t *testing.T) {
			bagian := MenuUtama(c.k)
			label := labelMenu(bagian)
			for _, l := range c.harusAda {
				if !ada(label, l) {
					t.Errorf("%q tidak ada; menu: %v", l, label)
				}
			}
			for _, l := range c.harusTiada {
				if ada(label, l) {
					t.Errorf("%q tidak boleh tampil untuk %s", l, c.nama)
				}
			}
			for _, b := range bagian {
				if len(b.Item) == 0 {
					t.Errorf("bagian %q kosong", b.Judul)
				}
				for _, it := range b.Item {
					if !ikon[it.Ikon] {
						t.Errorf("%q memakai ikon %q yang tidak ada di icons.templ", it.Label, it.Ikon)
					}
					if it.Jenis == MenuTautan && !strings.HasPrefix(it.Tautan, "/") {
						t.Errorf("%q: tautan %q bukan path web", it.Label, it.Tautan)
					}
					if it.Jenis == MenuAplikasi && it.Keadaan == "" {
						t.Errorf("%q: item aplikasi tanpa keadaan", it.Label)
					}
				}
			}
		})
	}
}

func TestItemAplikasiMengikutiPengaturan(t *testing.T) {
	if _, ada := itemAplikasi(SitusRingkas{}); ada {
		t.Error("aplikasi disembunyikan admin tetapi item muncul")
	}
	it, _ := itemAplikasi(SitusRingkas{AplikasiTampil: true})
	if it.Keadaan != AplikasiPWA || it.Tautan != "" {
		t.Errorf("tanpa tautan toko harus PWA: %+v", it)
	}
	it, _ = itemAplikasi(SitusRingkas{AplikasiTampil: true, PlaystoreURL: "https://play.google.com/x"})
	if it.Keadaan != AplikasiPlaystore || it.Tautan != "https://play.google.com/x" {
		t.Errorf("dengan tautan toko harus playstore: %+v", it)
	}
}

func TestBadgePesanMasukKeMenu(t *testing.T) {
	bagian := MenuUtama(MenuKonteks{User: &CurrentUser{ID: 1}, BelumDibaca: 4})
	for _, it := range bagian[0].Item {
		if it.Tautan == "/pesan" && it.Badge != 4 {
			t.Errorf("badge pesan = %d, harusnya 4", it.Badge)
		}
	}
}
