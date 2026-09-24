package service

import (
	"testing"

	"github.com/hermansyah/adojobsid/internal/model"
)

func TestValidasiAjukan(t *testing.T) {
	kasus := []struct {
		nama       string
		in         AjukanInput
		providerID int64
		adaGambar  bool
		field      string // "" = harus lolos
	}{
		{"sorotan sah", AjukanInput{Jenis: model.PromosiSorotan, PaketID: 1, ServiceID: 5}, 7, false, ""},
		{"sorotan tanpa jasa", AjukanInput{Jenis: model.PromosiSorotan, PaketID: 1}, 7, false, "service_id"},
		{"sorotan bukan penyedia", AjukanInput{Jenis: model.PromosiSorotan, PaketID: 1, ServiceID: 5}, 0, false, "jenis"},
		{"pilihan sah", AjukanInput{Jenis: model.PromosiPenyedia, PaketID: 2}, 7, false, ""},
		{"pilihan bukan penyedia", AjukanInput{Jenis: model.PromosiPenyedia, PaketID: 2}, 0, false, "jenis"},
		{"iklan sah, pengguna biasa", AjukanInput{Jenis: model.PromosiIklan, PaketID: 3, Judul: "Kedai Kopi Adojobs", TargetKecamatan: []string{"Bengkalis", "Bantan"}}, 0, true, ""},
		{"iklan tanpa gambar", AjukanInput{Jenis: model.PromosiIklan, PaketID: 3, Judul: "Kedai Kopi Adojobs"}, 0, false, "gambar"},
		{"iklan judul pendek", AjukanInput{Jenis: model.PromosiIklan, PaketID: 3, Judul: "Ko"}, 0, true, "judul"},
		{"iklan tautan tanpa skema", AjukanInput{Jenis: model.PromosiIklan, PaketID: 3, Judul: "Kedai Kopi", TautanURL: "adojobs.id"}, 0, true, "tautan_url"},
		{"iklan kecamatan asing", AjukanInput{Jenis: model.PromosiIklan, PaketID: 3, Judul: "Kedai Kopi", TargetKecamatan: []string{"Dumai"}}, 0, true, "target_kecamatan"},
		{"tanpa paket", AjukanInput{Jenis: model.PromosiPenyedia}, 7, false, "paket_id"},
		{"jenis asing", AjukanInput{Jenis: "spanduk", PaketID: 1}, 7, false, "jenis"},
	}
	for _, k := range kasus {
		errs := validasiAjukan(k.in, k.providerID, k.adaGambar)
		switch {
		case k.field == "" && errs.Any():
			t.Errorf("%s: harusnya lolos, dapat %v", k.nama, errs)
		case k.field != "" && !errs.Has(k.field):
			t.Errorf("%s: kesalahan pada %q tidak terdeteksi (%v)", k.nama, k.field, errs)
		}
	}
}
