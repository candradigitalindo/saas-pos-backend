/**
 * Sensus warna: berapa persen layar yang benar-benar memakai warna merek?
 *
 * Kenapa ada skrip ini. "Warnanya kurang terasa" adalah keluhan yang mudah
 * diperdebatkan dan sulit dibuktikan — dua orang melihat layar yang sama dan
 * berbeda pendapat. Angka menyelesaikannya: liputan Hijau Tumbuh di bawah 1%
 * bukan selera, itu fakta yang bisa diperiksa ulang setelah diperbaiki.
 *
 * Cara mengukurnya: menembak titik tiap 8 piksel, lalu menanyakan warna latar
 * efektif elemen paling atas di titik itu. Tiap titik dihitung SEKALI. Versi
 * pertama skrip ini menjumlahkan luas semua elemen ber-latar, sehingga kartu di
 * atas halaman terhitung dua kali dan totalnya bisa 200% — angka yang jelas
 * salah, dan itulah yang membuat bugnya ketahuan.
 *
 *   node skrip/sensus-warna.mjs
 */
import { chromium } from 'playwright'

const BASE = 'http://localhost:5173'
const KOTAK = 8 // jarak antar titik tembak, piksel

// Token dari ui/02-SISTEM-DESAIN.md yang ingin kita lacak.
const MEREK = {
  'rgb(4, 120, 87)': 'hijau-700 #047857 (warna utama)',
  'rgb(236, 253, 245)': 'hijau-50 #ECFDF5',
  'rgb(5, 150, 105)': 'hijau-600 #059669',
  'rgb(16, 185, 129)': 'hijau-500 #10B981',
}

async function masuk(page) {
  await page.goto(`${BASE}/masuk`, { waitUntil: 'networkidle' })
  await page.getByLabel('Nama pengguna').fill('sari')
  await page.getByLabel('Kata sandi').fill('rahasia123')
  await page.getByRole('button', { name: 'Masuk' }).click()
  await page.waitForURL((u) => !u.pathname.includes('masuk'), { timeout: 15000 })
  await page.waitForTimeout(1500)
}

const liputan = (kotak) => {
  const W = innerWidth, H = innerHeight
  const hitung = {}
  for (let y = 0; y < H; y += kotak) {
    for (let x = 0; x < W; x += kotak) {
      const el = document.elementFromPoint(x, y)
      if (!el) continue
      // Latar efektif: naik ke induk selama elemennya tembus pandang.
      let e = el, bg = 'rgba(0, 0, 0, 0)'
      while (e && (bg === 'rgba(0, 0, 0, 0)' || bg === 'transparent')) {
        bg = getComputedStyle(e).backgroundColor
        e = e.parentElement
      }
      hitung[bg] = (hitung[bg] || 0) + 1
    }
  }
  const total = Object.values(hitung).reduce((a, b) => a + b, 0)
  return { hitung, total }
}

const HALAMAN = [['beranda', '/'], ['kasir', '/kasir'], ['laporan', '/laporan'], ['barang', '/barang']]
const b = await chromium.launch({ channel: 'chrome' })

for (const lebar of [390, 1024, 1440]) {
  const ctx = await b.newContext({ viewport: { width: lebar, height: 844 }, locale: 'id-ID' })
  const p = await ctx.newPage()
  await masuk(p)

  console.log(`\n${'═'.repeat(52)}\n  LEBAR ${lebar}px\n${'═'.repeat(52)}`)
  for (const [nama, jalur] of HALAMAN) {
    await p.goto(BASE + jalur, { waitUntil: 'networkidle' })
    await p.waitForTimeout(1800)
    const { hitung, total } = await p.evaluate(liputan, KOTAK)

    const merek = Object.entries(hitung)
      .filter(([w]) => MEREK[w])
      .reduce((a, [, n]) => a + n, 0)

    const teratas = Object.entries(hitung).sort((a, c) => c[1] - a[1]).slice(0, 3)
    console.log(`\n  ${nama}`)
    console.log(`    warna merek : ${(merek / total * 100).toFixed(1)}%`)
    for (const [w, n] of teratas) {
      console.log(`    ${(n / total * 100).toFixed(1).padStart(5)}%  ${MEREK[w] ?? w}`)
    }
  }
  await ctx.close()
}
await b.close()
