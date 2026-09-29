import 'fake-indexeddb/auto'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/lib/api-client'
import { db } from './db'
import {
  antrekan,
  cobaLagi,
  daftarPerluDiperiksa,
  jumlahMenunggu,
  kirimAntrean,
  pulihkanYangMenggantung,
} from './antrean'

async function taruh(id: string, ringkasan = 'Transaksi', nominal = 10000) {
  await antrekan({ id, op: 'sale.create', payload: { x: 1 }, ringkasan, nominal })
}

describe('antrean offline', () => {
  beforeEach(async () => {
    await db.antrean.clear()
    await db.meta.clear()
    vi.restoreAllMocks()
  })

  it('menyimpan operasi berstatus menunggu', async () => {
    await taruh('A1')
    expect(await jumlahMenunggu()).toBe(1)
  })

  it('operasi yang diterapkan dibuang dari antrean', async () => {
    await taruh('A1')
    vi.spyOn(api, 'post').mockResolvedValue({
      device_id: 'd', applied: 1, duplicate: 0, rejected: 0,
      results: [{ id: 'A1', status: 'applied' }],
    })

    const hasil = await kirimAntrean()
    expect(hasil).toEqual({ terkirim: 1, duplikat: 0, gagal: 0 })
    expect(await jumlahMenunggu()).toBe(0)
  })

  it('balasan "duplicate" BUKAN error — ditandai terkirim dan tidak dilaporkan', async () => {
    await taruh('A1')
    vi.spyOn(api, 'post').mockResolvedValue({
      device_id: 'd', applied: 0, duplicate: 1, rejected: 0,
      results: [{ id: 'A1', status: 'duplicate' }],
    })

    const hasil = await kirimAntrean()
    // Dipisah dari `terkirim` justru supaya UI TIDAK memberitahu pengguna:
    // bagi kasir, transaksi yang sudah pernah masuk bukan peristiwa apa pun.
    expect(hasil).toEqual({ terkirim: 0, duplikat: 1, gagal: 0 })
    expect(await db.antrean.get('A1')).toBeUndefined()
  })

  it('satu operasi ditolak tidak menjatuhkan yang lain', async () => {
    await taruh('A1')
    await taruh('A2')
    await taruh('A3')
    vi.spyOn(api, 'post').mockResolvedValue({
      device_id: 'd', applied: 2, duplicate: 0, rejected: 1,
      results: [
        { id: 'A1', status: 'applied' },
        { id: 'A2', status: 'rejected', reason: 'produk sudah dihapus' },
        { id: 'A3', status: 'applied' },
      ],
    })

    const hasil = await kirimAntrean()
    expect(hasil).toEqual({ terkirim: 2, duplikat: 0, gagal: 1 })

    const perlu = await daftarPerluDiperiksa()
    expect(perlu).toHaveLength(1)
    expect(perlu[0]!.id).toBe('A2')
    // Alasan dari server dipertahankan apa adanya supaya bisa ditindaklanjuti.
    expect(perlu[0]!.alasan).toBe('produk sudah dihapus')
  })

  it('jaringan mati mengembalikan operasi ke antrean, bukan membuangnya', async () => {
    await taruh('A1')
    vi.spyOn(api, 'post').mockRejectedValue(new Error('offline'))

    expect(await kirimAntrean()).toBeNull()
    expect(await jumlahMenunggu()).toBe(1)
    expect((await db.antrean.get('A1'))?.percobaan).toBe(1)
  })

  it('setelah lima kali gagal, operasi diserahkan ke pemeriksaan manusia', async () => {
    await taruh('A1')
    vi.spyOn(api, 'post').mockRejectedValue(new Error('offline'))

    for (let i = 0; i < 5; i++) await kirimAntrean()

    expect(await jumlahMenunggu()).toBe(0)
    const perlu = await daftarPerluDiperiksa()
    expect(perlu).toHaveLength(1)
    // Kalimatnya tidak menyalahkan pengguna atas kegagalan jaringan.
    expect(perlu[0]!.alasan).not.toMatch(/salah|gagal Anda/i)
  })

  it('"coba kirim lagi" mengembalikan operasi ke antrean dengan hitungan bersih', async () => {
    await taruh('A1')
    vi.spyOn(api, 'post').mockRejectedValue(new Error('offline'))
    for (let i = 0; i < 5; i++) await kirimAntrean()

    await cobaLagi('A1')

    const o = await db.antrean.get('A1')
    expect(o?.status).toBe('menunggu')
    expect(o?.percobaan).toBe(0)
    expect(o?.alasan).toBeUndefined()
  })

  it('operasi yang tidak disebut di balasan tidak menggantung berstatus mengirim', async () => {
    await taruh('A1')
    await taruh('A2')
    vi.spyOn(api, 'post').mockResolvedValue({
      device_id: 'd', applied: 1, duplicate: 0, rejected: 0,
      results: [{ id: 'A1', status: 'applied' }], // A2 tidak dibalas
    })

    await kirimAntrean()
    expect((await db.antrean.get('A2'))?.status).toBe('menunggu')
  })

  it('mengirim antrean kosong tidak memanggil server sama sekali', async () => {
    const post = vi.spyOn(api, 'post')
    expect(await kirimAntrean()).toBeNull()
    expect(post).not.toHaveBeenCalled()
  })

  it('kunci idempotensi yang dikirim SAMA dengan id operasi', async () => {
    await taruh('A1')
    const post = vi.spyOn(api, 'post').mockResolvedValue({
      device_id: 'd', applied: 1, duplicate: 0, rejected: 0,
      results: [{ id: 'A1', status: 'applied' }],
    })

    await kirimAntrean()

    const badan = post.mock.calls[0]![1] as {
      operations: { id: string; idempotency_key: string }[]
    }
    expect(badan.operations[0]!.idempotency_key).toBe('A1')
    expect(badan.operations[0]!.id).toBe('A1')
  })

  it('operasi yang tertahan "mengirim" (tab tertutup di tengah kirim) dikirim ulang', async () => {
    await taruh('A1')
    await db.antrean.update('A1', { status: 'mengirim' })
    const post = vi.spyOn(api, 'post').mockResolvedValue({
      device_id: 'd', applied: 1, duplicate: 0, rejected: 0,
      results: [{ id: 'A1', status: 'applied' }],
    })

    // Tanpa pemulihan, kirimAntrean tidak pernah menyentuhnya.
    expect(await kirimAntrean()).toBeNull()
    expect(post).not.toHaveBeenCalled()

    expect(await pulihkanYangMenggantung()).toBe(1)
    expect(await kirimAntrean()).toEqual({ terkirim: 1, duplikat: 0, gagal: 0 })
    expect(await db.antrean.get('A1')).toBeUndefined()
  })
})
