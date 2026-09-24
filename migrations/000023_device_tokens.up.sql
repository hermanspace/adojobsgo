-- Token FCM per perangkat. Satu token satu baris; pindah akun di perangkat
-- yang sama menimpa pemiliknya (ON CONFLICT), sehingga notifikasi tidak
-- pernah sampai ke orang yang sudah keluar.
CREATE TABLE device_tokens (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token        TEXT        NOT NULL UNIQUE,
    platform     VARCHAR(16) NOT NULL DEFAULT 'android',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX device_tokens_user_idx ON device_tokens (user_id);
