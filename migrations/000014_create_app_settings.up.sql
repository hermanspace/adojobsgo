-- Pengaturan aplikasi yang bisa diubah admin tanpa deploy ulang.
-- Nilainya JSONB agar satu tabel melayani teks, angka, maupun struktur
-- bersarang seperti daftar slot iklan.
CREATE TABLE app_settings (
    key        VARCHAR(60) PRIMARY KEY,
    value      JSONB       NOT NULL DEFAULT '{}'::JSONB,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by BIGINT      REFERENCES users (id) ON DELETE SET NULL
);
