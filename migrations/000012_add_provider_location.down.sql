DROP INDEX IF EXISTS provider_profiles_earth_idx;

ALTER TABLE provider_profiles
    DROP CONSTRAINT IF EXISTS provider_profiles_radius_range,
    DROP CONSTRAINT IF EXISTS provider_profiles_longitude_range,
    DROP CONSTRAINT IF EXISTS provider_profiles_latitude_range,
    DROP CONSTRAINT IF EXISTS provider_profiles_koordinat_lengkap,
    DROP COLUMN IF EXISTS service_radius_km,
    DROP COLUMN IF EXISTS address_label,
    DROP COLUMN IF EXISTS longitude,
    DROP COLUMN IF EXISTS latitude;
