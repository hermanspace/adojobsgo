package service

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hermansyah/adojobsid/internal/config"
	"github.com/hermansyah/adojobsid/internal/repository"
)

// PushService mengirim notifikasi ke ponsel lewat FCM HTTP v1 — tanpa SDK,
// tanpa dependensi: JWT service account ditandatangani dengan crypto/rsa
// bawaan, ditukar token OAuth2, lalu satu POST per perangkat. Bila
// konfigurasinya kosong, seluruh metode diam: notifikasi tetap tersimpan dan
// tampil di aplikasi.
type PushService struct {
	repos *repository.Repositories
	cfg   config.Push
	http  *http.Client

	mu          sync.Mutex
	email       string
	kunci       *rsa.PrivateKey
	token       string
	kedaluwarsa time.Time
}

func NewPushService(repos *repository.Repositories, cfg config.Push) *PushService {
	s := &PushService{repos: repos, cfg: cfg, http: &http.Client{Timeout: 10 * time.Second}}
	if !cfg.Aktif() {
		return s
	}
	if err := s.muatServiceAccount(); err != nil {
		slog.Warn("push dimatikan: service account FCM tidak terbaca", "error", err)
		s.cfg = config.Push{}
	}
	return s
}

func (s *PushService) Aktif() bool { return s.cfg.Aktif() }

func (s *PushService) muatServiceAccount() error {
	raw, err := os.ReadFile(s.cfg.ServiceAccountFile)
	if err != nil {
		return err
	}
	var sa struct {
		ClientEmail string `json:"client_email"`
		PrivateKey  string `json:"private_key"`
	}
	if err := json.Unmarshal(raw, &sa); err != nil {
		return err
	}
	blok, _ := pem.Decode([]byte(sa.PrivateKey))
	if blok == nil {
		return errors.New("private_key bukan PEM")
	}
	kunci, err := x509.ParsePKCS8PrivateKey(blok.Bytes)
	if err != nil {
		return err
	}
	rsaKunci, ok := kunci.(*rsa.PrivateKey)
	if !ok {
		return errors.New("private_key bukan RSA")
	}
	s.email, s.kunci = sa.ClientEmail, rsaKunci
	return nil
}

// tokenAkses mengembalikan token OAuth2 yang masih berlaku, memperbaruinya
// bila perlu. Token FCM berumur satu jam; disegarkan lima menit lebih awal.
func (s *PushService) tokenAkses(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" && time.Until(s.kedaluwarsa) > 5*time.Minute {
		return s.token, nil
	}

	kini := time.Now()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	klaim, _ := json.Marshal(map[string]any{
		"iss":   s.email,
		"scope": "https://www.googleapis.com/auth/firebase.messaging",
		"aud":   "https://oauth2.googleapis.com/token",
		"iat":   kini.Unix(),
		"exp":   kini.Add(time.Hour).Unix(),
	})
	tanpaTanda := header + "." + base64.RawURLEncoding.EncodeToString(klaim)
	ringkas := sha256.Sum256([]byte(tanpaTanda))
	tanda, err := rsa.SignPKCS1v15(rand.Reader, s.kunci, crypto.SHA256, ringkas[:])
	if err != nil {
		return "", err
	}
	jwt := tanpaTanda + "." + base64.RawURLEncoding.EncodeToString(tanda)

	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {jwt}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := s.http.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil || out.AccessToken == "" {
		return "", fmt.Errorf("token oauth2 gagal (status %d)", res.StatusCode)
	}
	s.token = out.AccessToken
	s.kedaluwarsa = kini.Add(time.Duration(out.ExpiresIn) * time.Second)
	return s.token, nil
}

// Kirim mengantar satu notifikasi ke semua perangkat pengguna. data ikut
// sebagai payload supaya aplikasi bisa membuka layar yang tepat.
func (s *PushService) Kirim(ctx context.Context, userID int64, judul, isi string, data map[string]string) {
	if !s.Aktif() {
		return
	}
	tokens, err := s.repos.Device.ListByUser(ctx, userID)
	if err != nil || len(tokens) == 0 {
		return
	}
	akses, err := s.tokenAkses(ctx)
	if err != nil {
		slog.Warn("push: token akses FCM", "error", err)
		return
	}
	for _, t := range tokens {
		s.kirimSatu(ctx, akses, t, judul, isi, data)
	}
}

func (s *PushService) kirimSatu(ctx context.Context, akses, token, judul, isi string, data map[string]string) {
	pesan, _ := json.Marshal(map[string]any{"message": map[string]any{
		"token":        token,
		"notification": map[string]string{"title": judul, "body": isi},
		"data":         data,
		"android":      map[string]any{"priority": "high"},
	}})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://fcm.googleapis.com/v1/projects/"+s.cfg.ProjectID+"/messages:send", bytes.NewReader(pesan))
	req.Header.Set("Authorization", "Bearer "+akses)
	req.Header.Set("Content-Type", "application/json")
	res, err := s.http.Do(req)
	if err != nil {
		slog.Warn("push: kirim FCM", "error", err)
		return
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusOK {
		return
	}
	badan, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
	// Token yang sudah dicabut perangkat (aplikasi dihapus) dilaporkan
	// UNREGISTERED; buang supaya tidak terus dikirimi.
	if res.StatusCode == http.StatusNotFound || bytes.Contains(badan, []byte("UNREGISTERED")) {
		_ = s.repos.Device.DeleteToken(ctx, token)
		return
	}
	slog.Warn("push: FCM menolak", "status", res.StatusCode, "badan", string(badan))
}

// DaftarkanPerangkat menyimpan token FCM untuk pengguna.
func (s *PushService) DaftarkanPerangkat(ctx context.Context, userID int64, token, platform string) error {
	token = strings.TrimSpace(token)
	if len(token) < 20 || len(token) > 4096 {
		return InvalidMsg("Token perangkat tidak valid.")
	}
	if platform == "" {
		platform = "android"
	}
	if err := s.repos.Device.Upsert(ctx, userID, token, platform); err != nil {
		return Internal(err)
	}
	return nil
}

func (s *PushService) CabutPerangkat(ctx context.Context, userID int64, token string) error {
	if err := s.repos.Device.Delete(ctx, userID, strings.TrimSpace(token)); err != nil {
		return Internal(err)
	}
	return nil
}
