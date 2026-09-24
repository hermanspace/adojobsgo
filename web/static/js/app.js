/*
 * Perekat sisi klien AdoJobs.
 * Alpine.js menangani state kecil per komponen; berkas ini hanya berisi
 * hal yang bersifat global: tema, format rupiah, dan integrasi HTMX.
 */

/*
 * htmx dijalankan tanpa evaluasi JavaScript.
 *
 * Beberapa fitur htmx (hx-vals/hx-headers berisi objek, filter hx-trigger
 * dalam kurung siku) diparsing memakai Function(), yang diblokir CSP di sini.
 * Mematikannya secara eksplisit membuat htmx melewati jalur itu dengan rapi
 * alih-alih melempar EvalError pada setiap elemen yang diprosesnya.
 */
if (typeof htmx !== 'undefined' && htmx.config) {
  htmx.config.allowEval = false;
}

(function () {
  'use strict';

  var STORAGE_KEY = 'adojobs-theme';

  /* ---------- tema terang / gelap ---------- */

  function currentTheme() {
    return document.documentElement.classList.contains('dark') ? 'dark' : 'light';
  }

  function applyTheme(theme) {
    var dark = theme === 'dark';
    document.documentElement.classList.toggle('dark', dark);
    try {
      localStorage.setItem(STORAGE_KEY, theme);
    } catch (e) {
      /* Penyimpanan diblokir: tema tetap berlaku untuk kunjungan ini saja. */
    }
    document.querySelectorAll('[data-theme-toggle]').forEach(function (el) {
      el.setAttribute('aria-pressed', String(dark));
      el.setAttribute('aria-label', dark ? 'Ganti ke mode terang' : 'Ganti ke mode gelap');
    });
    document.dispatchEvent(new CustomEvent('adojobs:theme', { detail: { theme: theme } }));
  }

  window.AdoJobsTheme = {
    toggle: function () {
      applyTheme(currentTheme() === 'dark' ? 'light' : 'dark');
    },
    apply: applyTheme,
    current: currentTheme,
  };

  /* ---------- format rupiah pada input harga ---------- */

  var rupiah = new Intl.NumberFormat('id-ID');

  function formatRupiahInput(el) {
    var digits = el.value.replace(/\D/g, '');
    el.value = digits ? rupiah.format(parseInt(digits, 10)) : '';
  }

  window.AdoJobsFormatRupiah = formatRupiahInput;

  /* ---------- inisialisasi ---------- */

  function init() {
    // Lepas kunci transisi setelah frame pertama selesai dilukis.
    requestAnimationFrame(function () {
      requestAnimationFrame(function () {
        document.documentElement.classList.remove('theme-loading');
      });
    });

    applyTheme(currentTheme());

    document.addEventListener('click', function (event) {
      var toggle = event.target.closest('[data-theme-toggle]');
      if (toggle) {
        event.preventDefault();
        window.AdoJobsTheme.toggle();
      }
    });

    // Baris chip kategori bisa digeser horizontal; chip yang sedang aktif
    // digeser ke dalam layar supaya pengguna langsung melihat filter yang
    // sedang berlaku tanpa harus menggeser sendiri.
    document.querySelectorAll('.scroll-x .is-active').forEach(function (chip) {
      var baris = chip.closest('.scroll-x');
      if (!baris) return;
      var geser = chip.offsetLeft - baris.offsetLeft - 16;
      if (geser > 0) {
        baris.scrollLeft = geser;
      }
    });

    document.addEventListener('input', function (event) {
      if (event.target.matches('[data-rupiah]')) {
        formatRupiahInput(event.target);
      }
    });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }

  /* ---------- komponen Alpine ----------
   * Aplikasi memakai Alpine build "CSP-friendly", yang tidak memanggil
   * new Function() sama sekali. Konsekuensinya direktif hanya boleh menunjuk
   * nama properti atau method — bukan ekspresi JavaScript inline. Berkat itu
   * Content-Security-Policy bisa tetap ketat tanpa 'unsafe-eval'.
   */

  document.addEventListener('alpine:init', function () {
    // Tombol lihat/sembunyikan kata sandi pada form masuk.
    Alpine.data('sandi', function () {
      return {
        lihat: false,
        toggle: function () {
          this.lihat = !this.lihat;
        },
        get tipe() {
          return this.lihat ? 'text' : 'password';
        },
        get label() {
          return this.lihat ? 'Sembunyikan' : 'Lihat';
        },
      };
    });

    // Form listing: kolom harga menyesuaikan jenis harga yang dipilih.
    // Perubahan pilihan ditangani method `pilih`, bukan x-model — build CSP
    // tidak mendukung x-model karena direktif itu perlu mengevaluasi ekspresi
    // penugasan, dan itulah yang sengaja dilarang di sini.
    Alpine.data('formHarga', function () {
      return {
        jenis: 'fixed',
        init: function () {
          // $el baru tersedia setelah komponen terpasang, bukan saat objek dibuat.
          this.jenis = this.$el.dataset.jenis || 'fixed';
        },
        pilih: function (event) {
          this.jenis = event.target.value;
        },
        get pakaiHarga() {
          return this.jenis !== 'negotiable';
        },
        get pakaiRentang() {
          return this.jenis === 'fixed';
        },
      };
    });
  });

  /* ---------- HTMX ---------- */

  // Sesi kedaluwarsa saat request HTMX berjalan: arahkan ke halaman masuk
  // alih-alih menyuntikkan HTML halaman login ke tengah halaman.
  document.body.addEventListener('htmx:beforeSwap', function (event) {
    if (event.detail.xhr && event.detail.xhr.status === 401) {
      event.detail.shouldSwap = false;
      window.location.href = '/masuk?next=' + encodeURIComponent(window.location.pathname);
    }
  });

  // Jaga posisi fokus dan kembalikan scroll ke atas hasil setelah filter berubah.
  document.body.addEventListener('htmx:afterSwap', function (event) {
    if (event.detail.target && event.detail.target.id === 'hasil-pencarian') {
      var anchor = document.getElementById('anchor-hasil');
      if (anchor && window.scrollY > anchor.offsetTop) {
        anchor.scrollIntoView({ behavior: 'smooth', block: 'start' });
      }
    }
  });
})();

/*
 * Menu utama (tombol melayang + sheet).
 *
 * Sheet adalah <dialog>; peramban yang mengurus perangkap fokus, Esc, dan
 * latar. Di sini hanya: buka, tutup (tombol atau klik latar), sinkron badge
 * belum dibaca dari header, dan tawaran pasang aplikasi (PWA).
 */
(function () {
  'use strict';

  function sheet() {
    return document.getElementById('menu-utama');
  }

  document.addEventListener('click', function (event) {
    var buka = event.target.closest('[data-menu-buka]');
    var s = sheet();
    if (buka && s && typeof s.showModal === 'function') {
      event.preventDefault();
      s.showModal();
      return;
    }
    // Penutupan berlaku untuk semua sheet (menu utama, bagikan, …):
    // tombol tutup, atau klik tepat pada elemen dialog = klik pada latar.
    var tutup = event.target.closest('[data-menu-tutup], [data-sheet-tutup]');
    if (tutup) {
      var d = tutup.closest('dialog');
      if (d && d.open) d.close();
      return;
    }
    if (event.target.matches('dialog.sheet[open]')) event.target.close();
    // Ganti tema dari dalam sheet: sheet tetap terbuka, tombolnya sudah
    // ditangani listener tema global.
  });

  // Badge pesan di header diperbarui htmx tiap 30 detik; tombol melayang
  // menyalin hasilnya alih-alih memasang polling kedua.
  document.body.addEventListener('htmx:afterSwap', function (event) {
    if (!event.detail.target || event.detail.target.id !== 'badge-pesan') return;
    var tujuan = document.getElementById('badge-fab');
    if (tujuan) tujuan.innerHTML = event.detail.target.innerHTML;
  });

  /* ---------- pasang sebagai aplikasi (PWA) ----------
   * Item "Pasang di layar utama" tersembunyi sampai peramban sendiri
   * menawarkan pemasangan. Bila sudah berjalan sebagai aplikasi terpasang,
   * atau peramban tidak mendukung, item tidak pernah muncul. */
  var tawaranPasang = null;

  function tombolPasang() {
    return document.querySelectorAll('[data-pasang-pwa]');
  }

  function sudahTerpasang() {
    return window.matchMedia('(display-mode: standalone)').matches || window.navigator.standalone === true;
  }

  window.addEventListener('beforeinstallprompt', function (event) {
    if (sudahTerpasang()) return;
    event.preventDefault();
    tawaranPasang = event;
    tombolPasang().forEach(function (el) {
      el.hidden = false;
    });
  });

  window.addEventListener('appinstalled', function () {
    tawaranPasang = null;
    tombolPasang().forEach(function (el) {
      el.hidden = true;
    });
  });

  document.addEventListener('click', function (event) {
    var tombol = event.target.closest('[data-pasang-pwa]');
    if (!tombol || !tawaranPasang) return;
    event.preventDefault();
    var tawaran = tawaranPasang;
    tawaranPasang = null;
    tawaran.prompt();
    tawaran.userChoice.then(function () {
      var s = sheet();
      if (s && s.open) s.close();
    });
  });
})();

/*
 * Bagikan jasa / penyedia.
 *
 * Di peramban ponsel dipakai Web Share API (navigator.share): muncul lembar
 * bagikan bawaan sistem dengan semua aplikasi yang terpasang. Bila tidak
 * tersedia (desktop), sheet berisi tautan per platform yang dibuka. Tombol
 * "Salin tautan" memakai clipboard API dengan umpan balik singkat.
 */
(function () {
  'use strict';

  document.addEventListener('click', function (event) {
    var tombol = event.target.closest('[data-bagikan]');
    if (!tombol) return;
    event.preventDefault();
    var data = { title: tombol.dataset.judul, text: tombol.dataset.teks, url: tombol.dataset.url };
    var sheet = document.querySelector(tombol.dataset.bagikanSheet || '');

    if (navigator.share && navigator.canShare && navigator.canShare(data)) {
      navigator.share(data).catch(function (err) {
        // Pengguna membatalkan: bukan kesalahan. Kegagalan lain: buka sheet.
        if (err && err.name !== 'AbortError' && sheet && sheet.showModal) sheet.showModal();
      });
      return;
    }
    if (sheet && typeof sheet.showModal === 'function') sheet.showModal();
  });

  document.addEventListener('click', function (event) {
    var tombol = event.target.closest('[data-salin-tautan]');
    if (!tombol) return;
    event.preventDefault();
    var url = tombol.dataset.salinTautan;
    var label = tombol.querySelector('[data-salin-label]');
    var selesai = function (berhasil) {
      if (!label) return;
      var asli = label.textContent;
      label.textContent = berhasil ? 'Tersalin' : 'Gagal menyalin';
      tombol.classList.toggle('is-tersalin', berhasil);
      setTimeout(function () {
        label.textContent = asli;
        tombol.classList.remove('is-tersalin');
      }, 1800);
    };
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(url).then(function () { selesai(true); }, function () { selesai(false); });
      return;
    }
    // Peramban lama: pilih teks URL lalu execCommand.
    var ta = document.createElement('textarea');
    ta.value = url;
    ta.setAttribute('readonly', '');
    ta.style.position = 'fixed';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    ta.select();
    var ok = false;
    try { ok = document.execCommand('copy'); } catch (e) { ok = false; }
    document.body.removeChild(ta);
    selesai(ok);
  });
})();

/*
 * Peta pemilih lokasi.
 *
 * Leaflet dimuat hanya pada halaman yang benar-benar memerlukannya, ditandai
 * elemen [data-peta]. Konfigurasinya dibaca dari atribut data-, bukan dari
 * blok <script> sebaris, supaya Content-Security-Policy tetap melarang skrip
 * inline sepenuhnya.
 */
(function () {
  'use strict';

  function siapkanPeta(wadah) {
    if (typeof L === 'undefined' || wadah.dataset.petaSiap === '1') return;
    wadah.dataset.petaSiap = '1';

    var cfg = {};
    try {
      cfg = JSON.parse(wadah.dataset.peta || '{}');
    } catch (e) {
      cfg = {};
    }

    var lat = typeof cfg.lat === 'number' ? cfg.lat : 1.4667;
    var lng = typeof cfg.lng === 'number' ? cfg.lng : 102.1;

    var peta = L.map(wadah, { scrollWheelZoom: false }).setView([lat, lng], cfg.adaPin ? 14 : 11);

    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      maxZoom: 19,
      attribution: '&copy; Kontributor OpenStreetMap',
    }).addTo(peta);

    var inputLat = document.getElementById(wadah.dataset.petaLat || 'latitude');
    var inputLng = document.getElementById(wadah.dataset.petaLng || 'longitude');

    var pin = null;
    var lingkaran = null;

    function gambarRadius() {
      var km = parseInt(wadah.dataset.petaRadius || cfg.radiusKm || 0, 10);
      if (!pin || !km) return;
      if (lingkaran) peta.removeLayer(lingkaran);
      lingkaran = L.circle(pin.getLatLng(), {
        radius: km * 1000,
        color: '#5B5BF6',
        weight: 1,
        fillColor: '#5B5BF6',
        fillOpacity: 0.08,
      }).addTo(peta);
    }

    function taruhPin(ll) {
      if (pin) {
        pin.setLatLng(ll);
      } else {
        pin = L.marker(ll, { draggable: !cfg.bacaSaja }).addTo(peta);
        pin.on('dragend', function () {
          simpan(pin.getLatLng());
          gambarRadius();
        });
      }
      simpan(ll);
      gambarRadius();
    }

    function simpan(ll) {
      if (inputLat) inputLat.value = ll.lat.toFixed(6);
      if (inputLng) inputLng.value = ll.lng.toFixed(6);
      wadah.dispatchEvent(new CustomEvent('adojobs:pin', { bubbles: true, detail: ll }));
    }

    if (cfg.adaPin) taruhPin(L.latLng(lat, lng));

    // Peta baca-saja hanya menampilkan lokasi; pinnya tidak bisa digeser
    // dan tidak ada input koordinat yang perlu diperbarui.
    if (cfg.bacaSaja) {
      if (pin && pin.dragging) pin.dragging.disable();
      peta.dragging.disable();
      peta.doubleClickZoom.disable();
      return;
    }

    peta.on('click', function (e) {
      taruhPin(e.latlng);
    });

    // Dropdown kecamatan meminta pin dipindahkan ke pusat wilayah.
    wadah.addEventListener('adojobs:pindah-pin', function (e) {
      var ll = L.latLng(e.detail.lat, e.detail.lng);
      peta.setView(ll, 13);
      taruhPin(ll);
    });

    // Radius bisa diubah lewat dropdown di luar peta.
    document.addEventListener('change', function (event) {
      if (!event.target.matches('[data-radius-untuk="' + wadah.id + '"]')) return;
      wadah.dataset.petaRadius = event.target.value;
      gambarRadius();
    });

    // Tombol "pakai lokasi saya" menggeser pin ke posisi GPS.
    var tombolGPS = document.querySelector('[data-gps-untuk="' + wadah.id + '"]');
    if (tombolGPS && navigator.geolocation) {
      tombolGPS.addEventListener('click', function () {
        tombolGPS.disabled = true;
        navigator.geolocation.getCurrentPosition(
          function (pos) {
            var ll = L.latLng(pos.coords.latitude, pos.coords.longitude);
            peta.setView(ll, 15);
            taruhPin(ll);
            tombolGPS.disabled = false;
          },
          function () {
            tombolGPS.disabled = false;
            alert('Lokasi tidak dapat diambil. Geser pin di peta secara manual.');
          },
          { enableHighAccuracy: true, timeout: 10000 }
        );
      });
    }

    // Peta yang dirender di dalam elemen tersembunyi perlu dihitung ulang.
    setTimeout(function () {
      peta.invalidateSize();
    }, 200);
  }

  function pasangSemuaPeta() {
    document.querySelectorAll('[data-peta]').forEach(siapkanPeta);
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', pasangSemuaPeta);
  } else {
    pasangSemuaPeta();
  }
  // Jaring pengaman bila Leaflet belum sempat dieksekusi saat percobaan
  // pertama; siapkanPeta menandai wadah yang sudah dipasang sehingga
  // pemanggilan berulang tidak membuat peta ganda.
  window.addEventListener('load', pasangSemuaPeta);
  document.body.addEventListener('htmx:afterSwap', pasangSemuaPeta);
})();

/*
 * Deteksi lokasi pencari jasa untuk hero dan pencarian.
 * Koordinat dikirim ke server sebagai form biasa, lalu server yang menulis
 * cookie-nya — label wilayahnya ditentukan di server dari daftar kecamatan,
 * bukan dari kiriman peramban.
 */
(function () {
  'use strict';

  document.addEventListener('click', function (event) {
    var tombol = event.target.closest('[data-deteksi-lokasi]');
    if (!tombol || !navigator.geolocation) return;

    event.preventDefault();
    var form = tombol.closest('form');
    if (!form) return;

    tombol.disabled = true;
    tombol.dataset.teksAsli = tombol.textContent;
    tombol.textContent = 'Mencari lokasi…';

    navigator.geolocation.getCurrentPosition(
      function (pos) {
        form.querySelector('[name="latitude"]').value = pos.coords.latitude.toFixed(6);
        form.querySelector('[name="longitude"]').value = pos.coords.longitude.toFixed(6);
        form.submit();
      },
      function () {
        tombol.disabled = false;
        tombol.textContent = tombol.dataset.teksAsli || 'Pakai lokasi saya';
        var daftar = document.getElementById('pilih-kecamatan');
        if (daftar) daftar.hidden = false;
      },
      { enableHighAccuracy: true, timeout: 10000 }
    );
  });

  // Ruang obrolan selalu menggulir ke pesan terbaru setelah HTMX menyisipkan.
  document.body.addEventListener('htmx:afterSwap', function (event) {
    if (!event.detail.target || event.detail.target.id !== 'daftar-pesan') return;
    var wadah = document.getElementById('gulung-pesan');
    if (wadah) wadah.scrollTop = wadah.scrollHeight;
  });
})();

/*
 * Isi cepat titik lokasi dari daftar kecamatan.
 * Berfungsi sebagai jalan keluar bila peta gagal dimuat atau pengguna menolak
 * memberi akses GPS.
 */
(function () {
  'use strict';

  document.addEventListener('change', function (event) {
    var el = event.target;
    if (!el.matches('[data-isi-kecamatan]')) return;
    if (!el.value) return;

    var bagian = el.value.split(',');
    if (bagian.length !== 2) return;

    var lat = document.getElementById('latitude');
    var lng = document.getElementById('longitude');
    if (lat) lat.value = bagian[0];
    if (lng) lng.value = bagian[1];

    // Peta ikut dipindahkan bila Leaflet berhasil dimuat.
    var wadah = document.getElementById(el.dataset.isiKecamatan);
    if (wadah) {
      wadah.dispatchEvent(
        new CustomEvent('adojobs:pindah-pin', {
          bubbles: true,
          detail: { lat: parseFloat(bagian[0]), lng: parseFloat(bagian[1]) },
        })
      );
    }
  });
})();

/*
 * Pengganti handler sebaris.
 *
 * Content-Security-Policy melarang script-src selain 'self', sehingga atribut
 * seperti onchange="..." dan hx-on="..." tidak pernah dieksekusi — keduanya
 * dievaluasi sebagai JavaScript sebaris. Semua perilaku itu dipindahkan ke
 * sini sebagai listener yang didelegasikan.
 */
(function () {
  'use strict';

  // Input berkas yang langsung mengirim formnya begitu berkas dipilih.
  document.addEventListener('change', function (event) {
    var el = event.target;
    if (!el.matches('input[type="file"][data-kirim-otomatis]')) return;
    if (!el.files || el.files.length === 0) return;
    if (el.form) el.form.requestSubmit();
  });

  // Form yang perlu dikosongkan setelah permintaan HTMX-nya berhasil.
  document.body.addEventListener('htmx:afterRequest', function (event) {
    var el = event.target;
    if (!el || !el.matches || !el.matches('form[data-reset-setelah-kirim]')) return;
    if (event.detail.successful) el.reset();
  });

  /*
   * Ruang obrolan hanya meminta pesan yang lebih baru dari yang sudah tampil.
   *
   * Tanpa ini, polling mengambil seluruh riwayat setiap kali lalu menempelkan
   * ulang (hx-swap="beforeend"), sehingga pesan berlipat terus-menerus.
   * Nilainya dihitung di sini karena hx-vals="js:..." juga membutuhkan
   * evaluasi JavaScript yang dilarang CSP.
   */
  document.body.addEventListener('htmx:configRequest', function (event) {
    var daftar = document.getElementById('daftar-pesan');
    if (!daftar) return;

    var elemen = event.detail.elt;
    var menyasarDaftar =
      elemen === daftar ||
      (elemen.getAttribute &&
        (elemen.getAttribute('hx-target') === '#daftar-pesan' ||
          elemen.closest('#daftar-pesan')));
    if (!menyasarDaftar) return;

    var gelembung = daftar.querySelectorAll('[data-pesan-id]');
    var terakhir = gelembung.length ? gelembung[gelembung.length - 1] : null;
    event.detail.parameters.sejak = terakhir ? terakhir.dataset.pesanId : '0';
  });
})();
