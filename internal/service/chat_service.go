package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/repository"
	"github.com/hermansyah/adojobsid/internal/validator"
)

// ChatService melayani percakapan antara pencari jasa dan penyedia.
type ChatService struct {
	repos  *repository.Repositories
	siaran *Siaran
}

// Langgan meneruskan pendaftaran pendengar SSE untuk satu ruang; akses ke
// ruangnya diperiksa pemanggil lewat Akses.
func (s *ChatService) Langgan(convID int64) (<-chan model.Message, func()) {
	return s.siaran.Langgan(convID)
}

// TandaiTerbaca dipakai jalur SSE: pesan yang tersalur ke penerima dianggap
// terbaca, sama seperti saat diambil lewat polling.
func (s *ChatService) TandaiTerbaca(ctx context.Context, convID, userID int64) {
	_ = s.repos.Chat.MarkRead(ctx, convID, userID)
}

const panjangPesanMaks = 2000

// Mulai membuka percakapan untuk satu listing, atau mengembalikan percakapan
// yang sudah ada. Penyedia tidak bisa memulai percakapan pada listingnya
// sendiri, karena percakapan selalu berpangkal dari pencari jasa.
func (s *ChatService) Mulai(ctx context.Context, serviceID, seekerID int64) (*model.Conversation, error) {
	svc, err := s.repos.Service.GetByID(ctx, serviceID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, NotFound("Jasa tidak ditemukan.")
		}
		return nil, Internal(err)
	}

	provider, err := s.repos.Provider.GetByID(ctx, svc.ProviderID)
	if err != nil {
		return nil, Internal(err)
	}
	if provider.UserID == seekerID {
		return nil, InvalidMsg("Anda tidak dapat menghubungi listing milik Anda sendiri.")
	}

	conv, err := s.repos.Chat.EnsureConversation(ctx, serviceID, seekerID, svc.ProviderID)
	if err != nil {
		return nil, Internal(err)
	}
	return conv, nil
}

// Akses memastikan pengguna adalah salah satu pihak dalam percakapan.
// Admin sengaja tidak diberi akses membaca isi obrolan: moderasi platform
// tidak memerlukan pembacaan percakapan pribadi antar pengguna.
func (s *ChatService) Akses(ctx context.Context, convID, userID int64) (*model.ConversationRow, error) {
	row, err := s.repos.Chat.GetConversationRow(ctx, convID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, NotFound("Percakapan tidak ditemukan.")
		}
		return nil, Internal(err)
	}
	return row, nil
}

func (s *ChatService) Daftar(ctx context.Context, userID int64) ([]model.ConversationRow, error) {
	rows, err := s.repos.Chat.ListConversations(ctx, userID)
	if err != nil {
		return nil, Internal(err)
	}
	return rows, nil
}

// Kirim menyimpan pesan baru dan membuat notifikasi untuk lawan bicara.
func (s *ChatService) Kirim(ctx context.Context, convID, senderID int64, body string) (*model.Message, error) {
	row, err := s.Akses(ctx, convID, senderID)
	if err != nil {
		return nil, err
	}

	body = strings.TrimSpace(body)
	if body == "" {
		errs := validator.New()
		errs.Add("body", "Tulis pesan terlebih dahulu.")
		return nil, Invalid(errs)
	}
	if utf8.RuneCountInString(body) > panjangPesanMaks {
		errs := validator.New()
		errs.Add("body", fmt.Sprintf("Pesan maksimal %d karakter.", panjangPesanMaks))
		return nil, Invalid(errs)
	}

	msg := &model.Message{ConversationID: convID, SenderID: senderID, Body: body}
	if err := s.repos.Chat.AddMessage(ctx, msg); err != nil {
		return nil, Internal(err)
	}

	// Notifikasi dibuat untuk lawan bicara. Kegagalannya tidak boleh
	// membatalkan pesan yang sudah tersimpan.
	_ = s.repos.Notif.Create(ctx, row.LawanUserID, repository.NotifPesanBaru, map[string]any{
		"conversation_id": convID,
		"service_title":   row.ServiceTitle,
		"cuplikan":        potongPesan(body, 80),
	})
	if s.siaran != nil {
		s.siaran.Siarkan(convID, *msg)
	}
	return msg, nil
}

// Pesan mengembalikan riwayat percakapan sekaligus menandainya sudah dibaca.
// sinceID > 0 hanya mengambil pesan yang lebih baru, dipakai polling.
func (s *ChatService) Pesan(ctx context.Context, convID, userID, sinceID int64) ([]model.MessageRow, error) {
	if _, err := s.Akses(ctx, convID, userID); err != nil {
		return nil, err
	}
	msgs, err := s.repos.Chat.ListMessages(ctx, convID, sinceID, 200)
	if err != nil {
		return nil, Internal(err)
	}
	// Tanda baca hanya ditulis bila memang ada yang baru terbaca. Polling
	// tiap 3 detik dari ruang yang sunyi sebelumnya tetap menulis ke DB —
	// 1.200 write per jam per ruang, tanpa satu pun yang mengubah apa-apa.
	if len(msgs) > 0 {
		if err := s.repos.Chat.MarkRead(ctx, convID, userID); err != nil {
			return nil, Internal(err)
		}
	}
	return msgs, nil
}

// BelumDibaca menghitung seluruh pesan belum dibaca milik satu pengguna.
func (s *ChatService) BelumDibaca(ctx context.Context, userID int64) int {
	n, err := s.repos.Chat.CountUnread(ctx, userID)
	if err != nil {
		return 0
	}
	return n
}

func potongPesan(s string, maks int) string {
	r := []rune(s)
	if len(r) <= maks {
		return s
	}
	return string(r[:maks]) + "…"
}

// ---------- pesanan ----------

// OrderService mengatur siklus permintaan order.
type OrderService struct {
	repos *repository.Repositories
}

type OrderInput struct {
	ServiceID     int64
	ScheduledDate string
	Notes         string
}

// Buat membuat permintaan order dan menautkannya ke percakapan, supaya
// pembahasan order berlanjut di ruang obrolan yang sama.
func (s *OrderService) Buat(ctx context.Context, seekerID int64, in OrderInput) (*model.Order, *model.Conversation, error) {
	svc, err := s.repos.Service.GetByID(ctx, in.ServiceID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil, NotFound("Jasa tidak ditemukan.")
		}
		return nil, nil, Internal(err)
	}
	if svc.Status != model.ServiceActive {
		return nil, nil, InvalidMsg("Jasa ini sedang tidak tersedia.")
	}

	provider, err := s.repos.Provider.GetByID(ctx, svc.ProviderID)
	if err != nil {
		return nil, nil, Internal(err)
	}
	if provider.UserID == seekerID {
		return nil, nil, InvalidMsg("Anda tidak dapat memesan jasa Anda sendiri.")
	}

	errs := validator.New()
	notes := strings.TrimSpace(in.Notes)
	errs.Length("notes", "Catatan", notes, 10, 1000)
	if notes == "" {
		errs.Add("notes", "Jelaskan singkat pekerjaan yang Anda butuhkan.")
	}

	var jadwal *time.Time
	if v := strings.TrimSpace(in.ScheduledDate); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			errs.Add("scheduled_date", "Format tanggal tidak valid.")
		} else if t.Before(time.Now().AddDate(0, 0, -1)) {
			errs.Add("scheduled_date", "Tanggal tidak boleh di masa lalu.")
		} else {
			jadwal = &t
		}
	}
	if errs.Any() {
		return nil, nil, Invalid(errs)
	}

	order := &model.Order{
		SeekerID:      seekerID,
		ProviderID:    svc.ProviderID,
		ServiceID:     svc.ID,
		Status:        model.OrderPending,
		ScheduledDate: jadwal,
	}
	if notes != "" {
		order.Notes = &notes
	}

	var conv *model.Conversation
	err = s.repos.WithTx(ctx, func(tx *repository.Repositories) error {
		if err := tx.Order.Create(ctx, order); err != nil {
			return err
		}
		c, err := tx.Chat.EnsureConversation(ctx, svc.ID, seekerID, svc.ProviderID)
		if err != nil {
			return err
		}
		conv = c
		if err := tx.Chat.AttachOrder(ctx, c.ID, order.ID); err != nil {
			return err
		}
		// Isi permintaan dikirim sebagai pesan pertama, supaya penyedia
		// melihat konteksnya langsung di ruang obrolan.
		pesan := &model.Message{
			ConversationID: c.ID,
			SenderID:       seekerID,
			Body:           susunPesanOrder(notes, jadwal),
		}
		return tx.Chat.AddMessage(ctx, pesan)
	})
	if err != nil {
		return nil, nil, Internal(err)
	}

	_ = s.repos.Notif.Create(ctx, provider.UserID, repository.NotifOrderBaru, map[string]any{
		"order_id":        order.ID,
		"conversation_id": conv.ID,
		"service_title":   svc.Title,
	})
	return order, conv, nil
}

func susunPesanOrder(notes string, jadwal *time.Time) string {
	var b strings.Builder
	b.WriteString("Permintaan order baru.\n\n")
	b.WriteString(notes)
	if jadwal != nil {
		b.WriteString("\n\nTanggal yang diinginkan: ")
		b.WriteString(jadwal.Format("2 January 2006"))
	}
	return b.String()
}

// transisiSah membatasi perubahan status pesanan agar mengikuti alur yang wajar.
var transisiSah = map[model.OrderStatus][]model.OrderStatus{
	model.OrderPending:  {model.OrderAccepted, model.OrderRejected, model.OrderCancelled},
	model.OrderAccepted: {model.OrderCompleted, model.OrderCancelled},
}

// UbahStatus mengubah status pesanan sesuai peran pemanggil.
// Penyedia yang menerima, menolak, dan menyelesaikan; pencari jasa hanya
// dapat membatalkan pesanannya sendiri.
func (s *OrderService) UbahStatus(ctx context.Context, orderID, userID int64, baru model.OrderStatus) error {
	order, err := s.repos.Order.GetByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return NotFound("Pesanan tidak ditemukan.")
		}
		return Internal(err)
	}

	provider, err := s.repos.Provider.GetByID(ctx, order.ProviderID)
	if err != nil {
		return Internal(err)
	}

	adalahPenyedia := provider.UserID == userID
	adalahPencari := order.SeekerID == userID
	if !adalahPenyedia && !adalahPencari {
		return Forbidden("Anda bukan pihak dalam pesanan ini.")
	}

	if !baru.Valid() {
		return InvalidMsg("Status pesanan tidak dikenal.")
	}
	if adalahPencari && baru != model.OrderCancelled {
		return Forbidden("Hanya penyedia jasa yang dapat mengubah status ini.")
	}

	diizinkan := false
	for _, s := range transisiSah[order.Status] {
		if s == baru {
			diizinkan = true
			break
		}
	}
	if !diizinkan {
		return InvalidMsg(fmt.Sprintf("Pesanan berstatus %q tidak dapat diubah menjadi %q.",
			order.Status.Label(), baru.Label()))
	}

	if err := s.repos.Order.SetStatus(ctx, orderID, baru); err != nil {
		return Internal(err)
	}

	penerima := order.SeekerID
	if adalahPencari {
		penerima = provider.UserID
	}
	_ = s.repos.Notif.Create(ctx, penerima, repository.NotifOrderDiperbarui, map[string]any{
		"order_id": orderID,
		"status":   string(baru),
	})
	return nil
}

// Ambil mengembalikan pesanan bila pengguna adalah salah satu pihaknya.
func (s *OrderService) Ambil(ctx context.Context, orderID, userID int64) (*model.Order, error) {
	order, err := s.repos.Order.GetByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, NotFound("Pesanan tidak ditemukan.")
		}
		return nil, Internal(err)
	}
	provider, err := s.repos.Provider.GetByID(ctx, order.ProviderID)
	if err != nil {
		return nil, Internal(err)
	}
	if order.SeekerID != userID && provider.UserID != userID {
		return nil, Forbidden("Anda bukan pihak dalam pesanan ini.")
	}
	return order, nil
}

// TindakanTersedia mengembalikan status berikutnya yang boleh dipilih
// pengguna, sehingga antarmuka hanya menampilkan tombol yang memang sah.
func (s *OrderService) TindakanTersedia(order *model.Order, sebagaiPenyedia bool) []model.OrderStatus {
	if order == nil {
		return nil
	}
	out := make([]model.OrderStatus, 0, 3)
	for _, kandidat := range transisiSah[order.Status] {
		if !sebagaiPenyedia && kandidat != model.OrderCancelled {
			continue
		}
		out = append(out, kandidat)
	}
	return out
}
