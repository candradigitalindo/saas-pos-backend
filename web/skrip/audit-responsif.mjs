/**
 * Audit responsif: SEMUA halaman (toko, panel internal, portal mitra) di
 * banyak lebar layar — dari HP 320px sampai monitor 1920px — plus modal yang
 * bisa dibuka di setiap halaman.
 *
 * `periksa:tataletak` memeriksa satu lebar (390px) dan tidak membuka modal;
 * padahal kerusakan tata letak paling sering muncul di lebar "tanggung"
 * (tablet 768–1024, HP kecil 320–360) dan di dalam dialog.
 *
 * Yang diperiksa per halaman × lebar:
 *   - gulir mendatar pada halaman;
 *   - elemen yang keluar dari tepi layar (di luar wadah gulir);
 *   - teks terpotong TANPA elipsis (overflow hidden, tidak disengaja);
 *   - angka rupiah yang terpotong (elipsis pada nominal = informasi hilang);
 *   - tombol/tautan yang saling menumpuk;
 *   - target sentuh < 44px di lebar HP.
 * Per modal: muat di layar, tidak bergulir mendatar, isi yang panjang bisa
 * digulir, dan bisa ditutup.
 *
 * Jalankan terhadap SALINAN basis data — membuka modal kadang menekan tombol
 * yang mengubah data (mis. "Bayar sekarang" menerbitkan tagihan):
 *
 *   AUDIT_BASE=http://localhost:5174 AUDIT_FOTO=/tmp/foto node skrip/audit-responsif.mjs
 *
 * Akun (bisa ditimpa lewat env): toko sari/rahasia123, panel AUDIT_PANEL_EMAIL/
 * AUDIT_PANEL_SANDI, mitra AUDIT_MITRA_EMAIL/AUDIT_MITRA_SANDI. Realm yang
 * akunnya kosong dilewati.
 */
import { chromium } from 'playwright'
import { mkdirSync, writeFileSync } from 'node:fs'

const BASE = process.env.AUDIT_BASE ?? 'http://localhost:5173'
const FOTO = process.env.AUDIT_FOTO ?? ''
const HANYA = process.env.AUDIT_HANYA ? new RegExp(process.env.AUDIT_HANYA) : null
const TANPA_MODAL = !!process.env.AUDIT_TANPA_MODAL
if (FOTO) mkdirSync(FOTO, { recursive: true })

const LEBAR = [320, 360, 375, 390, 414, 430, 480, 540, 600, 720, 768, 820, 912, 1024, 1180, 1280, 1366, 1440, 1920]
const LEBAR_FOTO = [360, 768, 1280]
const LEBAR_MODAL = [360, 1280]
const tinggi = (w) => (w < 600 ? 780 : w < 1024 ? 1024 : 900)

const TOKO = [
  ['beranda', '/'], ['kasir', '/kasir'], ['lainnya', '/lainnya'],
  ['tutup-shift', '/kasir/tutup-shift'], ['ganti-shift', '/kasir/ganti-shift'],
  ['riwayat', '/kasir/riwayat'], ['kas', '/kasir/kas'], ['belum-terkirim', '/kasir/belum-terkirim'],
  ['barang', '/barang'], ['barang-baru', '/barang/baru'], ['impor-barang', '/barang/impor'],
  ['master-barang', '/barang/master'], ['stok', '/stok'], ['koreksi-stok', '/stok/koreksi'],
  ['barang-masuk', '/stok/masuk'], ['opname', '/stok/opname'], ['transfer-stok', '/stok/transfer'],
  ['pelanggan', '/pelanggan'], ['kasbon', '/kasbon'], ['pengaturan', '/pengaturan'],
  ['toko', '/pengaturan/toko'], ['pengguna', '/pengaturan/pengguna'], ['peran', '/pengaturan/peran'],
  ['karyawan', '/sdm'], ['gaji', '/sdm/gaji'], ['kanal', '/kanal'], ['langganan', '/langganan'],
  ['prospek', '/crm'], ['kunjungan', '/crm/kunjungan'], ['laporan', '/laporan'],
  ['selamat-datang', '/selamat-datang'],
]
const DETAIL = [
  ['kartu-stok', '/stok', 'a[href*="/stok/kartu/"]'],
  ['ubah-barang', '/barang', 'a[href^="/barang/"]:not([href="/barang/baru"]):not([href="/barang/impor"]):not([href="/barang/master"])'],
]
const MITRA = [['mitra-dasbor', '/mitra'], ['mitra-prospek', '/mitra/prospek'], ['mitra-komisi', '/mitra/komisi']]
const PANEL_TAB = ['Mitra', 'Konfirmasi Pembayaran', 'Pengembalian Dana', 'Komisi & Pencairan', 'Antrean Notifikasi']

// Tombol yang aman ditekan untuk membuka modal: kata kerja "membuka", bukan
// "menjalankan". Yang merusak / keluar tidak pernah ditekan.
const BUKA = /^(\+\s*)?(tambah|daftarkan|baru|ubah|edit|catat|bayar|konfirmasi|terima|buat|atur|lihat|rincian|detail|pilih|setor|ambil|transfer|undang|koreksi|pindah|berhenti|tandai|tolak|setujui|jalankan|mulai|isi|scan|pindai|kirim ulang|batalkan pindah)/i
const JANGAN = /keluar|hapus|nonaktif|hentikan langganan|simpan|kirim konfirmasi|proses|posting|tutup shift|serah|masuk$|daftar$|sinkron/i

const temuan = []
const catat = (halaman, lebar, jenis, rincian) => temuan.push({ halaman, lebar, jenis, rincian })

/** Laporan ditulis ulang setiap halaman: bila audit macet di tengah, hasilnya tidak hilang. */
function simpan() {
  const perJenis = {}
  for (const t of temuan) perJenis[t.jenis] = (perJenis[t.jenis] ?? 0) + 1
  if (process.env.AUDIT_KELUARAN) writeFileSync(process.env.AUDIT_KELUARAN, JSON.stringify({ perJenis, temuan }, null, 1))
  return perJenis
}

const b = await chromium.launch({ channel: 'chrome' })

/**
 * HP & tablet diukur sebagai layar SENTUH (`pointer: coarse`), desktop sebagai
 * tetikus. Tanpa ini kelas `pointer-fine:h-10` ikut berlaku di "HP" audit dan
 * setiap tombol padat terlapor 40px — padahal di HP sungguhan tingginya 48px.
 */
const cdpHalaman = new WeakMap()
async function aturSentuh(p, sentuh) {
  let cdp = cdpHalaman.get(p)
  if (!cdp) {
    cdp = await p.context().newCDPSession(p)
    cdpHalaman.set(p, cdp)
  }
  await cdp.send('Emulation.setTouchEmulationEnabled', sentuh ? { enabled: true, maxTouchPoints: 5 } : { enabled: false })
}

async function periksaHalaman(p, nama, pakaiFoto = true) {
  for (const w of LEBAR) {
    await aturSentuh(p, w <= 1024)
    await p.setViewportSize({ width: w, height: tinggi(w) })
    await p.waitForTimeout(220)
    const h = await p.evaluate(periksaDom, { lebar: w })
    for (const [jenis, isi] of Object.entries(h)) {
      if (Array.isArray(isi) ? isi.length : isi) catat(nama, w, jenis, isi)
    }
    if (FOTO && pakaiFoto && LEBAR_FOTO.includes(w)) {
      await p.screenshot({ path: `${FOTO}/${nama}@${w}.png`, fullPage: true }).catch(() => {})
    }
  }
}

/** Dijalankan DI halaman. Harus mandiri (tanpa penutupan dari luar). */
function periksaDom({ lebar }) {
  const W = window.innerWidth
  const out = { gulirMendatar: false, keluarLayar: [], terpotong: [], rupiahTerpotong: [], terhimpit: [], menumpuk: [], targetKecil: [] }
  // Kotak yang BENAR-BENAR terlihat: dipotong wadah ber-overflow di atasnya.
  // Tanpa ini, menu samping yang tergulir keluar area geser terlapor
  // "menumpuk" dengan tombol di kaki navigasi — padahal tidak kelihatan.
  const kotakTampak = (el) => {
    const r = el.getBoundingClientRect()
    let [l, t, ka, b] = [r.left, r.top, r.right, r.bottom]
    for (let n = el.parentElement; n && n !== document.documentElement; n = n.parentElement) {
      const s = getComputedStyle(n)
      if (s.overflowX !== 'visible' || s.overflowY !== 'visible') {
        const q = n.getBoundingClientRect()
        l = Math.max(l, q.left); t = Math.max(t, q.top); ka = Math.min(ka, q.right); b = Math.min(b, q.bottom)
      }
    }
    return { left: l, top: t, right: ka, bottom: b, width: Math.max(0, ka - l), height: Math.max(0, b - t) }
  }
  const tampak = (el) => {
    const s = getComputedStyle(el)
    if (s.display === 'none' || s.visibility === 'hidden' || +s.opacity === 0) return false
    // Di balik dialog modal (Radix menyembunyikannya dari pembaca layar).
    if (el.closest('[aria-hidden="true"], [data-aria-hidden="true"], [inert]')) return false
    const r = el.getBoundingClientRect()
    if (r.width <= 2 || r.height <= 2) return false // sr-only
    const v = kotakTampak(el)
    return v.width > 0 && v.height > 0
  }
  // Bilah tetap/menempel (navigasi bawah, bilah keranjang) memang menutupi isi
  // yang digulir di bawahnya — itu bukan tumpang tindih.
  const menempel = (el) => {
    for (let n = el; n && n !== document.body; n = n.parentElement) {
      const pos = getComputedStyle(n).position
      if (pos === 'fixed' || pos === 'sticky') return true
    }
    return false
  }
  const dalamWadahGulir = (el) => {
    for (let n = el.parentElement; n && n !== document.body; n = n.parentElement) {
      const s = getComputedStyle(n)
      if (/(auto|scroll|hidden|clip)/.test(s.overflowX)) return true
    }
    return false
  }
  const nama = (el) =>
    (el.getAttribute('aria-label') || el.textContent || el.tagName).replace(/\s+/g, ' ').trim().slice(0, 50)

  out.gulirMendatar = document.documentElement.scrollWidth > W + 1

  // Keluar layar: elemen tampak yang melewati tepi, dan TIDAK berada di wadah
  // gulir (baris chip yang memang digeser tidak dihitung). Hanya pelaku
  // teratas yang dilaporkan.
  const pelaku = new Set()
  for (const el of document.body.querySelectorAll('*')) {
    if (!tampak(el)) continue
    const r = el.getBoundingClientRect()
    if ((r.right > W + 1 || r.left < -1) && !dalamWadahGulir(el) && getComputedStyle(el).position !== 'fixed') {
      if (el.parentElement && pelaku.has(el.parentElement)) { pelaku.add(el); continue }
      pelaku.add(el)
      out.keluarLayar.push(`${el.tagName.toLowerCase()}.${String(el.className).split(' ').slice(0, 3).join('.')} «${nama(el)}» ${Math.round(r.left)}→${Math.round(r.right)}`)
    }
  }

  // Teks terpotong: daun teks yang isinya lebih lebar dari kotaknya.
  for (const el of document.body.querySelectorAll('p, span, a, button, h1, h2, h3, label, td, th, dd, dt, li, div')) {
    if (!tampak(el)) continue
    const punyaTeks = [...el.childNodes].some((n) => n.nodeType === 3 && n.textContent.trim())
    if (!punyaTeks) continue
    if (el.scrollWidth <= el.clientWidth + 1) continue
    const s = getComputedStyle(el)
    if (!/(hidden|clip)/.test(s.overflowX)) continue
    const teks = el.textContent.replace(/\s+/g, ' ').trim()
    if (s.textOverflow === 'ellipsis') {
      if (/Rp\s?[\d.]+|^\d[\d.,]*$/.test(teks)) out.rupiahTerpotong.push(teks.slice(0, 60))
    } else {
      out.terpotong.push(teks.slice(0, 60))
    }
  }

  // Terhimpit: teks yang kotaknya disempitkan tetangganya sampai tak terbaca —
  // "B…" untuk nama, atau tanggal yang tersusun tegak per kata. Elipsis dan
  // teks membungkus sama-sama lolos dari pemeriksaan "terpotong".
  for (const el of document.body.querySelectorAll('p, span, a, h1, h2, h3, dt, dd, td, li')) {
    if (!tampak(el) || el.children.length > 0) continue
    const teks = el.textContent.replace(/\s+/g, ' ').trim()
    if (teks.length < 6) continue
    const r = el.getBoundingClientRect()
    const baris = Math.round(r.height / parseFloat(getComputedStyle(el).lineHeight || '20'))
    const elipsis = getComputedStyle(el).textOverflow === 'ellipsis' && el.scrollWidth > el.clientWidth + 1
    if (r.width < 56 && (elipsis || baris >= 3)) out.terhimpit.push(`«${teks.slice(0, 30)}» ${Math.round(r.width)}px ${elipsis ? 'elipsis' : baris + ' baris'}`)
  }

  // Tombol / tautan yang saling menumpuk (bukan bersarang).
  const aksi = [...document.querySelectorAll('button, a[href], [role="button"], input, [role="combobox"]')].filter(tampak)
  for (let i = 0; i < aksi.length; i++) {
    const a = kotakTampak(aksi[i])
    for (let j = i + 1; j < aksi.length; j++) {
      if (aksi[i].contains(aksi[j]) || aksi[j].contains(aksi[i])) continue
      if (menempel(aksi[i]) !== menempel(aksi[j])) continue
      const c = kotakTampak(aksi[j])
      const x = Math.min(a.right, c.right) - Math.max(a.left, c.left)
      const y = Math.min(a.bottom, c.bottom) - Math.max(a.top, c.top)
      if (x > 4 && y > 4 && x * y > 0.25 * Math.min(a.width * a.height, c.width * c.height)) {
        out.menumpuk.push(`«${nama(aksi[i])}» × «${nama(aksi[j])}»`)
      }
    }
  }

  // Target sentuh di lebar HP (44px, pedoman platform; ui/01 memakai 48 di 390).
  if (lebar <= 480) {
    for (const el of aksi) {
      if (el.tagName === 'INPUT' && (el.type === 'checkbox' || el.type === 'radio') && el.closest('label')) continue
      if (el.tagName === 'INPUT' && el.type === 'file') continue
      let r = el.getBoundingClientRect()
      if (el.tagName === 'INPUT') {
        const pb = el.parentElement?.getBoundingClientRect()
        if (pb && pb.height >= r.height && pb.height - r.height <= 8) r = pb
      }
      if (r.height < 44 || r.width < 24) out.targetKecil.push(`«${nama(el)}» ${Math.round(r.width)}×${Math.round(r.height)}`)
    }
  }
  for (const k of Object.keys(out)) if (Array.isArray(out[k])) out[k] = [...new Set(out[k])].slice(0, 12)
  return out
}

/** Membuka setiap modal yang aman di halaman ini dan memeriksanya. */
async function periksaModal(p, nama, jalur) {
  if (TANPA_MODAL) return
  for (const w of LEBAR_MODAL) {
    await p.setViewportSize({ width: w, height: tinggi(w) })
    await p.goto(BASE + jalur, { waitUntil: 'networkidle' }).catch(() => {})
    await p.waitForTimeout(700)
    const label = await p.evaluate((pola) => {
      const [buka, jangan] = pola.map((s) => new RegExp(s, 'i'))
      const hasil = []
      for (const el of document.querySelectorAll('button, [role="button"]')) {
        const r = el.getBoundingClientRect()
        if (!r.width || !r.height || el.disabled || el.closest('[role="dialog"]')) continue
        const t = (el.getAttribute('aria-label') || el.textContent || '').replace(/\s+/g, ' ').trim()
        if (t && buka.test(t) && !jangan.test(t)) hasil.push(t)
      }
      return [...new Set(hasil)].slice(0, 14)
    }, [BUKA.source, JANGAN.source])

    for (const t of label) {
      const tombol = p.getByRole('button', { name: t, exact: true }).first()
      if (!(await tombol.isVisible().catch(() => false))) continue
      const sebelum = p.url()
      await tombol.click({ timeout: 3000 }).catch(() => {})
      await p.waitForTimeout(500)
      const dlg = p.locator('[role="dialog"]').last()
      if (await dlg.isVisible().catch(() => false)) {
        const h = await dlg.evaluate((d) => {
          const r = d.getBoundingClientRect()
          const W = window.innerWidth, H = window.innerHeight
          const lebihTinggi = d.scrollHeight > d.clientHeight + 1
          const bisaGulir = lebihTinggi && /(auto|scroll)/.test(getComputedStyle(d).overflowY)
          const keluar = []
          for (const el of d.querySelectorAll('*')) {
            const e = el.getBoundingClientRect()
            if (e.width && e.height && (e.right > r.right + 1 || e.left < r.left - 1) && getComputedStyle(el).position !== 'fixed') {
              keluar.push((el.textContent || el.tagName).replace(/\s+/g, ' ').trim().slice(0, 40))
            }
          }
          return {
            judul: (d.querySelector('h2')?.textContent || '').trim(),
            diLuarLayar: r.left < -1 || r.right > W + 1 || r.top < -1 || r.bottom > H + 1,
            gulirMendatar: d.scrollWidth > d.clientWidth + 1,
            tidakBisaDigulir: lebihTinggi && !bisaGulir,
            keluar: [...new Set(keluar)].slice(0, 6),
            ukuran: `${Math.round(r.width)}×${Math.round(r.height)} @${Math.round(r.left)},${Math.round(r.top)}`,
          }
        })
        const judul = h.judul || t
        if (h.diLuarLayar) catat(`${nama} ⟶ modal «${judul}»`, w, 'modalDiLuarLayar', h.ukuran)
        if (h.gulirMendatar) catat(`${nama} ⟶ modal «${judul}»`, w, 'modalGulirMendatar', h.ukuran)
        if (h.tidakBisaDigulir) catat(`${nama} ⟶ modal «${judul}»`, w, 'modalTidakBisaDigulir', h.ukuran)
        if (h.keluar.length) catat(`${nama} ⟶ modal «${judul}»`, w, 'modalIsiKeluar', h.keluar)
        const dom = await p.evaluate(periksaDom, { lebar: w })
        if (dom.terpotong.length) catat(`${nama} ⟶ modal «${judul}»`, w, 'terpotong', dom.terpotong)
        if (dom.rupiahTerpotong.length) catat(`${nama} ⟶ modal «${judul}»`, w, 'rupiahTerpotong', dom.rupiahTerpotong)
        if (FOTO) await p.screenshot({ path: `${FOTO}/${nama}~${judul.replace(/[^\p{L}\p{N}]+/gu, '-').slice(0, 40)}@${w}.png` }).catch(() => {})
        await p.keyboard.press('Escape')
        await p.waitForTimeout(300)
        if (await dlg.isVisible().catch(() => false)) {
          catat(`${nama} ⟶ modal «${judul}»`, w, 'modalTidakTertutupEsc', '')
          await p.goto(BASE + jalur, { waitUntil: 'networkidle' }).catch(() => {})
        }
      }
      if (p.url() !== sebelum) await p.goto(BASE + jalur, { waitUntil: 'networkidle' }).catch(() => {})
      await p.waitForTimeout(200)
    }
  }
}

async function kunjungi(p, nama, jalur, { modal = true } = {}) {
  if (HANYA && !HANYA.test(nama)) return
  await p.setViewportSize({ width: 1280, height: 900 })
  await p.goto(BASE + jalur, { waitUntil: 'networkidle', timeout: 25000 }).catch(() => {})
  await p.waitForTimeout(900)
  await periksaHalaman(p, nama)
  if (modal) await periksaModal(p, nama, jalur)
  simpan()
  process.stderr.write(`✓ ${nama}\n`)
}

// ── Publik ───────────────────────────────────────────────────────────────
{
  const p = await (await b.newContext()).newPage()
  for (const [nama, jalur] of [['masuk', '/masuk'], ['daftar', '/daftar'], ['mitra-masuk', '/mitra'], ['panel-masuk', '/panel']]) {
    await kunjungi(p, nama, jalur, { modal: false })
  }
}

// ── Toko ─────────────────────────────────────────────────────────────────
{
  const p = await (await b.newContext()).newPage()
  await p.goto(`${BASE}/masuk`, { waitUntil: 'networkidle' })
  await p.getByLabel('Nama pengguna').fill(process.env.AUDIT_TOKO_USER ?? 'sari')
  await p.getByLabel('Kata sandi').fill(process.env.AUDIT_TOKO_SANDI ?? 'rahasia123')
  await p.getByRole('button', { name: 'Masuk', exact: true }).click()
  await p.waitForURL((u) => !u.pathname.includes('masuk'), { timeout: 15000 })
  await p.waitForTimeout(1500)

  const semua = [...TOKO]
  for (const [nama, dari, pemilih] of DETAIL) {
    await p.goto(BASE + dari, { waitUntil: 'networkidle' })
    await p.waitForTimeout(1200)
    const jalur = await p.evaluate((sel) => document.querySelector(sel)?.getAttribute('href') ?? null, pemilih)
    if (jalur) semua.push([nama, jalur])
  }
  for (const [nama, jalur] of semua) await kunjungi(p, nama, jalur)

  // Kasir dengan isi keranjang: layar bayar hanya terbuka bila ada barang.
  if (!HANYA || HANYA.test('kasir-bayar')) {
    await p.setViewportSize({ width: 1280, height: 900 })
    await p.goto(`${BASE}/kasir`, { waitUntil: 'networkidle' })
    await p.waitForTimeout(1200)
    // Barang yang habis tombolnya nonaktif — pilih yang bisa dijual.
    const petak = p.locator('main button:not([disabled])').filter({ hasText: /Rp/ }).first()
    if (await petak.isVisible().catch(() => false)) {
      await petak.click()
      await p.waitForTimeout(400)
      await periksaHalaman(p, 'kasir-isi')
      for (const w of LEBAR_MODAL) {
        await p.setViewportSize({ width: w, height: tinggi(w) })
        await p.waitForTimeout(300)
        const bayar = p.getByRole('button', { name: /^BAYAR/ }).first()
        if (await bayar.isVisible().catch(() => false)) {
          await bayar.click()
          await p.waitForTimeout(600)
          await periksaHalaman(p, `kasir-layar-bayar`, w === 360)
          await p.keyboard.press('Escape')
          await p.waitForTimeout(300)
        }
      }
    }
  }
}

// ── Panel internal ──────────────────────────────────────────────────────
if (process.env.AUDIT_PANEL_EMAIL) {
  const p = await (await b.newContext()).newPage()
  await p.goto(`${BASE}/panel`, { waitUntil: 'networkidle' })
  await p.getByLabel('Email').fill(process.env.AUDIT_PANEL_EMAIL)
  await p.getByLabel('Kata sandi').fill(process.env.AUDIT_PANEL_SANDI)
  await p.getByRole('button', { name: 'Masuk', exact: true }).click()
  await p.waitForTimeout(1500)
  for (const tab of PANEL_TAB) {
    const nama = `panel-${tab.toLowerCase().replace(/[^a-z]+/g, '-')}`
    if (HANYA && !HANYA.test(nama)) continue
    await p.setViewportSize({ width: 1280, height: 900 })
    const t = p.getByRole('button', { name: new RegExp(`^${tab.replace('&', '\\&')}`) }).first()
    if (!(await t.isVisible().catch(() => false))) continue
    await t.click()
    await p.waitForTimeout(900)
    await periksaHalaman(p, nama)
    simpan()
    process.stderr.write(`✓ ${nama}\n`)
  }
}

// ── Portal mitra ────────────────────────────────────────────────────────
if (process.env.AUDIT_MITRA_EMAIL) {
  const p = await (await b.newContext()).newPage()
  await p.goto(`${BASE}/mitra`, { waitUntil: 'networkidle' })
  await p.getByLabel('Email').fill(process.env.AUDIT_MITRA_EMAIL)
  await p.getByLabel('Kata sandi').fill(process.env.AUDIT_MITRA_SANDI)
  await p.getByRole('button', { name: /^Masuk/ }).first().click()
  await p.waitForTimeout(1500)
  for (const [nama, jalur] of MITRA) await kunjungi(p, nama, jalur)
}

await b.close()

// ── Ringkasan ───────────────────────────────────────────────────────────
const perJenis = simpan()
if (process.env.AUDIT_KELUARAN) console.log(JSON.stringify(perJenis))
else console.log(JSON.stringify({ perJenis, temuan }, null, 1))
