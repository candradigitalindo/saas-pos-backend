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
}

export type Realm = 'tenant' | 'mitra' | 'platform'

// ── Pembaruan token ────────────────────────────────────────────────────────
//
// Satu pembaruan berjalan pada satu waktu. Permintaan lain yang kena 401
// MENUNGGU pembaruan itu, bukan memicu pembaruannya sendiri — tanpa ini, lima
// permintaan bersamaan menghasilkan lima kali refresh dan empat token terbuang.

const pembaruanBerjalan = new Map<Realm, Promise<string | null>>()

async function perbaruiToken(realm: Realm): Promise<string | null> {
  const sedang = pembaruanBerjalan.get(realm)
  if (sedang) return sedang

  const jalan = (async () => {
    const sesi = ambilSesi(realm)
    if (!sesi?.refresh_token) return null

    // Hanya realm tenant yang punya /auth/refresh. Realm lain: sesi habis =
    // masuk ulang.
    if (realm !== 'tenant') return null

    try {
      const res = await fetch(`${BASIS_API}/auth/refresh`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_token: sesi.refresh_token }),
      })
      if (!res.ok) return null
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
      return amplop.data.access_token
    } catch {
      return null
    }
  })()

  pembaruanBerjalan.set(realm, jalan)
  try {
    return await jalan
  } finally {
    pembaruanBerjalan.delete(realm)
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

  if (opsi.badan !== undefined) headers['Content-Type'] = 'application/json'
  if (opsi.idempotencyKey) headers['Idempotency-Key'] = opsi.idempotencyKey
  if (!opsi.tanpaToken) {
    const token = ambilSesi(realm)?.access_token
    if (token) headers.Authorization = `Bearer ${token}`
  }

  let res: Response
  try {
    res = await fetch(susunURL(jalur, opsi.query), {
      method: opsi.metode ?? 'GET',
      headers,
      body: opsi.badan === undefined ? undefined : JSON.stringify(opsi.badan),
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
    const baru = await perbaruiToken(realm)
    if (baru) return kirim<T>(jalur, opsi, false)
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
}
