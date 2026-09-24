-- Nomor WhatsApp tidak lagi wajib: fitur WhatsApp dimatikan dan seluruh
-- percakapan berlangsung di dalam aplikasi. Kolomnya tetap NOT NULL dengan
-- bawaan string kosong — bukan NULL — supaya seluruh kode yang membaca nomor
-- sebagai string tidak perlu berubah; kosong berarti "tidak diisi".
ALTER TABLE provider_profiles ALTER COLUMN whatsapp_number SET DEFAULT '';
