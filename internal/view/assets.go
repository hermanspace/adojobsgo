package view

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Assets memetakan URL aset statis ke versi berbasis isi berkasnya.
// Versi ini ditempelkan sebagai parameter ?v= sehingga berkas boleh di-cache
// lama di peramban, namun pengguna tetap menerima versi baru begitu isinya
// berubah — tanpa perlu mengganti nama berkas.
type Assets struct {
	versions map[string]string
	fallback string
}

// NewAssets memindai folder statis dan menghitung hash isi setiap berkas
// CSS dan JS. Dipanggil sekali saat aplikasi start.
func NewAssets(root, fallback string) *Assets {
	a := &Assets{versions: map[string]string{}, fallback: fallback}

	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // berkas yang tak terbaca cukup dilewati
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".css", ".js":
		default:
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		sum := sha256.Sum256(data)
		a.versions["/static/"+filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])[:12]
		return nil
	})
	return a
}

// URL mengembalikan path aset lengkap dengan parameter versinya.
func (a *Assets) URL(path string) string {
	if a == nil {
		return path
	}
	if v, ok := a.versions[path]; ok {
		return path + "?v=" + v
	}
	// Berkas yang tidak terdaftar tetap diberi versi build, supaya tidak ada
	// aset yang bisa tertinggal di cache peramban selamanya.
	return path + "?v=" + a.fallback
}
