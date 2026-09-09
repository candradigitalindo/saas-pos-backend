import { api, type Halaman } from '@/lib/api-client'
import type { Pelanggan } from '@/bersama/tipe/pos'

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
}

export interface InputPelanggan {
  name: string
  phone?: string
  email?: string
  address?: string
  credit_limit?: number
  note?: string
}

export const pelangganApi = {
  daftar: (cari?: string, page = 1, limit = 25) =>
    api.get<Halaman<Pelanggan>>('/customers', { query: { search: cari || undefined, page, limit } }),

  satu: (id: string) => api.get<Pelanggan>(`/customers/${id}`),

  buat: (input: InputPelanggan) => api.post<Pelanggan>('/customers', input),

  ubah: (id: string, input: Partial<InputPelanggan>) =>
    api.put<Pelanggan>(`/customers/${id}`, input),

  hapus: (id: string) => api.hapus<null>(`/customers/${id}`),

  // ── Kasbon (piutang pelanggan) ───────────────────────────────────────────
  daftarKasbon: (customer_id?: string, status?: string, page = 1, limit = 25) =>
    api.get<Halaman<Kasbon>>('/receivables', {
      query: { customer_id, status, page, limit },
    }),

  kasbon: (id: string) => api.get<Kasbon>(`/receivables/${id}`),

  /** Menerima setoran dari pelanggan atas kasbonnya. */
  terimaSetoran: (input: {
    receivable_id: string
    amount: number
    method: 'cash' | 'qris' | 'transfer' | 'card' | 'ewallet'
  }) => api.post<Kasbon>('/receivable-payments', input),
}
