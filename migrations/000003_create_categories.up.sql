CREATE TABLE categories (
    id        BIGSERIAL PRIMARY KEY,
    name      VARCHAR(80)  NOT NULL,
    slug      VARCHAR(100) NOT NULL,
    parent_id BIGINT       REFERENCES categories (id) ON DELETE SET NULL,
    icon      VARCHAR(60),
    -- Kategori tidak boleh menjadi induk bagi dirinya sendiri. Tanpa batasan ini
    -- satu baris yang mereferensi dirinya membuat penelusuran rekursif
    -- sub-kategori berputar tanpa henti.
    CONSTRAINT categories_no_self_parent CHECK (parent_id IS NULL OR parent_id <> id)
);

CREATE UNIQUE INDEX categories_slug_key ON categories (slug);
CREATE INDEX categories_parent_id_idx ON categories (parent_id);
