// Package model berisi struct domain yang dipakai bersama oleh repository,
// service, handler API (JSON) dan handler web (templ).
package model

import (
	"encoding/json"
	"time"
)

// ---------- enum ----------

type PriceType string

const (
	PriceFixed      PriceType = "fixed"
	PriceHourly     PriceType = "hourly"
	PriceNegotiable PriceType = "negotiable"
)

func (p PriceType) Valid() bool {
	switch p {
	case PriceFixed, PriceHourly, PriceNegotiable:
		return true
	}
	return false
}

// Label mengembalikan teks bahasa Indonesia untuk ditampilkan di UI.
func (p PriceType) Label() string {
	switch p {
	case PriceFixed:
		return "Harga tetap"
	case PriceHourly:
		return "Per jam"
	case PriceNegotiable:
		return "Nego"
	}
	return string(p)
}

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

func (r Role) Valid() bool { return r == RoleUser || r == RoleAdmin }

func (r Role) Label() string {
	if r == RoleAdmin {
		return "Admin"
	}
	return "Pengguna"
}

type ServiceStatus string

const (
	// ServicePending: menunggu persetujuan admin, belum tayang.
	ServicePending  ServiceStatus = "pending"
	ServiceActive   ServiceStatus = "active"
	ServiceInactive ServiceStatus = "inactive"
	ServiceRejected ServiceStatus = "rejected"
)

func (s ServiceStatus) Valid() bool {
	switch s {
	case ServicePending, ServiceActive, ServiceInactive, ServiceRejected:
		return true
	}
	return false
}

// DapatDipilihProvider membatasi status yang boleh disetel provider sendiri.
// Menyetujui dan menolak adalah wewenang admin.
func (s ServiceStatus) DapatDipilihProvider() bool {
	return s == ServiceActive || s == ServiceInactive
}

// Tayang menandai listing yang benar-benar terlihat publik.
func (s ServiceStatus) Tayang() bool { return s == ServiceActive }

func (s ServiceStatus) Label() string {
	switch s {
	case ServicePending:
		return "Menunggu peninjauan"
	case ServiceActive:
		return "Tayang"
	case ServiceInactive:
		return "Disembunyikan"
	case ServiceRejected:
		return "Ditolak"
	}
	return string(s)
}

type OrderStatus string

const (
	OrderPending   OrderStatus = "pending"
	OrderAccepted  OrderStatus = "accepted"
	OrderRejected  OrderStatus = "rejected"
	OrderCompleted OrderStatus = "completed"
	OrderCancelled OrderStatus = "cancelled"
)

func (o OrderStatus) Valid() bool {
	switch o {
	case OrderPending, OrderAccepted, OrderRejected, OrderCompleted, OrderCancelled:
		return true
	}
	return false
}

func (o OrderStatus) Label() string {
	switch o {
	case OrderPending:
		return "Menunggu"
	case OrderAccepted:
		return "Diterima"
	case OrderRejected:
		return "Ditolak"
	case OrderCompleted:
		return "Selesai"
	case OrderCancelled:
		return "Dibatalkan"
	}
	return string(o)
}

// ---------- entitas ----------

type User struct {
	ID           int64   `json:"id"`
	FullName     string  `json:"full_name"`
	Phone        string  `json:"phone"`
	Email        *string `json:"email,omitempty"`
	PasswordHash string  `json:"-"`
	AvatarURL    *string `json:"avatar_url,omitempty"`
	City         *string `json:"city,omitempty"`
	Kecamatan    *string `json:"kecamatan,omitempty"`
	IsProvider   bool    `json:"is_provider"`
	Role         Role    `json:"role"`
	// SuspendedAt bernilai nil untuk akun aktif. Penangguhan sengaja dibuat
	// reversible: mengaktifkan kembali cukup mengosongkan kolom ini.
	SuspendedAt     *time.Time `json:"suspended_at,omitempty"`
	SuspendedReason *string    `json:"suspended_reason,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// IsAdmin menandai akun yang boleh membuka panel admin.
func (u *User) IsAdmin() bool { return u.Role == RoleAdmin }

// IsSuspended menandai akun yang ditangguhkan: tidak bisa masuk dan seluruh
// listing miliknya disembunyikan dari halaman publik.
func (u *User) IsSuspended() bool { return u.SuspendedAt != nil }

type ProviderProfile struct {
	ID     int64 `json:"id"`
	UserID int64 `json:"user_id"`
	// Slug adalah alamat publik penyedia. Ditetapkan sekali saat profil dibuat
	// dan tidak ikut berubah saat nama diganti, supaya tautan yang sudah
	// beredar tidak putus.
	Slug           string     `json:"slug"`
	Bio            *string    `json:"bio,omitempty"`
	WhatsappNumber string     `json:"whatsapp_number,omitempty"`
	IsVerified     bool       `json:"is_verified"`
	AvgRating      float64    `json:"avg_rating"`
	TotalReviews   int        `json:"total_reviews"`
	FeaturedUntil  *time.Time `json:"featured_until,omitempty"`
	// Titik lokasi penyedia. Keduanya terisi bersama atau sama-sama kosong.
	Latitude        *float64  `json:"latitude,omitempty"`
	Longitude       *float64  `json:"longitude,omitempty"`
	AddressLabel    *string   `json:"address_label,omitempty"`
	ServiceRadiusKm int       `json:"service_radius_km"`
	CreatedAt       time.Time `json:"created_at"`
}

// PunyaLokasi menandai penyedia yang sudah menentukan titik lokasinya.
func (p ProviderProfile) PunyaLokasi() bool { return p.Latitude != nil && p.Longitude != nil }

// IsFeatured menandai penyedia pilihan yang masa tayangnya masih berlaku.
func (p ProviderProfile) IsFeatured() bool {
	return p.FeaturedUntil != nil && p.FeaturedUntil.After(time.Now())
}

// ProviderDetail menggabungkan profil provider dengan data user pemiliknya,
// dipakai di halaman detail provider dan kartu listing.
type ProviderDetail struct {
	ProviderProfile
	FullName  string  `json:"full_name"`
	AvatarURL *string `json:"avatar_url,omitempty"`
	City      *string `json:"city,omitempty"`
	Kecamatan *string `json:"kecamatan,omitempty"`
	Phone     string  `json:"-"`
	// SuspendedAt dipakai halaman publik untuk menyembunyikan penyedia yang
	// akunnya ditangguhkan; tidak pernah ikut dikirim ke klien.
	SuspendedAt *time.Time `json:"-"`
}

// IsSuspended menandai penyedia yang akun penggunanya sedang ditangguhkan.
func (p ProviderDetail) IsSuspended() bool { return p.SuspendedAt != nil }

type Category struct {
	ID       int64      `json:"id"`
	Name     string     `json:"name"`
	Slug     string     `json:"slug"`
	ParentID *int64     `json:"parent_id,omitempty"`
	Icon     *string    `json:"icon,omitempty"`
	Children []Category `json:"children,omitempty"`
}

type Service struct {
	ID              int64         `json:"id"`
	ProviderID      int64         `json:"provider_id"`
	CategoryID      int64         `json:"category_id"`
	Title           string        `json:"title"`
	Description     string        `json:"description"`
	PriceType       PriceType     `json:"price_type"`
	PriceMin        *float64      `json:"price_min,omitempty"`
	PriceMax        *float64      `json:"price_max,omitempty"`
	Status          ServiceStatus `json:"status"`
	FeaturedUntil   *time.Time    `json:"featured_until,omitempty"`
	ApprovedAt      *time.Time    `json:"approved_at,omitempty"`
	ApprovedBy      *int64        `json:"approved_by,omitempty"`
	RejectionReason *string       `json:"rejection_reason,omitempty"`
	SubmittedAt     time.Time     `json:"submitted_at"`
	CreatedAt       time.Time     `json:"created_at"`
}

// IsFeatured menandai listing yang sedang disorot dan masa tayangnya berlaku.
func (s Service) IsFeatured() bool {
	return s.FeaturedUntil != nil && s.FeaturedUntil.After(time.Now())
}

type ServiceImage struct {
	ID        int64  `json:"id"`
	ServiceID int64  `json:"service_id"`
	ImageURL  string `json:"image_url"`
	// ThumbURL kosong untuk gambar yang diunggah sebelum pemrosesan otomatis
	// diterapkan; tampilan jatuh kembali ke ImageURL bila begitu.
	ThumbURL  *string `json:"thumb_url,omitempty"`
	Width     *int    `json:"width,omitempty"`
	Height    *int    `json:"height,omitempty"`
	Bytes     *int    `json:"bytes,omitempty"`
	SortOrder int     `json:"sort_order"`
}

// Tampil mengembalikan URL thumbnail bila ada, selain itu gambar utamanya.
func (i ServiceImage) Tampil() string {
	if i.ThumbURL != nil && *i.ThumbURL != "" {
		return *i.ThumbURL
	}
	return i.ImageURL
}

// ServiceCard adalah bentuk denormalisasi untuk kartu listing di halaman pencarian.
// Semua data yang dibutuhkan kartu diambil dalam satu query, tanpa N+1.
type ServiceCard struct {
	Service
	CoverImage   *string `json:"cover_image,omitempty"`
	CategoryName string  `json:"category_name"`
	CategorySlug string  `json:"category_slug"`
	ProviderName string  `json:"provider_name"`
	ProviderCity *string `json:"provider_city,omitempty"`
	ProviderKec  *string `json:"provider_kecamatan,omitempty"`
	AvgRating    float64 `json:"avg_rating"`
	TotalReviews int     `json:"total_reviews"`
	IsVerified   bool    `json:"is_verified"`
	// ProviderFeaturedUntil membedakan sorotan pada penyedianya dari sorotan
	// pada listing ini sendiri. Keduanya disetel admin secara terpisah dan
	// ditandai dengan warna kartu yang berbeda.
	ProviderFeaturedUntil *time.Time `json:"provider_featured_until,omitempty"`

	ProviderLat      *float64 `json:"provider_latitude,omitempty"`
	ProviderLng      *float64 `json:"provider_longitude,omitempty"`
	ProviderRadiusKm int      `json:"provider_radius_km"`
	// JarakKm hanya terisi bila pencari jasa membawa titik lokasinya.
	JarakKm *float64 `json:"jarak_km,omitempty"`
}

// PenyediaDisorot menandai listing yang penyedianya sedang jadi penyedia
// pilihan, terlepas dari apakah listing ini sendiri sedang disorot.
func (c ServiceCard) PenyediaDisorot() bool {
	return c.ProviderFeaturedUntil != nil && c.ProviderFeaturedUntil.After(time.Now())
}

// Menjangkau menandai penyedia yang radius layanannya mencakup titik
// pencari jasa — artinya penyedia bersedia datang ke lokasi tersebut.
func (c ServiceCard) Menjangkau() bool {
	return c.JarakKm != nil && *c.JarakKm <= float64(c.ProviderRadiusKm)
}

// ServiceDetail dipakai di halaman detail jasa: listing + seluruh foto + provider.
type ServiceDetail struct {
	Service
	Images   []ServiceImage `json:"images"`
	Category Category       `json:"category"`
	Provider ProviderDetail `json:"provider"`
}

type Portfolio struct {
	ID          int64      `json:"id"`
	ProviderID  int64      `json:"provider_id"`
	Title       string     `json:"title"`
	ImageURL    string     `json:"image_url"`
	ThumbURL    *string    `json:"thumb_url,omitempty"`
	Width       *int       `json:"width,omitempty"`
	Height      *int       `json:"height,omitempty"`
	Bytes       *int       `json:"bytes,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Tampil mengembalikan URL thumbnail bila ada, selain itu gambar utamanya.
func (p Portfolio) Tampil() string {
	if p.ThumbURL != nil && *p.ThumbURL != "" {
		return *p.ThumbURL
	}
	return p.ImageURL
}

type Order struct {
	ID            int64       `json:"id"`
	SeekerID      int64       `json:"seeker_id"`
	ProviderID    int64       `json:"provider_id"`
	ServiceID     int64       `json:"service_id"`
	Status        OrderStatus `json:"status"`
	ScheduledDate *time.Time  `json:"scheduled_date,omitempty"`
	Notes         *string     `json:"notes,omitempty"`
	AgreedPrice   *float64    `json:"agreed_price,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
}

type Review struct {
	ID        int64     `json:"id"`
	OrderID   int64     `json:"order_id"`
	Rating    int       `json:"rating"`
	Comment   *string   `json:"comment,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ReviewRow adalah ulasan beserta konteks yang dibutuhkan tampilan:
// siapa penulisnya dan jasa apa yang diulas.
type ReviewRow struct {
	Review
	PenulisNama   string  `json:"penulis_nama"`
	PenulisAvatar *string `json:"penulis_avatar,omitempty"`
	ServiceID     int64   `json:"service_id"`
	ServiceTitle  string  `json:"service_title"`
}

type Notification struct {
	ID        int64           `json:"id"`
	UserID    int64           `json:"user_id"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	ReadAt    *time.Time      `json:"read_at,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

// ---------- percakapan ----------

type Conversation struct {
	ID            int64      `json:"id"`
	ServiceID     int64      `json:"service_id"`
	SeekerID      int64      `json:"seeker_id"`
	ProviderID    int64      `json:"provider_id"`
	OrderID       *int64     `json:"order_id,omitempty"`
	LastMessageAt *time.Time `json:"last_message_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// ConversationRow adalah percakapan beserta konteks yang dibutuhkan daftar
// obrolan: judul jasa, lawan bicara, cuplikan pesan terakhir, dan jumlah
// pesan yang belum dibaca.
type ConversationRow struct {
	Conversation
	ServiceTitle  string  `json:"service_title"`
	ServiceCover  *string `json:"service_cover,omitempty"`
	LawanNama     string  `json:"lawan_nama"`
	LawanAvatar   *string `json:"lawan_avatar,omitempty"`
	LawanUserID   int64   `json:"lawan_user_id"`
	PesanTerakhir *string `json:"pesan_terakhir,omitempty"`
	BelumDibaca   int     `json:"belum_dibaca"`
	OrderStatus   *string `json:"order_status,omitempty"`
}

type Message struct {
	ID             int64      `json:"id"`
	ConversationID int64      `json:"conversation_id"`
	SenderID       int64      `json:"sender_id"`
	Body           string     `json:"body"`
	ReadAt         *time.Time `json:"read_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// MessageRow menyertakan nama pengirim untuk ditampilkan di ruang obrolan.
type MessageRow struct {
	Message
	SenderNama   string  `json:"sender_nama"`
	SenderAvatar *string `json:"sender_avatar,omitempty"`
}
