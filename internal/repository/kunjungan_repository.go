package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hermansyah/adojobsid/internal/model"
)

// KunjunganRepository menyimpan penghitung kunjungan harian per jasa.
// Ditulis hanya oleh penyalinan berkala dari Redis, bukan per kunjungan.
type KunjunganRepository struct{ db *pgxpool.Pool }

// Tambah menambahkan jumlah kunjungan hari itu dan memperbarui perkiraan
// pengunjung unik (nilai absolut, diambil yang terbesar).
func (r *KunjunganRepository) Tambah(ctx context.Context, serviceID int64, tanggal time.Time, jumlah, unik int64) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO kunjungan_jasa (service_id, tanggal, jumlah, unik)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (service_id, tanggal) DO UPDATE
		   SET jumlah = kunjungan_jasa.jumlah + EXCLUDED.jumlah,
		       unik   = GREATEST(kunjungan_jasa.unik, EXCLUDED.unik)`,
		serviceID, tanggal, jumlah, unik)
	return err
}

// Statistik menyusun ringkasan 30 hari terakhir: deret harian (termasuk hari
// tanpa kunjungan, supaya grafik tidak berlubang), total 7 & 30 hari, dan
// total 7 hari sebelumnya sebagai pembanding.
func (r *KunjunganRepository) Statistik(ctx context.Context, serviceID int64, kini time.Time) (*model.StatistikKunjungan, error) {
	mulai := kini.AddDate(0, 0, -29)
	rows, err := r.db.Query(ctx, `
		SELECT tanggal, jumlah, unik
		  FROM kunjungan_jasa
		 WHERE service_id = $1 AND tanggal >= $2
		 ORDER BY tanggal`, serviceID, mulai)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	perHari := map[string]model.KunjunganHarian{}
	for rows.Next() {
		var h model.KunjunganHarian
		if err := rows.Scan(&h.Tanggal, &h.Jumlah, &h.Unik); err != nil {
			return nil, err
		}
		perHari[h.Tanggal.Format("2006-01-02")] = h
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	st := &model.StatistikKunjungan{Harian: make([]model.KunjunganHarian, 0, 30)}
	for i := 0; i < 30; i++ {
		hari := mulai.AddDate(0, 0, i)
		h, ada := perHari[hari.Format("2006-01-02")]
		if !ada {
			h = model.KunjunganHarian{Tanggal: hari}
		}
		st.Harian = append(st.Harian, h)
		st.Hari30 += h.Jumlah
		st.Unik30 += h.Unik
		switch {
		case i >= 23:
			st.Hari7 += h.Jumlah
		case i >= 16:
			st.MingguLalu += h.Jumlah
		}
	}
	err = r.db.QueryRow(ctx, `SELECT total_kunjungan FROM services WHERE id = $1`, serviceID).Scan(&st.Total)
	return st, err
}
