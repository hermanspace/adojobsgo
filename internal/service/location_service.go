package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/hermansyah/adojobsid/internal/database"
	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/repository"
	"github.com/hermansyah/adojobsid/internal/validator"
)

// LokasiAktif adalah titik acuan pencari jasa pada satu permintaan.
// Sumbernya berjenjang: koordinat yang dipilih pengguna, lalu kecamatan pada
// profilnya, lalu titik pusat bawaan dari pengaturan.
type LokasiAktif struct {
	Label     string  `json:"label"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	// Tepat menandai lokasi yang benar-benar berasal dari titik pilihan
	// pengguna, bukan tebakan dari kecamatan atau nilai bawaan.
	Tepat  bool   `json:"tepat"`
	Sumber string `json:"sumber"` // pilihan | profil | bawaan
}

// Koordinat mengembalikan pointer lintang dan bujur untuk dipakai filter.
func (l LokasiAktif) Koordinat() (*float64, *float64) {
	lat, lng := l.Latitude, l.Longitude
	return &lat, &lng
}

// LocationService menerjemahkan masukan lokasi menjadi titik acuan yang bisa
// dipakai pencarian, tanpa pernah memanggil layanan geocoding luar.
type LocationService struct {
	repos    *repository.Repositories
	cache    *database.Cache
	settings *SettingsService
}

// FormatKoordinat menyusun nilai cookie lokasi: "lat,lng,label".
func FormatKoordinat(lat, lng float64, label string) string {
	return fmt.Sprintf("%.6f,%.6f,%s", lat, lng, strings.TrimSpace(label))
}

// ParseKoordinat membaca nilai cookie lokasi. Nilai yang rusak diabaikan
// diam-diam, karena isinya sepenuhnya berasal dari sisi klien.
func ParseKoordinat(v string) (lat, lng float64, label string, ok bool) {
	bagian := strings.SplitN(strings.TrimSpace(v), ",", 3)
	if len(bagian) < 2 {
		return 0, 0, "", false
	}
	lat, err1 := strconv.ParseFloat(strings.TrimSpace(bagian[0]), 64)
	lng, err2 := strconv.ParseFloat(strings.TrimSpace(bagian[1]), 64)
	if err1 != nil || err2 != nil || !model.KoordinatValid(lat, lng) {
		return 0, 0, "", false
	}
	if len(bagian) == 3 {
		label = strings.TrimSpace(bagian[2])
	}
	return lat, lng, label, true
}

// Resolve menentukan lokasi aktif untuk satu permintaan.
// cookieValue berasal dari peramban dan diperlakukan sebagai data tidak
// tepercaya: koordinatnya divalidasi, dan labelnya selalu ditulis ulang dari
// daftar kecamatan di aplikasi — tidak pernah ditampilkan apa adanya.
func (s *LocationService) Resolve(ctx context.Context, cookieValue string, userID int64) LokasiAktif {
	pengaturan := s.settings.Get(ctx)

	if lat, lng, _, ok := ParseKoordinat(cookieValue); ok {
		kec, jarak := model.KecamatanTerdekat(lat, lng)
		label := kec.Nama
		// Titik yang jauh dari seluruh kecamatan Bengkalis tidak diberi nama
		// wilayah yang menyesatkan.
		if jarak > 60 {
			label = "Lokasi Anda"
		}
		return LokasiAktif{
			Label: label, Latitude: lat, Longitude: lng,
			Tepat: true, Sumber: "pilihan",
		}
	}

	if userID > 0 {
		if user, err := s.repos.User.GetByID(ctx, userID); err == nil &&
			user.Kecamatan != nil && strings.TrimSpace(*user.Kecamatan) != "" {
			if kec, ok := model.CariKecamatan(strings.TrimSpace(*user.Kecamatan)); ok {
				return LokasiAktif{
					Label: kec.Nama, Latitude: kec.Latitude, Longitude: kec.Longitude,
					Tepat: false, Sumber: "profil",
				}
			}
		}
	}

	return LokasiAktif{
		Label:     "Kabupaten Bengkalis",
		Latitude:  pengaturan.Lokasi.PusatLatitude,
		Longitude: pengaturan.Lokasi.PusatLongitude,
		Tepat:     false,
		Sumber:    "bawaan",
	}
}

// DariKecamatan membentuk nilai cookie dari nama kecamatan yang dipilih
// pengguna lewat daftar, sebagai jalan keluar bila GPS ditolak.
func DariKecamatan(nama string) (string, bool) {
	kec, ok := model.CariKecamatan(strings.TrimSpace(nama))
	if !ok {
		return "", false
	}
	return FormatKoordinat(kec.Latitude, kec.Longitude, kec.Nama), true
}

// ---------- lokasi penyedia ----------

type LokasiProviderInput struct {
	Latitude     string
	Longitude    string
	AddressLabel string
	RadiusKm     string
	// Hapus mencabut titik lokasi penyedia.
	Hapus bool
}

// SimpanLokasiProvider menyimpan titik lokasi dan radius layanan penyedia.
func (s *LocationService) SimpanLokasiProvider(ctx context.Context, providerID int64, in LokasiProviderInput) error {
	errs := validator.New()
	pengaturan := s.settings.Get(ctx)

	radius, err := strconv.Atoi(strings.TrimSpace(in.RadiusKm))
	if err != nil || radius < 1 || radius > pengaturan.Lokasi.RadiusMaksKm {
		errs.Add("service_radius_km",
			fmt.Sprintf("Radius layanan harus antara 1 dan %d km.", pengaturan.Lokasi.RadiusMaksKm))
	}

	if in.Hapus {
		if errs.Any() {
			return Invalid(errs)
		}
		if err := s.repos.Provider.SetLocation(ctx, providerID, nil, nil, nil, radius); err != nil {
			return Internal(err)
		}
		s.invalidate()
		return nil
	}

	lat, errLat := strconv.ParseFloat(strings.TrimSpace(in.Latitude), 64)
	lng, errLng := strconv.ParseFloat(strings.TrimSpace(in.Longitude), 64)
	if errLat != nil || errLng != nil || !model.KoordinatValid(lat, lng) {
		errs.Add("lokasi", "Tentukan titik lokasi di peta terlebih dahulu.")
	}

	label := strings.TrimSpace(in.AddressLabel)
	errs.Length("address_label", "Keterangan alamat", label, 0, 200)

	if errs.Any() {
		return Invalid(errs)
	}

	var labelPtr *string
	if label != "" {
		labelPtr = &label
	}
	if err := s.repos.Provider.SetLocation(ctx, providerID, &lat, &lng, labelPtr, radius); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return NotFound("Profil penyedia tidak ditemukan.")
		}
		return Internal(err)
	}
	s.invalidate()
	return nil
}

func (s *LocationService) invalidate() {
	// Jarak ikut tersimpan di hasil pencarian yang di-cache.
	invalidateSearchCache(s.cache)
}
