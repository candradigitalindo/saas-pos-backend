import { api, type Halaman } from '@/lib/api-client'
import type { SaldoStok } from '@/bersama/tipe/katalog'

export interface GerakanStok {
  id: string
  outlet_id: string
  product_id: string
  variant_id?: string
  kind: string
  qty_delta: string
  balance_after: string
  unit_cost: number
  ref_table?: string
  ref_id?: string
  reason?: string
  occurred_at: string
  business_date: string
}

export interface ItemPembelian {
  id: string
  product_id: string
  qty: string
  unit_cost: number
  line_total: number
}

export interface Pembelian {
  id: string
  outlet_id: string
  supplier_id?: string
  invoice_no?: string
  status: string
  subtotal: number
  discount_amount: number
  tax_amount: number
  total: number
  paid_amount: number
  occurred_at: string
  business_date: string
  items?: ItemPembelian[]
  created_at: string
}

export const stokApi = {
  saldo: (outlet_id: string, low?: boolean, page = 1, limit = 100) =>
    api.get<Halaman<SaldoStok>>('/stocks', {
      query: { outlet_id, low: low ? 'true' : undefined, page, limit },
    }),

  kartuStok: (product_id: string, outlet_id?: string, page = 1, limit = 50) =>
    api.get<Halaman<GerakanStok>>('/stock-movements', {
      query: { product_id, outlet_id, page, limit },
    }),

  /**
   * Koreksi stok. Kirim `new_qty` (set ke angka pasti, dipakai saat mengisi
   * saldo awal atau hasil hitung fisik) ATAU `delta` (geser naik/turun) —
   * tidak boleh dua-duanya.
   */
  koreksi: (input: {
    outlet_id: string
    product_id: string
    new_qty?: string
    delta?: string
    reason: string
  }) => api.post<GerakanStok>('/stock-adjustments', input),

  /** Barang masuk. Wajib Idempotency-Key: ini menciptakan stok DAN utang. */
  barangMasuk: (
    input: {
      outlet_id: string
      supplier_id?: string
      invoice_no?: string
      paid_amount?: number
      items: { product_id: string; qty: string; unit_cost: number }[]
    },
    kunci: string,
  ) => api.post<Pembelian>('/purchases', input, { idempotencyKey: kunci }),

  daftarPembelian: (page = 1, limit = 20) =>
    api.get<Halaman<Pembelian>>('/purchases', { query: { page, limit } }),

  pembelian: (id: string) => api.get<Pembelian>(`/purchases/${id}`),
}
