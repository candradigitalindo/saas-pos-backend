import { api } from '@/lib/api-client'
import { db, idPerangkat, type AntreanOperasi } from './db'

/** Hasil satu operasi menurut backend. */
interface HasilOperasi {
  id: string
  /** "applied" | "duplicate" | "rejected" */
  status: string
  reason?: string
}

interface HasilPush {
  device_id: string
  applied: number
  duplicate: number
  rejected: number
  results: HasilOperasi[]
}

/** Berapa kali sebuah operasi dicoba sebelum diserahkan ke pemeriksaan manusia. */
const BATAS_PERCOBAAN = 5

export async function antrekan(op: Omit<AntreanOperasi, 'status' | 'percobaan' | 'dibuatPada'>) {
  await db.antrean.put({
    ...op,
    status: 'menunggu',
    percobaan: 0,
    dibuatPada: Date.now(),
  })
}

export async function jumlahMenunggu(): Promise<number> {
  return db.antrean.where('status').anyOf('menunggu', 'mengirim').count()
}

export async function jumlahPerluDiperiksa(): Promise<number> {
  return db.antrean.where('status').equals('perlu-diperiksa').count()
}

export async function daftarPerluDiperiksa(): Promise<AntreanOperasi[]> {
  return db.antrean.where('status').equals('perlu-diperiksa').toArray()
}

/** Mengembalikan satu operasi bermasalah ke antrean untuk dicoba lagi. */
export async function cobaLagi(id: string): Promise<void> {
  await db.antrean.update(id, { status: 'menunggu', percobaan: 0, alasan: undefined })
}

export async function buangDariAntrean(id: string): Promise<void> {
  await db.antrean.delete(id)
}

export interface RingkasanKirim {
  terkirim: number
  duplikat: number
  gagal: number
}

/**
 * Mengirim antrean dalam satu batch.
 *
 * Tiga aturan yang mengikat (ui/03-ARSITEKTUR-FRONTEND.md):
 *
 *  1. Satu operasi gagal TIDAK menjatuhkan seluruh batch — backend membalas per
 *     operasi, dan hanya yang gagal yang ditandai.
 *
 *  2. Balasan "duplicate" BUKAN error. Artinya operasi itu sudah pernah masuk;
 *     tandai terkirim dan jangan tampilkan apa pun ke pengguna. Inilah yang
 *     terjadi ketika jaringan putus setelah server sempat menyimpan.
 *
 *  3. Kegagalan jaringan bukan kesalahan pengguna: operasi tetap di antrean dan
 *     dicoba lagi nanti, tanpa pesan menakutkan.
 */
export async function kirimAntrean(): Promise<RingkasanKirim | null> {
  const menunggu = await db.antrean.where('status').equals('menunggu').toArray()
  if (menunggu.length === 0) return null

  const batch = menunggu.slice(0, 50)
  const ids = batch.map((o) => o.id)
  await db.antrean.where('id').anyOf(ids).modify({ status: 'mengirim' })

  let hasil: HasilPush
  try {
    hasil = await api.post<HasilPush>('/sync/push', {
      device_id: await idPerangkat(),
      operations: batch.map((o) => ({
        op: o.op,
        id: o.id,
        idempotency_key: o.id,
        payload: o.payload,
      })),
    })
  } catch {
    // Aturan 3: kembalikan ke antrean. Percobaan dinaikkan supaya operasi yang
    // terus gagal akhirnya sampai ke halaman pemeriksaan alih-alih berputar diam.
    for (const o of batch) {
      const percobaan = o.percobaan + 1
      await db.antrean.update(o.id, {
        status: percobaan >= BATAS_PERCOBAAN ? 'perlu-diperiksa' : 'menunggu',
        percobaan,
        alasan:
          percobaan >= BATAS_PERCOBAAN
            ? 'Sudah beberapa kali dicoba tapi belum berhasil terkirim.'
            : undefined,
      })
    }
    return null
  }

  const ringkasan: RingkasanKirim = { terkirim: 0, duplikat: 0, gagal: 0 }
  const dibalas = new Set<string>()

  for (const r of hasil.results) {
    dibalas.add(r.id)
    if (r.status === 'applied') {
      ringkasan.terkirim++
      await db.antrean.delete(r.id)
    } else if (r.status === 'duplicate') {
      // Aturan 2: aman, sudah pernah masuk. Tidak dihitung sebagai kabar untuk
      // pengguna — makanya duplikat dipisah dari terkirim.
      ringkasan.duplikat++
      await db.antrean.delete(r.id)
    } else {
      // Ditolak server (mis. barang sudah dihapus). Mengulang tidak akan
      // menolong, jadi langsung ke pemeriksaan manusia.
      ringkasan.gagal++
      await db.antrean.update(r.id, {
        status: 'perlu-diperiksa',
        alasan: r.reason || 'Ditolak server tanpa keterangan.',
      })
    }
  }

  // Operasi yang tidak disebut di balasan dikembalikan ke antrean daripada
  // menggantung selamanya berstatus "mengirim".
  for (const o of batch) {
    if (!dibalas.has(o.id)) {
      await db.antrean.update(o.id, { status: 'menunggu' })
    }
  }

  return ringkasan
}
