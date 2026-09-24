package database

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache adalah pembungkus tipis di atas Redis untuk menyimpan hasil query
// yang sering diakses (hasil pencarian, daftar kategori).
type Cache struct {
	rdb        *redis.Client
	defaultTTL time.Duration
	prefix     string
}

func NewCache(rdb *redis.Client, defaultTTL time.Duration) *Cache {
	return &Cache{rdb: rdb, defaultTTL: defaultTTL, prefix: "cache:"}
}

// Remember mengembalikan nilai dari cache; jika belum ada, load dijalankan dan
// hasilnya disimpan. Kegagalan Redis tidak pernah menggagalkan request —
// aplikasi hanya kehilangan manfaat cache.
func Remember[T any](ctx context.Context, c *Cache, key string, ttl time.Duration, load func() (T, error)) (T, error) {
	var zero T
	if c == nil {
		return load()
	}
	if ttl <= 0 {
		ttl = c.defaultTTL
	}
	full := c.prefix + key

	if raw, err := c.rdb.Get(ctx, full).Bytes(); err == nil {
		var out T
		if json.Unmarshal(raw, &out) == nil {
			return out, nil
		}
	}

	value, err := load()
	if err != nil {
		return zero, err
	}
	if payload, err := json.Marshal(value); err == nil {
		c.rdb.Set(ctx, full, payload, ttl)
	}
	return value, nil
}

// Forget menghapus seluruh entri cache dengan awalan tertentu.
// Dipanggil saat listing dibuat/diubah agar hasil pencarian tidak basi.
func (c *Cache) Forget(ctx context.Context, pattern string) {
	if c == nil {
		return
	}
	iter := c.rdb.Scan(ctx, 0, c.prefix+pattern, 256).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
		if len(keys) >= 256 {
			c.rdb.Del(ctx, keys...)
			keys = keys[:0]
		}
	}
	if len(keys) > 0 {
		c.rdb.Del(ctx, keys...)
	}
}

// Tambah menaikkan penghitung. Dipakai untuk tayangan iklan: satu INCR per
// render, lalu disalin ke database secara berkala — bukan satu UPDATE per
// tampilan halaman.
func (c *Cache) Tambah(ctx context.Context, key string) {
	if c == nil {
		return
	}
	_ = c.rdb.Incr(ctx, c.prefix+key).Err()
}

// AmbilDanHapus mengambil seluruh penghitung yang cocok pola lalu
// menghapusnya secara atomik (GETDEL), sehingga tayangan yang masuk di sela
// penyalinan tidak hilang maupun terhitung dua kali. Kunci yang dikembalikan
// tanpa awalan cache.
func (c *Cache) AmbilDanHapus(ctx context.Context, pattern string) map[string]int64 {
	out := map[string]int64{}
	if c == nil {
		return out
	}
	iter := c.rdb.Scan(ctx, 0, c.prefix+pattern, 200).Iterator()
	for iter.Next(ctx) {
		full := iter.Val()
		n, err := c.rdb.GetDel(ctx, full).Int64()
		if err != nil || n == 0 {
			continue
		}
		out[strings.TrimPrefix(full, c.prefix)] = n
	}
	return out
}
