package model

import (
	"testing"
	"time"
)

func TestRole(t *testing.T) {
	if !RoleAdmin.Valid() || !RoleUser.Valid() {
		t.Error("peran yang sah ditolak")
	}
	if Role("superadmin").Valid() {
		t.Error("peran di luar daftar seharusnya ditolak")
	}
}

func TestUserIsAdminDanIsSuspended(t *testing.T) {
	u := &User{Role: RoleUser}
	if u.IsAdmin() {
		t.Error("pengguna biasa tidak boleh dianggap admin")
	}
	if u.IsSuspended() {
		t.Error("akun tanpa suspended_at seharusnya aktif")
	}

	now := time.Now()
	u.Role = RoleAdmin
	u.SuspendedAt = &now
	if !u.IsAdmin() || !u.IsSuspended() {
		t.Error("status admin atau penangguhan tidak terbaca")
	}
}

// Sorotan yang sudah lewat tanggalnya tidak boleh dianggap masih aktif —
// kolomnya sengaja tidak dibersihkan otomatis, jadi pemeriksaan waktu
// inilah yang menjadi penentu.
func TestIsFeaturedMenghormatiKedaluwarsa(t *testing.T) {
	lalu := time.Now().Add(-time.Hour)
	depan := time.Now().Add(24 * time.Hour)

	if (Service{FeaturedUntil: &lalu}).IsFeatured() {
		t.Error("listing dengan sorotan kedaluwarsa masih dianggap disorot")
	}
	if !(Service{FeaturedUntil: &depan}).IsFeatured() {
		t.Error("listing dengan sorotan berlaku tidak terbaca")
	}
	if (Service{}).IsFeatured() {
		t.Error("listing tanpa sorotan tidak boleh dianggap disorot")
	}

	if (ProviderProfile{FeaturedUntil: &lalu}).IsFeatured() {
		t.Error("penyedia dengan sorotan kedaluwarsa masih dianggap pilihan")
	}
	if !(ProviderProfile{FeaturedUntil: &depan}).IsFeatured() {
		t.Error("penyedia pilihan yang berlaku tidak terbaca")
	}
}

func TestProviderDetailIsSuspended(t *testing.T) {
	now := time.Now()
	if (ProviderDetail{}).IsSuspended() {
		t.Error("penyedia tanpa penangguhan dianggap ditangguhkan")
	}
	if !(ProviderDetail{SuspendedAt: &now}).IsSuspended() {
		t.Error("penangguhan penyedia tidak terbaca")
	}
}

// Dua jenis sorotan disetel admin terpisah dan harus bisa dibedakan kartu.
func TestPenyediaDisorotTerpisahDariSorotanJasa(t *testing.T) {
	depan := time.Now().Add(24 * time.Hour)
	lalu := time.Now().Add(-24 * time.Hour)

	// Jasa biasa milik penyedia pilihan.
	c := ServiceCard{ProviderFeaturedUntil: &depan}
	if !c.PenyediaDisorot() {
		t.Error("penyedia pilihan seharusnya terdeteksi di kartu jasanya")
	}
	if c.IsFeatured() {
		t.Error("sorotan penyedia tidak boleh membuat jasanya ikut dianggap disorot")
	}

	// Sorotan penyedia yang sudah lewat masa tayangnya ikut berhenti.
	if (ServiceCard{ProviderFeaturedUntil: &lalu}).PenyediaDisorot() {
		t.Error("sorotan penyedia yang kedaluwarsa masih dianggap aktif")
	}
	if (ServiceCard{}).PenyediaDisorot() {
		t.Error("kartu tanpa data sorotan penyedia tidak boleh dianggap disorot")
	}
}
