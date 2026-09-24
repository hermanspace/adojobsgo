package handler

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Setiap rute yang menerima berkas harus melewati batasUnggah. Rute unggah
// baru yang lupa memasangnya tidak gagal di mana pun — ia cuma jadi jalan
// masuk tanpa penjaga untuk membanjiri disk.
func TestSemuaRuteUnggahDibatasi(t *testing.T) {
	// Handler yang membaca berkas, dikumpulkan dari kode handler sendiri
	// supaya daftarnya tidak perlu diingat manual di sini.
	pembacaBerkas := kumpulkanHandlerBerkas(t)
	if len(pembacaBerkas) == 0 {
		t.Fatal("tidak ada handler pembaca berkas yang ditemukan")
	}

	isi, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, baris := range strings.Split(string(isi), "\n") {
		for _, h := range pembacaBerkas {
			if !strings.Contains(baris, "."+h+")") {
				continue
			}
			if !strings.Contains(baris, "batasUnggah") {
				t.Errorf("rute unggah tanpa pembatas laju:\n%s", strings.TrimSpace(baris))
			}
		}
	}
}

// kumpulkanHandlerBerkas memindai berkas handler dan mengembalikan nama fungsi
// yang memanggil FormFile atau MultipartForm.
func kumpulkanHandlerBerkas(t *testing.T) []string {
	t.Helper()
	handler := regexp.MustCompile(`^func \(h \*Handler\) (\w+)\(c \*fiber\.Ctx\) error \{`)
	// Badan sebuah fungsi berakhir di deklarasi func berikutnya — apa pun
	// jenisnya. Kalau batasnya hanya handler berikutnya, fungsi bantu biasa
	// yang membaca berkas di antara dua handler akan dihitung milik handler
	// sebelumnya, dan rute GET yang tidak pernah membaca berkas dituduh.
	batas := regexp.MustCompile(`^func `)
	pembaca := regexp.MustCompile(`c\.(FormFile|MultipartForm)\(`)

	var hasil []string
	for _, dir := range []string{"web", "api"} {
		entri, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entri {
			if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			isi, err := os.ReadFile(dir + "/" + e.Name())
			if err != nil {
				t.Fatal(err)
			}
			var nama string
			var badan strings.Builder
			simpan := func() {
				if nama != "" && pembaca.MatchString(badan.String()) {
					hasil = append(hasil, nama)
				}
				nama = ""
				badan.Reset()
			}
			for _, baris := range strings.Split(string(isi), "\n") {
				if batas.MatchString(baris) {
					simpan()
					if m := handler.FindStringSubmatch(baris); m != nil {
						nama = m[1]
					}
					continue
				}
				if nama != "" {
					badan.WriteString(baris)
					badan.WriteByte('\n')
				}
			}
			simpan()
		}
	}
	return hasil
}
