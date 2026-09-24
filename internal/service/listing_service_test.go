package service

import "testing"

func TestParseRupiah(t *testing.T) {
	kasus := []struct {
		masukan string
		harapan float64
		nil     bool
		galat   bool
	}{
		{masukan: "150000", harapan: 150000},
		{masukan: "150.000", harapan: 150000},
		{masukan: "Rp 150.000", harapan: 150000},
		{masukan: "rp150000", harapan: 150000},
		{masukan: "2.500.000", harapan: 2500000},
		{masukan: "", nil: true},
		{masukan: "   ", nil: true},
		{masukan: "abc", galat: true},
		{masukan: "-5000", galat: true},
	}

	for _, k := range kasus {
		got, err := parseRupiah(k.masukan)
		switch {
		case k.galat:
			if err == nil {
				t.Errorf("parseRupiah(%q) seharusnya galat", k.masukan)
			}
		case k.nil:
			if err != nil || got != nil {
				t.Errorf("parseRupiah(%q) seharusnya nil tanpa galat, dapat %v, %v", k.masukan, got, err)
			}
		default:
			if err != nil {
				t.Errorf("parseRupiah(%q) galat tak terduga: %v", k.masukan, err)
			} else if got == nil || *got != k.harapan {
				t.Errorf("parseRupiah(%q) = %v, harusnya %v", k.masukan, got, k.harapan)
			}
		}
	}
}

func TestWhatsappLink(t *testing.T) {
	got := WhatsappLink("08117512001", "Halo, saya tertarik")
	harapan := "https://wa.me/628117512001?text=Halo%2C+saya+tertarik"
	if got != harapan {
		t.Errorf("WhatsappLink = %q, harusnya %q", got, harapan)
	}
	if WhatsappLink("", "halo") != "" {
		t.Error("nomor kosong seharusnya menghasilkan tautan kosong")
	}
}

func TestErrorDomain(t *testing.T) {
	err := NotFound("Jasa tidak ditemukan.")
	e, ok := AsError(err)
	if !ok {
		t.Fatal("NotFound seharusnya dikenali sebagai error domain")
	}
	if e.Code != CodeNotFound {
		t.Errorf("kode = %q, harusnya %q", e.Code, CodeNotFound)
	}
}
