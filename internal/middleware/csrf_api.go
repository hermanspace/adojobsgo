package middleware

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// HeaderPermintaanAPI wajib menyertai setiap permintaan API yang mengubah data.
const HeaderPermintaanAPI = "X-Requested-With"

// TolakLintasSitusAPI menutup celah CSRF pada API yang memakai cookie sesi.
//
// Form HTML lintas-situs tidak bisa menambahkan header kustom, dan fetch
// lintas-situs yang membawa header kustom memicu preflight CORS — yang tidak
// pernah diizinkan server ini. Jadi kehadiran header ini saja sudah cukup
// membuktikan permintaan datang dari klien yang memang kita kenal, tanpa token
// yang harus disimpan dan diputar. Unggahan multipart ikut terlindungi,
// padahal jenis konten itu termasuk "sederhana" dan lolos tanpa preflight —
// itulah sebabnya pemeriksaan tipe konten saja tidak memadai.
//
// Metode aman (GET, HEAD, OPTIONS) dilewatkan: tidak ada yang berubah karenanya.
func TolakLintasSitusAPI(c *fiber.Ctx) error {
	switch c.Method() {
	case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
		return c.Next()
	}
	// Permintaan yang membawa header Authorization tidak memakai cookie, jadi
	// tidak ada yang bisa dipalsukan lintas-situs — peramban tidak pernah
	// menambahkan header itu sendiri.
	if c.Get(fiber.HeaderAuthorization) != "" {
		return c.Next()
	}
	if c.Get(HeaderPermintaanAPI) == "" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": fiber.Map{
				"code":    "forbidden",
				"message": "Sertakan header " + HeaderPermintaanAPI + " pada permintaan yang mengubah data.",
			},
		})
	}
	return c.Next()
}

// KunciPembatasPengguna mengunci pembatas laju pada akun, bukan alamat IP.
// Pengguna di balik satu NAT kantor atau operator seluler berbagi IP; kalau
// kuncinya IP, satu orang yang mengunggah banyak foto akan menutup jalan
// semua orang lain di jaringan yang sama. Pengunjung tanpa akun jatuh ke IP.
func KunciPembatasPengguna(c *fiber.Ctx) string {
	if u := CurrentUser(c); u != nil {
		return "u:" + strconv.FormatInt(u.ID, 10)
	}
	return "ip:" + c.IP()
}
