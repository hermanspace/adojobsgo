package service

import (
	"testing"
	"time"
)

func TestHitungBatasSorot(t *testing.T) {
	// Durasi 0 berarti sorotan dicabut, bukan sorotan berdurasi nol.
	until, err := hitungBatasSorot(0)
	if err != nil || until != nil {
		t.Errorf("durasi 0 seharusnya mencabut sorotan, dapat %v, %v", until, err)
	}

	until, err = hitungBatasSorot(30)
	if err != nil {
		t.Fatalf("durasi 30 hari ditolak: %v", err)
	}
	selisih := time.Until(*until)
	if selisih < 29*24*time.Hour || selisih > 31*24*time.Hour {
		t.Errorf("batas sorotan 30 hari meleset: %v", selisih)
	}

	// Durasi di luar daftar ditolak, supaya admin tidak bisa menyetel
	// sorotan seumur hidup lewat permintaan yang dirakit sendiri.
	if _, err := hitungBatasSorot(9999); err == nil {
		t.Error("durasi di luar daftar seharusnya ditolak")
	}
	if _, err := hitungBatasSorot(-7); err == nil {
		t.Error("durasi negatif seharusnya ditolak")
	}
}

func TestDurasiSorotTersedia(t *testing.T) {
	if len(DurasiSorot) == 0 {
		t.Fatal("daftar durasi sorotan kosong")
	}
	for _, d := range DurasiSorot {
		if d.Hari <= 0 {
			t.Errorf("durasi %q tidak masuk akal: %d hari", d.Label, d.Hari)
		}
		if _, err := hitungBatasSorot(d.Hari); err != nil {
			t.Errorf("durasi %q ada di daftar tapi ditolak: %v", d.Label, err)
		}
	}
}

func TestIkonKategoriValid(t *testing.T) {
	if len(IkonKategori) == 0 {
		t.Fatal("daftar ikon kategori kosong")
	}
	for _, i := range IkonKategori {
		if !ikonValid(i.Nama) {
			t.Errorf("ikon %q ada di daftar tapi dianggap tidak valid", i.Nama)
		}
	}
	if ikonValid("ikon-yang-tidak-ada") {
		t.Error("ikon di luar daftar seharusnya ditolak")
	}
}
