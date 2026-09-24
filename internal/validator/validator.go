// Package validator menyediakan validasi input yang dipakai bersama oleh
// handler API dan handler web, sehingga aturannya tidak pernah berbeda.
package validator

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Errors memetakan nama field ke pesan kesalahan berbahasa Indonesia.
type Errors map[string]string

func (e Errors) Add(field, msg string) {
	if _, exists := e[field]; !exists {
		e[field] = msg
	}
}

func (e Errors) Has(field string) bool   { _, ok := e[field]; return ok }
func (e Errors) Get(field string) string { return e[field] }
func (e Errors) Any() bool               { return len(e) > 0 }

// Error membuat Errors memenuhi interface error agar bisa dialirkan
// lewat service layer tanpa tipe pembungkus tambahan.
func (e Errors) Error() string {
	parts := make([]string, 0, len(e))
	for k, v := range e {
		parts = append(parts, fmt.Sprintf("%s: %s", k, v))
	}
	return strings.Join(parts, "; ")
}

func New() Errors { return Errors{} }

var (
	phoneDigits = regexp.MustCompile(`\D`)
	emailRe     = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[a-zA-Z]{2,}$`)
	slugUnsafe  = regexp.MustCompile(`[^a-z0-9]+`)
)

// NormalizePhone menyeragamkan nomor HP Indonesia ke format 62xxxxxxxxxx.
// Menerima masukan 08xx, +62 8xx, 62-8xx, atau 8xx.
func NormalizePhone(raw string) string {
	digits := phoneDigits.ReplaceAllString(raw, "")
	switch {
	case digits == "":
		return ""
	case strings.HasPrefix(digits, "62"):
		return digits
	case strings.HasPrefix(digits, "0"):
		return "62" + strings.TrimLeft(digits, "0")
	default:
		return "62" + digits
	}
}

// ValidPhone memastikan nomor sudah ternormalisasi dan panjangnya masuk akal
// untuk nomor seluler Indonesia (62 + 9..13 digit).
func ValidPhone(normalized string) bool {
	if !strings.HasPrefix(normalized, "628") {
		return false
	}
	n := len(normalized)
	return n >= 11 && n <= 15
}

func ValidEmail(s string) bool { return emailRe.MatchString(strings.TrimSpace(s)) }

// Required menambahkan error bila nilai kosong setelah di-trim.
func (e Errors) Required(field, label, value string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		e.Add(field, label+" wajib diisi.")
	}
	return v
}

// Length memeriksa panjang karakter (bukan byte) agar aman untuk teks non-ASCII.
func (e Errors) Length(field, label, value string, min, max int) {
	n := utf8.RuneCountInString(strings.TrimSpace(value))
	if n == 0 {
		return // biar Required yang menangani
	}
	if n < min {
		e.Add(field, fmt.Sprintf("%s minimal %d karakter.", label, min))
	}
	if n > max {
		e.Add(field, fmt.Sprintf("%s maksimal %d karakter.", label, max))
	}
}

// Slugify mengubah teks menjadi slug URL, dipakai untuk kategori di seeder.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugUnsafe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
