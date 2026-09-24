package service

import (
	"sync"

	"github.com/hermansyah/adojobsid/internal/model"
)

// Siaran adalah penyalur pesan dalam proses untuk SSE: tiap ruang obrolan
// punya daftar pelanggan, dan setiap pesan yang tersimpan disiarkan ke
// semuanya. Sengaja dalam proses — satu instance sudah cukup untuk skala
// kabupaten; bila kelak ada replika, ganti isi Siarkan dengan Redis pub/sub
// tanpa mengubah pemanggilnya.
type Siaran struct {
	mu        sync.Mutex
	pelanggan map[int64]map[chan model.Message]struct{}
}

func NewSiaran() *Siaran {
	return &Siaran{pelanggan: map[int64]map[chan model.Message]struct{}{}}
}

// Langgan mendaftarkan satu pendengar untuk satu ruang. batal harus
// dipanggil saat koneksi ditutup; tanpa itu saluran menumpuk.
func (s *Siaran) Langgan(convID int64) (<-chan model.Message, func()) {
	ch := make(chan model.Message, 16)
	s.mu.Lock()
	if s.pelanggan[convID] == nil {
		s.pelanggan[convID] = map[chan model.Message]struct{}{}
	}
	s.pelanggan[convID][ch] = struct{}{}
	s.mu.Unlock()

	return ch, func() {
		s.mu.Lock()
		delete(s.pelanggan[convID], ch)
		if len(s.pelanggan[convID]) == 0 {
			delete(s.pelanggan, convID)
		}
		s.mu.Unlock()
	}
}

// Siarkan mengirim ke semua pendengar ruang itu tanpa pernah memblokir
// pengirim: pendengar yang penuh (klien lambat) dilewati — ia akan menyusul
// lewat GET messages?since= saat menyambung ulang.
func (s *Siaran) Siarkan(convID int64, m model.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.pelanggan[convID] {
		select {
		case ch <- m:
		default:
		}
	}
}

// JumlahPendengar dipakai uji dan pemantauan.
func (s *Siaran) JumlahPendengar(convID int64) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pelanggan[convID])
}
