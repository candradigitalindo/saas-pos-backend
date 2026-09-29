// Sidecar WhatsApp: satu-satunya jembatan antara backend Go dan Baileys.
//
// Kenapa ada proses terpisah? Baileys adalah pustaka Node yang bicara WebSocket
// ke WhatsApp Web — tidak ada cara memanggilnya dari Go. Jadi Node yang memegang
// sambungan WhatsApp, dan Go memanggilnya lewat HTTP lokal.
//
// KEAMANAN. Siapa pun yang bisa memanggil layanan ini bisa mengirim WhatsApp
// atas nama usaha Anda. Karena itu bawaannya mengikat ke 127.0.0.1 saja dan
// menolak jalan tanpa WA_GATEWAY_TOKEN. Bila perlu dijangkau dari mesin lain,
// taruh di belakang reverse proxy ber-TLS — jangan buka portnya apa adanya.

import { createServer } from 'node:http'
import { timingSafeEqual } from 'node:crypto'
import pino from 'pino'
import { buatKoneksi, GalatPermanen, GalatSementara } from './wa.js'

const env = (k, bawaan) => {
  const v = process.env[k]
  return v === undefined || v.trim() === '' ? bawaan : v.trim()
}

const HOST = env('WA_GATEWAY_HOST', '127.0.0.1')
const PORT = Number(env('WA_GATEWAY_PORT', '8090'))
const TOKEN = env('WA_GATEWAY_TOKEN', '')
const AUTH_DIR = env('WA_AUTH_DIR', './auth')
const JEDA_KIRIM_MS = Number(env('WA_JEDA_KIRIM_MS', '2000'))
const BATAS_BODY = 64 * 1024

const log = pino({
  level: env('WA_LOG_LEVEL', 'info'),
  transport: env('WA_LOG_FORMAT', 'text') === 'text' ? { target: 'pino-pretty' } : undefined,
})

if (!TOKEN) {
  log.error(
    'WA_GATEWAY_TOKEN belum diisi. Layanan ini bisa mengirim WhatsApp atas nama usaha Anda, ' +
      'jadi ia menolak jalan tanpa token. Buat satu dengan: openssl rand -hex 32',
  )
  process.exit(1)
}
if (TOKEN.length < 32) {
  log.error('WA_GATEWAY_TOKEN terlalu pendek (minimal 32 karakter).')
  process.exit(1)
}

const wa = buatKoneksi({ authDir: AUTH_DIR, jedaKirimMs: JEDA_KIRIM_MS, log })

// ── Pembantu HTTP ─────────────────────────────────────────────────────────

function balas(res, kode, isi) {
  const teks = JSON.stringify(isi)
  res.writeHead(kode, {
    'Content-Type': 'application/json; charset=utf-8',
    'Content-Length': Buffer.byteLength(teks),
  })
  res.end(teks)
}

/** Membandingkan token tanpa membocorkan panjang kecocokan lewat waktu. */
function tokenSah(header) {
  const dikirim = (header ?? '').replace(/^Bearer\s+/i, '')
  const a = Buffer.from(dikirim)
  const b = Buffer.from(TOKEN)
  if (a.length !== b.length) return false
  return timingSafeEqual(a, b)
}

function bacaBody(req) {
  return new Promise((resolve, reject) => {
    let data = ''
    let ukuran = 0
    req.on('data', (c) => {
      ukuran += c.length
      if (ukuran > BATAS_BODY) {
        reject(new Error('body terlalu besar'))
        req.destroy()
        return
      }
      data += c
    })
    req.on('end', () => resolve(data))
    req.on('error', reject)
  })
}

// ── Rute ──────────────────────────────────────────────────────────────────

const server = createServer(async (req, res) => {
  const jalur = (req.url ?? '').split('?')[0]

  // Liveness sengaja TANPA token: ia hanya menjawab "proses ini hidup", tidak
  // membocorkan apa pun, dan pemeriksa kesehatan tidak perlu memegang rahasia.
  if (jalur === '/health' && req.method === 'GET') {
    return balas(res, 200, { status: 'hidup' })
  }

  if (!tokenSah(req.headers.authorization)) {
    return balas(res, 401, { galat: 'token tidak sah' })
  }

  if (jalur === '/status' && req.method === 'GET') {
    return balas(res, 200, wa.keadaan())
  }

  // QR untuk menautkan nomor. Isinya rahasia sesaat — siapa pun yang memindainya
  // menjadi perangkat tertaut, jadi jalur ini ikut berpagar token.
  if (jalur === '/qr' && req.method === 'GET') {
    const qr = wa.qr
    if (!qr) return balas(res, 404, { galat: 'tidak ada QR menunggu', ...wa.keadaan() })
    return balas(res, 200, { qr, ...wa.keadaan() })
  }

  if (jalur === '/kirim' && req.method === 'POST') {
    let body
    try {
      body = JSON.parse(await bacaBody(req))
    } catch (e) {
      return balas(res, 400, { galat: `body bukan JSON yang sah: ${e.message}` })
    }

    const { to, isi, topik_id: topikID } = body ?? {}
    if (typeof to !== 'string' || typeof isi !== 'string') {
      return balas(res, 400, { galat: 'wajib memuat "to" dan "isi" berupa teks' })
    }

    try {
      const hasil = await wa.kirim(to, isi)
      log.info({ ke: to, topik_id: topikID, pesan_id: hasil.id }, 'pesan terkirim')
      return balas(res, 200, { status: 'terkirim', pesan_id: hasil.id, jid: hasil.jid })
    } catch (e) {
      if (e instanceof GalatPermanen) {
        log.warn({ ke: to, topik_id: topikID, err: e.message }, 'ditolak permanen')
        return balas(res, 422, { galat: e.message, permanen: true })
      }
      if (e instanceof GalatSementara) {
        log.warn({ ke: to, topik_id: topikID, err: e.message }, 'gagal sementara')
        return balas(res, 503, { galat: e.message, permanen: false })
      }
      log.error({ ke: to, topik_id: topikID, err: e?.message }, 'galat tak terduga')
      return balas(res, 503, { galat: e?.message ?? 'galat tak terduga', permanen: false })
    }
  }

  balas(res, 404, { galat: 'rute tidak dikenal' })
})

server.listen(PORT, HOST, () => {
  log.info(`sidecar WhatsApp mendengarkan di http://${HOST}:${PORT}`)
  log.info(`kredensial di "${AUTH_DIR}" — folder ini WAJIB ikut dicadangkan`)
  void wa.mulai().catch((e) => log.error({ err: e }, 'gagal memulai sambungan WhatsApp'))
})

for (const sinyal of ['SIGINT', 'SIGTERM']) {
  process.on(sinyal, () => {
    log.info('menutup sidecar')
    void wa.tutup()
    server.close(() => process.exit(0))
    // Jangan menggantung selamanya bila ada koneksi yang enggan tutup.
    setTimeout(() => process.exit(0), 5000).unref()
  })
}
