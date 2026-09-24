package model

import "testing"

// Slot bisa saja ditandai aktif lalu materinya dihapus belakangan. Halaman
// publik tidak boleh menyisakan kotak kosong karena itu.
func TestSlotIklanLayakTayang(t *testing.T) {
	kasus := []struct {
		nama    string
		slot    SlotIklan
		harapan bool
	}{
		{"aktif dengan gambar", SlotIklan{Aktif: true, GambarURL: "/uploads/iklan/a.webp"}, true},
		{"aktif dengan teks", SlotIklan{Aktif: true, Teks: "Kedai Kopi"}, true},
		{"aktif tanpa isi", SlotIklan{Aktif: true}, false},
		{"berisi tapi nonaktif", SlotIklan{GambarURL: "/uploads/iklan/a.webp"}, false},
		{"kosong", SlotIklan{}, false},
	}
	for _, k := range kasus {
		if got := k.slot.LayakTayang(); got != k.harapan {
			t.Errorf("%s: LayakTayang = %v, harusnya %v", k.nama, got, k.harapan)
		}
	}
}

// Iklan tanpa tautan tetap ditayangkan, hanya saja tidak bisa diklik.
func TestSlotIklanDapatDiklik(t *testing.T) {
	if (SlotIklan{Teks: "Kedai Kopi"}).DapatDiklik() {
		t.Error("slot tanpa tautan tidak boleh diklaim bisa diklik")
	}
	if !(SlotIklan{TautanURL: "https://contoh.co.id"}).DapatDiklik() {
		t.Error("slot dengan tautan seharusnya bisa diklik")
	}
}

// Pencarian slot mengembalikan nil untuk apa pun yang belum layak tayang,
// sehingga template cukup memeriksa nil tanpa tahu alasannya.
func TestCariSlotIklan(t *testing.T) {
	p := PengaturanIklan{Slot: []SlotIklan{
		{Kunci: "beranda_atas", Aktif: true, Teks: "Kedai Kopi"},
		{Kunci: "cari_atas", Aktif: true},
		{Kunci: "detail_samping", Teks: "Nonaktif"},
	}}

	if got := p.Cari("beranda_atas"); got == nil || got.Teks != "Kedai Kopi" {
		t.Errorf("slot layak tayang tidak ditemukan: %v", got)
	}
	if p.Cari("cari_atas") != nil {
		t.Error("slot aktif tanpa isi seharusnya tidak ditayangkan")
	}
	if p.Cari("detail_samping") != nil {
		t.Error("slot nonaktif seharusnya tidak ditayangkan")
	}
	if p.Cari("kunci_asing") != nil {
		t.Error("kunci yang tidak dikenal seharusnya mengembalikan nil")
	}
}
