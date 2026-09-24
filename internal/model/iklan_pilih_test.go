package model

import (
	"testing"
	"time"
)

func ptrStr(s string) *string { return &s }

func iklanUji(id int64, slot []string, bobot int, target []string, selesai time.Time) IklanTayang {
	return IklanTayang{
		Promosi: Promosi{ID: id, Jenis: PromosiIklan, Status: StatusAktif, SelesaiAt: &selesai,
			TargetKecamatan: target, Judul: ptrStr("Iklan")},
		Slot: slot, Bobot: bobot,
	}
}

// Tiga tahap berurutan: slot dari paket, kecamatan penonton, lalu bobot.
func TestPilihIklanBertahap(t *testing.T) {
	kini := time.Now()
	besok, kemarin := kini.Add(24*time.Hour), kini.Add(-time.Hour)
	daftar := []IklanTayang{
		iklanUji(1, []string{"cari_atas"}, 1, nil, besok),
		iklanUji(2, []string{"beranda_atas", "cari_atas"}, 3, []string{"Rupat"}, besok),
		iklanUji(3, []string{"cari_atas"}, 1, nil, kemarin), // sudah habis
	}
	pertama := func(int) int { return 0 }

	// slot yang tidak ada di paket mana pun → nihil
	if got := PilihIklan(daftar, "detail_samping", "", kini, pertama); got != nil {
		t.Errorf("slot tanpa iklan seharusnya nil, dapat #%d", got.ID)
	}
	// beranda_atas hanya milik #2; penonton Bengkalis tidak ditargetkan #2 → nihil
	if got := PilihIklan(daftar, "beranda_atas", "Bengkalis", kini, pertama); got != nil {
		t.Errorf("iklan bertarget Rupat tampil ke penonton Bengkalis: #%d", got.ID)
	}
	if got := PilihIklan(daftar, "beranda_atas", "Rupat", kini, pertama); got == nil || got.ID != 2 {
		t.Errorf("penonton Rupat seharusnya melihat #2, dapat %v", got)
	}
	// tanpa kecamatan (lokasi belum dipilih) → semua yang belum habis lolos
	if got := PilihIklan(daftar, "cari_atas", "", kini, pertama); got == nil || got.ID != 1 {
		t.Errorf("undian 0 pada cari_atas seharusnya #1, dapat %v", got)
	}
	// yang sudah habis tidak pernah terpilih walau undian menunjuk ke sana
	if got := PilihIklan(daftar, "cari_atas", "", kini, func(n int) int { return n - 1 }); got == nil || got.ID == 3 {
		t.Errorf("iklan kedaluwarsa terpilih: %v", got)
	}
}

// Bobot 3 berarti tiga dari empat undian; pilihannya deterministik per undian.
func TestPilihIklanBerbobot(t *testing.T) {
	besok := time.Now().Add(24 * time.Hour)
	daftar := []IklanTayang{
		iklanUji(1, []string{"cari_atas"}, 1, nil, besok),
		iklanUji(2, []string{"cari_atas"}, 3, nil, besok),
	}
	harapan := map[int]int64{0: 1, 1: 2, 2: 2, 3: 2}
	for undian, id := range harapan {
		got := PilihIklan(daftar, "cari_atas", "Bengkalis", time.Now(), func(int) int { return undian })
		if got == nil || got.ID != id {
			t.Errorf("undian %d → #%v, harusnya #%d", undian, got, id)
		}
	}
}

func TestSlotDariIklan(t *testing.T) {
	ik := IklanTayang{Promosi: Promosi{ID: 7, Judul: ptrStr("Kedai Kopi"), GambarURL: ptrStr("/uploads/iklan/a.webp"), TautanURL: ptrStr("https://contoh.co.id")}}
	s := SlotDariIklan(ik, "cari_atas")
	if s.TautanURL != "/iklan/7/klik" {
		t.Errorf("tautan harus lewat rute klik, dapat %q", s.TautanURL)
	}
	if !s.LayakTayang() || s.Teks != "Kedai Kopi" || s.GambarURL != "/uploads/iklan/a.webp" {
		t.Errorf("slot tidak lengkap: %+v", s)
	}
	tanpaTautan := SlotDariIklan(IklanTayang{Promosi: Promosi{ID: 8, Judul: ptrStr("X")}}, "cari_atas")
	if tanpaTautan.DapatDiklik() {
		t.Error("iklan tanpa tautan tidak boleh bisa diklik")
	}
}

// Penyedia sekecamatan penonton didahulukan; lebih dari itu diacak.
func TestPilihPenyediaPilihan(t *testing.T) {
	lat, lng := PusatBengkalis.Latitude, PusatBengkalis.Longitude
	rupat := KecamatanBengkalisKoordinat[7] // Rupat
	buat := func(id int64, la, lo float64) ProviderDetail {
		var p ProviderDetail
		p.ID, p.Latitude, p.Longitude = id, &la, &lo
		return p
	}
	daftar := []ProviderDetail{
		buat(1, rupat.Latitude, rupat.Longitude),
		buat(2, lat, lng),
		buat(3, rupat.Latitude, rupat.Longitude),
		buat(4, lat, lng),
		buat(5, rupat.Latitude, rupat.Longitude),
	}
	got := PilihPenyediaPilihan(daftar, "Bengkalis", 3, func(n int) int { return 0 })
	if len(got) != 3 {
		t.Fatalf("jumlah = %d", len(got))
	}
	sekecamatan := 0
	for _, p := range got[:2] {
		if p.ID == 2 || p.ID == 4 {
			sekecamatan++
		}
	}
	if sekecamatan != 2 {
		t.Errorf("dua penyedia Bengkalis harus tampil lebih dulu, dapat %v", []int64{got[0].ID, got[1].ID, got[2].ID})
	}
	if semua := PilihPenyediaPilihan(daftar[:2], "Bengkalis", 4, func(int) int { return 0 }); len(semua) != 2 {
		t.Error("daftar yang lebih pendek dari n harus dikembalikan utuh")
	}
}
