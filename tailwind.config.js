/** @type {import('tailwindcss').Config} */
const plugin = require('tailwindcss/plugin');
// Sumber tunggal token desain — juga dibaca aplikasi Android. Tidak ada
// nilai warna, radius, spasi, atau skala huruf yang ditulis di berkas ini.
const tokens = require('./web/static/design/tokens.json');

// Nilai objek token, tanpa kunci catatan.
function nilai(obj) {
  const out = {};
  for (const k of Object.keys(obj)) if (!k.startsWith('_')) out[k] = obj[k];
  return out;
}

// Variabel CSS untuk satu mode: warna + bayangan (--shadow-*) + color-scheme.
function variabel(mode) {
  const v = {};
  for (const [k, val] of Object.entries(nilai(tokens.warna[mode]))) v['--' + k] = val;
  for (const [k, val] of Object.entries(nilai(tokens.bayangan[mode]))) v['--shadow-' + k] = val;
  v['color-scheme'] = mode === 'gelap' ? 'dark' : 'light';
  return v;
}

const fontSize = {};
for (const [k, s] of Object.entries(tokens.huruf.skala)) {
  const opsi = { lineHeight: s.tinggiBaris };
  if (s.spasiHuruf) opsi.letterSpacing = s.spasiHuruf;
  fontSize[k] = [s.ukuran, opsi];
}
const fontFamily = {};
for (const [k, daftar] of Object.entries(tokens.huruf.keluarga)) {
  fontFamily[k] = daftar.map((n) => (n.includes(' ') ? `"${n}"` : n));
}

module.exports = {
  // Dark mode dikendalikan class pada <html>, bukan preferensi sistem,
  // karena pengguna memilih sendiri lewat toggle di navigasi.
  darkMode: 'class',
  content: [
    './web/templates/**/*.templ',
    './web/templates/**/*_templ.go',
    './web/static/js/**/*.js',
  ],
  theme: {
    spacing: nilai(tokens.spasi),
    borderRadius: tokens.radius,
    extend: {
      // Ukuran elemen dipisahkan dari skala spacing: ritme tata letak tetap
      // terkunci ke 4/8/12/16/24/…, ukuran komponen boleh memakai nilai lain.
      width: tokens.ukuran.lebar,
      height: tokens.ukuran.tinggi,
      colors: {
        bg: 'var(--bg)',
        surface: 'var(--surface)',
        raised: 'var(--surface-raised)',
        ink: {
          DEFAULT: 'var(--text)',
          muted: 'var(--text-muted)',
          subtle: 'var(--text-subtle)',
        },
        line: {
          DEFAULT: 'var(--border)',
          strong: 'var(--border-strong)',
        },
        brand: {
          DEFAULT: 'var(--primary-solid)',
          soft: 'var(--primary-soft)',
          from: 'var(--primary-from)',
          to: 'var(--primary-to)',
          contrast: 'var(--primary-contrast)',
        },
        star: 'var(--star)',
        positive: 'var(--positive)',
        danger: 'var(--danger)',
      },
      fontFamily,
      fontSize,
      boxShadow: {
        subtle: 'var(--shadow-subtle)',
        raised: 'var(--shadow-raised)',
        pop: 'var(--shadow-pop)',
      },
      maxWidth: tokens.ukuran.lebarMaks,
      transitionTimingFunction: {
        out: tokens.gerak.easingKeluar,
      },
    },
  },
  plugins: [
    // Blok :root / .dark dihasilkan dari tokens.json, bukan ditulis di CSS.
    plugin(({ addBase }) => {
      const ukuran = {};
      for (const [k, val] of Object.entries(nilai(tokens.komponen))) ukuran['--ukuran-' + k] = val;
      addBase({ ':root': { ...variabel('terang'), ...ukuran }, '.dark': variabel('gelap') });
    }),
  ],
};
