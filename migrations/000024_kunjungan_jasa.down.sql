DROP TRIGGER IF EXISTS trg_kunjungan_total ON kunjungan_jasa;
DROP FUNCTION IF EXISTS sinkron_total_kunjungan();
DROP TABLE IF EXISTS kunjungan_jasa;
ALTER TABLE services DROP COLUMN IF EXISTS total_kunjungan;
