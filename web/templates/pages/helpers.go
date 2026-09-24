package pages

import (
	"fmt"

	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/view"
)

// kecamatanBengkalis dipakai form profil dan registrasi agar pilihan lokasi
// konsisten dengan data yang dipakai filter pencarian.
func kecamatanBengkalis() []string { return model.KecamatanBengkalis }

// jumlahAktif menghitung listing berstatus aktif milik provider.
func jumlahAktif(items []model.ServiceCard) int {
	n := 0
	for _, it := range items {
		if it.Status == model.ServiceActive {
			n++
		}
	}
	return n
}

// labelRating menampilkan tanda hubung bila provider belum punya ulasan,
// bukan angka 0.0 yang bisa disalahartikan sebagai rating buruk.
func labelRating(avg float64, total int) string {
	if total == 0 {
		return "—"
	}
	return view.RatingLabel(avg)
}

// lokasiSubtitle menjelaskan keadaan lokasi penyedia di baris pengaturan.
func lokasiSubtitle(p *model.ProviderProfile) string {
	if p == nil || !p.PunyaLokasi() {
		return "Belum ditentukan — listing Anda tidak muncul di pencarian terdekat"
	}
	return fmt.Sprintf("Radius layanan %d km", p.ServiceRadiusKm)
}
