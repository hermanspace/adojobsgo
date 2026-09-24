package service

import (
	"testing"

	"github.com/hermansyah/adojobsid/internal/model"
)

func TestValidasiPaket(t *testing.T) {
	dasar := PaketInput{Jenis: model.PromosiSorotan, Nama: "30 hari", DurasiHari: 30, Bobot: 1}
	if errs := validasiPaket(dasar); errs.Any() {
		t.Fatalf("paket sah ditolak: %v", errs)
	}

	kasus := []struct {
		nama  string
		ubah  func(*PaketInput)
		field string
	}{
		{"jenis asing", func(p *PaketInput) { p.Jenis = "banner" }, "jenis"},
		{"durasi nol", func(p *PaketInput) { p.DurasiHari = 0 }, "durasi_hari"},
		{"durasi setahun lebih", func(p *PaketInput) { p.DurasiHari = 400 }, "durasi_hari"},
		{"harga negatif", func(p *PaketInput) { p.Harga = -1 }, "harga"},
		{"bobot di luar 1-10", func(p *PaketInput) { p.Bobot = 11 }, "bobot"},
		{"iklan tanpa slot", func(p *PaketInput) { p.Jenis = model.PromosiIklan }, "slot_iklan"},
		{"iklan slot asing", func(p *PaketInput) { p.Jenis = model.PromosiIklan; p.SlotIklan = []string{"footer"} }, "slot_iklan"},
		{"sorotan membawa slot", func(p *PaketInput) { p.SlotIklan = []string{"cari_atas"} }, "slot_iklan"},
	}
	for _, k := range kasus {
		in := dasar
		k.ubah(&in)
		if errs := validasiPaket(in); !errs.Has(k.field) {
			t.Errorf("%s: kesalahan pada %q tidak terdeteksi (%v)", k.nama, k.field, errs)
		}
	}

	iklan := PaketInput{Jenis: model.PromosiIklan, Nama: "Utama", DurasiHari: 14, Bobot: 3,
		SlotIklan: []string{"beranda_atas", "cari_atas"}}
	if errs := validasiPaket(iklan); errs.Any() {
		t.Errorf("paket iklan sah ditolak: %v", errs)
	}
}
