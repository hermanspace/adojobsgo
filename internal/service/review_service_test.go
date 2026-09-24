package service

import (
	"testing"

	"github.com/hermansyah/adojobsid/internal/model"
)

// Ulasan hanya boleh ditulis pemesan, dan hanya untuk pesanan yang sudah
// ditandai selesai. Aturan itu dijaga di service layer, bukan di antarmuka.
func TestDapatDiulasMenolakKondisiYangSalah(t *testing.T) {
	const pemesan int64 = 7

	kasus := []struct {
		nama    string
		order   *model.Order
		userID  int64
		harapan bool
	}{
		{"pesanan nil", nil, pemesan, false},
		{"belum selesai",
			&model.Order{ID: 1, SeekerID: pemesan, Status: model.OrderPending}, pemesan, false},
		{"diterima tapi belum selesai",
			&model.Order{ID: 1, SeekerID: pemesan, Status: model.OrderAccepted}, pemesan, false},
		{"ditolak",
			&model.Order{ID: 1, SeekerID: pemesan, Status: model.OrderRejected}, pemesan, false},
		{"dibatalkan",
			&model.Order{ID: 1, SeekerID: pemesan, Status: model.OrderCancelled}, pemesan, false},
		{"bukan pemesannya",
			&model.Order{ID: 1, SeekerID: 99, Status: model.OrderCompleted}, pemesan, false},
	}

	// Pemeriksaan yang tidak menyentuh basis data dijalankan tanpa repository;
	// seluruh kasus di atas gugur sebelum pencarian ulasan dilakukan.
	s := &ReviewService{}
	for _, k := range kasus {
		if got := s.DapatDiulas(nil, k.order, k.userID); got != k.harapan {
			t.Errorf("%s: DapatDiulas = %v, harusnya %v", k.nama, got, k.harapan)
		}
	}
}

func TestRingkasanUlasanJumlahBintang(t *testing.T) {
	r := RingkasanUlasan{Total: 4, Sebaran: map[int]int{5: 3, 2: 1}}

	if got := r.JumlahBintang(5); got != 3 {
		t.Errorf("jumlah bintang 5 = %d, harusnya 3", got)
	}
	// Nilai bintang yang tidak pernah diberikan mengembalikan nol, bukan panik.
	if got := r.JumlahBintang(4); got != 0 {
		t.Errorf("bintang tanpa ulasan = %d, harusnya 0", got)
	}
}

func TestProfilPortofolioMenghasilkanThumbnail(t *testing.T) {
	// Galeri portofolio memuat banyak gambar sekaligus, jadi thumbnail wajib.
	if ProfilPortofolio.ThumbSisi <= 0 {
		t.Error("profil portofolio harus menghasilkan thumbnail")
	}
	// Ukuran tampilannya lebih kecil daripada foto listing karena karya
	// portofolio dilihat dalam galeri, bukan sebagai gambar utama halaman.
	if ProfilPortofolio.MaksSisi >= ProfilListing.MaksSisi {
		t.Errorf("portofolio %d px tidak lebih kecil dari listing %d px",
			ProfilPortofolio.MaksSisi, ProfilListing.MaksSisi)
	}
	if ProfilPortofolio.Subdir == ProfilListing.Subdir {
		t.Error("portofolio dan foto listing seharusnya disimpan di folder berbeda")
	}
}
