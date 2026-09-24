-- Pencarian berbasis jarak.
-- cube dan earthdistance sudah tersedia di image postgres:16-alpine, sehingga
-- tidak perlu mengganti image ke PostGIS hanya untuk kebutuhan radius ini.
CREATE EXTENSION IF NOT EXISTS cube;
CREATE EXTENSION IF NOT EXISTS earthdistance;

ALTER TABLE provider_profiles
    ADD COLUMN latitude          DOUBLE PRECISION,
    ADD COLUMN longitude         DOUBLE PRECISION,
    -- Alamat dalam bentuk teks untuk ditampilkan; titik koordinatnya yang dipakai menghitung jarak.
    ADD COLUMN address_label     VARCHAR(200),
    -- Sejauh mana penyedia bersedia mendatangi pelanggan.
    ADD COLUMN service_radius_km SMALLINT NOT NULL DEFAULT 15,
    -- Lintang dan bujur harus terisi bersama; setengah koordinat tidak berarti apa-apa.
    ADD CONSTRAINT provider_profiles_koordinat_lengkap CHECK (
        (latitude IS NULL) = (longitude IS NULL)
    ),
    ADD CONSTRAINT provider_profiles_latitude_range CHECK (
        latitude IS NULL OR (latitude BETWEEN -90 AND 90)
    ),
    ADD CONSTRAINT provider_profiles_longitude_range CHECK (
        longitude IS NULL OR (longitude BETWEEN -180 AND 180)
    ),
    ADD CONSTRAINT provider_profiles_radius_range CHECK (
        service_radius_km BETWEEN 1 AND 200
    );

-- Indeks GiST atas titik bumi: membuat penyaringan radius memakai indeks,
-- bukan memindai seluruh tabel lalu menghitung jarak satu per satu.
CREATE INDEX provider_profiles_earth_idx
    ON provider_profiles USING GIST (ll_to_earth(latitude, longitude))
    WHERE latitude IS NOT NULL;
