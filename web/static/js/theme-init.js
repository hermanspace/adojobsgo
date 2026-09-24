/*
 * Dijalankan sinkron di <head> sebelum body dirender, supaya halaman tidak
 * sempat berkedip terang saat pengguna memilih mode gelap (FOUC).
 * Urutan penentuan: pilihan tersimpan -> preferensi sistem -> terang.
 */
(function () {
  var STORAGE_KEY = 'adojobs-theme';
  var stored = null;
  try {
    stored = localStorage.getItem(STORAGE_KEY);
  } catch (e) {
    /* localStorage bisa diblokir; abaikan dan pakai preferensi sistem. */
  }
  var prefersDark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
  var dark = stored ? stored === 'dark' : prefersDark;

  var root = document.documentElement;
  root.classList.toggle('dark', dark);
  // Matikan transisi warna untuk render pertama; dilepas setelah halaman siap.
  root.classList.add('theme-loading');
})();
