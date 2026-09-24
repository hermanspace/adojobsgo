package pages

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// tokens.json adalah sumber tunggal token desain untuk web DAN Android.
// Tiga hal yang bisa rusak diam-diam: (1) mode terang dan gelap punya
// himpunan variabel berbeda, sehingga satu mode kehilangan warna tanpa
// error; (2) template atau CSS memakai var(--x) yang tidak ada di tokens,
// yang di peramban hanya jatuh ke nilai bawaan tanpa peringatan; (3) app.css
// yang dibangun tertinggal dari tokens.json. Uji ini menjaga ketiganya.
func TestTokensJSONSumberTunggal(t *testing.T) {
	// Kunci _catatan berdampingan dengan objek mode, jadi mode diurai terpisah.
	var tokens struct {
		Warna    map[string]json.RawMessage `json:"warna"`
		Bayangan map[string]json.RawMessage `json:"bayangan"`
		Komponen map[string]string          `json:"komponen"`
	}
	raw, err := os.ReadFile("../../static/design/tokens.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &tokens); err != nil {
		t.Fatal(err)
	}
	mode := func(m map[string]json.RawMessage) map[string]map[string]string {
		out := map[string]map[string]string{}
		for _, nama := range []string{"terang", "gelap"} {
			var nilai map[string]string
			if err := json.Unmarshal(m[nama], &nilai); err != nil {
				t.Fatalf("mode %s: %v", nama, err)
			}
			out[nama] = nilai
		}
		return out
	}
	if strings.Contains(string(raw), "\t") {
		t.Error("tokens.json memakai tab; pakai dua spasi supaya diff antar-platform rapi")
	}

	dikenal := map[string]bool{}
	for _, kelompok := range []struct {
		nama   string
		awalan string
		mode   map[string]map[string]string
	}{{"warna", "", mode(tokens.Warna)}, {"bayangan", "shadow-", mode(tokens.Bayangan)}} {
		terang, gelap := kunciTanpaCatatan(kelompok.mode["terang"]), kunciTanpaCatatan(kelompok.mode["gelap"])
		if strings.Join(terang, ",") != strings.Join(gelap, ",") {
			t.Errorf("%s: mode terang %v ≠ mode gelap %v", kelompok.nama, terang, gelap)
		}
		for _, k := range terang {
			dikenal[kelompok.awalan+k] = true
		}
	}

	for k := range tokens.Komponen {
		if !strings.HasPrefix(k, "_") {
			dikenal["ukuran-"+k] = true
		}
	}

	// Setiap var(--x) di CSS sumber dan seluruh template harus ada di tokens.
	dipakai := regexp.MustCompile(`var\(--([a-z0-9-]+)\)`)
	berkas := []string{"../../static/css/source.css"}
	_ = filepath.Walk("..", func(p string, info os.FileInfo, err error) error {
		if err == nil && strings.HasSuffix(p, ".templ") {
			berkas = append(berkas, p)
		}
		return nil
	})
	for _, b := range berkas {
		isi, err := os.ReadFile(b)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range dipakai.FindAllStringSubmatch(string(isi), -1) {
			if !dikenal[m[1]] {
				t.Errorf("%s memakai var(--%s) yang tidak ada di tokens.json", b, m[1])
			}
		}
	}
	if src, _ := os.ReadFile("../../static/css/source.css"); regexp.MustCompile(`\n\s*--[a-z-]+:`).Match(src) {
		t.Error("source.css mendefinisikan variabel --x sendiri; pindahkan ke tokens.json")
	}

	// app.css hasil bangun harus memuat setiap variabel — kalau tidak,
	// tokens.json diubah tanpa `make css`.
	app, err := os.ReadFile("../../static/css/app.css")
	if err != nil {
		// app.css adalah hasil build yang tidak masuk git; di clone baru
		// belum ada sampai `make css`. Image Docker selalu membangunnya.
		t.Log("app.css belum dibangun; pemeriksaan keselarasan dilewati — jalankan `make css`")
		return
	}
	for nama := range dikenal {
		if !strings.Contains(string(app), "--"+nama+":") {
			t.Errorf("app.css tidak memuat --%s; jalankan `make css` setelah mengubah tokens.json", nama)
		}
	}
	cfg, _ := os.ReadFile("../../../tailwind.config.js")
	if !strings.Contains(string(cfg), "tokens.json") {
		t.Error("tailwind.config.js tidak membaca tokens.json")
	}
}

func kunciTanpaCatatan(m map[string]string) []string {
	var out []string
	for k := range m {
		if !strings.HasPrefix(k, "_") {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
