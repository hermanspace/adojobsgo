-- Varian gambar hasil pemrosesan unggahan.
-- thumb_url dipakai kartu listing di halaman pencarian, yang memuat belasan
-- gambar sekaligus; image_url yang lebih besar hanya dimuat di halaman detail.
ALTER TABLE service_images
    ADD COLUMN thumb_url TEXT,
    ADD COLUMN width     INTEGER,
    ADD COLUMN height    INTEGER,
    ADD COLUMN bytes     INTEGER;
