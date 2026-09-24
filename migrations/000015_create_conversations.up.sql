-- Percakapan antara pencari jasa dan penyedia.
-- order_id boleh kosong: percakapan lazimnya dimulai sebagai pertanyaan
-- sebelum pesanan dibuat, lalu ditautkan ke pesanan begitu order terbentuk.
CREATE TABLE conversations (
    id              BIGSERIAL PRIMARY KEY,
    service_id      BIGINT      NOT NULL REFERENCES services (id) ON DELETE CASCADE,
    seeker_id       BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider_id     BIGINT      NOT NULL REFERENCES provider_profiles (id) ON DELETE CASCADE,
    order_id        BIGINT      REFERENCES orders (id) ON DELETE SET NULL,
    last_message_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Satu pencari jasa hanya punya satu percakapan per listing, sehingga
-- riwayatnya tidak terpecah menjadi banyak ruang obrolan.
CREATE UNIQUE INDEX conversations_service_seeker_key ON conversations (service_id, seeker_id);
CREATE INDEX conversations_seeker_idx ON conversations (seeker_id, last_message_at DESC NULLS LAST);
CREATE INDEX conversations_provider_idx ON conversations (provider_id, last_message_at DESC NULLS LAST);

CREATE TABLE messages (
    id              BIGSERIAL PRIMARY KEY,
    conversation_id BIGINT      NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    sender_id       BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    body            TEXT        NOT NULL,
    read_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT messages_body_tidak_kosong CHECK (body <> '')
);

CREATE INDEX messages_conversation_idx ON messages (conversation_id, created_at);
-- Penghitung pesan belum dibaca hanya menyentuh baris yang read_at-nya kosong.
CREATE INDEX messages_belum_dibaca_idx ON messages (conversation_id, sender_id)
    WHERE read_at IS NULL;
