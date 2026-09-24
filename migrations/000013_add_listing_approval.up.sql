-- Alur persetujuan listing.
-- Listing baru masuk antrean dan belum tayang sampai disetujui admin.
ALTER TABLE services DROP CONSTRAINT IF EXISTS services_status_check;

ALTER TABLE services
    ADD COLUMN approved_at      TIMESTAMPTZ,
    ADD COLUMN approved_by      BIGINT REFERENCES users (id) ON DELETE SET NULL,
    ADD COLUMN rejection_reason TEXT,
    -- Waktu listing terakhir masuk antrean; dipakai mengurutkan antrean admin.
    ADD COLUMN submitted_at     TIMESTAMPTZ NOT NULL DEFAULT NOW();

ALTER TABLE services
    ADD CONSTRAINT services_status_check CHECK (
        status IN ('pending', 'active', 'inactive', 'rejected')
    ),
    -- Listing yang ditolak wajib punya alasan yang bisa dibaca provider.
    ADD CONSTRAINT services_rejection_reason_check CHECK (
        status <> 'rejected' OR (rejection_reason IS NOT NULL AND rejection_reason <> '')
    );

-- Listing yang sudah tayang sebelum fitur ini ada dianggap sudah disetujui,
-- supaya tidak mendadak hilang dari pencarian saat migrasi dijalankan.
UPDATE services SET approved_at = created_at, submitted_at = created_at
 WHERE status = 'active';

ALTER TABLE services ALTER COLUMN status SET DEFAULT 'pending';

CREATE INDEX services_antrean_idx ON services (submitted_at)
    WHERE status = 'pending';
