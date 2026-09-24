package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"

	"github.com/hermansyah/adojobsid/internal/middleware"
)

// StreamMessages mengalirkan pesan baru satu ruang sebagai Server-Sent Events.
// Dipilih SSE, bukan WebSocket: satu arah sudah cukup (pengiriman lewat
// POST), tanpa protokol tambahan, dan OkHttp/EventSource mendukungnya native.
//
// Koneksi ditutup server tiap 5 menit supaya klien yang sudah tidak ada
// tidak menahan goroutine; klien menyambung ulang dan mengejar ketinggalan
// lewat GET messages?since=.
func (h *Handler) StreamMessages(c *fiber.Ctx) error {
	convID, err := idParam(c, "Percakapan tidak ditemukan.")
	if err != nil {
		return fail(c, err)
	}
	current := middleware.CurrentUser(c)
	if _, err := h.svc.Chat.Akses(c.Context(), convID, current.ID); err != nil {
		return fail(c, err)
	}
	userID := current.ID
	ch, batal := h.svc.Chat.Langgan(convID)

	c.Set(fiber.HeaderContentType, "text/event-stream")
	c.Set(fiber.HeaderCacheControl, "no-cache")
	c.Set(fiber.HeaderConnection, "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		defer batal()
		denyut := time.NewTicker(25 * time.Second)
		defer denyut.Stop()
		batasUmur := time.NewTimer(5 * time.Minute)
		defer batasUmur.Stop()

		// retry: memberi tahu klien jeda sambung-ulang bila putus.
		fmt.Fprint(w, "retry: 3000\n\n")
		if w.Flush() != nil {
			return
		}
		for {
			select {
			case m := <-ch:
				isi, err := json.Marshal(m)
				if err != nil {
					continue
				}
				fmt.Fprintf(w, "id: %d\nevent: pesan\ndata: %s\n\n", m.ID, isi)
				if w.Flush() != nil {
					return
				}
				if m.SenderID != userID {
					h.svc.Chat.TandaiTerbaca(c.Context(), convID, userID)
				}
			case <-denyut.C:
				fmt.Fprint(w, ": denyut\n\n")
				if w.Flush() != nil {
					return
				}
			case <-batasUmur.C:
				return
			}
		}
	}))
	return nil
}
