-- Alamat publik penyedia memakai slug agar mudah dibagikan lewat WhatsApp
-- dan terbaca mesin pencari, bukan sekadar nomor id.
ALTER TABLE provider_profiles ADD COLUMN slug VARCHAR(160);

-- Isi slug untuk penyedia yang sudah ada, diturunkan dari nama penggunanya.
-- Nama kembar dibedakan dengan menambahkan id di belakangnya.
WITH dasar AS (
    SELECT p.id,
           COALESCE(
               NULLIF(trim(BOTH '-' FROM lower(regexp_replace(u.full_name, '[^a-zA-Z0-9]+', '-', 'g'))), ''),
               'penyedia'
           ) AS pangkal
      FROM provider_profiles p
      JOIN users u ON u.id = p.user_id
), bernomor AS (
    SELECT id, pangkal,
           row_number() OVER (PARTITION BY pangkal ORDER BY id) AS urutan
      FROM dasar
)
UPDATE provider_profiles p
   SET slug = CASE WHEN b.urutan = 1 THEN b.pangkal
                   ELSE b.pangkal || '-' || p.id END
  FROM bernomor b
 WHERE b.id = p.id;

ALTER TABLE provider_profiles ALTER COLUMN slug SET NOT NULL;
CREATE UNIQUE INDEX provider_profiles_slug_key ON provider_profiles (slug);

-- Varian gambar untuk portofolio, mengikuti pola yang sama dengan foto listing.
ALTER TABLE portfolios
    ADD COLUMN thumb_url TEXT,
    ADD COLUMN width     INTEGER,
    ADD COLUMN height    INTEGER,
    ADD COLUMN bytes     INTEGER,
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE INDEX portfolios_provider_urut_idx ON portfolios (provider_id, created_at DESC);
