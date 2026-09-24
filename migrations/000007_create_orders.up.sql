CREATE TABLE orders (
    id             BIGSERIAL PRIMARY KEY,
    seeker_id      BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider_id    BIGINT      NOT NULL REFERENCES provider_profiles (id) ON DELETE CASCADE,
    service_id     BIGINT      NOT NULL REFERENCES services (id) ON DELETE RESTRICT,
    status         VARCHAR(12) NOT NULL DEFAULT 'pending',
    scheduled_date DATE,
    notes          TEXT,
    agreed_price   NUMERIC(12, 2),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT orders_status_check CHECK (
        status IN ('pending', 'accepted', 'rejected', 'completed', 'cancelled')
    )
);

CREATE INDEX orders_seeker_idx ON orders (seeker_id, created_at DESC);
CREATE INDEX orders_provider_idx ON orders (provider_id, created_at DESC);
CREATE INDEX orders_service_idx ON orders (service_id);
CREATE INDEX orders_status_idx ON orders (status);
