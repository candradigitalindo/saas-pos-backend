/**
 * Satu-satunya pintu ke backend.
 *
 * Dua tugasnya (ui/03-ARSITEKTUR-FRONTEND.md):
 *   1. Membuka amplop respons SEKALI, sehingga komponen hanya berurusan dengan
 *      `data` — tidak ada `res.data.data.data` bertebaran di layar.
 *   2. Menerjemahkan kode status jadi kesalahan yang bisa dibaca manusia.
 *      Pengguna tidak pernah melihat angka 401/422 di layar.
 */

import { ambilSesi, hapusSesi, simpanSesi } from './penyimpanan-sesi'

export const BASIS_API = import.meta.env.VITE_API_URL ?? '/api/v1'

/** Bentuk amplop sukses dari backend. */
interface AmplopSukses<T> {
  success: true
  message: string
  data: T
}

/** Bentuk amplop gagal dari backend. */
interface AmplopGagal {
  success: false
  message: string
  errors?: Record<string, string>
}

/** Halaman data, sebagaimana dikembalikan backend (bentuk mirip Laravel). */
export interface Halaman<T> {
  current_page: number
  data: T[]
  last_page: number
  per_page: number
  total: number
  from: number
  to: number
  next_page_url: string | null
  prev_page_url: string | null
}

/**
 * Kesalahan yang sudah diterjemahkan. `pesan` SELALU kalimat bahasa Indonesia
 * yang layak ditampilkan apa adanya ke pengguna.
 */
export class GalatAPI extends Error {
  readonly status: number
  /** Pesan per kolom untuk 422 — ditempel di bawah kolom yang bersangkutan. */
  readonly kolom: Record<string, string>
  /** true bila kegagalan jaringan/server, artinya aman untuk diantre ulang. */
  readonly bisaDiantre: boolean

  constructor(
    status: number,
    pesan: string,
    kolom: Record<string, string> = {},
    bisaDiantre = false,
  ) {
    super(pesan)
    this.name = 'GalatAPI'
    this.status = status
    this.kolom = kolom
    this.bisaDiantre = bisaDiantre
  }

  get pesan(): string {
    return this.message
  }
}

/** Dipakai router untuk melempar pengguna ke halaman masuk. */
export const GALAT_SESI_HABIS = 'sesi-habis'

interface OpsiMinta {
  metode?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  badan?: unknown
  query?: Record<string, string | number | boolean | undefined | null>
  /** Wajib untuk aksi yang menciptakan uang atau stok. */
  idempotencyKey?: string
  signal?: AbortSignal
  /** Portal mitra & panel internal memakai realm token yang berbeda. */
  realm?: Realm
  /** Lewati penyisipan token — dipakai endpoint publik (masuk, daftar). */
  tanpaToken?: boolean
  /**
   * Kirim badan apa adanya dengan tipe konten ini, bukan sebagai JSON.
   * Dipakai impor CSV. Tetap lewat jalur yang sama supaya pembaruan token dan
   * penerjemahan galat tidak perlu ditulis ulang di tempat kedua.
   */
  mentah?: { isi: BodyInit; tipe: string }
  /**
   * Kirim sebagai multipart (unggah berkas).
   *
   * Sengaja TERPISAH dari `mentah`: Content-Type multipart harus memuat
   * boundary yang dibuat browser, jadi header itu justru TIDAK boleh diisi
   * sendiri. Menyetelnya manual membuat server gagal mengurai berkasnya.
   */
  formulir?: FormData
}

export type Realm = 'tenant' | 'mitra' | 'platform'

// ── Pembaruan token ────────────────────────────────────────────────────────
//
// Satu pembaruan berjalan pada satu waktu. Permintaan lain yang kena 401
// MENUNGGU pembaruan itu, bukan memicu pembaruannya sendiri — tanpa ini, lima
// permintaan bersamaan menghasilkan lima kali refresh dan empat token terbuang.
//
// Dua aturan tambahan yang dulu tidak ada, keduanya pernah menendang kasir
// keluar di tengah jualan:
//
//   1. Gagal SEMENTARA bukan sesi habis. Refresh yang putus di jalan (sinyal
//      hilang sesaat) atau dibalas 5xx/429 dulu langsung menghapus sesi. Kini
//      sesi hanya dibuang bila server benar-benar MENOLAK refresh token-nya;
//      selain itu permintaannya gagal seperti gangguan jaringan biasa dan
//      bisa diantre, sesi tetap utuh.
//
//   2. Satu refresh untuk SEMUA tab. Refresh token dirotasi setiap dipakai,
//      dan server menganggap refresh token lama yang dipakai lagi sebagai
//      tanda PENCURIAN lalu mencabut seluruh sesi. Dua tab yang kedaluwarsa
//      bersamaan dulu sama-sama menukar token yang sama — tab kedua memicu
//      pencabutan dan keduanya terlempar. Kini penukaran dikunci lintas tab
//      (Web Locks), dan sebelum menukar, token tersimpan dibaca ulang: bila
//      tab lain sudah memperbaruinya, token itu yang dipakai.

type HasilPembaruan =
  | { jenis: 'baru'; token: string }
  /** Server menolak refresh token — sesi memang sudah habis. */
  | { jenis: 'ditolak' }
  /** Jaringan putus / server bermasalah — sesi tetap, coba lagi nanti. */
  | { jenis: 'sementara'; sebab: 'jaringan' | 'server' }

const pembaruanBerjalan = new Map<Realm, Promise<HasilPembaruan>>()

async function perbaruiToken(realm: Realm, tokenDipakai: string): Promise<HasilPembaruan> {
  const sedang = pembaruanBerjalan.get(realm)
  if (sedang) return sedang

  const jalan = denganKunciLintasTab(`pos.perbarui-token.${realm}`, () =>
    tukarRefreshToken(realm, tokenDipakai),
  )
  pembaruanBerjalan.set(realm, jalan)
  try {
    return await jalan
  } finally {
    pembaruanBerjalan.delete(realm)
  }
}

/** Menjalankan fn di bawah kunci Web Locks bila peramban mendukungnya. */
async function denganKunciLintasTab<T>(nama: string, fn: () => Promise<T>): Promise<T> {
  const kunci = typeof navigator !== 'undefined' ? navigator.locks : undefined
  if (!kunci?.request) return fn()
  return kunci.request(nama, fn) as Promise<T>
}

async function tukarRefreshToken(realm: Realm, tokenDipakai: string): Promise<HasilPembaruan> {
  const sesi = ambilSesi(realm)
  if (!sesi?.refresh_token) return { jenis: 'ditolak' }

  // Sudah diperbarui tab lain (atau permintaan lain) selagi kita menunggu
  // kunci: token tersimpan kini berbeda dan masih berlaku. Menukar refresh
  // token lagi justru akan memakai token yang sudah dirotasi.
  if (sesi.access_token && sesi.access_token !== tokenDipakai) {
    return { jenis: 'baru', token: sesi.access_token }
  }

  // Hanya realm tenant yang punya /auth/refresh. Realm lain: sesi habis =
  // masuk ulang.
  if (realm !== 'tenant') return { jenis: 'ditolak' }

  let res: Response
  try {
    res = await fetch(`${BASIS_API}/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: sesi.refresh_token }),
    })
  } catch {
    return { jenis: 'sementara', sebab: 'jaringan' }
  }
  if (res.status >= 500 || res.status === 429) return { jenis: 'sementara', sebab: 'server' }
  if (!res.ok) return { jenis: 'ditolak' }

  try {
    const amplop = (await res.json()) as AmplopSukses<{
      access_token: string
      refresh_token: string
      expires_in: number
    }>
    simpanSesi(realm, {
      ...sesi,
      access_token: amplop.data.access_token,
      refresh_token: amplop.data.refresh_token,
      kedaluwarsa: Date.now() + amplop.data.expires_in * 1000,
    })
    return { jenis: 'baru', token: amplop.data.access_token }
  } catch {
    // Balasan terpotong di jalan: refresh token lama mungkin sudah dirotasi
    // server, tapi itu tetap bukan alasan membuang sesi diam-diam di sini.
    return { jenis: 'sementara', sebab: 'jaringan' }
  }
}

function susunURL(jalur: string, query?: OpsiMinta['query']): string {
  const url = `${BASIS_API}${jalur}`
  if (!query) return url
  const p = new URLSearchParams()
  for (const [k, v] of Object.entries(query)) {
    if (v === undefined || v === null || v === '') continue
    p.set(k, String(v))
  }
  const s = p.toString()
  return s ? `${url}?${s}` : url
}

/**
 * Menerjemahkan balasan gagal jadi GalatAPI dengan kalimat yang sudah manusiawi.
 * Peta ini berasal dari tabel "kode status → perilaku UI" di dokumen arsitektur.
 */
async function terjemahkanGalat(res: Response): Promise<GalatAPI> {
  let badan: AmplopGagal | null = null
  try {
    badan = (await res.json()) as AmplopGagal
  } catch {
    // Balasan bukan JSON (mis. proxy mati) — tetap ditangani di bawah.
  }

  const kolom = badan?.errors ?? {}
  const dariServer = badan?.message?.trim()

  switch (res.status) {
    case 400:
      return new GalatAPI(400, dariServer || 'Data yang dikirim belum lengkap.', kolom)
    case 401:
      return new GalatAPI(401, GALAT_SESI_HABIS, kolom)
    case 403:
      return new GalatAPI(403, 'Fitur ini tidak tersedia untuk akun Anda.', kolom)
    case 404:
      return new GalatAPI(404, 'Data tidak ditemukan.', kolom)
    case 409:
      // Bentrok: sudah dibatalkan, sudah dikunci, nomor sudah dipakai. Pesan
      // dari server lebih spesifik daripada apa pun yang bisa ditebak di sini.
      return new GalatAPI(409, dariServer || 'Data ini sudah berubah. Muat ulang dulu.', kolom)
    case 422:
      return new GalatAPI(422, dariServer || 'Ada isian yang belum benar.', kolom)
    case 429:
      return new GalatAPI(429, 'Terlalu banyak percobaan. Coba lagi sebentar lagi.', kolom)
    default:
      if (res.status >= 500) {
        return new GalatAPI(
          res.status,
          'Server sedang bermasalah. Data Anda tidak hilang — coba lagi sebentar lagi.',
          kolom,
          true,
        )
      }
      return new GalatAPI(res.status, dariServer || 'Terjadi kesalahan.', kolom)
  }
}

async function kirim<T>(jalur: string, opsi: OpsiMinta, ulangi: boolean): Promise<T> {
  const realm = opsi.realm ?? 'tenant'
  const headers: Record<string, string> = { Accept: 'application/json' }

  if (opsi.formulir) {
    // Content-Type sengaja dibiarkan kosong — lihat catatan pada `formulir`.
  } else if (opsi.mentah) headers['Content-Type'] = opsi.mentah.tipe
  else if (opsi.badan !== undefined) headers['Content-Type'] = 'application/json'
  if (opsi.idempotencyKey) headers['Idempotency-Key'] = opsi.idempotencyKey
  let tokenDipakai = ''
  if (!opsi.tanpaToken) {
    tokenDipakai = ambilSesi(realm)?.access_token ?? ''
    if (tokenDipakai) headers.Authorization = `Bearer ${tokenDipakai}`
  }

  let res: Response
  try {
    res = await fetch(susunURL(jalur, opsi.query), {
      method: opsi.metode ?? 'GET',
      headers,
      body: opsi.formulir
        ? opsi.formulir
        : opsi.mentah
          ? opsi.mentah.isi
          : opsi.badan === undefined
            ? undefined
            : JSON.stringify(opsi.badan),
      signal: opsi.signal,
    })
  } catch (e) {
    if (e instanceof DOMException && e.name === 'AbortError') throw e
    // Jaringan mati. Untuk aksi yang bisa diantre, pemanggil akan menyimpannya
    // ke antrean offline alih-alih menampilkan kesalahan.
    throw new GalatAPI(
      0,
      'Belum ada internet. Data disimpan di HP dan dikirim otomatis nanti.',
      {},
      true,
    )
  }

  if (res.status === 401 && ulangi && !opsi.tanpaToken) {
    const hasil = await perbaruiToken(realm, tokenDipakai)
    if (hasil.jenis === 'baru') return kirim<T>(jalur, opsi, false)
    if (hasil.jenis === 'sementara') {
      // Sesi TIDAK dibuang: gangguan ini lewat sendiri, dan aksi yang bisa
      // diantre (checkout) masuk antrean offline alih-alih gagal.
      throw hasil.sebab === 'jaringan'
        ? new GalatAPI(0, 'Belum ada internet. Data disimpan di HP dan dikirim otomatis nanti.', {}, true)
        : new GalatAPI(503, 'Server sedang bermasalah. Data Anda tidak hilang — coba lagi sebentar lagi.', {}, true)
    }
    hapusSesi(realm)
    throw new GalatAPI(401, GALAT_SESI_HABIS)
  }

  if (!res.ok) throw await terjemahkanGalat(res)

  if (res.status === 204) return undefined as T

  const teks = await res.text()
  if (!teks) return undefined as T

  const amplop = JSON.parse(teks) as AmplopSukses<T>
  // Amplop dibuka DI SINI, sekali. Komponen tidak pernah melihat `success`.
  return amplop.data
}

export const api = {
  get: <T>(jalur: string, opsi: Omit<OpsiMinta, 'metode' | 'badan'> = {}) =>
    kirim<T>(jalur, { ...opsi, metode: 'GET' }, true),

  post: <T>(jalur: string, badan?: unknown, opsi: Omit<OpsiMinta, 'metode' | 'badan'> = {}) =>
    kirim<T>(jalur, { ...opsi, metode: 'POST', badan }, true),

  put: <T>(jalur: string, badan?: unknown, opsi: Omit<OpsiMinta, 'metode' | 'badan'> = {}) =>
    kirim<T>(jalur, { ...opsi, metode: 'PUT', badan }, true),

  hapus: <T>(jalur: string, opsi: Omit<OpsiMinta, 'metode' | 'badan'> = {}) =>
    kirim<T>(jalur, { ...opsi, metode: 'DELETE' }, true),

  /** POST dengan badan mentah (mis. CSV) — bukan JSON. */
  postMentah: <T>(
    jalur: string,
    isi: BodyInit,
    tipe: string,
    opsi: Omit<OpsiMinta, 'metode' | 'badan' | 'mentah'> = {},
  ) => kirim<T>(jalur, { ...opsi, metode: 'POST', mentah: { isi, tipe } }, true),

  /** POST unggahan berkas (multipart). Content-Type diurus browser. */
  postBerkas: <T>(
    jalur: string,
    formulir: FormData,
    opsi: Omit<OpsiMinta, 'metode' | 'badan' | 'mentah' | 'formulir'> = {},
  ) => kirim<T>(jalur, { ...opsi, metode: 'POST', formulir }, true),
}
