import 'fake-indexeddb/auto'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/lib/api-client'
import { ambilKursor, db, kunciStok } from './db'
import { tarikMasterData, type PerubahanTarik } from './tarik'

const KOSONG: PerubahanTarik = {
  cursor: 0,
  has_more: false,
  safety_lag: 1000,
  categories: [],
  units: [],
  products: [],
  product_variants: [],
  customers: [],
  stocks: [],
  deleted: {},
}

function halaman(isi: Partial<PerubahanTarik>): PerubahanTarik {
  return { ...KOSONG, ...isi }
}

describe('tarik master data', () => {
  beforeEach(async () => {
    await Promise.all([
      db.produk.clear(),
      db.varian.clear(),
      db.satuan.clear(),
      db.kategori.clear(),
      db.stok.clear(),
      db.meta.clear(),
    ])
    vi.restoreAllMocks()
  })

  it('menyimpan produk beserta teks pencarian yang sudah dirangkai', async () => {
    vi.spyOn(api, 'get').mockResolvedValue(
      halaman({
        cursor: 5000,
        has_more: false,
        products: [
          {
            id: 'P1',
            category_id: null,
            unit_id: 'U1',
            name: 'Kopi Susu',
            sku: 'KS-01',
            barcode: null,
            sell_price: 18000,
            cost_price: 12000,
            track_stock: true,
            min_stock: '5',
            is_active: true,
            image_url: '',
            sync_version: 4,
          },
        ],
      }),
    )

    await tarikMasterData('O1')

    const p = await db.produk.get('P1')
    expect(p?.name).toBe('Kopi Susu')
    // Pencarian nanti memakai kolom ini, bukan memindai nama satu per satu.
    expect(p?.cari).toBe('kopi susu ks-01 ')
  })

  it('kursor disimpan DIKURANGI jeda aman', async () => {
    vi.spyOn(api, 'get').mockResolvedValue(
      halaman({ cursor: 5000, has_more: false, safety_lag: 1000 }),
    )

    await tarikMasterData('O1')

    // Nomor sync_version dialokasikan sebelum transaksi commit, jadi menyimpan
    // kursor mentah bisa melewatkan baris yang commit belakangan.
    expect(await ambilKursor()).toBe(4000)
  })

  it('kursor tidak pernah negatif walau jeda aman lebih besar dari kursor', async () => {
    vi.spyOn(api, 'get').mockResolvedValue(
      halaman({ cursor: 26, has_more: false, safety_lag: 1000 }),
    )

    await tarikMasterData('O1')
    expect(await ambilKursor()).toBe(0)
  })

  it('menarik sampai has_more habis supaya katalog tidak setengah', async () => {
    const get = vi
      .spyOn(api, 'get')
      .mockResolvedValueOnce(
        halaman({
          cursor: 3000,
          has_more: true,
          safety_lag: 0,
          units: [{ id: 'U1', name: 'pcs', sync_version: 1 }],
        }),
      )
      .mockResolvedValueOnce(
        halaman({
          cursor: 6000,
          has_more: false,
          safety_lag: 0,
          units: [{ id: 'U2', name: 'kg', sync_version: 2 }],
        }),
      )

    const { halaman: jml } = await tarikMasterData('O1')

    expect(jml).toBe(2)
    expect(get).toHaveBeenCalledTimes(2)
    expect(await db.satuan.count()).toBe(2)
  })

  it('berhenti bila kursor tidak maju, supaya tidak menarik halaman sama selamanya', async () => {
    // has_more selalu true tapi cursor tidak pernah bergerak: tanpa penjaga,
    // ini gelung tak berujung yang menghabiskan kuota kasir.
    const get = vi
      .spyOn(api, 'get')
      .mockResolvedValue(halaman({ cursor: 0, has_more: true, safety_lag: 0 }))

    await tarikMasterData('O1')
    expect(get).toHaveBeenCalledTimes(1)
  })

  it('stok disimpan sebagai snapshot per outlet dan bisa ditimpa', async () => {
    const get = vi.spyOn(api, 'get')
    get.mockResolvedValueOnce(
      halaman({
        cursor: 1,
        has_more: false,
        safety_lag: 0,
        stocks: [
          { outlet_id: 'O1', product_id: 'P1', variant_id: '', qty: '44', reserved_qty: '0' },
        ],
      }),
    )
    await tarikMasterData('O1')
    expect((await db.stok.get(kunciStok('O1', 'P1')))?.qty).toBe('44')

    get.mockResolvedValueOnce(
      halaman({
        cursor: 2,
        has_more: false,
        safety_lag: 0,
        stocks: [
          { outlet_id: 'O1', product_id: 'P1', variant_id: '', qty: '40', reserved_qty: '0' },
        ],
      }),
    )
    await tarikMasterData('O1')

    expect((await db.stok.get(kunciStok('O1', 'P1')))?.qty).toBe('40')
    expect(await db.stok.count()).toBe(1)
  })

  it('batu nisan penghapusan membuang produk beserta saldo stoknya', async () => {
    const get = vi.spyOn(api, 'get')
    get.mockResolvedValueOnce(
      halaman({
        cursor: 1,
        has_more: false,
        safety_lag: 0,
        products: [
          {
            id: 'P1', category_id: null, unit_id: 'U1', name: 'Kopi', sku: null,
            barcode: null, sell_price: 1, cost_price: 1, track_stock: true,
            min_stock: '0', is_active: true, image_url: '', sync_version: 1,
          },
        ],
        stocks: [
          { outlet_id: 'O1', product_id: 'P1', variant_id: '', qty: '5', reserved_qty: '0' },
        ],
      }),
    )
    await tarikMasterData('O1')

    get.mockResolvedValueOnce(
      halaman({ cursor: 2, has_more: false, safety_lag: 0, deleted: { products: ['P1'] } }),
    )
    await tarikMasterData('O1')

    expect(await db.produk.get('P1')).toBeUndefined()
    // Saldo stok ikut dibuang supaya tidak jadi hantu di layar kasir.
    expect(await db.stok.count()).toBe(0)
  })

  it('varian tersimpan, terhapus sendiri, dan ikut terhapus bersama barangnya', async () => {
    const varian = (id: string, product_id: string) => ({
      id,
      product_id,
      name: id,
      sku: null,
      barcode: null,
      price_delta: 5000,
      is_active: true,
      sync_version: 1,
    })
    const get = vi.spyOn(api, 'get')
    get.mockResolvedValueOnce(
      halaman({
        cursor: 1,
        safety_lag: 0,
        product_variants: [varian('V1', 'P1'), varian('V2', 'P1'), varian('V3', 'P2')],
      }),
    )
    await tarikMasterData('O1')
    expect(await db.varian.count()).toBe(3)

    get.mockResolvedValueOnce(
      halaman({ cursor: 2, safety_lag: 0, deleted: { product_variants: ['V3'], products: ['P1'] } }),
    )
    await tarikMasterData('O1')
    expect(await db.varian.count()).toBe(0)
  })

  it('tabel yang belum dipakai layar mana pun diabaikan, bukan dianggap galat', async () => {
    vi.spyOn(api, 'get').mockResolvedValue(
      halaman({
        cursor: 1,
        has_more: false,
        safety_lag: 0,
        deleted: { price_lists: ['X1'] },
      }),
    )
    await expect(tarikMasterData('O1')).resolves.toBeDefined()
  })
})
