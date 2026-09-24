DROP INDEX IF EXISTS portfolios_provider_urut_idx;

ALTER TABLE portfolios
    DROP COLUMN IF EXISTS created_at,
    DROP COLUMN IF EXISTS bytes,
    DROP COLUMN IF EXISTS height,
    DROP COLUMN IF EXISTS width,
    DROP COLUMN IF EXISTS thumb_url;

DROP INDEX IF EXISTS provider_profiles_slug_key;
ALTER TABLE provider_profiles DROP COLUMN IF EXISTS slug;
