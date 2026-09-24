CREATE TABLE reviews (
    id         BIGSERIAL PRIMARY KEY,
    order_id   BIGINT      NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    rating     SMALLINT    NOT NULL,
    comment    TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT reviews_rating_range CHECK (rating BETWEEN 1 AND 5)
);

-- Satu order hanya boleh menghasilkan satu ulasan.
CREATE UNIQUE INDEX reviews_order_id_key ON reviews (order_id);
