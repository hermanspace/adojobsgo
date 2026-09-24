package repository

import (
	"strings"
	"testing"
)

func TestBuildUserFilterKosongTanpaKlausa(t *testing.T) {
	where, args := buildUserFilter(UserFilter{})
	if where != "" {
		t.Errorf("filter kosong seharusnya tanpa WHERE, dapat %q", where)
	}
	if len(args) != 0 {
		t.Errorf("filter kosong seharusnya tanpa argumen, dapat %d", len(args))
	}
}

func TestBuildUserFilterStatusDanTipe(t *testing.T) {
	where, _ := buildUserFilter(UserFilter{Status: "ditangguhkan", OnlyType: "provider"})
	if !strings.Contains(where, "u.suspended_at IS NOT NULL") {
		t.Errorf("filter ditangguhkan tidak diterapkan: %q", where)
	}
	if !strings.Contains(where, "u.is_provider = TRUE") {
		t.Errorf("filter tipe provider tidak diterapkan: %q", where)
	}

	where2, _ := buildUserFilter(UserFilter{Status: "aktif", OnlyType: "pencari"})
	if !strings.Contains(where2, "u.suspended_at IS NULL") {
		t.Errorf("filter aktif tidak diterapkan: %q", where2)
	}
	if !strings.Contains(where2, "u.is_provider = FALSE") {
		t.Errorf("filter tipe pencari tidak diterapkan: %q", where2)
	}

	// Nilai status yang tidak dikenal tidak boleh diam-diam menyaring apa pun.
	where3, _ := buildUserFilter(UserFilter{Status: "entah"})
	if strings.Contains(where3, "suspended_at") {
		t.Errorf("status tak dikenal seharusnya diabaikan: %q", where3)
	}
}

func TestBuildUserFilterPencarianTeks(t *testing.T) {
	where, args := buildUserFilter(UserFilter{Query: "hafiz"})
	if len(args) != 1 {
		t.Fatalf("jumlah argumen = %d, harusnya 1", len(args))
	}
	if s, ok := args[0].(string); !ok || s != "%hafiz%" {
		t.Errorf("argumen pencarian = %v, harusnya %%hafiz%%", args[0])
	}
	for _, kolom := range []string{"u.full_name", "u.phone", "u.email"} {
		if !strings.Contains(where, kolom) {
			t.Errorf("pencarian tidak mencakup %s: %q", kolom, where)
		}
	}
}

func TestUserOrderClause(t *testing.T) {
	if !strings.Contains(userOrderClause("nama"), "u.full_name ASC") {
		t.Error("urutan nama salah")
	}
	if !strings.Contains(userOrderClause("listing"), "s.total") {
		t.Error("urutan listing terbanyak salah")
	}
	if !strings.HasPrefix(userOrderClause("entah"), "ORDER BY") {
		t.Error("urutan bawaan harus tetap menghasilkan ORDER BY yang sah")
	}
}

func TestBuildAdminServiceFilterMenampilkanSemuaStatus(t *testing.T) {
	// Berbeda dengan pencarian publik, daftar admin tidak boleh otomatis
	// menyaring listing nonaktif — justru itu yang perlu dimoderasi.
	where, _ := buildAdminServiceFilter(AdminServiceFilter{})
	if strings.Contains(where, "status = 'active'") {
		t.Errorf("daftar admin seharusnya tidak menyaring status: %q", where)
	}
	if strings.Contains(where, "suspended_at") {
		t.Errorf("daftar admin seharusnya tetap menampilkan akun ditangguhkan: %q", where)
	}
}

func TestBuildAdminServiceFilterSorotDanTanpaFoto(t *testing.T) {
	where, _ := buildAdminServiceFilter(AdminServiceFilter{Featured: "ya", TanpaFoto: true})
	if !strings.Contains(where, "s.featured_until > NOW()") {
		t.Errorf("filter sedang disorot tidak diterapkan: %q", where)
	}
	if !strings.Contains(where, "service_images") {
		t.Errorf("filter tanpa foto tidak diterapkan: %q", where)
	}

	where2, _ := buildAdminServiceFilter(AdminServiceFilter{Featured: "tidak"})
	if !strings.Contains(where2, "featured_until IS NULL") {
		t.Errorf("filter tidak disorot harus mencakup yang belum pernah disorot: %q", where2)
	}
}

// Filter kategori pada panel admin harus ikut menangkap sub-kategori,
// sama seperti perilaku filter di halaman pencarian publik.
func TestBuildAdminServiceFilterKategoriMencakupAnak(t *testing.T) {
	where, args := buildAdminServiceFilter(AdminServiceFilter{CategoryID: 7})
	if !strings.Contains(where, "c.parent_id") {
		t.Errorf("filter kategori tidak mencakup sub-kategori: %q", where)
	}
	if len(args) != 1 || args[0] != int64(7) {
		t.Errorf("argumen kategori salah: %v", args)
	}
}

// Pencarian publik tidak boleh menampilkan listing milik akun yang ditangguhkan.
func TestBuildFilterPublikMenyembunyikanAkunDitangguhkan(t *testing.T) {
	where, _ := buildFilter(ServiceFilter{}, nil)
	if !strings.Contains(where, "u.suspended_at IS NULL") {
		t.Errorf("pencarian publik harus menyembunyikan akun ditangguhkan: %q", where)
	}
}
