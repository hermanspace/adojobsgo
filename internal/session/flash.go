package session

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// FlashStore menyimpan pesan sekali-tampil di Redis. Isi pesan tidak pernah
// diletakkan di cookie — cookie hanya membawa kunci acak — sehingga pesan
// tidak bisa dipalsukan dari sisi klien.
type FlashStore struct {
	rdb *redis.Client
}

func NewFlashStore(rdb *redis.Client) *FlashStore { return &FlashStore{rdb: rdb} }

type FlashPayload struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

const flashPrefix = "flash:"
const flashTTL = 2 * time.Minute

// Put menyimpan pesan dan mengembalikan kunci untuk disimpan di cookie.
func (s *FlashStore) Put(ctx context.Context, kind, message string) (string, error) {
	id, err := newID()
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(FlashPayload{Kind: kind, Message: message})
	if err != nil {
		return "", err
	}
	if err := s.rdb.Set(ctx, flashPrefix+id, payload, flashTTL).Err(); err != nil {
		return "", err
	}
	return id, nil
}

// Take mengambil pesan sekaligus menghapusnya, sehingga hanya tampil sekali.
func (s *FlashStore) Take(ctx context.Context, id string) *FlashPayload {
	if id == "" {
		return nil
	}
	raw, err := s.rdb.GetDel(ctx, flashPrefix+id).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			return nil
		}
		return nil
	}
	var p FlashPayload
	if json.Unmarshal(raw, &p) != nil {
		return nil
	}
	return &p
}
