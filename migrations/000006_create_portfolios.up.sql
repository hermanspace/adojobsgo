CREATE TABLE portfolios (
    id           BIGSERIAL PRIMARY KEY,
    provider_id  BIGINT       NOT NULL REFERENCES provider_profiles (id) ON DELETE CASCADE,
    title        VARCHAR(140) NOT NULL,
    image_url    TEXT         NOT NULL,
    completed_at DATE
);

CREATE INDEX portfolios_provider_completed_idx ON portfolios (provider_id, completed_at DESC NULLS LAST);
