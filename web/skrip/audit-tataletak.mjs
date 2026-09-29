import { chromium } from 'playwright'
const BASE = 'http://localhost:5173'

const HALAMAN = [
  ['beranda','/'], ['kasir','/kasir'], ['laporan','/laporan'], ['barang','/barang'],
  ['stok','/stok'], ['pelanggan','/pelanggan'], ['kasbon','/kasbon'],
  ['riwayat','/kasir/riwayat'], ['tutup-shift','/kasir/tutup-shift'], ['kas','/kasir/kas'],
  ['barang-masuk','/stok/masuk'], ['opname','/stok/opname'], ['kanal','/kanal'],
  ['karyawan','/sdm'], ['gaji','/sdm/gaji'], ['crm','/crm'], ['kunjungan','/crm/kunjungan'],
  ['langganan','/langganan'], ['pengaturan','/pengaturan'], ['peran','/pengaturan/peran'],
  ['pengguna','/pengaturan/pengguna'], ['lainnya','/lainnya'],
]

/**
 * Halaman DETAIL, yang alamatnya baru diketahui setelah membuka daftarnya.
 *
 * Tanpa ini audit hanya memeriksa halaman daftar — dan kebocoran istilah
 * sistem yang sebenarnya justru ditemukan di kartu stok, halaman detail yang
 * tidak pernah dikunjungi.
 */
const DETAIL = [
  ['kartu-stok', '/stok', 'a[href*="/stok/kartu/"]'],
  ['form-barang', '/barang', 'a[href*="/barang/"]'],
]

const b = await chromium.launch({ channel: 'chrome' })
const ctx = await b.newContext({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 1, hasTouch: true, isMobile: true })
const p = await ctx.newPage()

const temuan = []

// Halaman PUBLIK diperiksa sebelum masuk: layar pertama yang dilihat setiap
// orang, dan satu-satunya yang tidak bisa dicapai setelah sesi terbentuk.
for (const [nama, jalur] of [['masuk', '/masuk'], ['daftar', '/daftar']]) {
  await periksa(nama, jalur)
}

await p.goto(`${BASE}/masuk`, { waitUntil: 'networkidle' })
await p.getByLabel('Nama pengguna').fill('sari')
await p.getByLabel('Kata sandi').fill('rahasia123')
await p.getByRole('button', { name: 'Masuk', exact: true }).click()
await p.waitForURL((u) => !u.pathname.includes('masuk'), { timeout: 15000 })

/** Menemukan alamat halaman detail dari daftarnya. */
async function jalurDetail(dariJalur, pemilih) {
  await p.goto(BASE + dariJalur, { waitUntil: 'networkidle', timeout: 20000 })
  await p.waitForTimeout(1200)
  return p.evaluate((sel) => document.querySelector(sel)?.getAttribute('href') ?? null, pemilih)
}
let jumlahRefresh = 0
p.on('request', (r) => { if (r.url().includes('/auth/refresh')) jumlahRefresh++ })

// Alamat halaman detail diselesaikan dulu, lalu diaudit bersama sisanya.
const SEMUA = [...HALAMAN]
for (const [nama, dari, pemilih] of DETAIL) {
  const jalur = await jalurDetail(dari, pemilih)
  if (jalur) SEMUA.push([nama, jalur])
  else console.error(`(lewat ${nama}: tidak ada tautan di ${dari})`)
}

for (const [nama, jalur] of SEMUA) await periksa(nama, jalur)

/** Memeriksa satu halaman dan mencatat pelanggarannya ke `temuan`. */
async function periksa(nama, jalur) {
  await p.goto(BASE + jalur, { waitUntil: 'networkidle', timeout: 20000 })
  await p.waitForTimeout(1200)

  const hasil = await p.evaluate(() => {
    const out = { targetKecil: [], teksKecil: [], gesarMendatar: false, tumpangTindih: [],
                  kontrasRendah: [], istilahAsing: [], idMentah: [] }

    // 1. Target sentuh minimal 48px (ui/01 §3)
    //
    // Yang diukur adalah kotak yang BENAR-BENAR bisa disentuh, bukan selalu
    // elemennya sendiri. Sebuah <input> di dalam pembungkus ber-`h-12` melaporkan
    // 46px karena border 1px atas-bawah memakan kotak isinya — padahal menekan
    // di mana pun pada 48px itu tetap memfokuskan input tersebut. Versi
    // sebelumnya melaporkannya sebagai pelanggaran di empat halaman, dan itu
    // positif palsu yang membuat audit ini lama-lama diabaikan.
    const kotakSentuh = (el) => {
      const r = el.getBoundingClientRect()
      if (el.tagName !== 'INPUT') return r

      // a) Input di dalam <label>: labelnya yang disentuh. Checkbox 20×20 di
      //    dalam baris <label> setinggi 56px bukan target 20px — menekan di
      //    mana pun pada barisnya tetap mencentangnya.
      const label = el.closest('label')
      if (label) {
        const rl = label.getBoundingClientRect()
        if (rl.height >= r.height) return rl
      }

      // b) Input di dalam pembungkus rapat: pembungkusnya yang disentuh. Input
      //    di dalam kotak ber-`h-12` melaporkan 46px karena border 1px
      //    atas-bawah memakan kotak isinya, padahal targetnya tetap 48px.
      let n = el.parentElement
      for (let i = 0; i < 2 && n; i++, n = n.parentElement) {
        const rn = n.getBoundingClientRect()
        if (rn.height >= r.height && rn.height - r.height <= 8) return rn
      }
      return r
    }

    for (const el of document.querySelectorAll('button, a[href], input, select, [role="button"]')) {
      const r = kotakSentuh(el)
      if (r.width === 0 || r.height === 0) continue
      const s = getComputedStyle(el)
      if (s.display === 'none' || s.visibility === 'hidden') continue
      if (r.height < 48) {
        out.targetKecil.push({ t: (el.textContent || el.getAttribute('aria-label') || el.tagName).trim().slice(0,40), h: Math.round(r.height) })
      }
    }

    // 2. Teks di bawah 13px (ui/02: keterangan 13px minimum absolut)
    for (const el of document.querySelectorAll('p, span, dt, dd, li, td, th, label, h1, h2, h3, button, a')) {
      if (!el.textContent?.trim()) continue
      if (el.children.length > 0) continue
      const fs = parseFloat(getComputedStyle(el).fontSize)
      if (fs < 12.9) out.teksKecil.push({ t: el.textContent.trim().slice(0,40), fs: fs.toFixed(1) })
    }

    // 3. Gulir mendatar (anti-pola: tabel lebar yang digeser)
    out.gesarMendatar = document.documentElement.scrollWidth > window.innerWidth + 2

    // 4. Istilah sistem bocor ke layar (ui/01 §2)
    //
    // Dua aturan. Yang pertama daftar kata yang memang dilarang dokumen.
    //
    // Yang kedua lebih tajam dan tidak perlu dirawat: APA PUN yang berbentuk
    // snake_case. Tidak ada kalimat Indonesia di layar ini yang memuat
    // `transfer_in`, `on_hold`, atau `written_off` — bentuk itu hanya lahir
    // dari nilai enum basis data yang lupa diterjemahkan. Daftar kata harus
    // diperbarui tiap kali backend menambah enum; aturan bentuk tidak.
    //
    // Ini lahir dari kebocoran nyata: `initial` dan `void` tampil apa adanya di
    // kartu stok. `void` bahkan SUDAH ada di daftar larangan — yang salah
    // adalah halamannya tidak pernah dikunjungi audit ini.
    const larangan = /\b(tenant|payload|idempoten|void|reconcile|sync|gross profit|net amount|receivable|opname)\b/i
    const enumSnake = /\b[a-z]{2,}_[a-z]{2,}(_[a-z]{2,})?\b/
    const teks = document.body.innerText
    for (const baris of teks.split('\n')) {
      const t = baris.trim()
      if (!t) continue
      if (larangan.test(t) || enumSnake.test(t)) out.istilahAsing.push(t.slice(0, 70))
    }

    // 5. ULID mentah tampil ke pengguna
    for (const baris of teks.split('\n')) {
      if (/\b01[0-9A-HJKMNP-TV-Z]{8,}/.test(baris)) out.idMentah.push(baris.trim().slice(0, 70))
    }

    // 6. Angka sorotan: cari elemen berkelas text-angka
    out.angkaSorotan = [...document.querySelectorAll('*')].filter(e =>
      e.className && typeof e.className === 'string' && e.className.includes('text-angka')
    ).map(e => ({ t: e.textContent.trim().slice(0,24), fs: getComputedStyle(e).fontSize }))

    return out
  })

  const ringkas = {}
  for (const [k, v] of Object.entries(hasil)) {
    if (Array.isArray(v) && v.length) ringkas[k] = v.slice(0, 6)
    else if (v === true) ringkas[k] = true
  }
  if (Object.keys(ringkas).length) temuan.push({ halaman: nama, ...ringkas })
}

console.log(JSON.stringify({ jumlahRefresh, temuan }, null, 1))
await b.close()
