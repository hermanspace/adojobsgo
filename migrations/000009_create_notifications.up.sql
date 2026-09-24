CREATE TABLE notifications (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    type       VARCHAR(60) NOT NULL,
    payload    JSONB       NOT NULL DEFAULT '{}'::JSONB,
    read_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX notifications_user_created_idx ON notifications (user_id, created_at DESC);
-- Badge "belum dibaca" di navigasi hanya menghitung baris dengan read_at NULL.
CREATE INDEX notifications_unread_idx ON notifications (user_id) WHERE read_at IS NULL;
