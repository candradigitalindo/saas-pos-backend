import { api, type Halaman } from '@/lib/api-client'
import type { Pelanggan, Transaksi } from '@/bersama/tipe/pos'
import type { SumberBayar } from '@/fitur/stok/api'

export interface Kasbon {
  id: string
  customer_id: string
  source_table: string
  source_id: string
  amount: number
  paid_amount: number
  outstanding: number
  due_date?: string
  status: string
  created_at: string
  /** Nota asal (kasbon dari kasir) — hanya di daftar. */
  receipt_no?: string
  business_date?: string
  outlet_name?: string
}

/** Kasbon belum lunas seorang pelanggan, dirangkum (GET /receivable-summary). */
export interface KasbonPelanggan {
  customer_id: string
  customer_name: string
  phone?: string
  outstanding: number
  count: number
  overdue_count: number
  overdue_amount: number
  due_soon_count: number
  due_soon_amount: number
  nearest_due?: string
  oldest_at: string
  last_paid_at?: string
  credit_limit: number
}

export interface RingkasanKasbon {
  /** Tanggal usaha toko hari ini (YYYY-MM-DD) — pembanding jatuh tempo. */
  today: string
  due_soon_days: number
  outstanding: number
  count: number
  /** Jumlah PELANGGAN yang punya kasbon lewat jatuh tempo. */
  overdue_count: number
  overdue_amount: number
  due_soon_count: number
  due_soon_amount: number
  /** Yang lewat jatuh tempo terbesar dulu, lalu sisa terbesar. */
  customers: KasbonPelanggan[]
}

export type CaraSetor = 'cash' | 'qris' | 'transfer'

export interface HasilSetoran {
  amount: number
  /** Sisa kasbon pelanggan setelah setoran ini. */
  outstanding: number
  to_drawer: boolean
  allocations: { receivable_id: string; receipt_no?: string; amount: number; settled: boolean }[]
}

export interface Setoran {
  id: string
  receivable_id: string
  receipt_no?: string
  amount: number
  method: string
  to_drawer: boolean
  paid_at: string
  collected_name?: string
  note?: string
}

export interface BarangFavorit {
  product_id: string
  product_name: string
  unit_name: string
  times: number
  qty: string
  last_at: string
}

export interface InputPelanggan {
  name: string
  phone?: string
  email?: string
  address?: string
  credit_limit?: number
  /** Tempo kasbon (hari); 0 = tanpa jatuh tempo. */
  credit_term_days?: number
  note?: string
  /** "" = kembali ke harga umum. */
  price_list_id?: string
}

export const pelangganApi = {
  /** outlet_id hanya menentukan "hari ini" untuk kasbon yang lewat jatuh tempo. */
  daftar: (cari?: string, page = 1, limit = 25, outlet_id?: string) =>
    api.get<Halaman<Pelanggan>>('/customers', { query: { search: cari || undefined, page, limit, outlet_id } }),

  satu: (id: string, outlet_id?: string) => api.get<Pelanggan>(`/customers/${id}`, { query: { outlet_id } }),

  seringDibeli: (id: string) => api.get<BarangFavorit[]>(`/customers/${id}/top-products`),

  riwayatBelanja: (customer_id: string, page = 1, limit = 20) =>
    api.get<Halaman<Transaksi>>('/sales', { query: { customer_id, page, limit } }),

  buat: (input: InputPelanggan) => api.post<Pelanggan>('/customers', input),

  ubah: (id: string, input: Partial<InputPelanggan>) =>
    api.put<Pelanggan>(`/customers/${id}`, input),

  hapus: (id: string) => api.hapus<null>(`/customers/${id}`),

  // ── Kasbon (piutang pelanggan) ───────────────────────────────────────────
  /** status 'unpaid' = belum lunas (termasuk yang sudah dicicil), terlama dulu. */
  daftarKasbon: (customer_id?: string, status?: string, page = 1, limit = 25) =>
    api.get<Halaman<Kasbon>>('/receivables', {
      query: { customer_id, status, page, limit },
    }),

  ringkasanKasbon: (outlet_id?: string) =>
    api.get<RingkasanKasbon>('/receivable-summary', { query: { outlet_id } }),

  /** Atur jatuh tempo satu kasbon ("janji bayar"); '' = tanpa jatuh tempo. */
  aturJatuhTempo: (id: string, due_date: string) => api.put<Kasbon>(`/receivables/${id}`, { due_date }),

  /**
   * Setoran pelanggan atas SEMUA kasbonnya — server melunasi yang terlama
   * dulu. Tunai boleh masuk laci kasir (source 'drawer', outlet_id wajib).
   */
  setor: (
    customer_id: string,
    input: { amount: number; method: CaraSetor; source?: SumberBayar; outlet_id?: string; note?: string },
    kunci: string,
  ) => api.post<HasilSetoran>(`/customers/${customer_id}/receivable-payments`, input, { idempotencyKey: kunci }),

  riwayatSetoran: (customer_id: string) => api.get<Setoran[]>(`/customers/${customer_id}/receivable-payments`),

  kasbon: (id: string) => api.get<Kasbon>(`/receivables/${id}`),

  /**
   * Menerima setoran dari pelanggan atas kasbonnya. Wajib Idempotency-Key:
   * setoran adalah uang masuk, dan kiriman ulang dengan kunci yang sama tidak
   * boleh mencatatnya dua kali.
   */
  terimaSetoran: (
    input: {
      receivable_id: string
      amount: number
      method: 'cash' | 'qris' | 'transfer' | 'card' | 'ewallet'
    },
    kunci: string,
  ) => api.post<Kasbon>('/receivable-payments', input, { idempotencyKey: kunci }),
}
