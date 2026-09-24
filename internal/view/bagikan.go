package view

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/hermansyah/adojobsid/internal/model"
)

// Bagikan adalah data untuk membagikan satu halaman: dipakai tombol
// "Bagikan" di web (Web Share API atau sheet), meta Open Graph di <head>,
// dan dikirim apa adanya lewat API supaya aplikasi Android memakai teks dan
// tautan yang persis sama.
type Bagikan struct {
	URL       string          `json:"url"`
	Judul     string          `json:"judul"`
	Teks      string          `json:"teks"`
	GambarURL string          `json:"gambar_url,omitempty"`
	Target    []TargetBagikan `json:"target"`
}

// TargetBagikan adalah satu tujuan berbagi. Kunci "salin" tidak punya URL:
// klien menyalin URL utama ke papan klip.
type TargetBagikan struct {
	Kunci string `json:"kunci"`
	Label string `json:"label"`
	URL   string `json:"url,omitempty"`
}

// PesanBagikan menyatukan teks dan tautan seperti yang dikirim ke aplikasi
// pesan: teks, baris kosong, tautan.
func (b Bagikan) PesanBagikan() string { return b.Teks + "\n\n" + b.URL }

// BagikanJasa menyusun data berbagi untuk satu jasa. Teksnya menyebut jasa,
// penyedia, dan kecamatan — cukup untuk penerima memutuskan membuka tautan
// tanpa harus mengenal Adojobs lebih dulu.
func BagikanJasa(baseURL string, s *model.ServiceDetail) Bagikan {
	b := Bagikan{
		URL:   absolut(baseURL, "/jasa/"+strconv.FormatInt(s.ID, 10)),
		Judul: s.Title + " · " + s.Provider.FullName,
		Teks:  "Lihat jasa \"" + s.Title + "\" oleh " + s.Provider.FullName + lokasiBagikan(s.Provider.Kecamatan) + " di Adojobs. Ada harga, jarak, dan ulasannya.",
	}
	for _, im := range s.Images {
		if im.ImageURL != "" {
			b.GambarURL = absolut(baseURL, im.ImageURL)
			break
		}
	}
	b.Target = susunTarget(b)
	return b
}

// BagikanPenyedia menyusun data berbagi untuk halaman publik penyedia.
func BagikanPenyedia(baseURL string, p *model.ProviderDetail) Bagikan {
	b := Bagikan{
		URL:   absolut(baseURL, URLPenyedia(p.Slug)),
		Judul: p.FullName + " · penyedia jasa di Adojobs",
		Teks:  "Lihat " + p.FullName + lokasiBagikan(p.Kecamatan) + " di Adojobs: daftar jasa, portofolio, dan ulasan pelanggannya.",
	}
	if p.AvatarURL != nil && *p.AvatarURL != "" {
		b.GambarURL = absolut(baseURL, *p.AvatarURL)
	}
	b.Target = susunTarget(b)
	return b
}

func lokasiBagikan(kecamatan *string) string {
	if kecamatan == nil || *kecamatan == "" {
		return ""
	}
	return " (" + *kecamatan + ")"
}

// susunTarget membangun tautan berbagi per platform. Semua memakai skema
// web resmi masing-masing, sehingga di ponsel otomatis dibuka aplikasinya.
func susunTarget(b Bagikan) []TargetBagikan {
	pesan := url.QueryEscape(b.PesanBagikan())
	u := url.QueryEscape(b.URL)
	return []TargetBagikan{
		{Kunci: "whatsapp", Label: "WhatsApp", URL: "https://wa.me/?text=" + pesan},
		{Kunci: "facebook", Label: "Facebook", URL: "https://www.facebook.com/sharer/sharer.php?u=" + u},
		{Kunci: "telegram", Label: "Telegram", URL: "https://t.me/share/url?url=" + u + "&text=" + url.QueryEscape(b.Teks)},
		{Kunci: "x", Label: "X", URL: "https://x.com/intent/post?text=" + url.QueryEscape(b.Teks) + "&url=" + u},
		{Kunci: "salin", Label: "Salin tautan"},
	}
}

// absolut menyambung path relatif dengan base URL; path yang sudah absolut
// dibiarkan.
func absolut(baseURL, path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return strings.TrimRight(baseURL, "/") + path
}
