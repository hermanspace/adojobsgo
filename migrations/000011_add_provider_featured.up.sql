-- Penyedia pilihan. Sejajar dengan services.featured_until: admin menyetelnya
-- secara manual di MVP ini, belum terkait pembayaran apa pun.
ALTER TABLE provider_profiles
    ADD COLUMN featured_until TIMESTAMPTZ;

-- Hanya penyedia yang masa pilihannya masih berlaku yang perlu diindeks.
CREATE INDEX provider_profiles_featured_idx
    ON provider_profiles (featured_until DESC)
    WHERE featured_until IS NOT NULL;
