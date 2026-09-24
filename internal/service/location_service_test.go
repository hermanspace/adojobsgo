package service

import (
	"testing"

	"github.com/hermansyah/adojobsid/internal/model"
)

func TestParseKoordinat(t *testing.T) {
	lat, lng, label, ok := ParseKoordinat("1.466700,102.100000,Bengkalis")
	if !ok {
		t.Fatal("nilai cookie yang sah ditolak")
	}
	if lat != 1.4667 || lng != 102.1 || label != "Bengkalis" {
		t.Errorf("hasil parse salah: %v, %v, %q", lat, lng, label)
	}

	// Tanpa label tetap sah.
	if _, _, _, ok := ParseKoordinat("1.4667,102.1"); !ok {
		t.Error("koordinat tanpa label seharusnya diterima")
	}

	// Nilai rusak diabaikan diam-diam; isinya sepenuhnya dari sisi klien.
	for _, rusak := range []string{
		"", "abc", "1.4667", "abc,def,Bengkalis", "999,999,X", "0,0,Nol",
	} {
		if _, _, _, ok := ParseKoordinat(rusak); ok {
			t.Errorf("nilai rusak %q seharusnya ditolak", rusak)
		}
	}
}

func TestFormatKoordinatBolakBalik(t *testing.T) {
	asli := FormatKoordinat(1.4667, 102.1, "Bengkalis")
	lat, lng, label, ok := ParseKoordinat(asli)
	if !ok || lat != 1.4667 || lng != 102.1 || label != "Bengkalis" {
		t.Errorf("format lalu parse tidak konsisten: %q -> %v %v %q", asli, lat, lng, label)
	}
}

func TestDariKecamatan(t *testing.T) {
	nilai, ok := DariKecamatan("Bantan")
	if !ok {
		t.Fatal("kecamatan yang ada ditolak")
	}
	lat, lng, label, parsed := ParseKoordinat(nilai)
	if !parsed || label != "Bantan" {
		t.Errorf("nilai kecamatan tidak terbaca: %q", nilai)
	}
	kec, _ := model.CariKecamatan("Bantan")
	if lat != kec.Latitude || lng != kec.Longitude {
		t.Errorf("koordinat tidak sesuai daftar: %v,%v vs %v,%v",
			lat, lng, kec.Latitude, kec.Longitude)
	}

	if _, ok := DariKecamatan("Entah Di Mana"); ok {
		t.Error("kecamatan yang tidak ada seharusnya ditolak")
	}
}

func TestLokasiAktifKoordinat(t *testing.T) {
	l := LokasiAktif{Latitude: 1.4667, Longitude: 102.1}
	lat, lng := l.Koordinat()
	if lat == nil || lng == nil || *lat != 1.4667 || *lng != 102.1 {
		t.Error("koordinat tidak dikembalikan dengan benar")
	}
	// Pointer harus menunjuk salinan, bukan field struct aslinya, supaya
	// pemanggil tidak bisa mengubah lokasi lewat pointer itu.
	*lat = 9
	if l.Latitude != 1.4667 {
		t.Error("mengubah pointer seharusnya tidak mengubah LokasiAktif")
	}
}

func TestNormalkanPengaturan(t *testing.T) {
	// Pengaturan kosong diisi nilai bawaan.
	p := normalkanPengaturan(model.Pengaturan{})
	if p.Umum.NamaSitus == "" {
		t.Error("nama situs kosong tidak diisi bawaan")
	}
	if p.Lokasi.RadiusDefaultKm <= 0 || p.Lokasi.RadiusMaksKm <= 0 {
		t.Error("radius kosong tidak diisi bawaan")
	}
	if !model.KoordinatValid(p.Lokasi.PusatLatitude, p.Lokasi.PusatLongitude) {
		t.Error("titik pusat kosong tidak diisi bawaan")
	}

	// Radius bawaan tidak boleh melampaui radius maksimum.
	p2 := normalkanPengaturan(model.Pengaturan{
		Lokasi: model.PengaturanLokasi{
			PusatLatitude: 1.4, PusatLongitude: 102.1,
			RadiusDefaultKm: 200, RadiusMaksKm: 50,
		},
	})
	if p2.Lokasi.RadiusDefaultKm > p2.Lokasi.RadiusMaksKm {
		t.Errorf("radius bawaan %d melebihi maksimum %d",
			p2.Lokasi.RadiusDefaultKm, p2.Lokasi.RadiusMaksKm)
	}

	// Slot iklan yang belum dikonfigurasi tetap muncul agar bisa diisi admin.
	if len(p.Iklan.Slot) != len(model.SlotIklanBawaan()) {
		t.Errorf("jumlah slot = %d, harusnya %d",
			len(p.Iklan.Slot), len(model.SlotIklanBawaan()))
	}

	// Slot lama yang sudah tidak dikenal dibuang.
	p3 := normalkanPengaturan(model.Pengaturan{
		Iklan: model.PengaturanIklan{Slot: []model.SlotIklan{
			{Kunci: "slot_yang_sudah_dihapus", Aktif: true},
		}},
	})
	for _, s := range p3.Iklan.Slot {
		if s.Kunci == "slot_yang_sudah_dihapus" {
			t.Error("slot yang tidak lagi didefinisikan seharusnya dibuang")
		}
	}
}

func TestBidangPentingBerubah(t *testing.T) {
	lama := &model.Service{Title: "Cuci AC", Description: "Deskripsi awal", CategoryID: 1}

	harga := *lama
	if bidangPentingBerubah(lama, &harga) {
		t.Error("tanpa perubahan apa pun tidak boleh dianggap berubah")
	}

	judul := *lama
	judul.Title = "Cuci AC dan isi freon"
	if !bidangPentingBerubah(lama, &judul) {
		t.Error("perubahan judul harus memicu peninjauan ulang")
	}

	deskripsi := *lama
	deskripsi.Description = "Deskripsi yang sudah diganti seluruhnya"
	if !bidangPentingBerubah(lama, &deskripsi) {
		t.Error("perubahan deskripsi harus memicu peninjauan ulang")
	}

	kategori := *lama
	kategori.CategoryID = 2
	if !bidangPentingBerubah(lama, &kategori) {
		t.Error("perubahan kategori harus memicu peninjauan ulang")
	}

	// Harga sengaja tidak memicu peninjauan ulang: itu perubahan paling lazim
	// dan paling tidak berisiko, dan menahannya membuat provider enggan
	// memperbarui harganya.
	hargaBaru := 150000.0
	ubahHarga := *lama
	ubahHarga.PriceMin = &hargaBaru
	if bidangPentingBerubah(lama, &ubahHarga) {
		t.Error("perubahan harga saja tidak boleh memicu peninjauan ulang")
	}
}

func TestTransisiStatusPesanan(t *testing.T) {
	svc := &OrderService{}

	menunggu := &model.Order{Status: model.OrderPending}
	pilihanPenyedia := svc.TindakanTersedia(menunggu, true)
	if len(pilihanPenyedia) != 3 {
		t.Errorf("penyedia pada pesanan menunggu punya %d pilihan, harusnya 3", len(pilihanPenyedia))
	}

	// Pencari jasa hanya boleh membatalkan.
	pilihanPencari := svc.TindakanTersedia(menunggu, false)
	if len(pilihanPencari) != 1 || pilihanPencari[0] != model.OrderCancelled {
		t.Errorf("pencari jasa seharusnya hanya bisa membatalkan, dapat %v", pilihanPencari)
	}

	// Status akhir tidak punya lanjutan.
	for _, s := range []model.OrderStatus{model.OrderCompleted, model.OrderRejected, model.OrderCancelled} {
		if got := svc.TindakanTersedia(&model.Order{Status: s}, true); len(got) != 0 {
			t.Errorf("status akhir %q masih menawarkan tindakan %v", s, got)
		}
	}

	if svc.TindakanTersedia(nil, true) != nil {
		t.Error("pesanan nil seharusnya tidak menawarkan tindakan")
	}
}
