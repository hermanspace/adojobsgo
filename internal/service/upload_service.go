package service

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/hermansyah/adojobsid/internal/config"
)

// UploadService menyimpan gambar ke volume lokal yang di-mount ke container.
// URL publik selalu diawali /uploads/ dan dilayani oleh handler statis aplikasi.
type UploadService struct {
	cfg config.Upload
}

func NewUploadService(cfg config.Upload) *UploadService {
	return &UploadService{cfg: cfg}
}

// allowedImageTypes dibatasi ke format yang benar-benar dipakai listing jasa.
var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

const publicPrefix = "/uploads/"

// GambarTersimpan adalah hasil unggahan yang sudah diproses.
type GambarTersimpan struct {
	URL      string
	ThumbURL string
	Lebar    int
	Tinggi   int
	Bytes    int
}

// SaveImage memvalidasi, memperkecil, dan menyandikan ulang satu gambar
// sebelum menyimpannya.
//
// Berkas tidak pernah disimpan apa adanya: seluruh gambar disandikan ulang
// dari pikselnya. Itu memperkecil ukuran, menyeragamkan format, dan sekaligus
// membuang seluruh metadata bawaan — termasuk koordinat GPS yang lazim
// disematkan kamera ponsel.
func (s *UploadService) SaveImage(fh *multipart.FileHeader, profil ProfilGambar) (*GambarTersimpan, error) {
	if fh == nil {
		return nil, InvalidMsg("Berkas foto tidak terbaca.")
	}
	maxBytes := s.cfg.MaxSizeMB * 1024 * 1024
	if fh.Size > maxBytes {
		return nil, InvalidMsg(fmt.Sprintf("Ukuran foto maksimal %d MB.", s.cfg.MaxSizeMB))
	}

	src, err := fh.Open()
	if err != nil {
		return nil, Internal(err)
	}
	defer src.Close()

	// Tipe berkas ditentukan dari isinya, bukan dari ekstensi atau header
	// kiriman klien.
	head := make([]byte, 512)
	n, _ := io.ReadFull(src, head)
	if !allowedImageTypes[http.DetectContentType(head[:n])] {
		return nil, InvalidMsg("Format foto harus JPG, PNG, atau WEBP.")
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return nil, Internal(err)
	}
	return s.simpanDari(src, profil)
}

// SaveImageBytes menyimpan gambar yang sudah ada di memori lewat pipeline
// yang sama dengan unggahan pengguna (perkecil, sandikan ulang, thumbnail).
// Dipakai seeder demo; tidak pernah dipanggil dari handler.
func (s *UploadService) SaveImageBytes(data []byte, profil ProfilGambar) (*GambarTersimpan, error) {
	if !allowedImageTypes[http.DetectContentType(data)] {
		return nil, InvalidMsg("Format gambar harus JPG, PNG, atau WEBP.")
	}
	return s.simpanDari(bytes.NewReader(data), profil)
}

func (s *UploadService) simpanDari(src io.ReadSeeker, profil ProfilGambar) (*GambarTersimpan, error) {
	hasil, err := prosesGambar(src, profil)
	if err != nil {
		return nil, err
	}

	dir, err := s.siapkanFolder(profil.Subdir)
	if err != nil {
		return nil, err
	}
	nama, err := randomName()
	if err != nil {
		return nil, Internal(err)
	}

	urlUtama, err := s.tulis(dir, nama+hasil.Ekstensi, hasil.Utama)
	if err != nil {
		return nil, err
	}

	out := &GambarTersimpan{
		URL:    urlUtama,
		Lebar:  hasil.Lebar,
		Tinggi: hasil.Tinggi,
		Bytes:  len(hasil.Utama),
	}

	if len(hasil.Thumb) > 0 {
		urlThumb, err := s.tulis(dir, nama+"-thumb"+hasil.Ekstensi, hasil.Thumb)
		if err != nil {
			// Berkas utama yang sudah tersimpan ikut dibuang supaya tidak
			// meninggalkan berkas yatim di volume.
			s.Delete(urlUtama)
			return nil, err
		}
		out.ThumbURL = urlThumb
	}
	return out, nil
}

// siapkanFolder membuat folder tujuan berdasarkan subdir dan bulan berjalan.
func (s *UploadService) siapkanFolder(subdir string) (string, error) {
	dir := filepath.Join(s.cfg.Dir, sanitizeSubdir(subdir), time.Now().Format("2006/01"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", Internal(fmt.Errorf("buat folder upload: %w", err))
	}
	return dir, nil
}

// tulis menyimpan satu berkas dan mengembalikan URL publiknya.
func (s *UploadService) tulis(dir, nama string, data []byte) (string, error) {
	jalur := filepath.Join(dir, nama)
	if err := os.WriteFile(jalur, data, 0o644); err != nil {
		return "", Internal(fmt.Errorf("tulis berkas upload: %w", err))
	}

	rel, err := filepath.Rel(s.cfg.Dir, jalur)
	if err != nil {
		_ = os.Remove(jalur)
		return "", Internal(err)
	}
	return publicPrefix + filepath.ToSlash(rel), nil
}

// Delete menghapus berkas berdasarkan URL publiknya. URL di luar folder upload diabaikan.
func (s *UploadService) Delete(publicURL string) {
	rel, ok := strings.CutPrefix(publicURL, publicPrefix)
	if !ok || rel == "" {
		return
	}
	full := filepath.Join(s.cfg.Dir, filepath.FromSlash(path.Clean("/"+rel)))
	if !strings.HasPrefix(full, filepath.Clean(s.cfg.Dir)+string(os.PathSeparator)) {
		return // lindungi dari path traversal
	}
	_ = os.Remove(full)
}

// Dir mengembalikan folder fisik penyimpanan upload.
func (s *UploadService) Dir() string { return s.cfg.Dir }

// MaxPerListing membatasi jumlah foto per listing jasa.
func (s *UploadService) MaxPerListing() int { return s.cfg.MaxPerListing }

func sanitizeSubdir(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "..", "")
	s = strings.Trim(s, "/\\")
	if s == "" {
		return "misc"
	}
	return s
}

func randomName() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
