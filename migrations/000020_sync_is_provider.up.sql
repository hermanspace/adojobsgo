-- users.is_provider adalah denormalisasi dari keberadaan baris
-- provider_profiles. Dua sumber kebenaran tanpa penjaga akan menyimpang;
-- trigger ini menjadikan database satu-satunya penulisnya, sehingga kode
-- aplikasi tidak perlu — dan tidak boleh — menyentuh kolom itu lagi.
CREATE OR REPLACE FUNCTION sinkron_is_provider() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE users SET is_provider = TRUE WHERE id = NEW.user_id;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE users SET is_provider = FALSE WHERE id = OLD.user_id;
    END IF;
    RETURN NULL;
END
$$ LANGUAGE plpgsql;

CREATE TRIGGER provider_profiles_sinkron_is_provider
AFTER INSERT OR DELETE ON provider_profiles
FOR EACH ROW EXECUTE FUNCTION sinkron_is_provider();

-- Luruskan data yang mungkin sudah menyimpang sebelum trigger ada.
UPDATE users u
   SET is_provider = EXISTS (SELECT 1 FROM provider_profiles p WHERE p.user_id = u.id)
 WHERE is_provider <> EXISTS (SELECT 1 FROM provider_profiles p WHERE p.user_id = u.id);
