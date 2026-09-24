CREATE TABLE services (
    id             BIGSERIAL PRIMARY KEY,
    provider_id    BIGINT       NOT NULL REFERENCES provider_profiles (id) ON DELETE CASCADE,
    category_id    BIGINT       NOT NULL REFERENCES categories (id) ON DELETE RESTRICT,
    title          VARCHAR(140) NOT NULL,
    description    TEXT         NOT NULL,
    price_type     VARCHAR(12)  NOT NULL,
    price_min      NUMERIC(12, 2),
    price_max      NUMERIC(12, 2),
    status         VARCHAR(10)  NOT NULL DEFAULT 'active',
    -- Disiapkan untuk fitur listing berbayar; di MVP kolom ini selalu NULL.
    featured_until TIMESTAMPTZ,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT services_price_type_check CHECK (price_type IN ('fixed', 'hourly', 'negotiable')),
    CONSTRAINT services_status_check CHECK (status IN ('active', 'inactive')),
    CONSTRAINT services_price_range_check CHECK (
        price_min IS NULL OR price_max IS NULL OR price_max >= price_min
    ),
    CONSTRAINT services_price_required_check CHECK (
        price_type = 'negotiable' OR price_min IS NOT NULL
    )
);

CREATE INDEX services_provider_id_idx ON services (provider_id);
CREATE INDEX services_category_id_idx ON services (category_id);
-- Index utama halaman pencarian: hanya listing aktif, diurutkan terbaru.
CREATE INDEX services_active_created_idx ON services (created_at DESC) WHERE status = 'active';
CREATE INDEX services_featured_idx ON services (featured_until DESC NULLS LAST) WHERE status = 'active';
CREATE INDEX services_title_trgm_idx ON services USING GIN (title gin_trgm_ops);
