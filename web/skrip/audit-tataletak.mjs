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

const b = await chromium.launch({ channel: 'chrome' })
const ctx = await b.newContext({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 1, hasTouch: true, isMobile: true })
const p = await ctx.newPage()

await p.goto(`${BASE}/masuk`, { waitUntil: 'networkidle' })
await p.getByLabel('Nama pengguna').fill('sari')
await p.getByLabel('Kata sandi').fill('rahasia123')
await p.getByRole('button', { name: 'Masuk' }).click()
await p.waitForURL((u) => !u.pathname.includes('masuk'), { timeout: 15000 })

const temuan = []
let jumlahRefresh = 0
p.on('request', (r) => { if (r.url().includes('/auth/refresh')) jumlahRefresh++ })

for (const [nama, jalur] of HALAMAN) {
  await p.goto(BASE + jalur, { waitUntil: 'networkidle', timeout: 20000 })
  await p.waitForTimeout(1200)

  const hasil = await p.evaluate(() => {
    const out = { targetKecil: [], teksKecil: [], gesarMendatar: false, tumpangTindih: [],
                  kontrasRendah: [], istilahAsing: [], idMentah: [] }

    // 1. Target sentuh minimal 48px (ui/01 §3)
    for (const el of document.querySelectorAll('button, a[href], input, select, [role="button"]')) {
      const r = el.getBoundingClientRect()
      if (r.width === 0 || r.height === 0) continue
      const s = getComputedStyle(el)
      if (s.display === 'none' || s.visibility === 'hidden') continue
      if (r.height < 48) {
        if (el.tagName !== 'INPUT' || !el.closest('label')) out.targetKecil.push({ t: (el.textContent || el.getAttribute('aria-label') || el.tagName).trim().slice(0,40), h: Math.round(r.height) })
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
    const larangan = /\b(tenant|payload|idempoten|void|reconcile|outlet_id|sync|gross profit|net amount|receivable|opname|stock)\b/i
    const teks = document.body.innerText
    for (const baris of teks.split('\n')) {
      const t = baris.trim()
      if (t && larangan.test(t)) out.istilahAsing.push(t.slice(0, 70))
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
