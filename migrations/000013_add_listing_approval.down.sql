DROP INDEX IF EXISTS services_antrean_idx;

-- Listing yang masih menunggu atau ditolak dikembalikan menjadi nonaktif,
-- karena kedua status itu tidak ada lagi setelah rollback.
UPDATE services SET status = 'inactive' WHERE status IN ('pending', 'rejected');

ALTER TABLE services
    DROP CONSTRAINT IF EXISTS services_rejection_reason_check,
    DROP CONSTRAINT IF EXISTS services_status_check,
    DROP COLUMN IF EXISTS submitted_at,
    DROP COLUMN IF EXISTS rejection_reason,
    DROP COLUMN IF EXISTS approved_by,
    DROP COLUMN IF EXISTS approved_at;

ALTER TABLE services ALTER COLUMN status SET DEFAULT 'active';
ALTER TABLE services
    ADD CONSTRAINT services_status_check CHECK (status IN ('active', 'inactive'));
