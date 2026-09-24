-- Pengingat H-1: pemohon diberi tahu sehari sebelum masa tayang habis supaya
-- sempat mengajukan lagi. Ditandai per baris agar pengingatnya terkirim
-- tepat satu kali walau ticker berjalan berulang.
ALTER TABLE promosi ADD COLUMN diingatkan_at TIMESTAMPTZ;
CREATE INDEX promosi_pengingat_idx ON promosi (selesai_at)
    WHERE status = 'aktif' AND diingatkan_at IS NULL;
