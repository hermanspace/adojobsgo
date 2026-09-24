package view

import (
	"strings"
	"testing"
)

func TestGayaHeroHanyaSaatAdaGambar(t *testing.T) {
	if got := (SitusRingkas{}).GayaHero(); got != "" {
		t.Errorf("tanpa gambar = %q, harusnya kosong", got)
	}
	if (SitusRingkas{}).PakaiHeroGambar() {
		t.Error("hero tanpa gambar tidak boleh mengaku bergambar")
	}

	s := SitusRingkas{HeroGambarURL: "/uploads/situs/abc.webp"}
	if got := s.GayaHero(); got != "background-image: url(/uploads/situs/abc.webp)" {
		t.Errorf("GayaHero = %q", got)
	}
}

// Overlay ditulis sebagai pecahan 0..1, bukan persen, karena dipasang ke
// properti opacity.
func TestGayaOverlayHero(t *testing.T) {
	kasus := map[int]string{
		55: "opacity: 0.55",
		25: "opacity: 0.25",
		85: "opacity: 0.85",
	}
	for persen, harapan := range kasus {
		if got := (SitusRingkas{HeroOverlay: persen}).GayaOverlayHero(); got != harapan {
			t.Errorf("overlay %d = %q, harusnya %q", persen, got, harapan)
		}
	}
}

// Lencana dikendalikan sakelarnya sendiri, bukan oleh ada-tidaknya tautan:
// selama aplikasinya belum terbit, lencana "segera hadir" tetap perlu tampil
// walau tautannya masih kosong.
func TestAdaAplikasi(t *testing.T) {
	if (SitusRingkas{PlaystoreURL: "https://play.google.com/store/apps/details?id=x"}).AdaAplikasi() {
		t.Error("tautan saja tidak boleh memunculkan lencana bila sakelarnya mati")
	}
	if !(SitusRingkas{AplikasiTampil: true}).AdaAplikasi() {
		t.Error("sakelar menyala seharusnya memunculkan lencana meski tanpa tautan")
	}
}

// Lencana tanpa tautan dirender sebagai keterangan, bukan tautan mati.
func TestAplikasiDapatDiklik(t *testing.T) {
	if (SitusRingkas{AplikasiTampil: true}).AplikasiDapatDiklik() {
		t.Error("lencana tanpa tautan tidak boleh diklaim bisa diklik")
	}
	s := SitusRingkas{AplikasiTampil: true, PlaystoreURL: "https://play.google.com/store/apps/details?id=x"}
	if !s.AplikasiDapatDiklik() {
		t.Error("lencana dengan tautan seharusnya bisa diklik")
	}
}

// Gaya hero tidak boleh bisa keluar dari properti CSS-nya sendiri. Nilai yang
// mengandung tanda kutip atau titik koma ditolak sanitizer templ; di sini yang
// dijaga adalah bentuk keluarannya tetap satu properti tunggal.
func TestGayaHeroSatuPropertiSaja(t *testing.T) {
	s := SitusRingkas{HeroGambarURL: "/uploads/situs/abc.webp"}
	if strings.Contains(s.GayaHero(), ";") {
		t.Error("GayaHero tidak boleh menghasilkan lebih dari satu properti")
	}
}
