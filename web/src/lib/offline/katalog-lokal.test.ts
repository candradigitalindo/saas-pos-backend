import 'fake-indexeddb/auto'
import { beforeEach, describe, expect, it } from 'vitest'
import { db, teksCari, type ProdukLokal } from './db'
import { produkDariBarcode } from './katalog-lokal'

function produk(p: Partial<ProdukLokal> & { id: string; name: string }): ProdukLokal {
  const dasar: ProdukLokal = {
    category_id: null,
    unit_id: 'U1',
    sku: null,
    barcode: null,
    sell_price: 5000,
    cost_price: 3000,
    track_stock: true,
    min_stock: '0',
    is_active: true,
    image_url: '',
    sync_version: 1,
    cari: '',
    ...p,
  }
  return { ...dasar, cari: teksCari(dasar) }
}

describe('produkDariBarcode', () => {
  beforeEach(async () => {
    await db.produk.clear()
    await db.satuan.clear()
    await db.satuan.put({ id: 'U1', name: 'pcs', sync_version: 1 })
  })

  it('menemukan barang dari barcode persis', async () => {
    await db.produk.put(produk({ id: 'P1', name: 'Teh Botol', barcode: '8991002101234' }))
    const p = await produkDariBarcode('8991002101234')
    expect(p?.name).toBe('Teh Botol')
    // Nama satuan dirangkai lokal — pull tidak mengirimnya.
    expect(p?.unit_name).toBe('pcs')
  })

  it('mengabaikan spasi di ujung, karena pemindai kadang menyertakannya', async () => {
    await db.produk.put(produk({ id: 'P1', name: 'Teh Botol', barcode: '8991002101234' }))
    expect((await produkDariBarcode('  8991002101234 '))?.id).toBe('P1')
  })

  it('cocok walau nol di depan berbeda (UPC-A 12 digit vs EAN-13)', async () => {
    // Barcode yang sama sering tercetak 12 digit di kemasan Amerika dan 13
    // digit di basis data — bedanya cuma nol di depan.
    await db.produk.put(produk({ id: 'P1', name: 'Susu Kotak', barcode: '0012345678905' }))
    expect((await produkDariBarcode('12345678905'))?.id).toBe('P1')
  })

  it('jatuh ke SKU, karena banyak warung menempel kode sendiri', async () => {
    await db.produk.put(produk({ id: 'P2', name: 'Gorengan', sku: 'GRG-01' }))
    expect((await produkDariBarcode('GRG-01'))?.id).toBe('P2')
  })

  it('barang nonaktif TIDAK ikut terpindai', async () => {
    await db.produk.put(
      produk({ id: 'P3', name: 'Barang Lama', barcode: '111', is_active: false }),
    )
    expect(await produkDariBarcode('111')).toBeUndefined()
  })

  it('kode kosong tidak mengembalikan barang asal-asalan', async () => {
    await db.produk.put(produk({ id: 'P1', name: 'Teh Botol', barcode: '8991002101234' }))
    expect(await produkDariBarcode('')).toBeUndefined()
    expect(await produkDariBarcode('   ')).toBeUndefined()
  })

  it('kode tak dikenal mengembalikan undefined, bukan melempar', async () => {
    await expect(produkDariBarcode('9999999999999')).resolves.toBeUndefined()
  })
})
