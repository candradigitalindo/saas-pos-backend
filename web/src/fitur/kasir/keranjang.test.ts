import { act, renderHook } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { itemUntukCheckout, namaBaris, stokCukup, useKeranjang } from './keranjang'
import type { Produk, VarianProduk } from '@/bersama/tipe/katalog'

function produk(id: string, harga: number, nama = id): Produk {
  return {
    id,
    name: nama,
    unit_id: 'U1',
    unit_name: 'pcs',
    sell_price: harga,
    cost_price: 0,
    track_stock: true,
    min_stock: '0',
    is_active: true,
    created_at: '',
    updated_at: '',
  }
}

describe('keranjang', () => {
  it('menambah barang yang sama menaikkan jumlahnya, bukan membuat baris baru', () => {
    const { result } = renderHook(() => useKeranjang())
    const kopi = produk('P1', 18000, 'Kopi Susu')

    act(() => result.current.tambah(kopi))
    act(() => result.current.tambah(kopi))

    expect(result.current.baris).toHaveLength(1)
    expect(result.current.qtyDari('P1')).toBe('2')
  })

  it('menghitung pratinjau total dari harga bulat', () => {
    const { result } = renderHook(() => useKeranjang())
    act(() => result.current.tambah(produk('P1', 18000), '2'))
    act(() => result.current.tambah(produk('P2', 8000), '1'))
    // 36.000 + 8.000 — persis contoh di ui/05-ALUR-UTAMA.md
    expect(result.current.pratinjauTotal).toBe(44000)
  })

  it('mengubah jumlah jadi nol MENGHAPUS barisnya', () => {
    const { result } = renderHook(() => useKeranjang())
    act(() => result.current.tambah(produk('P1', 18000)))
    act(() => result.current.ubahQty('P1', '0'))
    expect(result.current.baris).toHaveLength(0)
  })

  it('diskon per baris ikut mengurangi pratinjau', () => {
    const { result } = renderHook(() => useKeranjang())
    act(() => result.current.tambah(produk('P1', 18000), '2'))
    act(() => result.current.ubahDiskon('P1', 6000))
    expect(result.current.pratinjauTotal).toBe(30000)
  })

  it('jumlah desimal tetap presisi', () => {
    const { result } = renderHook(() => useKeranjang())
    act(() => result.current.tambah(produk('P1', 20000), '0.5'))
    act(() => result.current.tambah(produk('P1', 20000), '0.25'))
    expect(result.current.qtyDari('P1')).toBe('0.75')
    expect(result.current.pratinjauTotal).toBe(15000)
  })

  it('item checkout TIDAK menyertakan harga — server yang menentukannya', () => {
    const baris = [
      { kunci: 'P1', produk: produk('P1', 18000), qty: '2', diskon: 0 },
      { kunci: 'P2', produk: produk('P2', 8000), qty: '1', diskon: 500 },
    ]
    const item = itemUntukCheckout(baris)

    expect(item).toEqual([
      { product_id: 'P1', qty: '2' },
      { product_id: 'P2', qty: '1', discount_amount: 500 },
    ])
    for (const i of item) {
      expect(i).not.toHaveProperty('unit_price')
      expect(i).not.toHaveProperty('sell_price')
      expect(i).not.toHaveProperty('line_total')
    }
  })

  it('varian: baris sendiri per varian, harga + selisih, variant_id ikut terkirim', () => {
    const { result } = renderHook(() => useKeranjang())
    const kopi = produk('P1', 15000, 'Kopi')
    const besar: VarianProduk = { id: 'V1', product_id: 'P1', name: 'Besar', price_delta: 5000, is_active: true }
    const kecil: VarianProduk = { id: 'V2', product_id: 'P1', name: 'Kecil', price_delta: -2000, is_active: true }

    act(() => result.current.tambah(kopi, '1', besar))
    act(() => result.current.tambah(kopi, '1', besar))
    act(() => result.current.tambah(kopi, '1', kecil))

    expect(result.current.baris).toHaveLength(2)
    expect(namaBaris(result.current.baris[0]!)).toBe('Kopi (Besar)')
    // Stok dihitung per barang: lencana kartu menjumlah semua variannya.
    expect(result.current.qtyDari('P1')).toBe('3')
    // 2 × 20.000 + 13.000
    expect(result.current.pratinjauTotal).toBe(53000)
    expect(itemUntukCheckout(result.current.baris)).toEqual([
      { product_id: 'P1', variant_id: 'V1', qty: '2' },
      { product_id: 'P1', variant_id: 'V2', qty: '1' },
    ])

    act(() => result.current.ubahQty(result.current.baris[1]!.kunci, '0'))
    expect(result.current.baris).toHaveLength(1)
    expect(result.current.qtyDari('P1')).toBe('2')
  })

  it('mengosongkan keranjang setelah transaksi selesai', () => {
    const { result } = renderHook(() => useKeranjang())
    act(() => result.current.tambah(produk('P1', 18000)))
    act(() => result.current.kosongkan())
    expect(result.current.baris).toHaveLength(0)
    expect(result.current.pratinjauTotal).toBe(0)
  })
})

describe('stokCukup', () => {
  it('membandingkan sebagai angka, bukan teks', () => {
    expect(stokCukup('10', '9')).toBe(true)
    expect(stokCukup('9', '10')).toBe(false)
    expect(stokCukup('0', '1')).toBe(false)
  })

  it('barang tanpa lacak stok selalu boleh dijual', () => {
    expect(stokCukup(undefined, '999')).toBe(true)
  })
})
