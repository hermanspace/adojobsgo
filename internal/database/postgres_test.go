package database

import "testing"

// Bawaan Postgres: max_connections=100, superuser_reserved=3 → tersedia 97.
// Pool bawaan aplikasi 10 harus longgar; yang menyentuh separuh diperingatkan;
// yang melebihi ditolak saat aplikasi dinyalakan, bukan saat ramai.
func TestNilaiKapasitas(t *testing.T) {
	kasus := []struct {
		pool, tersedia int32
		harapan        kapasitas
	}{
		{10, 97, kapasitasLonggar},
		{48, 97, kapasitasLonggar},
		{49, 97, kapasitasSempit},
		{97, 97, kapasitasSempit},
		{98, 97, kapasitasMelebihi},
		{10, 5, kapasitasMelebihi},
	}
	for _, k := range kasus {
		if got := nilaiKapasitas(k.pool, k.tersedia); got != k.harapan {
			t.Errorf("pool %d dari %d tersedia = %v, harusnya %v", k.pool, k.tersedia, got, k.harapan)
		}
	}
}
