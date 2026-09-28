import 'fake-indexeddb/auto'
import type { ReactNode } from 'react'
import { act, renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider, onlineManager } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { GalatAPI } from '@/lib/api-client'
import { db, teksCari } from '@/lib/offline/db'
import { kasirApi } from './api'
import { useCheckout } from './hooks'

function pembungkus() {
  const qc = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
  return ({ children }: { children: ReactNode }) => <QueryClientProvider client={qc}>{children}</QueryClientProvider>
}

describe('useCheckout saat offline', () => {
  beforeEach(async () => {
    await Promise.all([db.produk.clear(), db.varian.clear(), db.satuan.clear(), db.antrean.clear(), db.stok.clear()])
    await db.satuan.put({ id: 'U1', name: 'pcs', sync_version: 1 })
    const dasar = {
      id: 'P1',
      name: 'Kopi',
      category_id: null,
      unit_id: 'U1',
      sku: null,
      barcode: null,
      sell_price: 15000,
      cost_price: 9000,
      track_stock: true,
      min_stock: '0',
      is_active: true,
      image_url: '',
      sync_version: 1,
      cari: '',
    }
    await db.produk.put({ ...dasar, cari: teksCari(dasar) })
    await db.varian.put({
      id: 'V1',
      product_id: 'P1',
      name: 'Besar',
      sku: null,
      barcode: null,
      price_delta: 5000,
      is_active: true,
      sync_version: 1,
    })
    vi.spyOn(kasirApi, 'bayar').mockRejectedValue(new GalatAPI(0, 'Tidak ada koneksi', {}, true))
  })

  afterEach(() => {
    onlineManager.setOnline(true)
    vi.restoreAllMocks()
  })

  // Regresi: bawaan TanStack menjeda mutasi selama browser melapor offline,
  // sehingga checkout tidak pernah sampai ke jalur antrean.
  it('tetap berjalan & masuk antrean walau browser melapor offline, dengan harga & nama varian', async () => {
    onlineManager.setOnline(false)
    const { result } = renderHook(() => useCheckout(), { wrapper: pembungkus() })

    act(() =>
      result.current.mutate({
        input: {
          outlet_id: 'O1',
          shift_id: 'S1',
          items: [{ product_id: 'P1', variant_id: 'V1', qty: '2' }],
          payments: [{ method: 'cash', amount: 40000 }],
        },
        kunci: 'K1',
      }),
    )

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.diantre).toBe(true)
    const [baris] = result.current.data!.transaksi.items ?? []
    expect(baris?.product_name).toBe('Kopi (Besar)')
    expect(baris?.unit_price).toBe(20000)
    expect(result.current.data?.transaksi.total).toBe(40000)
    expect(await db.antrean.count()).toBe(1)
  })
})
