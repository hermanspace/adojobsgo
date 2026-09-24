package layout

import (
	"regexp"
	"strings"
)

var polaRuangObrolan = regexp.MustCompile(`^/pesan/\d+`)

// TampilkanMenu memutuskan apakah tombol melayang dirender di halaman ini.
// Ruang obrolan dikecualikan: tombolnya akan menutupi kotak kirim pesan,
// dan di sana fokus pengguna memang cuma satu.
func TampilkanMenu(path string) bool {
	return !polaRuangObrolan.MatchString(path) && !strings.HasPrefix(path, "/admin")
}
