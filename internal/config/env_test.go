package config

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// Kunci lingkungan yang dibaca kode tetapi tidak ada di .env.example gagal
// diam-diam di server: nilainya jatuh ke bawaan tanpa ada yang tahu bahwa ia
// bisa disetel. Arah sebaliknya sama pentingnya — kunci di .env.example yang
// tidak dibaca siapa pun hanya membingungkan orang yang menyalinnya.

var (
	polaEnvKode  = regexp.MustCompile(`\benv(?:Int|Bool|List)?\("([A-Z_]+)"`)
	polaGetenv   = regexp.MustCompile(`os\.(?:Getenv|LookupEnv)\("([A-Z_]+)"\)`)
	polaContoh   = regexp.MustCompile(`(?m)^([A-Z_]+)=`)
	polaMakefile = regexp.MustCompile(`\$[({]([A-Z_]+)[)}]`)
	polaCompose  = regexp.MustCompile(`\$\{([A-Z_]+)`)
)

func kumpulkan(t *testing.T, pola *regexp.Regexp, berkas ...string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, f := range berkas {
		isi, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for _, m := range pola.FindAllStringSubmatch(string(isi), -1) {
			out[m[1]] = true
		}
	}
	return out
}

func TestSetiapKunciYangDibacaAdaDiContoh(t *testing.T) {
	dibaca := kumpulkan(t, polaEnvKode, "config.go")
	for k := range kumpulkan(t, polaGetenv, "../../cmd/server/main.go") {
		dibaca[k] = true
	}
	contoh := kumpulkan(t, polaContoh, "../../.env.example")

	if len(dibaca) < 20 {
		t.Fatalf("hanya %d kunci terbaca dari config.go; pola pemindainya mungkin meleset", len(dibaca))
	}
	for k := range dibaca {
		if !contoh[k] {
			t.Errorf("%s dibaca kode tetapi tidak ada di .env.example", k)
		}
	}
}

func TestSetiapKunciContohDipakai(t *testing.T) {
	contoh := kumpulkan(t, polaContoh, "../../.env.example")

	dipakai := kumpulkan(t, polaEnvKode, "config.go")
	for k := range kumpulkan(t, polaGetenv, "../../cmd/server/main.go") {
		dipakai[k] = true
	}
	for k := range kumpulkan(t, polaMakefile, "../../Makefile") {
		dipakai[k] = true
	}
	compose, _ := filepath.Glob("../../docker-compose*.yml")
	for k := range kumpulkan(t, polaCompose, compose...) {
		dipakai[k] = true
	}

	for k := range contoh {
		if !dipakai[k] {
			t.Errorf("%s ada di .env.example tetapi tidak dibaca kode, Makefile, maupun compose", k)
		}
	}
}

// Variabel compose berasal dari .env; yang tidak ada di .env.example akan
// kosong diam-diam saat `make up` di server baru.
func TestSetiapVariabelComposeAdaDiContoh(t *testing.T) {
	contoh := kumpulkan(t, polaContoh, "../../.env.example")
	compose, _ := filepath.Glob("../../docker-compose*.yml")
	if len(compose) == 0 {
		t.Fatal("berkas compose tidak ditemukan")
	}
	for k := range kumpulkan(t, polaCompose, compose...) {
		if !contoh[k] {
			t.Errorf("%s dipakai compose tetapi tidak ada di .env.example", k)
		}
	}
}
