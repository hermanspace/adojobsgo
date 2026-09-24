package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hermansyah/adojobsid/internal/model"
)

// ---------- paket ----------

type PaketRepository struct{ db DBTX }

const paketKolom = `id, jenis, nama, deskripsi, durasi_hari, harga, bobot, slot_iklan, aktif, urutan, created_at, updated_at`

func targetPaket(p *model.PaketPromosi) []any {
	return []any{&p.ID, &p.Jenis, &p.Nama, &p.Deskripsi, &p.DurasiHari, &p.Harga, &p.Bobot,
		&p.SlotIklan, &p.Aktif, &p.Urutan, &p.CreatedAt, &p.UpdatedAt}
}

// List mengembalikan paket berurutan seperti di UI: per jenis, lalu urutan
// yang disetel admin, lalu durasi.
func (r *PaketRepository) List(ctx context.Context, hanyaAktif bool) ([]model.PaketPromosi, error) {
	q := `SELECT ` + paketKolom + ` FROM paket_promosi`
	if hanyaAktif {
		q += ` WHERE aktif`
	}
	q += ` ORDER BY jenis, urutan, durasi_hari, id`

	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.PaketPromosi
	for rows.Next() {
		var p model.PaketPromosi
		if err := rows.Scan(targetPaket(&p)...); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *PaketRepository) Get(ctx context.Context, id int64) (*model.PaketPromosi, error) {
	var p model.PaketPromosi
	err := r.db.QueryRow(ctx, `SELECT `+paketKolom+` FROM paket_promosi WHERE id = $1`, id).
		Scan(targetPaket(&p)...)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *PaketRepository) Create(ctx context.Context, p *model.PaketPromosi) error {
	err := r.db.QueryRow(ctx, `
		INSERT INTO paket_promosi (jenis, nama, deskripsi, durasi_hari, harga, bobot, slot_iklan, aktif, urutan)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at`,
		p.Jenis, p.Nama, p.Deskripsi, p.DurasiHari, p.Harga, p.Bobot, p.SlotIklan, p.Aktif, p.Urutan).
		Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

func (r *PaketRepository) Update(ctx context.Context, p *model.PaketPromosi) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE paket_promosi
		   SET jenis = $2, nama = $3, deskripsi = $4, durasi_hari = $5, harga = $6,
		       bobot = $7, slot_iklan = $8, aktif = $9, urutan = $10, updated_at = NOW()
		 WHERE id = $1`,
		p.ID, p.Jenis, p.Nama, p.Deskripsi, p.DurasiHari, p.Harga, p.Bobot, p.SlotIklan, p.Aktif, p.Urutan)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete menghapus paket. Paket yang pernah dipakai promosi ditahan oleh FK
// (ON DELETE RESTRICT) dan dilaporkan sebagai ErrConflict — admin
// menonaktifkannya saja supaya riwayat promosi lama tetap bisa dibaca.
func (r *PaketRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM paket_promosi WHERE id = $1`, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return ErrConflict
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Upsert dipakai seeder: nama per jenis adalah kuncinya, dan nilai yang
// disetel admin lewat panel (aktif) tidak ditimpa.
func (r *PaketRepository) Upsert(ctx context.Context, p *model.PaketPromosi) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO paket_promosi (jenis, nama, deskripsi, durasi_hari, harga, bobot, slot_iklan, aktif, urutan)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (jenis, LOWER(nama)) DO UPDATE
		   SET deskripsi = EXCLUDED.deskripsi, durasi_hari = EXCLUDED.durasi_hari,
		       bobot = EXCLUDED.bobot, slot_iklan = EXCLUDED.slot_iklan,
		       urutan = EXCLUDED.urutan, updated_at = NOW()
		RETURNING id`,
		p.Jenis, p.Nama, p.Deskripsi, p.DurasiHari, p.Harga, p.Bobot, p.SlotIklan, p.Aktif, p.Urutan).
		Scan(&p.ID)
}

// JumlahPromosi menghitung promosi yang memakai paket ini, apa pun statusnya.
func (r *PaketRepository) JumlahPromosi(ctx context.Context, paketID int64) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM promosi WHERE paket_id = $1`, paketID).Scan(&n)
	return n, err
}

// ---------- promosi ----------

type PromosiRepository struct{ db DBTX }

const promosiKolom = `id, jenis, user_id, provider_id, service_id, paket_id, status, sumber,
	judul, deskripsi, gambar_url, tautan_url, target_kecamatan,
	alasan, catatan_admin, ditinjau_oleh, ditinjau_at,
	bukti_bayar_url, dibayar_at, dikonfirmasi_oleh,
	mulai_at, selesai_at, tayang, klik, created_at, updated_at`

func targetPromosi(p *model.Promosi) []any {
	return []any{&p.ID, &p.Jenis, &p.UserID, &p.ProviderID, &p.ServiceID, &p.PaketID, &p.Status, &p.Sumber,
		&p.Judul, &p.Deskripsi, &p.GambarURL, &p.TautanURL, &p.TargetKecamatan,
		&p.Alasan, &p.CatatanAdmin, &p.DitinjauOleh, &p.DitinjauAt,
		&p.BuktiBayarURL, &p.DibayarAt, &p.DikonfirmasiOleh,
		&p.MulaiAt, &p.SelesaiAt, &p.Tayang, &p.Klik, &p.CreatedAt, &p.UpdatedAt}
}

// Create menyimpan pengajuan baru. Indeks unik parsial menolak pengajuan
// kedua yang masih hidup untuk listing/penyedia yang sama → ErrConflict.
func (r *PromosiRepository) Create(ctx context.Context, p *model.Promosi) error {
	if p.TargetKecamatan == nil {
		p.TargetKecamatan = []string{}
	}
	err := r.db.QueryRow(ctx, `
		INSERT INTO promosi (jenis, user_id, provider_id, service_id, paket_id, status, sumber,
		                     judul, deskripsi, gambar_url, tautan_url, target_kecamatan, alasan,
		                     mulai_at, selesai_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id, created_at, updated_at`,
		p.Jenis, p.UserID, p.ProviderID, p.ServiceID, p.PaketID, p.Status, p.Sumber,
		p.Judul, p.Deskripsi, p.GambarURL, p.TautanURL, p.TargetKecamatan, p.Alasan,
		p.MulaiAt, p.SelesaiAt).
		Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

func (r *PromosiRepository) Get(ctx context.Context, id int64) (*model.Promosi, error) {
	var p model.Promosi
	err := r.db.QueryRow(ctx, `SELECT `+promosiKolom+` FROM promosi WHERE id = $1`, id).
		Scan(targetPromosi(&p)...)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

// Simpan menulis seluruh kolom yang boleh berubah setelah pengajuan dibuat.
func (r *PromosiRepository) Simpan(ctx context.Context, p *model.Promosi) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE promosi
		   SET status = $2, judul = $3, deskripsi = $4, gambar_url = $5, tautan_url = $6,
		       target_kecamatan = $7, alasan = $8, catatan_admin = $9, ditinjau_oleh = $10,
		       ditinjau_at = $11, bukti_bayar_url = $12, dibayar_at = $13, dikonfirmasi_oleh = $14,
		       mulai_at = $15, selesai_at = $16, paket_id = $17, updated_at = NOW()
		 WHERE id = $1`,
		p.ID, p.Status, p.Judul, p.Deskripsi, p.GambarURL, p.TautanURL,
		p.TargetKecamatan, p.Alasan, p.CatatanAdmin, p.DitinjauOleh,
		p.DitinjauAt, p.BuktiBayarURL, p.DibayarAt, p.DikonfirmasiOleh,
		p.MulaiAt, p.SelesaiAt, p.PaketID)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// PromosiFilter menyaring daftar di panel admin. Status boleh lebih dari
// satu karena satu tab admin memuat beberapa status (tayang = aktif + dijeda).
type PromosiFilter struct {
	Status []string
	Jenis  string
	Limit  int
	Offset int
}

func klausaPromosi(f PromosiFilter, alias string) (string, []any) {
	var conds []string
	var args []any
	if len(f.Status) > 0 {
		args = append(args, f.Status)
		conds = append(conds, fmt.Sprintf("%sstatus = ANY($%d)", alias, len(args)))
	}
	if f.Jenis != "" {
		args = append(args, f.Jenis)
		conds = append(conds, fmt.Sprintf("%sjenis = $%d", alias, len(args)))
	}
	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// PromosiAdminRow adalah baris antrean admin: pengajuan beserta nama pemohon
// dan sasarannya, supaya daftar terbaca tanpa membuka tiap detail.
type PromosiAdminRow struct {
	model.Promosi
	NamaPemohon  string
	JudulJasa    *string
	NamaPenyedia *string
	NamaPaket    *string
}

const promosiAdminKolom = `pr.id, pr.jenis, pr.user_id, pr.provider_id, pr.service_id, pr.paket_id, pr.status, pr.sumber,
	pr.judul, pr.deskripsi, pr.gambar_url, pr.tautan_url, pr.target_kecamatan,
	pr.alasan, pr.catatan_admin, pr.ditinjau_oleh, pr.ditinjau_at,
	pr.bukti_bayar_url, pr.dibayar_at, pr.dikonfirmasi_oleh,
	pr.mulai_at, pr.selesai_at, pr.tayang, pr.klik, pr.created_at, pr.updated_at,
	u.full_name, s.title, pu.full_name, pk.nama`

const promosiAdminDari = ` FROM promosi pr
	JOIN users u ON u.id = pr.user_id
	LEFT JOIN services s ON s.id = pr.service_id
	LEFT JOIN provider_profiles pp ON pp.id = pr.provider_id
	LEFT JOIN users pu ON pu.id = pp.user_id
	LEFT JOIN paket_promosi pk ON pk.id = pr.paket_id`

// ListAdmin mengembalikan antrean admin, yang paling lama menunggu lebih dulu
// untuk status yang perlu ditindak, dan terbaru dulu untuk riwayat.
func (r *PromosiRepository) ListAdmin(ctx context.Context, f PromosiFilter) ([]PromosiAdminRow, error) {
	where, args := klausaPromosi(f, "pr.")
	if f.Limit <= 0 {
		f.Limit = 50
	}
	args = append(args, f.Limit, f.Offset)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`SELECT %s%s%s
		ORDER BY CASE WHEN pr.status IN ('menunggu', 'disetujui') THEN pr.created_at END ASC,
		         pr.updated_at DESC
		LIMIT $%d OFFSET $%d`, promosiAdminKolom, promosiAdminDari, where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PromosiAdminRow
	for rows.Next() {
		var b PromosiAdminRow
		target := append(targetPromosi(&b.Promosi), &b.NamaPemohon, &b.JudulJasa, &b.NamaPenyedia, &b.NamaPaket)
		if err := rows.Scan(target...); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// CountPerluTindakan menghitung yang menunggu keputusan admin: pengajuan
// baru, dan yang sudah mengirim bukti bayar tapi belum dikonfirmasi.
func (r *PromosiRepository) CountPerluTindakan(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM promosi
		WHERE status = 'menunggu' OR (status = 'disetujui' AND bukti_bayar_url IS NOT NULL)`).Scan(&n)
	return n, err
}

// ListHidupUntuk mengembalikan pengajuan yang masih hidup untuk satu sasaran
// (listing atau penyedia), dipakai tombol sorot admin.
func (r *PromosiRepository) ListHidupUntuk(ctx context.Context, jenis string, targetID int64) ([]model.Promosi, error) {
	kolom := "service_id"
	if jenis == model.PromosiPenyedia {
		kolom = "provider_id"
	}
	rows, err := r.db.Query(ctx, `SELECT `+promosiKolom+` FROM promosi
		WHERE jenis = $1 AND `+kolom+` = $2 AND status IN ('menunggu', 'disetujui', 'aktif', 'dijeda')
		ORDER BY id`, jenis, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return kumpulkanPromosi(rows)
}

func (r *PromosiRepository) List(ctx context.Context, f PromosiFilter) ([]model.Promosi, error) {
	where, args := klausaPromosi(f, "")
	if f.Limit <= 0 {
		f.Limit = 50
	}
	args = append(args, f.Limit, f.Offset)
	rows, err := r.db.Query(ctx, fmt.Sprintf(
		`SELECT %s FROM promosi%s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		promosiKolom, where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return kumpulkanPromosi(rows)
}

func (r *PromosiRepository) Count(ctx context.Context, f PromosiFilter) (int, error) {
	where, args := klausaPromosi(f, "")
	var n int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM promosi`+where, args...).Scan(&n)
	return n, err
}

// ListByUser mengembalikan seluruh pengajuan milik satu pengguna, terbaru dulu.
func (r *PromosiRepository) ListByUser(ctx context.Context, userID int64) ([]model.Promosi, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+promosiKolom+` FROM promosi WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return kumpulkanPromosi(rows)
}

// ListTayang mengembalikan promosi satu jenis yang sedang tayang.
func (r *PromosiRepository) ListTayang(ctx context.Context, jenis string) ([]model.Promosi, error) {
	rows, err := r.db.Query(ctx, `SELECT `+promosiKolom+` FROM promosi
		WHERE jenis = $1 AND status = 'aktif' AND selesai_at > NOW() ORDER BY id`, jenis)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return kumpulkanPromosi(rows)
}

// ListKedaluwarsa mengembalikan promosi aktif yang masa tayangnya sudah lewat
// tetapi belum ditandai selesai — dipakai ticker.
func (r *PromosiRepository) ListKedaluwarsa(ctx context.Context) ([]model.Promosi, error) {
	rows, err := r.db.Query(ctx, `SELECT `+promosiKolom+` FROM promosi
		WHERE status IN ('aktif', 'dijeda') AND selesai_at <= NOW() ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return kumpulkanPromosi(rows)
}

func kumpulkanPromosi(rows pgx.Rows) ([]model.Promosi, error) {
	var out []model.Promosi
	for rows.Next() {
		var p model.Promosi
		if err := rows.Scan(targetPromosi(&p)...); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// HitungHidupPengguna menghitung pengajuan satu jenis milik pengguna yang
// masih menempati kuota — dipakai membatasi jumlah iklan per akun.
func (r *PromosiRepository) HitungHidupPengguna(ctx context.Context, userID int64, jenis string) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM promosi
		WHERE user_id = $1 AND jenis = $2 AND status IN ('menunggu', 'disetujui', 'aktif', 'dijeda')`,
		userID, jenis).Scan(&n)
	return n, err
}

// ListIklanTayang mengembalikan iklan aktif beserta slot dan bobot paketnya.
func (r *PromosiRepository) ListIklanTayang(ctx context.Context) ([]model.IklanTayang, error) {
	rows, err := r.db.Query(ctx, `SELECT `+promosiAdminKolomTanpaGabungan+`, pk.slot_iklan, pk.bobot
		FROM promosi pr JOIN paket_promosi pk ON pk.id = pr.paket_id
		WHERE pr.jenis = 'iklan' AND pr.status = 'aktif' AND pr.selesai_at > NOW()
		ORDER BY pr.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.IklanTayang
	for rows.Next() {
		var ik model.IklanTayang
		target := append(targetPromosi(&ik.Promosi), &ik.Slot, &ik.Bobot)
		if err := rows.Scan(target...); err != nil {
			return nil, err
		}
		out = append(out, ik)
	}
	return out, rows.Err()
}

// promosiAdminKolomTanpaGabungan adalah kolom promosi beralias pr — tanpa
// kolom gabungan — untuk query yang menggabung tabel lain.
const promosiAdminKolomTanpaGabungan = `pr.id, pr.jenis, pr.user_id, pr.provider_id, pr.service_id, pr.paket_id, pr.status, pr.sumber,
	pr.judul, pr.deskripsi, pr.gambar_url, pr.tautan_url, pr.target_kecamatan,
	pr.alasan, pr.catatan_admin, pr.ditinjau_oleh, pr.ditinjau_at,
	pr.bukti_bayar_url, pr.dibayar_at, pr.dikonfirmasi_oleh,
	pr.mulai_at, pr.selesai_at, pr.tayang, pr.klik, pr.created_at, pr.updated_at`

// TambahTayang menambahkan hitungan tayang yang disalin dari Redis.
func (r *PromosiRepository) TambahTayang(ctx context.Context, id, n int64) error {
	_, err := r.db.Exec(ctx, `UPDATE promosi SET tayang = tayang + $2 WHERE id = $1`, id, n)
	return err
}

// TambahKlik mencatat satu klik dan mengembalikan tautan tujuannya.
func (r *PromosiRepository) TambahKlik(ctx context.Context, id int64) (string, error) {
	var tautan *string
	err := r.db.QueryRow(ctx, `UPDATE promosi SET klik = klik + 1
		WHERE id = $1 AND jenis = 'iklan' AND status = 'aktif' RETURNING tautan_url`, id).Scan(&tautan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	if tautan == nil {
		return "", ErrNotFound
	}
	return *tautan, nil
}

// StatistikPromosi adalah ringkasan untuk dasbor admin.
type StatistikPromosi struct {
	Menunggu       int `json:"menunggu"`
	MenungguBayar  int `json:"menunggu_bayar"`
	BuktiMasuk     int `json:"bukti_masuk"`
	TayangIklan    int `json:"tayang_iklan"`
	TayangSorotan  int `json:"tayang_sorotan"`
	TayangPenyedia int `json:"tayang_penyedia"`
	// Tayang & klik seluruh iklan yang aktif saat ini.
	TotalTayang int64 `json:"total_tayang"`
	TotalKlik   int64 `json:"total_klik"`
	// Selesai30Hari: promosi yang berakhir dalam 30 hari terakhir — ukuran
	// seberapa hidup sistemnya, bukan hanya berapa yang sedang menunggu.
	Selesai30Hari int `json:"selesai_30_hari"`
}

func (r *PromosiRepository) Statistik(ctx context.Context) (*StatistikPromosi, error) {
	var st StatistikPromosi
	err := r.db.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'menunggu'),
			COUNT(*) FILTER (WHERE status = 'disetujui'),
			COUNT(*) FILTER (WHERE status = 'disetujui' AND bukti_bayar_url IS NOT NULL),
			COUNT(*) FILTER (WHERE status = 'aktif' AND jenis = 'iklan'),
			COUNT(*) FILTER (WHERE status = 'aktif' AND jenis = 'sorotan_jasa'),
			COUNT(*) FILTER (WHERE status = 'aktif' AND jenis = 'penyedia_pilihan'),
			COALESCE(SUM(tayang) FILTER (WHERE status = 'aktif' AND jenis = 'iklan'), 0),
			COALESCE(SUM(klik)   FILTER (WHERE status = 'aktif' AND jenis = 'iklan'), 0),
			COUNT(*) FILTER (WHERE status = 'selesai' AND selesai_at > NOW() - INTERVAL '30 days')
		FROM promosi`).Scan(&st.Menunggu, &st.MenungguBayar, &st.BuktiMasuk,
		&st.TayangIklan, &st.TayangSorotan, &st.TayangPenyedia,
		&st.TotalTayang, &st.TotalKlik, &st.Selesai30Hari)
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// ListAkanSelesai mengembalikan promosi aktif yang berakhir dalam rentang
// waktu tertentu dan belum pernah diingatkan.
func (r *PromosiRepository) ListAkanSelesai(ctx context.Context, dalam time.Duration) ([]model.Promosi, error) {
	rows, err := r.db.Query(ctx, `SELECT `+promosiKolom+` FROM promosi
		WHERE status = 'aktif' AND diingatkan_at IS NULL
		  AND selesai_at > NOW() AND selesai_at <= NOW() + $1::interval
		ORDER BY selesai_at`, dalam)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return kumpulkanPromosi(rows)
}

// TandaiDiingatkan menandai satu promosi dan mengembalikan true hanya bagi
// pemanggil yang benar-benar menandainya. Dua instance yang berlomba tidak
// bisa sama-sama mendapat true, jadi pengingatnya tidak pernah ganda.
func (r *PromosiRepository) TandaiDiingatkan(ctx context.Context, id int64) (bool, error) {
	tag, err := r.db.Exec(ctx, `UPDATE promosi SET diingatkan_at = NOW()
		WHERE id = $1 AND diingatkan_at IS NULL`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// PemilikBukti mengembalikan user_id pemilik bukti bayar dengan URL tertentu,
// dipakai penjaga akses berkas bukti.
func (r *PromosiRepository) PemilikBukti(ctx context.Context, url string) (int64, error) {
	var userID int64
	err := r.db.QueryRow(ctx, `SELECT user_id FROM promosi WHERE bukti_bayar_url = $1`, url).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	return userID, nil
}
