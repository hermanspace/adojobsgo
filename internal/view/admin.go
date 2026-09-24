package view

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/hermansyah/adojobsid/internal/model"
	"github.com/hermansyah/adojobsid/internal/repository"
	"github.com/hermansyah/adojobsid/internal/service"
)

// AdminBase adalah data bersama seluruh halaman panel admin.
type AdminBase struct {
	Base
	ActiveMenu string // dasbor | pengguna | antrean | jasa | kategori | pengaturan
	// Antrean adalah jumlah listing yang menunggu peninjauan.
	Antrean int
	// AntreanPromosi adalah pengajuan promosi yang menunggu keputusan admin.
	AntreanPromosi int
}

// DenganPeta menandai halaman admin yang menggambar peta.
func (b AdminBase) DenganPeta() AdminBase { b.Base = b.Base.DenganPeta(); return b }

// AdminDashboardData adalah data halaman ringkasan.
type AdminDashboardData struct {
	AdminBase
	Stats    *repository.Stats
	Kategori []repository.KategoriTeratas
	Terbaru  []repository.AdminUserRow
	Promosi  *repository.StatistikPromosi
}

// CTR menghitung rasio klik per tayang seluruh iklan aktif, dalam persen.
func (d AdminDashboardData) CTR() string {
	return persenKlik(d.Promosi.TotalTayang, d.Promosi.TotalKlik)
}

// persenKlik memformat klik/tayang sebagai persen satu desimal; tanpa tayang
// hasilnya "—", bukan pembagian nol yang menyamar jadi 0,0%.
func persenKlik(tayang, klik int64) string {
	if tayang == 0 {
		return "—"
	}
	return strings.Replace(strconv.FormatFloat(float64(klik)*100/float64(tayang), 'f', 1, 64), ".", ",", 1) + "%"
}

// PersenDari menghitung lebar batang grafik kategori relatif terhadap
// kategori dengan listing terbanyak.
func (d AdminDashboardData) PersenDari(jumlah int) int {
	maks := 0
	for _, k := range d.Kategori {
		if k.Jumlah > maks {
			maks = k.Jumlah
		}
	}
	if maks == 0 {
		return 0
	}
	p := jumlah * 100 / maks
	if p < 4 {
		return 4 // batang tetap terlihat walau nilainya kecil
	}
	return p
}

// AdminUsersData adalah data halaman kelola pengguna.
type AdminUsersData struct {
	AdminBase
	Filter     repository.UserFilter
	Items      []repository.AdminUserRow
	Total      int
	Page       int
	TotalPages int
	Kecamatan  []string
	Durasi     []DurasiSorot
	AdminID    int64
}

// AdminServicesData adalah data halaman kelola listing jasa.
type AdminServicesData struct {
	AdminBase
	Filter     repository.AdminServiceFilter
	Items      []model.ServiceCard
	Total      int
	Page       int
	TotalPages int
	Kategori   []model.Category
	Durasi     []DurasiSorot
}

// AdminCategoriesData adalah data halaman kelola kategori.
type AdminCategoriesData struct {
	AdminBase
	Items  []repository.CategoryRow
	Induk  []model.Category
	Ikon   []IkonPilihan
	Form   Form
	EditID int64
	IsEdit bool
}

// AdminPaketData adalah data halaman kelola paket promosi.
type AdminPaketData struct {
	AdminBase
	Items  []model.PaketPromosi
	Slot   []model.SlotIklan
	Jenis  []string
	Form   Form
	EditID int64
	IsEdit bool
}

// SlotDipilih membaca pilihan slot dari form; nilainya disimpan sebagai
// daftar yang dipisah koma supaya muat di peta nilai Form yang bertipe string.
func (d AdminPaketData) SlotDipilih(kunci string) bool {
	for _, s := range strings.Split(d.Form.Value("slot_iklan"), ",") {
		if s == kunci {
			return true
		}
	}
	return false
}

// DurasiSorot adalah pilihan lama tayang untuk penyedia dan listing pilihan.
type DurasiSorot struct {
	Label string
	Hari  int
}

// IkonPilihan adalah satu opsi ikon kategori.
type IkonPilihan struct {
	Nama  string
	Label string
}

// DurasiSorotOptions menyalin pilihan durasi dari service layer, supaya
// template tidak perlu mengimpor package service.
func DurasiSorotOptions() []DurasiSorot {
	out := make([]DurasiSorot, 0, len(service.DurasiSorot))
	for _, d := range service.DurasiSorot {
		out = append(out, DurasiSorot{Label: d.Label, Hari: d.Hari})
	}
	return out
}

// IkonOptions menyalin pilihan ikon kategori dari service layer.
func IkonOptions() []IkonPilihan {
	out := make([]IkonPilihan, 0, len(service.IkonKategori))
	for _, i := range service.IkonKategori {
		out = append(out, IkonPilihan{Nama: i.Nama, Label: i.Label})
	}
	return out
}

// ---------- helper URL & paginasi ----------

// URLPengguna membangun ulang URL daftar pengguna dengan satu parameter diganti.
func (d AdminUsersData) URLPengguna(key, value string) string {
	q := url.Values{}
	set := func(k, v string) {
		if strings.TrimSpace(v) != "" {
			q.Set(k, v)
		}
	}
	set("q", d.Filter.Query)
	set("peran", d.Filter.Role)
	set("status", d.Filter.Status)
	set("tipe", d.Filter.OnlyType)
	set("kecamatan", d.Filter.Kecamatan)
	set("urut", d.Filter.Sort)

	if strings.TrimSpace(value) == "" {
		q.Del(key)
	} else {
		q.Set(key, value)
	}
	if len(q) == 0 {
		return "/admin/pengguna"
	}
	return "/admin/pengguna?" + q.Encode()
}

func (d AdminUsersData) URLHalaman(page int) string {
	base := d.URLPengguna("", "")
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + "page=" + strconv.Itoa(page)
}

func (d AdminUsersData) HalamanSebelum() int { return d.Page - 1 }
func (d AdminUsersData) HalamanSesudah() int { return d.Page + 1 }
func (d AdminUsersData) AdaSebelum() bool    { return d.Page > 1 }
func (d AdminUsersData) AdaSesudah() bool    { return d.Page < d.TotalPages }

// URLJasa membangun ulang URL daftar listing dengan satu parameter diganti.
func (d AdminServicesData) URLJasa(key, value string) string {
	q := url.Values{}
	set := func(k, v string) {
		if strings.TrimSpace(v) != "" {
			q.Set(k, v)
		}
	}
	set("q", d.Filter.Query)
	set("status", d.Filter.Status)
	set("sorot", d.Filter.Featured)
	set("urut", d.Filter.Sort)
	if d.Filter.CategoryID > 0 {
		set("kategori", strconv.FormatInt(d.Filter.CategoryID, 10))
	}
	if d.Filter.TanpaFoto {
		set("tanpa_foto", "1")
	}

	if strings.TrimSpace(value) == "" {
		q.Del(key)
	} else {
		q.Set(key, value)
	}
	if len(q) == 0 {
		return "/admin/jasa"
	}
	return "/admin/jasa?" + q.Encode()
}

func (d AdminServicesData) URLHalaman(page int) string {
	base := d.URLJasa("", "")
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + "page=" + strconv.Itoa(page)
}

func (d AdminServicesData) HalamanSebelum() int { return d.Page - 1 }
func (d AdminServicesData) HalamanSesudah() int { return d.Page + 1 }
func (d AdminServicesData) AdaSebelum() bool    { return d.Page > 1 }
func (d AdminServicesData) AdaSesudah() bool    { return d.Page < d.TotalPages }

// KategoriTerpilih menandai opsi kategori yang sedang aktif di filter.
func (d AdminServicesData) KategoriTerpilih(id int64) bool { return d.Filter.CategoryID == id }

// AdminPromosiData adalah antrean promosi di panel admin.
type AdminPromosiData struct {
	AdminBase
	Tab        string // menunggu | bayar | tayang | riwayat
	Jenis      string
	Items      []repository.PromosiAdminRow
	Total      int
	Page       int
	TotalPages int
}

func (d AdminPromosiData) URLTab(tab string) string {
	u := "/admin/promosi?tab=" + tab
	if d.Jenis != "" {
		u += "&jenis=" + d.Jenis
	}
	return u
}

func (d AdminPromosiData) URLJenis(jenis string) string {
	u := "/admin/promosi?tab=" + d.Tab
	if jenis != "" {
		u += "&jenis=" + jenis
	}
	return u
}

// Sasaran menyusun teks singkat apa yang dipromosikan.
func (d AdminPromosiData) Sasaran(b repository.PromosiAdminRow) string {
	switch {
	case b.Judul != nil:
		return *b.Judul
	case b.JudulJasa != nil:
		return *b.JudulJasa
	case b.NamaPenyedia != nil:
		return *b.NamaPenyedia
	}
	return "—"
}

// AdminPromosiDetailData adalah satu pengajuan di panel admin.
type AdminPromosiDetailData struct {
	AdminBase
	Promosi      model.Promosi
	Paket        *model.PaketPromosi
	Pemohon      *model.User
	JudulJasa    string
	NamaPenyedia string
	SlugPenyedia string
	Form         Form
}

// Boleh menjawab apakah admin dapat memindahkan pengajuan ke status tujuan.
func (d AdminPromosiDetailData) Boleh(ke string) bool {
	return model.BolehTransisi(d.Promosi.Status, ke, model.AktorAdmin)
}

// PerluKonfirmasiBayar: disetujui dan pemohon sudah mengirim bukti.
func (d AdminPromosiDetailData) PerluKonfirmasiBayar() bool {
	return d.Promosi.Status == model.StatusDisetujui && d.Promosi.BuktiBayarURL != nil
}
