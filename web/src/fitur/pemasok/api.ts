import { api, type Halaman } from '@/lib/api-client'
import type { Pemasok } from '@/bersama/tipe/katalog'

/** GET /supplier-stats — angka belanja satu pemasok di toko aktif. */
export interface StatPemasok {
  supplier_id: string
  purchase_count: number
  spent_30d: number
  last_purchase_at?: string
  outstanding: number
  overdue_count: number
}

/** GET /suppliers/:id/products — barang yang pernah dibeli dari pemasok. */
export interface BarangPemasok {
  product_id: string
  product_name: string
  base_unit_name: string
  /** Harga per satuan beli terakhir (per dus bila kemasan). */
  unit_cost: number
  /** Satuan beli terakhir. */
  unit_name: string
  /** Isi satuan beli terakhir, dalam satuan dasar ("1" = satuan dasar). */
  unit_conversion: string
  product_unit_id?: string
  last_bought_at: string
  /** Berapa nota memuat barang ini. */
  times: number
  /** Satuan dasar, 90 hari terakhir. */
  qty_90d: string
}

export interface InputPemasok {
  name: string
  phone?: string
  address?: string
  note?: string
}

export const pemasokApi = {
  daftar: (cari?: string) =>
    api.get<Halaman<Pemasok>>('/suppliers', { query: { search: cari || undefined, limit: 100 } }),
  satu: (id: string) => api.get<Pemasok>(`/suppliers/${id}`),
  buat: (input: InputPemasok) => api.post<Pemasok>('/suppliers', input),
  ubah: (id: string, input: InputPemasok) => api.put<Pemasok>(`/suppliers/${id}`, input),
  hapus: (id: string) => api.hapus<null>(`/suppliers/${id}`),
  statistik: (outlet_id: string) => api.get<StatPemasok[]>('/supplier-stats', { query: { outlet_id } }),
  barang: (id: string, outlet_id: string) =>
    api.get<BarangPemasok[]>(`/suppliers/${id}/products`, { query: { outlet_id } }),
}
