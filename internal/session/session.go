// Package session mengelola sesi login berbasis cookie dengan penyimpanan di Redis.
// Tidak memakai JWT: cookie hanya membawa ID sesi acak, seluruh data ada di Redis
// sehingga sesi bisa dicabut kapan saja dari sisi server.
package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrNotFound = errors.New("sesi tidak ditemukan")

const (
	keyPrefix = "sess:"
	// userIndexPrefix menyimpan daftar ID sesi milik satu pengguna, sehingga
	// seluruh sesinya bisa dicabut sekaligus saat akun ditangguhkan.
	userIndexPrefix = "usess:"
)

// Data adalah isi sesi yang disimpan di Redis.
type Data struct {
	UserID     int64     `json:"user_id"`
	FullName   string    `json:"full_name"`
	IsProvider bool      `json:"is_provider"`
	IsAdmin    bool      `json:"is_admin,omitempty"`
	ProviderID int64     `json:"provider_id,omitempty"`
	AvatarURL  string    `json:"avatar_url,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type Store struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewStore(rdb *redis.Client, ttl time.Duration) *Store {
	return &Store{rdb: rdb, ttl: ttl}
}

func (s *Store) TTL() time.Duration { return s.ttl }

// Create membuat sesi baru dan mengembalikan ID sesi untuk disimpan di cookie.
func (s *Store) Create(ctx context.Context, data Data) (string, error) {
	id, err := newID()
	if err != nil {
		return "", err
	}
	data.CreatedAt = time.Now()
	payload, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	pipe := s.rdb.TxPipeline()
	pipe.Set(ctx, keyPrefix+id, payload, s.ttl)
	// Indeks per pengguna diberi masa berlaku sedikit lebih panjang dari sesi,
	// supaya entri terakhir tidak hilang lebih dulu dari sesinya sendiri.
	pipe.SAdd(ctx, userIndexKey(data.UserID), id)
	pipe.Expire(ctx, userIndexKey(data.UserID), s.ttl+time.Hour)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", fmt.Errorf("simpan sesi: %w", err)
	}
	return id, nil
}

// Get mengambil sesi dan memperpanjang masa berlakunya (sliding expiration).
func (s *Store) Get(ctx context.Context, id string) (*Data, error) {
	if id == "" {
		return nil, ErrNotFound
	}
	raw, err := s.rdb.Get(ctx, keyPrefix+id).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var data Data
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	s.rdb.Expire(ctx, keyPrefix+id, s.ttl)
	return &data, nil
}

// Save menimpa isi sesi yang sudah ada, misalnya setelah user menjadi provider.
func (s *Store) Save(ctx context.Context, id string, data Data) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, keyPrefix+id, payload, s.ttl).Err()
}

func (s *Store) Destroy(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	// Isi sesi dibaca lebih dulu agar entri indeks penggunanya ikut dibersihkan.
	if data, err := s.peek(ctx, id); err == nil {
		s.rdb.SRem(ctx, userIndexKey(data.UserID), id)
	}
	return s.rdb.Del(ctx, keyPrefix+id).Err()
}

// DestroyAllForUser mencabut seluruh sesi milik satu pengguna.
// Dipanggil saat akun ditangguhkan, sehingga sesi yang sedang berjalan
// langsung berhenti berlaku, bukan menunggu cookie-nya kedaluwarsa.
func (s *Store) DestroyAllForUser(ctx context.Context, userID int64) error {
	key := userIndexKey(userID)
	ids, err := s.rdb.SMembers(ctx, key).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	if len(ids) == 0 {
		return s.rdb.Del(ctx, key).Err()
	}

	keys := make([]string, 0, len(ids)+1)
	for _, id := range ids {
		keys = append(keys, keyPrefix+id)
	}
	keys = append(keys, key)
	return s.rdb.Del(ctx, keys...).Err()
}

// peek membaca sesi tanpa memperpanjang masa berlakunya.
func (s *Store) peek(ctx context.Context, id string) (*Data, error) {
	raw, err := s.rdb.Get(ctx, keyPrefix+id).Bytes()
	if err != nil {
		return nil, err
	}
	var data Data
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func userIndexKey(userID int64) string {
	return userIndexPrefix + strconv.FormatInt(userID, 10)
}

func newID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("buat id sesi: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
