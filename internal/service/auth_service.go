package service

import (
	"context"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/repository"
	"github.com/hermansyah/adojobsid/internal/validator"
)

type AuthService struct {
	repos *repository.Repositories
}

type RegisterInput struct {
	FullName        string
	Phone           string
	Email           string
	Password        string
	PasswordConfirm string
	City            string
	Kecamatan       string
}

type LoginInput struct {
	Identifier string // nomor HP atau email
	Password   string
}

// Register membuat akun baru. Nomor HP adalah identitas utama; email opsional.
func (s *AuthService) Register(ctx context.Context, in RegisterInput) (*model.User, error) {
	errs := validator.New()

	name := errs.Required("full_name", "Nama lengkap", in.FullName)
	errs.Length("full_name", "Nama lengkap", name, 3, 120)

	phone := validator.NormalizePhone(in.Phone)
	if strings.TrimSpace(in.Phone) == "" {
		errs.Add("phone", "Nomor HP wajib diisi.")
	} else if !validator.ValidPhone(phone) {
		errs.Add("phone", "Format nomor HP tidak valid. Contoh: 0812xxxxxxx.")
	}

	email := strings.TrimSpace(in.Email)
	if email != "" && !validator.ValidEmail(email) {
		errs.Add("email", "Format email tidak valid.")
	}

	if len([]rune(in.Password)) < 8 {
		errs.Add("password", "Kata sandi minimal 8 karakter.")
	}
	if in.Password != in.PasswordConfirm {
		errs.Add("password_confirm", "Konfirmasi kata sandi tidak sama.")
	}

	city := strings.TrimSpace(in.City)
	if city == "" {
		city = "Bengkalis" // default pasar awal platform
	}

	if errs.Any() {
		return nil, Invalid(errs)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, Internal(err)
	}

	user := &model.User{
		FullName:     name,
		Phone:        phone,
		PasswordHash: string(hash),
		City:         strPtr(city),
		Kecamatan:    strPtrOrNil(in.Kecamatan),
	}
	if email != "" {
		user.Email = &email
	}

	if err := s.repos.User.Create(ctx, user); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			// Bedakan bentrok nomor HP dan email supaya pesan form tepat sasaran.
			if exists, _ := s.repos.User.PhoneExists(ctx, phone); exists {
				errs.Add("phone", "Nomor HP ini sudah terdaftar. Silakan masuk.")
			} else {
				errs.Add("email", "Email ini sudah terdaftar.")
			}
			return nil, Invalid(errs)
		}
		return nil, Internal(err)
	}
	return user, nil
}

// Login menerima nomor HP atau email. Pesan kesalahan sengaja dibuat sama untuk
// kedua kasus (akun tidak ada / sandi salah) agar tidak membocorkan akun terdaftar.
func (s *AuthService) Login(ctx context.Context, in LoginInput) (*model.User, error) {
	id := strings.TrimSpace(in.Identifier)
	if id == "" || in.Password == "" {
		return nil, Unauthorized("Nomor HP dan kata sandi wajib diisi.")
	}

	var (
		user *model.User
		err  error
	)
	if strings.Contains(id, "@") {
		user, err = s.repos.User.GetByEmail(ctx, id)
	} else {
		user, err = s.repos.User.GetByPhone(ctx, validator.NormalizePhone(id))
	}
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// Tetap jalankan perbandingan hash palsu agar waktu respons seragam.
			_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(in.Password))
			return nil, Unauthorized("Nomor HP atau kata sandi salah.")
		}
		return nil, Internal(err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.Password)); err != nil {
		return nil, Unauthorized("Nomor HP atau kata sandi salah.")
	}

	// Pemeriksaan penangguhan dilakukan setelah kata sandi diverifikasi, supaya
	// orang luar tidak bisa memakai halaman masuk untuk menebak akun mana saja
	// yang sedang ditangguhkan.
	if user.IsSuspended() {
		pesan := "Akun Anda sedang ditangguhkan."
		if user.SuspendedReason != nil && *user.SuspendedReason != "" {
			pesan += " Alasan: " + *user.SuspendedReason
		}
		return nil, Forbidden(pesan + " Hubungi pengelola bila ini keliru.")
	}
	return user, nil
}

// dummyHash adalah hash bcrypt valid untuk string acak, dipakai hanya sebagai
// pengimbang waktu saat akun tidak ditemukan.
const dummyHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

func (s *AuthService) GetUser(ctx context.Context, id int64) (*model.User, error) {
	user, err := s.repos.User.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, NotFound("Akun tidak ditemukan.")
		}
		return nil, Internal(err)
	}
	return user, nil
}

type UpdateProfileInput struct {
	FullName  string
	Email     string
	City      string
	Kecamatan string
}

func (s *AuthService) UpdateProfile(ctx context.Context, userID int64, in UpdateProfileInput) (*model.User, error) {
	user, err := s.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	errs := validator.New()
	name := errs.Required("full_name", "Nama lengkap", in.FullName)
	errs.Length("full_name", "Nama lengkap", name, 3, 120)

	email := strings.TrimSpace(in.Email)
	if email != "" && !validator.ValidEmail(email) {
		errs.Add("email", "Format email tidak valid.")
	}
	if errs.Any() {
		return nil, Invalid(errs)
	}

	user.FullName = name
	user.Email = nil
	if email != "" {
		user.Email = &email
	}
	user.City = strPtrOrNil(in.City)
	user.Kecamatan = strPtrOrNil(in.Kecamatan)

	if err := s.repos.User.Update(ctx, user); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			errs.Add("email", "Email ini sudah dipakai akun lain.")
			return nil, Invalid(errs)
		}
		return nil, Internal(err)
	}
	return user, nil
}

func (s *AuthService) ChangePassword(ctx context.Context, userID int64, current, next, confirm string) error {
	user, err := s.GetUser(ctx, userID)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(current)) != nil {
		errs := validator.New()
		errs.Add("current_password", "Kata sandi saat ini salah.")
		return Invalid(errs)
	}
	errs := validator.New()
	if len([]rune(next)) < 8 {
		errs.Add("password", "Kata sandi baru minimal 8 karakter.")
	}
	if next != confirm {
		errs.Add("password_confirm", "Konfirmasi kata sandi tidak sama.")
	}
	if errs.Any() {
		return Invalid(errs)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		return Internal(err)
	}
	if err := s.repos.User.UpdatePassword(ctx, userID, string(hash)); err != nil {
		return Internal(err)
	}
	return nil
}

func strPtr(s string) *string { return &s }

func strPtrOrNil(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
