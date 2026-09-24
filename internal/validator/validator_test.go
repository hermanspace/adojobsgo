package validator

import "testing"

func TestNormalizePhone(t *testing.T) {
	kasus := []struct {
		masukan string
		harapan string
	}{
		{"08117512001", "628117512001"},
		{"+62 811-7512-001", "628117512001"},
		{"62 811 7512 001", "628117512001"},
		{"8117512001", "628117512001"},
		{"0811-7512-001", "628117512001"},
		{"", ""},
	}
	for _, k := range kasus {
		if got := NormalizePhone(k.masukan); got != k.harapan {
			t.Errorf("NormalizePhone(%q) = %q, harusnya %q", k.masukan, got, k.harapan)
		}
	}
}

func TestValidPhone(t *testing.T) {
	valid := []string{"628117512001", "6281234567890"}
	for _, v := range valid {
		if !ValidPhone(v) {
			t.Errorf("ValidPhone(%q) = false, harusnya true", v)
		}
	}
	// Nomor tetap (bukan seluler), terlalu pendek, dan belum ternormalisasi.
	invalid := []string{"6276121234", "62811", "08117512001", "", "62"}
	for _, v := range invalid {
		if ValidPhone(v) {
			t.Errorf("ValidPhone(%q) = true, harusnya false", v)
		}
	}
}

func TestSlugify(t *testing.T) {
	kasus := map[string]string{
		"Servis AC":              "servis-ac",
		"Tukang Kayu & Mebel":    "tukang-kayu-mebel",
		"  Cat & Plafon  ":       "cat-plafon",
		"Foto Pernikahan":        "foto-pernikahan",
		"Servis Komputer/Laptop": "servis-komputer-laptop",
	}
	for masukan, harapan := range kasus {
		if got := Slugify(masukan); got != harapan {
			t.Errorf("Slugify(%q) = %q, harusnya %q", masukan, got, harapan)
		}
	}
}

func TestValidEmail(t *testing.T) {
	if !ValidEmail("nama@email.com") {
		t.Error("email valid ditolak")
	}
	for _, s := range []string{"nama@", "@email.com", "nama email.com", ""} {
		if ValidEmail(s) {
			t.Errorf("ValidEmail(%q) = true, harusnya false", s)
		}
	}
}

func TestErrorsLengthMenghitungRune(t *testing.T) {
	errs := New()
	// "Dekorasi" = 8 karakter; batas minimal 10 harus memicu error.
	errs.Length("judul", "Judul", "Dekorasi", 10, 140)
	if !errs.Has("judul") {
		t.Error("teks di bawah batas minimal seharusnya ditolak")
	}

	errs2 := New()
	// Teks kosong dilewati Length; itu tugas Required.
	errs2.Length("judul", "Judul", "   ", 10, 140)
	if errs2.Any() {
		t.Error("teks kosong seharusnya tidak dinilai oleh Length")
	}
}
