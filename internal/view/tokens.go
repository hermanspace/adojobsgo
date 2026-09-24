package view

import (
	"encoding/json"
	"os"
	"sync"
)

// Token desain dibaca dari berkas yang sama dengan Tailwind dan aplikasi
// Android, supaya warna yang dikirim ke peramban lewat manifest atau meta
// tidak pernah menyimpang dari CSS. Dimuat sekali, dengan cadangan aman bila
// berkasnya tidak terbaca (mis. uji unit di direktori lain).
var (
	tokenSekali sync.Once
	tokenWarna  map[string]map[string]string
)

const berkasToken = "web/static/design/tokens.json"

func muatToken() {
	tokenWarna = map[string]map[string]string{
		"terang": {"bg": "#ffffff", "primary-solid": "#5b5bf6"},
		"gelap":  {"bg": "#0b0b12", "primary-solid": "#7b73f8"},
	}
	raw, err := os.ReadFile(berkasToken)
	if err != nil {
		return
	}
	var t struct {
		Warna map[string]json.RawMessage `json:"warna"`
	}
	if json.Unmarshal(raw, &t) != nil {
		return
	}
	for _, mode := range []string{"terang", "gelap"} {
		var m map[string]string
		if json.Unmarshal(t.Warna[mode], &m) == nil && len(m) > 0 {
			tokenWarna[mode] = m
		}
	}
}

// TokenWarna mengembalikan nilai token warna untuk mode "terang" atau
// "gelap"; kosong bila nama tidak dikenal.
func TokenWarna(mode, nama string) string {
	tokenSekali.Do(muatToken)
	return tokenWarna[mode][nama]
}
