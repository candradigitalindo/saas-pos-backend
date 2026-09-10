import { chromium } from 'playwright'
import fs from 'node:fs'

const DIR = process.env.SHOT_DIR
const BASE = 'http://localhost:5173'

// Tiga rentang dari ui/02-SISTEM-DESAIN.md
const LAYAR = {
  hp:      { width: 390,  height: 844 },   // HP
  tablet:  { width: 1024, height: 768 },   // tablet lanskap — layar kasir utama
  desktop: { width: 1440, height: 900 },   // desktop
}

const galat = []

async function masuk(page) {
  await page.goto(`${BASE}/masuk`, { waitUntil: 'networkidle' })
  await page.getByLabel('Nama pengguna').fill('sari')
  await page.getByLabel('Kata sandi').fill('rahasia123')
  await page.getByRole('button', { name: 'Masuk' }).click()
  await page.waitForURL((u) => !u.pathname.includes('masuk'), { timeout: 15000 })
  await page.waitForTimeout(1200)
}

async function jepret(ctx, nama, jalur, layar, opsi = {}) {
  const page = await ctx.newPage()
  page.on('pageerror', (e) => galat.push(`${nama}/${layar}: ${e.message}`))
  page.on('console', (m) => {
    if (m.type() === 'error' && !m.text().includes('favicon')) {
      galat.push(`${nama}/${layar} console: ${m.text().slice(0, 160)}`)
    }
  })
  try {
    await page.goto(BASE + jalur, { waitUntil: 'networkidle', timeout: 20000 })
    await page.waitForTimeout(opsi.tunggu ?? 1500)
    if (opsi.aksi) await opsi.aksi(page)
    await page.screenshot({ path: `${DIR}/${layar}-${nama}.png`, fullPage: opsi.penuh !== false })
  } catch (e) {
    galat.push(`${nama}/${layar}: ${e.message.slice(0, 200)}`)
  }
  await page.close()
}

const b = await chromium.launch({ channel: 'chrome' })

for (const [layar, ukuran] of Object.entries(LAYAR)) {
  const ctx = await b.newContext({ viewport: ukuran, deviceScaleFactor: 2, locale: 'id-ID' })
  const p = await ctx.newPage()
  await masuk(p)
  await p.close()

  const halaman = [
    ['beranda', '/'],
    ['kasir', '/kasir', { penuh: false, tunggu: 2500 }],
    ['laporan', '/laporan', { tunggu: 2500 }],
    ['barang', '/barang'],
    ['stok', '/stok'],
    ['pelanggan', '/pelanggan'],
    ['kasbon', '/kasbon'],
    ['riwayat', '/kasir/riwayat'],
    ['tutup-shift', '/kasir/tutup-shift'],
    ['kas', '/kasir/kas'],
    ['barang-masuk', '/stok/masuk'],
    ['opname', '/stok/opname'],
    ['kanal', '/kanal'],
    ['karyawan', '/sdm'],
    ['gaji', '/sdm/gaji'],
    ['crm', '/crm'],
    ['kunjungan', '/crm/kunjungan'],
    ['langganan', '/langganan'],
    ['pengaturan', '/pengaturan'],
    ['peran', '/pengaturan/peran'],
    ['pengguna', '/pengaturan/pengguna'],
    ['lainnya', '/lainnya'],
  ]
  for (const [nama, jalur, opsi] of halaman) await jepret(ctx, nama, jalur, layar, opsi)
  await ctx.close()
  console.log(`selesai: ${layar}`)
}

// Layar publik & realm lain (tanpa sesi toko)
const ctxPublik = await b.newContext({ viewport: LAYAR.hp, deviceScaleFactor: 2, locale: 'id-ID' })
for (const [nama, jalur] of [['masuk','/masuk'], ['daftar','/daftar'], ['mitra-masuk','/mitra']])
  await jepret(ctxPublik, nama, jalur, 'hp')
await ctxPublik.close()

const ctxPanel = await b.newContext({ viewport: LAYAR.desktop, deviceScaleFactor: 2, locale: 'id-ID' })
await jepret(ctxPanel, 'panel-masuk', '/panel', 'desktop')
await ctxPanel.close()

await b.close()
fs.writeFileSync(`${DIR}/galat.txt`, galat.join('\n') || '(tidak ada)')
console.log('\n=== GALAT RUNTIME ===')
console.log(galat.length ? galat.join('\n') : '(tidak ada)')
