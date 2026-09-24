package service

import "testing"

// Nomor WhatsApp opsional: kosong lolos, salah format ditolak, benar
// dinormalkan ke format internasional.
func TestValidasiWhatsappOpsional(t *testing.T) {
	svc := &ProviderService{}
	dasar := ProviderProfileInput{Bio: "Teknisi pendingin ruangan sejak 2014, melayani rumah dan ruko."}

	in := dasar
	in.WhatsappNumber = ""
	if errs, _ := svc.validate(in); errs.Has("whatsapp_number") {
		t.Errorf("nomor kosong ditolak: %v", errs["whatsapp_number"])
	}

	in.WhatsappNumber = "   "
	if errs, _ := svc.validate(in); errs.Has("whatsapp_number") {
		t.Error("nomor berisi spasi saja harus dianggap kosong, bukan salah format")
	}

	in.WhatsappNumber = "abc"
	if errs, _ := svc.validate(in); !errs.Has("whatsapp_number") {
		t.Error("nomor salah format lolos begitu saja")
	}

	in.WhatsappNumber = "0812-3456-7890"
	errs, wa := svc.validate(in)
	if errs.Has("whatsapp_number") {
		t.Errorf("nomor sah ditolak: %v", errs["whatsapp_number"])
	}
	if wa != "6281234567890" {
		t.Errorf("nomor tidak dinormalkan: %q", wa)
	}
}
