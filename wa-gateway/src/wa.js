// Pengelola sambungan WhatsApp lewat Baileys.
//
// Baileys bukan API resmi: ia meniru WhatsApp Web lewat WebSocket. Konsekuensinya
// nyata dan membentuk hampir semua keputusan di berkas ini:
//
//   - Sambungan PUTUS sendiri — sinyal, WhatsApp merestart sesi, ponsel mati.
//     Jadi status sambungan adalah keadaan yang dikelola, bukan asumsi.
//   - Nomor bisa DIBLOKIR bila berkelakuan seperti robot. Jadi pengiriman
//     diserialkan dengan jeda minimum, bukan dihambur secepat mungkin.
//   - Sesi bisa DIAKHIRI dari ponsel ("Perangkat tertaut" → keluar). Itu tidak
//     bisa dipulihkan dengan mencoba ulang; hanya pindai QR baru yang menolong.
//
// Dokumen resminya: https://baileys.wiki/

import makeWASocket, {
  DisconnectReason,
  fetchLatestBaileysVersion,
  makeCacheableSignalKeyStore,
  useMultiFileAuthState,
} from '@whiskeysockets/baileys'
import qrcode from 'qrcode-terminal'

/**
 * Galat yang PANTAS dicoba ulang: sambungan sedang putus, WhatsApp sedang rewel.
 * Backend memetakannya ke HTTP 503 lalu menunda percobaan berikutnya.
 */
export class GalatSementara extends Error {
  constructor(pesan) {
    super(pesan)
    this.name = 'GalatSementara'
  }
}

/**
 * Galat yang TIDAK akan membaik dengan dicoba ulang: nomornya tidak terdaftar
 * di WhatsApp, atau bentuknya memang salah. Backend memetakannya ke HTTP 422
 * dan langsung menaruh peristiwanya di antrean mati — mengulang 10 kali selama
 * berjam-jam tidak akan membuat nomor yang tidak ada jadi ada.
 */
export class GalatPermanen extends Error {
  constructor(pesan) {
    super(pesan)
    this.name = 'GalatPermanen'
  }
}

const tidur = (ms) => new Promise((r) => setTimeout(r, ms))

/**
 * Membuka dan menjaga satu sambungan WhatsApp.
 *
 * @param {object} opsi
 * @param {string} opsi.authDir  folder kredensial (WAJIB persisten — lihat README)
 * @param {number} opsi.jedaKirimMs jeda minimum antar pesan
 * @param {import('pino').Logger} opsi.log
 */
export function buatKoneksi({ authDir, jedaKirimMs, log }) {
  /** @type {'memulai'|'menunggu-pindai'|'tersambung'|'terputus'|'keluar'} */
  let status = 'memulai'
  let sock = null
  let qrTerakhir = null
  let alasanTerakhir = ''
  let percobaanSambung = 0
  let berhenti = false

  // Hasil pemeriksaan "nomor ini ada di WhatsApp?" disimpan sebentar. Tanpa ini
  // setiap percobaan ulang menanyakan hal yang sama ke server WhatsApp, dan
  // lalu lintas berpola robot itulah yang memancing pemblokiran.
  const cacheNomor = new Map()
  const UMUR_CACHE_MS = 6 * 60 * 60 * 1000

  async function sambung() {
    if (berhenti) return

    const { state, saveCreds } = await useMultiFileAuthState(authDir)

    // Versi protokol diambil dari jaringan karena WhatsApp sering menaikkannya;
    // versi bawaan paket cepat basi. Gagal ambil bukan alasan berhenti.
    let version
    try {
      ;({ version } = await fetchLatestBaileysVersion())
    } catch (e) {
      log.warn({ err: e?.message }, 'gagal mengambil versi terbaru, pakai bawaan paket')
    }

    sock = makeWASocket({
      version,
      auth: {
        creds: state.creds,
        // Key store di-cache: tanpa ini setiap pesan membaca banyak berkas kecil.
        keys: makeCacheableSignalKeyStore(state.keys, log),
      },
      logger: log.child({ modul: 'baileys' }),
      // Sidecar ini hanya MENGIRIM. Menarik seluruh riwayat percakapan memakan
      // memori dan waktu sambung tanpa memberi apa pun yang kita pakai.
      syncFullHistory: false,
      shouldSyncHistoryMessage: () => false,
      // Jangan menandai "online" terus-menerus: ponsel pemilik jadi tidak
      // menerima notifikasi pesan masuk selama sidecar hidup.
      markOnlineOnConnect: false,
    })

    sock.ev.on('creds.update', saveCreds)

    sock.ev.on('connection.update', (u) => {
      const { connection, lastDisconnect, qr } = u

      if (qr) {
        // printQRInTerminal sudah DIHAPUS di Baileys v7, jadi QR dirender sendiri.
        qrTerakhir = qr
        status = 'menunggu-pindai'
        log.warn('WhatsApp belum tertaut — pindai QR di bawah ini dengan menu "Perangkat tertaut"')
        qrcode.generate(qr, { small: true })
      }

      if (connection === 'open') {
        status = 'tersambung'
        qrTerakhir = null
        alasanTerakhir = ''
        percobaanSambung = 0
        log.info({ nomor: sock?.user?.id }, 'WhatsApp tersambung')
      }

      if (connection === 'close') {
        // Galat dari Baileys sudah berbentuk Boom, jadi kode statusnya
        // terbaca langsung — tidak perlu menarik @hapi/boom sebagai
        // dependensi sendiri hanya untuk membungkus ulang.
        const kode = lastDisconnect?.error?.output?.statusCode
        alasanTerakhir = namaAlasan(kode)

        if (kode === DisconnectReason.loggedOut) {
          // Sesi diakhiri dari ponsel. Kredensial di disk sudah tidak berguna;
          // menyambung ulang hanya akan ditolak berulang kali. Butuh manusia.
          status = 'keluar'
          log.error(
            `Sesi WhatsApp DIAKHIRI dari ponsel. Hapus folder "${authDir}" lalu jalankan ulang untuk memindai QR baru.`,
          )
          return
        }

        status = 'terputus'
        if (berhenti) return

        // Menyambung ulang dengan jeda bertambah. Menyambung tanpa jeda saat
        // WhatsApp menolak adalah cara tercepat membuat nomor dicurigai.
        percobaanSambung++
        const jeda = Math.min(30_000, 1000 * 2 ** Math.min(percobaanSambung, 5))
        log.warn(
          { kode, alasan: alasanTerakhir, percobaan: percobaanSambung },
          `sambungan putus, mencoba lagi dalam ${jeda / 1000} detik`,
        )
        setTimeout(() => void sambung().catch((e) => log.error({ err: e }, 'gagal menyambung ulang')), jeda)
      }
    })
  }

  // ── Antrean kirim ────────────────────────────────────────────────────────
  //
  // Semua pengiriman lewat satu rantai promise dengan jeda minimum. Dua alasan:
  // mengirim berbarengan membuat pola lalu lintas yang tidak manusiawi (risiko
  // blokir), dan urutan pesan jadi tidak terduga.
  let rantai = Promise.resolve()
  let waktuKirimTerakhir = 0

  function antre(tugas) {
    const hasil = rantai.then(async () => {
      const sisa = jedaKirimMs - (Date.now() - waktuKirimTerakhir)
      if (sisa > 0) await tidur(sisa)
      try {
        return await tugas()
      } finally {
        waktuKirimTerakhir = Date.now()
      }
    })
    // Rantai tidak boleh putus gara-gara satu pengiriman gagal.
    rantai = hasil.then(
      () => undefined,
      () => undefined,
    )
    return hasil
  }

  /**
   * Mengirim satu pesan teks.
   * @param {string} nomor MSISDN angka saja, mis. "628123456789"
   * @param {string} isi
   * @returns {Promise<{id: string, jid: string}>}
   */
  async function kirim(nomor, isi) {
    if (!/^\d{8,15}$/.test(nomor)) {
      throw new GalatPermanen(`nomor "${nomor}" bukan MSISDN yang sah`)
    }
    if (!isi || !isi.trim()) {
      throw new GalatPermanen('isi pesan kosong')
    }
    if (status === 'keluar') {
      throw new GalatSementara('sesi WhatsApp sudah diakhiri dari ponsel, perlu pindai QR ulang')
    }
    if (status !== 'tersambung' || !sock) {
      throw new GalatSementara(`WhatsApp belum siap (status: ${status}${alasanTerakhir ? ', ' + alasanTerakhir : ''})`)
    }

    return antre(async () => {
      const jid = await cariJid(nomor)
      try {
        const hasil = await sock.sendMessage(jid, { text: isi })
        return { id: hasil?.key?.id ?? '', jid }
      } catch (e) {
        // Kegagalan di tahap ini hampir selalu soal sambungan, bukan soal
        // nomornya — nomornya sudah diperiksa di atas. Jadi: boleh diulang.
        throw new GalatSementara(`gagal mengirim: ${e?.message ?? e}`)
      }
    })
  }

  /** Memastikan nomor terdaftar di WhatsApp, dan mengembalikan JID resminya. */
  async function cariJid(nomor) {
    const tersimpan = cacheNomor.get(nomor)
    if (tersimpan && Date.now() - tersimpan.waktu < UMUR_CACHE_MS) {
      if (!tersimpan.jid) throw new GalatPermanen(`nomor ${nomor} tidak terdaftar di WhatsApp`)
      return tersimpan.jid
    }

    let hasil
    try {
      hasil = await sock.onWhatsApp(nomor)
    } catch (e) {
      throw new GalatSementara(`gagal memeriksa nomor: ${e?.message ?? e}`)
    }

    const cocok = hasil?.find((r) => r?.exists)
    if (!cocok) {
      // Ditandai permanen: nomor yang tidak punya WhatsApp tidak akan tiba-tiba
      // punya dalam beberapa jam ke depan. Operator harus memperbaiki datanya.
      cacheNomor.set(nomor, { jid: null, waktu: Date.now() })
      throw new GalatPermanen(`nomor ${nomor} tidak terdaftar di WhatsApp`)
    }
    cacheNomor.set(nomor, { jid: cocok.jid, waktu: Date.now() })
    return cocok.jid
  }

  function keadaan() {
    return {
      status,
      nomor: sock?.user?.id?.split(':')[0] ?? null,
      nama: sock?.user?.name ?? null,
      alasan_terakhir: alasanTerakhir || null,
      perlu_pindai: status === 'menunggu-pindai' || status === 'keluar',
    }
  }

  async function tutup() {
    berhenti = true
    try {
      // end() melepas WebSocket tanpa mengakhiri sesi — kredensial tetap sah,
      // jadi restart berikutnya tidak meminta pindai QR lagi.
      sock?.end(undefined)
    } catch {
      /* menutup yang sudah tertutup bukan galat */
    }
  }

  return {
    mulai: sambung,
    kirim,
    keadaan,
    tutup,
    get qr() {
      return qrTerakhir
    },
  }
}

function namaAlasan(kode) {
  for (const [nama, nilai] of Object.entries(DisconnectReason)) {
    if (nilai === kode && Number.isNaN(Number(nama))) return nama
  }
  return kode ? `kode ${kode}` : 'tidak diketahui'
}
