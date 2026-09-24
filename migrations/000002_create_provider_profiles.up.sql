CREATE TABLE provider_profiles (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    bio             TEXT,
    whatsapp_number VARCHAR(20)   NOT NULL,
    service_area    VARCHAR(160),
    is_verified     BOOLEAN       NOT NULL DEFAULT FALSE,
    -- avg_rating & total_reviews sengaja di-cache di sini (bukan agregasi on-the-fly)
    -- supaya listing pencarian tidak perlu JOIN ke reviews setiap kali dirender.
    avg_rating      NUMERIC(3, 2) NOT NULL DEFAULT 0.00,
    total_reviews   INTEGER       NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    CONSTRAINT provider_profiles_avg_rating_range CHECK (avg_rating >= 0 AND avg_rating <= 5),
    CONSTRAINT provider_profiles_total_reviews_positive CHECK (total_reviews >= 0)
);

CREATE UNIQUE INDEX provider_profiles_user_id_key ON provider_profiles (user_id);
CREATE INDEX provider_profiles_rating_idx ON provider_profiles (avg_rating DESC, total_reviews DESC);
