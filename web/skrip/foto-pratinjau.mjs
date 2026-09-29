/**
 * Membuat gambar pratinjau produk untuk halaman masuk & daftar
 * (public/gambar/pratinjau-kasir.jpg & pratinjau-laporan.jpg) dari APLIKASI
 * YANG SEDANG BERJALAN.
 *
 * Kenapa tangkapan layar asli, bukan ilustrasi: halaman masuk SaaS kasir yang
 * paling meyakinkan (Pawoon, Square) memperlihatkan produknya sendiri —
 * orang langsung melihat apa yang akan ia pakai. Gambar buatan tangan cepat
 * basi dan berbohong sedikit demi sedikit; skrip ini membuatnya ulang dari
 * layar sungguhan kapan pun tampilan aplikasi berubah.
 *
 * Syarat: backend + `npm run dev` berjalan, dengan data contoh
 * (`go run ./cmd/seed-demo`) — akun pemilik `sari` dan kasir `budi`.
 *
 *   npm run foto:pratinjau
 */
import { chromium } from 'playwright'

const BASE = process.env.BASE ?? 'http://localhost:5173'
const SANDI = process.env.SANDI ?? 'rahasia123'
const KELUAR = new URL('../public/gambar/', import.meta.url).pathname

const b = await chromium.launch({ channel: 'chrome' })

async function masuk(username, viewport) {
  const ctx = await b.newContext({ viewport, deviceScaleFactor: 1, colorScheme: 'light', locale: 'id-ID' })
  const p = await ctx.newPage()
  await p.goto(`${BASE}/masuk`, { waitUntil: 'networkidle' })
  await p.getByLabel('Nama pengguna').fill(username)
  await p.getByLabel('Kata sandi').fill(SANDI)
  await p.getByRole('button', { name: 'Masuk', exact: true }).click()
  await p.waitForURL((u) => !u.pathname.includes('masuk'), { timeout: 15000 })
  return p
}

// 1. Layar kasir di tablet lanskap, dengan keranjang terisi supaya hidup.
const kasir = await masuk('budi', { width: 1280, height: 800 })
await kasir.goto(`${BASE}/kasir`, { waitUntil: 'networkidle' })
await kasir.waitForTimeout(800)
const kartu = kasir.locator('button[aria-label*="Rp"]:not([aria-label*="habis"])')
for (const i of [0, 1, 1, 3]) await kartu.nth(i).click()
await kasir.waitForTimeout(400)
await kasir.screenshot({ path: `${KELUAR}pratinjau-kasir.jpg`, type: 'jpeg', quality: 82 })

// 2. Laporan pemilik di HP, periode "Bulan ini". Bukan beranda / "Hari ini":
//    keduanya bergantung pada penjualan HARI INI, dan data contoh yang tidak
//    diperbarui membuat etalase produk memamerkan "Rp 0".
const laporan = await masuk('sari', { width: 390, height: 844 })
await laporan.goto(`${BASE}/laporan`, { waitUntil: 'networkidle' })
await laporan.getByRole('button', { name: 'Bulan ini' }).click()
await laporan.waitForLoadState('networkidle')
await laporan.waitForTimeout(1200)
await laporan.evaluate(() => window.scrollTo(0, 0))
await laporan.screenshot({ path: `${KELUAR}pratinjau-laporan.jpg`, type: 'jpeg', quality: 82 })

await b.close()
console.log('Gambar pratinjau diperbarui di public/gambar/.')
