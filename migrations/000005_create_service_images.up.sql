CREATE TABLE service_images (
    id         BIGSERIAL PRIMARY KEY,
    service_id BIGINT   NOT NULL REFERENCES services (id) ON DELETE CASCADE,
    image_url  TEXT     NOT NULL,
    sort_order SMALLINT NOT NULL DEFAULT 0
);

-- Kartu listing selalu mengambil foto dengan sort_order terkecil sebagai cover.
CREATE INDEX service_images_service_sort_idx ON service_images (service_id, sort_order);
