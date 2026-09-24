-- Area layanan tidak lagi disimpan sebagai teks bebas. Ia diturunkan dari
-- pin lokasi dan radius layanan (lihat model.ProviderProfile.AreaLayanan),
-- sehingga tidak bisa lagi menyimpang dari keduanya.
ALTER TABLE provider_profiles DROP COLUMN service_area;
