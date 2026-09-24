package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hermansyah/adojobsid/internal/database"
	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/repository"
)

// KunjunganService menghitung kunjungan halaman jasa dengan pola yang sama
// seperti tayangan iklan: satu INCR Redis per kunjungan, disalin ke tabel
// harian oleh tugas berkala. Database tidak pernah disentuh per kunjungan.
type KunjunganService struct {
	repos *repository.Repositories
	cache *database.Cache
	kini  func() time.Time
}

// polaBot mengenali perayap, pemindai, dan pengambil pratinjau tautan
// (WhatsApp/Facebook/Telegram mengambil halaman untuk kartu pratinjau).
var polaBot = regexp.MustCompile(`(?i)bot|crawl|spider|slurp|fetch|preview|facebookexternalhit|whatsapp|telegram|twitterbot|discord|curl/|wget/|python-requests|go-http-client|headless|lighthouse|pingdom|uptime`)

// AdalahBot menjawab apakah user agent bukan manusia. UA kosong ikut
// dianggap bot: peramban sungguhan selalu mengirimnya.
func AdalahBot(userAgent string) bool {
	ua := strings.TrimSpace(userAgent)
	return ua == "" || polaBot.MatchString(ua)
}

// SidikPengunjung menghasilkan penanda pengunjung untuk hitungan unik harian.
// IP dan user agent di-hash bersama tanggal sehingga tidak ada IP mentah
// yang disimpan, dan sidik yang sama tidak bisa dilacak lintas hari.
func SidikPengunjung(ip, userAgent string, hari time.Time) string {
	h := sha256.Sum256([]byte(ip + "|" + userAgent + "|" + hari.Format("2006-01-02")))
	return hex.EncodeToString(h[:8])
}

func (s *KunjunganService) sekarang() time.Time {
	if s.kini != nil {
		return s.kini()
	}
	return time.Now()
}

// Catat menaikkan penghitung kunjungan hari ini untuk satu jasa. Tanpa
// Redis (perintah CLI) tidak melakukan apa pun.
func (s *KunjunganService) Catat(ctx context.Context, serviceID int64, sidik string) {
	if s.cache == nil {
		return
	}
	hari := s.sekarang().Format("2006-01-02")
	id := strconv.FormatInt(serviceID, 10)
	s.cache.Tambah(ctx, "kunjungan:"+id+":"+hari)
	s.cache.TambahUnik(ctx, "kunjungan-unik:"+id+":"+hari, sidik, 48*time.Hour)
}

// Salin memindahkan penghitung Redis ke tabel harian. Mengembalikan jumlah
// baris (jasa-hari) yang diperbarui.
func (s *KunjunganService) Salin(ctx context.Context) int {
	if s.cache == nil {
		return 0
	}
	n := 0
	for kunci, jumlah := range s.cache.AmbilDanHapus(ctx, "kunjungan:*") {
		bagian := strings.Split(strings.TrimPrefix(kunci, "kunjungan:"), ":")
		if len(bagian) != 2 {
			continue
		}
		id, err := strconv.ParseInt(bagian[0], 10, 64)
		if err != nil {
			continue
		}
		tanggal, err := time.Parse("2006-01-02", bagian[1])
		if err != nil {
			continue
		}
		unik := s.cache.HitungUnik(ctx, "kunjungan-unik:"+bagian[0]+":"+bagian[1])
		if err := s.repos.Kunjungan.Tambah(ctx, id, tanggal, jumlah, unik); err == nil {
			n++
		}
	}
	return n
}

// Statistik menyusun ringkasan 30 hari. Publik dan tampil di setiap halaman
// jasa, jadi di-cache 60 detik: satu query per jasa per menit, bukan per
// tampilan.
func (s *KunjunganService) Statistik(ctx context.Context, serviceID int64) (*model.StatistikKunjungan, error) {
	return database.Remember(ctx, s.cache, "statistik:jasa:"+strconv.FormatInt(serviceID, 10), time.Minute, func() (*model.StatistikKunjungan, error) {
		return s.repos.Kunjungan.Statistik(ctx, serviceID, s.sekarang())
	})
}
