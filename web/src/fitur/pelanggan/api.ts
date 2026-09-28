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
  /** "" = kembali ke harga umum. */
  price_list_id?: string
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
