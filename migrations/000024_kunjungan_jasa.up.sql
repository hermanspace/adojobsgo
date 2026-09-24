-- Statistik kunjungan jasa.
-- Penghitung harian per jasa; kolom total di services diturunkan lewat
-- trigger (satu penulis), sehingga kartu dan pengurutan tidak perlu JOIN.
ALTER TABLE services ADD COLUMN total_kunjungan INTEGER NOT NULL DEFAULT 0;

CREATE TABLE kunjungan_jasa (
    service_id BIGINT  NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    tanggal    DATE    NOT NULL,
    jumlah     INTEGER NOT NULL DEFAULT 0,
    -- Perkiraan pengunjung unik hari itu (HyperLogLog di Redis), nilai
    -- absolut yang diperbarui tiap penyalinan — bukan penjumlahan.
    unik       INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (service_id, tanggal)
);

CREATE OR REPLACE FUNCTION sinkron_total_kunjungan() RETURNS trigger AS $$
BEGIN
    UPDATE services
       SET total_kunjungan = total_kunjungan + (NEW.jumlah - COALESCE(OLD.jumlah, 0))
     WHERE id = NEW.service_id;
    RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_kunjungan_total
AFTER INSERT OR UPDATE OF jumlah ON kunjungan_jasa
FOR EACH ROW EXECUTE FUNCTION sinkron_total_kunjungan();
