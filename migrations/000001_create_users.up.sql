-- Ekstensi pg_trgm dipakai untuk pencarian judul jasa (ILIKE / similarity) di halaman search.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    full_name     VARCHAR(120)  NOT NULL,
    phone         VARCHAR(20)   NOT NULL,
    email         VARCHAR(160),
    password_hash VARCHAR(255)  NOT NULL,
    avatar_url    TEXT,
    city          VARCHAR(80),
    kecamatan     VARCHAR(80),
    is_provider   BOOLEAN       NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- Nomor HP adalah identitas login utama, wajib unik.
CREATE UNIQUE INDEX users_phone_key ON users (phone);
-- Email opsional; unik hanya jika diisi.
CREATE UNIQUE INDEX users_email_key ON users (LOWER(email)) WHERE email IS NOT NULL;
CREATE INDEX users_is_provider_idx ON users (is_provider) WHERE is_provider = TRUE;
