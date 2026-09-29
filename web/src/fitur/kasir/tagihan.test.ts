import 'fake-indexeddb/auto'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { GalatAPI } from '@/lib/api-client'
import { db, teksCari, type ProdukLokal, type TagihanLokal } from '@/lib/offline/db'
import type { Produk } from '@/bersama/tipe/katalog'
import { kasirApi } from './api'
import type { BarisKeranjang } from './keranjang'
import {
  barisKeItemTagihan,
  diskonKeTagihan,
  itemTagihanKeBaris,
  segarkanTagihan,
  simpanTagihan,
  tagihanKeDiskon,
} from './tagihan'

const produk = (id: string, harga: number): Produk => ({
  id,
  name: id,
  unit_id: 'U1',
  sell_price: harga,
  cost_price: 0,
  track_stock: true,
  min_stock: '0',
  is_active: true,
  created_at: '',
  updated_at: '',
})

const tagihan = (id: string, isi: Partial<TagihanLokal> = {}): TagihanLokal => ({
  id,
  outlet_id: 'O1',
  label: id,
  order_type: 'dine_in',
  items: [],
  status: 'open',
  version: 1,
  created_by: 'U',
  created_at: '2026-09-28T10:00:00Z',
  updated_at: '2026-09-28T10:00:00Z',
  ...isi,
})

beforeEach(async () => {
  await Promise.all([db.tagihan.clear(), db.antrean.clear(), db.produk.clear(), db.varian.clear(), db.satuan.clear()])
})
afterEach(() => vi.restoreAllMocks())

describe('konversi keranjang ↔ tagihan', () => {
  it('persen tetap persen, nominal tetap nominal, catatan dirapikan', () => {
    const baris: BarisKeranjang[] = [
      { kunci: 'P1', produk: produk('P1', 10000), qty: '2', diskon: 0, diskonPersen: 10, catatan: ' tanpa es ' },
      { kunci: 'P2', produk: produk('P2', 5000), qty: '1', diskon: 500 },
    ]
    expect(barisKeItemTagihan(baris)).toEqual([
      { product_id: 'P1', qty: '2', discount_percent: 10, note: 'tanpa es' },
      { product_id: 'P2', qty: '1', discount_amount: 500 },
    ])
    expect(diskonKeTagihan({ jenis: 'persen', nilai: 5 })).toEqual({ kind: 'percent', value: 5 })
    expect(tagihanKeDiskon({ kind: 'percent', value: 5 })).toEqual({ jenis: 'persen', nilai: 5 })
    expect(diskonKeTagihan(null)).toBeUndefined()
  })

  it('barang yang sudah hilang dari katalog dilewati dan dihitung', async () => {
    const dasar: ProdukLokal = {
      id: 'P1', name: 'Kopi', category_id: null, unit_id: 'U1', sku: null, barcode: null, sell_price: 15000,
      cost_price: 0, track_stock: true, min_stock: '0', is_active: true, image_url: '', sync_version: 1, cari: '',
    }
    await db.produk.put({ ...dasar, cari: teksCari(dasar) })
    const { baris, hilang } = await itemTagihanKeBaris([
      { product_id: 'P1', qty: '2', discount_percent: 10 },
      { product_id: 'HILANG', qty: '1' },
    ])
    expect(hilang).toBe(1)
    expect(baris).toHaveLength(1)
    expect(baris[0]).toMatchObject({ kunci: 'P1', qty: '2', diskonPersen: 10 })
  })
})

describe('simpan tagihan saat offline', () => {
  it('diantre dengan id operasi sendiri; versi lokal naik tiap simpan', async () => {
    vi.spyOn(kasirApi, 'simpanTagihan').mockRejectedValue(new GalatAPI(0, 'Tidak ada koneksi', {}, true))
    const baris: BarisKeranjang[] = [{ kunci: 'P1', produk: produk('P1', 10000), qty: '1', diskon: 0 }]

    const a = await simpanTagihan({ outletId: 'O1', aktif: null, label: ' Meja 5 ', baris, diskonTransaksi: null, nominal: 10000 })
    expect(a.diantre).toBe(true)
    expect(a.tagihan).toMatchObject({ label: 'Meja 5', version: 1, tertunda: true })

    const b = await simpanTagihan({
      outletId: 'O1',
      aktif: { id: a.tagihan.id, version: 1, label: 'Meja 5' },
      label: 'Meja 5',
      baris,
      diskonTransaksi: null,
      nominal: 10000,
    })
    expect(b.tagihan.version).toBe(2)

    const ops = await db.antrean.orderBy('dibuatPada').toArray()
    expect(ops.map((o) => o.op)).toEqual(['open_bill.upsert', 'open_bill.upsert'])
    expect(ops[0]!.id).not.toBe(ops[1]!.id)
    expect(ops.map((o) => (o.payload as { base_version: number }).base_version)).toEqual([0, 1])
  })

  it('penolakan server (409) diteruskan, tidak diantre', async () => {
    vi.spyOn(kasirApi, 'simpanTagihan').mockRejectedValue(new GalatAPI(409, 'Tagihan sudah diubah di perangkat lain'))
    await expect(
      simpanTagihan({ outletId: 'O1', aktif: { id: 'T1', version: 1, label: 'X' }, label: 'X', baris: [], diskonTransaksi: null, nominal: 0 }),
    ).rejects.toThrow(/perangkat lain/)
    expect(await db.antrean.count()).toBe(0)
  })
})

describe('menyegarkan daftar dari server', () => {
  it('tidak menimpa perubahan yang masih diantre, dan menyembunyikan yang dibayar offline', async () => {
    // Lokal: A diubah offline (antrean), C basi (sudah ditutup di perangkat lain).
    await db.tagihan.bulkPut([tagihan('A', { version: 4, tertunda: true }), tagihan('C')])
    await db.antrean.bulkPut([
      { id: 'op1', op: 'open_bill.upsert', payload: { id: 'A' }, status: 'menunggu', dibuatPada: 1, percobaan: 0, ringkasan: '', nominal: 0 },
      { id: 'op2', op: 'sale.create', payload: { open_bill_id: 'B' }, status: 'menunggu', dibuatPada: 2, percobaan: 0, ringkasan: '', nominal: 0 },
    ])
    vi.spyOn(kasirApi, 'daftarTagihan').mockResolvedValue([
      tagihan('A', { version: 3 }),
      tagihan('B'),
      tagihan('D'),
    ])

    await segarkanTagihan('O1')

    const lokal = await db.tagihan.toArray()
    expect(lokal.map((t) => t.id).sort()).toEqual(['A', 'D'])
    expect(lokal.find((t) => t.id === 'A')?.version).toBe(4)
  })
})
