-- Sistem promosi: iklan, sorotan jasa, dan penyedia pilihan dalam SATU
-- mekanisme. Ketiganya berbeda hanya pada apa yang ditampilkan dan di mana;
-- alur pengajuan, peninjauan, pembayaran, dan masa tayangnya identik. Satu
-- tabel dengan kolom jenis berarti satu antrean admin dan satu mesin status,
-- bukan tiga subsistem yang saling tumpang tindih.

CREATE TABLE paket_promosi (
    id          BIGSERIAL PRIMARY KEY,
    jenis       VARCHAR(20) NOT NULL
                CHECK (jenis IN ('iklan', 'sorotan_jasa', 'penyedia_pilihan')),
    nama        VARCHAR(80) NOT NULL,
    deskripsi   TEXT        NOT NULL DEFAULT '',
    durasi_hari INT         NOT NULL CHECK (durasi_hari BETWEEN 1 AND 365),
    -- Harga dalam rupiah bulat. Nol berarti gratis.
    harga       BIGINT      NOT NULL DEFAULT 0 CHECK (harga >= 0),
    -- Bobot menentukan peluang terpilih saat rotasi iklan; paket bobot 3
    -- tampil tiga kali lebih sering daripada bobot 1 di slot yang sama.
    bobot       SMALLINT    NOT NULL DEFAULT 1 CHECK (bobot BETWEEN 1 AND 10),
    -- Slot penempatan; hanya berarti untuk jenis iklan.
    slot_iklan  TEXT[]      NOT NULL DEFAULT '{}',
    aktif       BOOLEAN     NOT NULL DEFAULT TRUE,
    urutan      INT         NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Nama paket unik per jenis (tanpa peduli huruf besar) — dipakai seeder
-- untuk upsert dan mencegah admin membuat dua "30 hari" yang membingungkan.
CREATE UNIQUE INDEX paket_promosi_jenis_nama_key ON paket_promosi (jenis, LOWER(nama));

CREATE TABLE promosi (
    id            BIGSERIAL PRIMARY KEY,
    jenis         VARCHAR(20) NOT NULL
                  CHECK (jenis IN ('iklan', 'sorotan_jasa', 'penyedia_pilihan')),
    user_id       BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider_id   BIGINT      REFERENCES provider_profiles (id) ON DELETE CASCADE,
    service_id    BIGINT      REFERENCES services (id) ON DELETE CASCADE,
    -- Paket boleh kosong hanya untuk promosi yang ditetapkan admin langsung.
    paket_id      BIGINT      REFERENCES paket_promosi (id) ON DELETE RESTRICT,
    status        VARCHAR(20) NOT NULL DEFAULT 'menunggu'
                  CHECK (status IN ('menunggu', 'disetujui', 'aktif', 'dijeda',
                                    'selesai', 'ditolak', 'dibatalkan', 'dihentikan')),
    sumber        VARCHAR(10) NOT NULL DEFAULT 'pengguna'
                  CHECK (sumber IN ('pengguna', 'admin')),

    -- Materi iklan. Terkunci begitu disetujui: mengubah materi setelah tayang
    -- berarti pengajuan baru, sama seperti persetujuan listing.
    judul            VARCHAR(120),
    deskripsi        TEXT,
    gambar_url       TEXT,
    tautan_url       TEXT,
    -- Kecamatan sasaran; kosong berarti seluruh kabupaten.
    target_kecamatan TEXT[] NOT NULL DEFAULT '{}',

    -- Pengajuan & peninjauan.
    alasan        TEXT,
    catatan_admin TEXT,
    ditinjau_oleh BIGINT      REFERENCES users (id) ON DELETE SET NULL,
    ditinjau_at   TIMESTAMPTZ,

    -- Pembayaran manual: pemohon mengunggah bukti, admin mengonfirmasi.
    bukti_bayar_url   TEXT,
    dibayar_at        TIMESTAMPTZ,
    dikonfirmasi_oleh BIGINT REFERENCES users (id) ON DELETE SET NULL,

    -- Masa tayang dihitung sejak aktif, bukan sejak diajukan.
    mulai_at   TIMESTAMPTZ,
    selesai_at TIMESTAMPTZ,
    tayang     BIGINT NOT NULL DEFAULT 0,
    klik       BIGINT NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT promosi_sorotan_butuh_jasa
        CHECK (jenis <> 'sorotan_jasa' OR service_id IS NOT NULL),
    CONSTRAINT promosi_pilihan_butuh_penyedia
        CHECK (jenis <> 'penyedia_pilihan' OR provider_id IS NOT NULL),
    CONSTRAINT promosi_iklan_butuh_judul
        CHECK (jenis <> 'iklan' OR (judul IS NOT NULL AND judul <> '')),
    CONSTRAINT promosi_aktif_butuh_jadwal
        CHECK (status NOT IN ('aktif', 'dijeda', 'selesai') OR (mulai_at IS NOT NULL AND selesai_at IS NOT NULL)),
    -- Penolakan wajib beralasan; alasannya tampil ke pemohon.
    CONSTRAINT promosi_tolak_butuh_catatan
        CHECK (status <> 'ditolak' OR (catatan_admin IS NOT NULL AND catatan_admin <> ''))
);

CREATE INDEX promosi_status_jenis_idx  ON promosi (status, jenis);
CREATE INDEX promosi_user_idx          ON promosi (user_id, created_at DESC);
CREATE INDEX promosi_aktif_selesai_idx ON promosi (selesai_at) WHERE status = 'aktif';

-- Satu pengajuan yang masih hidup per listing dan per penyedia. Dijaga di
-- database, bukan di kode: dua permintaan bersamaan pun tidak bisa menggandakan.
CREATE UNIQUE INDEX promosi_sorotan_hidup_key ON promosi (service_id)
    WHERE jenis = 'sorotan_jasa' AND status IN ('menunggu', 'disetujui', 'aktif', 'dijeda');
CREATE UNIQUE INDEX promosi_pilihan_hidup_key ON promosi (provider_id)
    WHERE jenis = 'penyedia_pilihan' AND status IN ('menunggu', 'disetujui', 'aktif', 'dijeda');

-- featured_until di listing dan penyedia kini TURUNAN dari promosi yang aktif,
-- dijaga trigger — satu penulis, seperti users.is_provider. Nilainya dihitung
-- ulang dari seluruh baris aktif yang tersisa, sehingga jeda, henti, selesai,
-- dan hapus semuanya ditangani satu jalur yang sama.
CREATE OR REPLACE FUNCTION sinkron_featured_promosi() RETURNS trigger AS $$
DECLARE
    r promosi%ROWTYPE;
BEGIN
    IF TG_OP = 'DELETE' THEN r := OLD; ELSE r := NEW; END IF;

    IF r.jenis = 'sorotan_jasa' AND r.service_id IS NOT NULL THEN
        UPDATE services s
           SET featured_until = (SELECT MAX(p.selesai_at) FROM promosi p
                                  WHERE p.service_id = r.service_id
                                    AND p.jenis = 'sorotan_jasa' AND p.status = 'aktif')
         WHERE s.id = r.service_id;
    ELSIF r.jenis = 'penyedia_pilihan' AND r.provider_id IS NOT NULL THEN
        UPDATE provider_profiles pp
           SET featured_until = (SELECT MAX(p.selesai_at) FROM promosi p
                                  WHERE p.provider_id = r.provider_id
                                    AND p.jenis = 'penyedia_pilihan' AND p.status = 'aktif')
         WHERE pp.id = r.provider_id;
    END IF;
    RETURN NULL;
END
$$ LANGUAGE plpgsql;

CREATE TRIGGER promosi_sinkron_featured
AFTER INSERT OR UPDATE OR DELETE ON promosi
FOR EACH ROW EXECUTE FUNCTION sinkron_featured_promosi();

-- Sorotan yang sudah ditetapkan admin sebelum sistem ini ada dipindahkan
-- menjadi promosi bersumber admin, supaya riwayatnya utuh sejak hari pertama.
INSERT INTO promosi (jenis, user_id, provider_id, service_id, status, sumber, mulai_at, selesai_at)
SELECT 'sorotan_jasa', p.user_id, s.provider_id, s.id, 'aktif', 'admin', NOW(), s.featured_until
  FROM services s JOIN provider_profiles p ON p.id = s.provider_id
 WHERE s.featured_until IS NOT NULL AND s.featured_until > NOW();

INSERT INTO promosi (jenis, user_id, provider_id, status, sumber, mulai_at, selesai_at)
SELECT 'penyedia_pilihan', p.user_id, p.id, 'aktif', 'admin', NOW(), p.featured_until
  FROM provider_profiles p
 WHERE p.featured_until IS NOT NULL AND p.featured_until > NOW();
