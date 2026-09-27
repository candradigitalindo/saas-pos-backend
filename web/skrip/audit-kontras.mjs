import { chromium } from 'playwright'

function lum(hex) {
  const c = hex.replace('#','').match(/../g).map(h => {
    const v = parseInt(h,16)/255
    return v <= 0.03928 ? v/12.92 : Math.pow((v+0.055)/1.055, 2.4)
  })
  return 0.2126*c[0] + 0.7152*c[1] + 0.0722*c[2]
}
const rasio = (a,b) => { const [x,y] = [lum(a),lum(b)].sort((p,q)=>q-p); return (x+0.05)/(y+0.05) }

const b = await chromium.launch({ channel: 'chrome' })
let gagal = 0
for (const mode of ['light','dark']) {
  const ctx = await b.newContext({ colorScheme: mode })
  const p = await ctx.newPage()
  await p.goto('http://localhost:5173/masuk', { waitUntil: 'networkidle' })

  // Resolusi SEBENARNYA: pasang warnanya ke elemen lalu baca computed style.
  const warna = await p.evaluate(() => {
    const el = document.createElement('span')
    document.body.appendChild(el)
    const baca = (v) => { el.style.color = `var(${v})`; return getComputedStyle(el).color }
    const hex = (s) => '#' + s.match(/\d+/g).slice(0,3).map(n => (+n).toString(16).padStart(2,'0')).join('')
    const out = {}
    for (const n of ['--color-bahaya-teks','--color-info-teks','--color-jingga-700',
                     '--color-hijau-700','--color-hijau-800','--color-teks-utama',
                     '--color-teks-sekunder','--color-teks-redup','--color-permukaan',
                     '--color-permukaan-2','--color-utama','--color-utama-teks']) {
      out[n.replace('--color-','')] = hex(baca(n))
    }
    el.remove()
    return out
  })

  console.log(`\n===== MODE ${mode.toUpperCase()} =====`)
  console.log(`permukaan ${warna.permukaan} · permukaan-2 ${warna['permukaan-2']}`)
  const uji = []
  for (const bg of ['permukaan','permukaan-2']) {
    for (const fg of ['teks-utama','teks-sekunder','teks-redup','bahaya-teks','info-teks',
                      'jingga-700','hijau-700','hijau-800']) {
      uji.push([`${fg} / ${bg}`, warna[fg], warna[bg]])
    }
  }
  uji.push(['utama-teks / utama (tombol)', warna['utama-teks'], warna.utama])
  for (const [nama, fg, bg] of uji) {
    const r = rasio(fg, bg)
    const ok = r >= 4.5
    if (!ok) gagal++
    console.log(`  ${ok ? 'LULUS' : 'GAGAL'} ${r.toFixed(2).padStart(6)}:1  ${nama}`)
  }

  // ── Kontras yang BENAR-BENAR TERENDER ────────────────────────────────
  //
  // Diperiksa di BEBERAPA HALAMAN, bukan hanya layar masuk: penjaga yang cuma
  // melihat satu layar tidak menjaga apa pun.
  //
  // Memeriksa pasangan token saja tidak cukup, dan itu terbukti mahal:
  // pesan galat di 20-an layar memakai `bg-red-50` — warna palet Tailwind
  // mentah yang TIDAK ikut berubah di mode gelap — sedangkan teksnya
  // `text-bahaya-teks` yang ikut berubah. Hasilnya merah muda terang di atas
  // merah muda terang, 2,53:1, praktis tidak terbaca. Semua pasangan token
  // lulus saat itu; yang bocor justru kombinasi yang tidak pernah diuji.
  //
  // Di sini yang diukur adalah piksel sungguhan: warna teks terhadap latar
  // efektif setelah komposisi, pada halaman nyata, di kedua mode.
  const bacaTerender = () => p.evaluate(() => {
    const c = document.createElement('canvas')
    c.width = c.height = 1
    const g = c.getContext('2d', { willReadFrequently: true })
    // getImageData memberi byte sRGB sebenarnya; `fillStyle` sendiri tidak
    // menormalkan oklch, dan Tailwind v4 memang memancarkan oklch.
    // Menumpuk lapisan di atas putih opak. Tanpa dasar opak, lapisan
    // separuh-tembus terbaca sebagai warna penuh karena getImageData
    // mengembalikan RGB tak-terpremultiplikasi.
    const piksel = (...warna) => {
      g.clearRect(0, 0, 1, 1)
      g.fillStyle = '#ffffff'
      g.fillRect(0, 0, 1, 1)
      for (const w of warna) { g.fillStyle = w; g.fillRect(0, 0, 1, 1) }
      const d = g.getImageData(0, 0, 1, 1).data
      return [d[0], d[1], d[2]]
    }
    // Alfa dibaca dari PIKSEL, bukan dari bentuk teks CSS-nya.
    //
    // Versi pertama menebak transparansi dengan mencocokkan "rgba(". Tailwind v4
    // memancarkan `oklab(… / 0.15)` untuk tinta seperti `bg-jingga-700/15`, jadi
    // pencocokan itu meleset: tinta 15% dianggap opak, dan lencana jingga
    // dilaporkan sebagai teks jingga di atas latar jingga — 1,01:1. Aplikasinya
    // sehat; alat ukurnya yang salah.
    const alfa = (v) => {
      g.clearRect(0, 0, 1, 1)
      g.fillStyle = v
      g.fillRect(0, 0, 1, 1)
      return g.getImageData(0, 0, 1, 1).data[3]
    }

    // Mengembalikan null bila ada GRADIEN di tumpukan latarnya: warna di balik
    // teks tidak bisa dihitung dari satu nilai saat latarnya background-image.
    const dasar = getComputedStyle(document.body).backgroundColor || 'rgb(255,255,255)'
    const tumpukanLatar = (el) => {
      const keluar = []
      for (let e = el; e; e = e.parentElement) {
        const cs = getComputedStyle(e)
        if (cs.backgroundImage && cs.backgroundImage !== 'none') return null
        const bg = cs.backgroundColor
        if (!bg) continue
        const a = alfa(bg)
        if (a === 0) continue
        keluar.unshift(bg)
        if (a === 255) return keluar
      }
      // Belum ketemu lapisan opak — pakai latar halaman sebagai dasarnya.
      keluar.unshift(dasar)
      return keluar
    }

    const hasil = []
    for (const el of document.querySelectorAll('p,span,div,button,a,td,th,li,h1,h2,h3,label')) {
      if (el.children.length > 0) continue
      const teks = el.textContent?.trim()
      if (!teks) continue
      const cs = getComputedStyle(el)
      if (cs.visibility === 'hidden' || cs.display === 'none' || +cs.opacity === 0) continue
      const r = el.getBoundingClientRect()
      if (r.width < 4 || r.height < 4) continue
      const latar = tumpukanLatar(el)
      if (!latar) { hasil.push({ teks: teks.slice(0, 40), takTerukur: true }); continue }
      hasil.push({
        teks: teks.slice(0, 40),
        fg: piksel(cs.color),
        bg: piksel(...latar),
      })
    }
    return hasil
  })

  // rasio() yang sudah ada menerima hex; piksel datang sebagai [r,g,b].
  const keHex = ([r, g, b]) =>
    '#' + [r, g, b].map((v) => v.toString(16).padStart(2, '0')).join('')

  const terender = []

  // Halaman PUBLIK diukur sebelum masuk, di layar lebar dan di HP: layar ini
  // yang pertama dilihat setiap orang, dan daftar keunggulan berlatar `sorot`
  // hanya tampil di HP. Teks di atas panel merek (gradien) tercatat "tak
  // terukur" — pasangannya sama dengan kartu sorotan beranda, yang kedua ujung
  // gradiennya sudah diverifikasi di ui/02.
  for (const lebar of [1280, 390]) {
    await p.setViewportSize({ width: lebar, height: 844 })
    for (const jalur of ['/masuk', '/daftar']) {
      await p.goto('http://localhost:5173' + jalur, { waitUntil: 'networkidle' })
      await p.waitForTimeout(600)
      terender.push(...(await bacaTerender()))
    }
  }
  await p.setViewportSize({ width: 1280, height: 720 })
  await p.goto('http://localhost:5173/masuk', { waitUntil: 'networkidle' })

  await p.getByLabel('Nama pengguna').fill('sari')
  await p.getByLabel('Kata sandi').fill('rahasia123')
  await p.getByRole('button', { name: 'Masuk', exact: true }).click()
  await p.waitForURL((u) => !u.pathname.includes('masuk'), { timeout: 15000 })
  await p.waitForTimeout(1500)

  // Halaman dengan keadaan berwarna: galat, lencana, peringatan, kartu sorotan.
  for (const jalur of ['/', '/laporan', '/kasir', '/stok', '/kasbon', '/pengaturan', '/sdm/gaji']) {
    await p.goto('http://localhost:5173' + jalur, { waitUntil: 'networkidle' })
    await p.waitForTimeout(1200)
    terender.push(...(await bacaTerender()))
  }

  let takTerukur = 0
  for (const t of terender) {
    if (t.takTerukur) { takTerukur++; continue }
    const r = rasio(keHex(t.fg), keHex(t.bg))
    if (r < 4.5) {
      gagal++
      console.log(`  GAGAL ${r.toFixed(2).padStart(6)}:1  terender: "${t.teks}"`)
    }
  }
  console.log(`  (${terender.length - takTerukur} teks terender diperiksa, ${takTerukur} di atas gradien — tak terukur)`)

  await ctx.close()
}
await b.close()
console.log(`\n${gagal === 0 ? 'SEMUA LULUS 4.5:1' : gagal + ' GAGAL'}`)
process.exit(gagal === 0 ? 0 : 1)
