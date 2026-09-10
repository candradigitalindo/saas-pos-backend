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
  await ctx.close()
}
await b.close()
console.log(`\n${gagal === 0 ? 'SEMUA PASANGAN LULUS 4.5:1' : gagal + ' pasangan GAGAL'}`)
process.exit(gagal === 0 ? 0 : 1)
