-- Peran dan status akun.
-- Admin adalah user biasa dengan role 'admin', sehingga seluruh alur sesi,
-- cookie, dan middleware yang sudah ada dipakai ulang tanpa jalur auth kedua.
ALTER TABLE users
    ADD COLUMN role VARCHAR(10) NOT NULL DEFAULT 'user',
    -- Penangguhan bersifat reversible: cukup dikosongkan untuk mengaktifkan
    -- kembali, dan waktunya tetap tercatat untuk keperluan audit.
    ADD COLUMN suspended_at     TIMESTAMPTZ,
    ADD COLUMN suspended_reason TEXT,
    ADD CONSTRAINT users_role_check CHECK (role IN ('user', 'admin')),
    -- Alasan wajib diisi saat menangguhkan, supaya tidak ada penangguhan
    -- tanpa keterangan yang bisa dipertanggungjawabkan.
    ADD CONSTRAINT users_suspended_reason_check CHECK (
        suspended_at IS NULL OR (suspended_reason IS NOT NULL AND suspended_reason <> '')
    );

CREATE INDEX users_role_idx ON users (role) WHERE role <> 'user';
CREATE INDEX users_suspended_idx ON users (suspended_at) WHERE suspended_at IS NOT NULL;
