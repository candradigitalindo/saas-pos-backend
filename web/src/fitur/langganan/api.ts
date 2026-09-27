import { api, type Halaman } from '@/lib/api-client'

export interface HargaMasa {
  term_months: number
  discount_rate: string
  gross_amount: number
  discount_amount: number
  total_amount: number
}

export interface Paket {
  code: string
  name: string
  monthly_price: number
  max_outlets: number | null
  max_users: number | null
  max_products: number | null
  max_monthly_transactions: number | null
  features: unknown
  term_prices: HargaMasa[]
}

export interface Langganan {
  id: string
  plan_code: string
  plan_name: string
  term_months: number
  discount_rate: string
  status: string
  trial_ends_at?: string
  current_period_start: string
  current_period_end: string
  auto_renew: boolean
  canceled_at?: string
  cancel_reason?: string
}

export interface TagihanLangganan {
  id: string
  number: string
  term_months: number
  period_start: string
  period_end: string
  gross_amount: number
  discount_amount: number
  total_amount: number
  paid_amount: number
  due_date: string
  status: string
  paid_at?: string
}

export interface RingkasanLangganan {
  subscription: Langganan
  open_invoice?: TagihanLangganan
}

export const langgananApi = {
  /** Katalog paket. Cukup terautentikasi — tidak perlu billing.manage. */
  paket: () => api.get<Paket[]>('/plans'),

  ringkasan: () => api.get<RingkasanLangganan>('/subscription'),

  /** Mengembalikan langganannya saja — tagihannya dibaca ulang lewat ringkasan(). */
  mulai: (plan_code: string, term_months: number) =>
    api.post<Langganan>('/subscription', { plan_code, term_months }),

  /** Ganti paket mengembalikan TAGIHAN prorata, bukan langganannya. */
  gantiPaket: (plan_code: string, term_months: number) =>
    api.post<TagihanLangganan>('/subscription/change-plan', { plan_code, term_months }),

  berhenti: (reason?: string) =>
    api.post<{
      refund_amount: number
      earned_amount: number
      months_used: number
      subscription: Langganan
    }>('/subscription/cancel', { reason }),

  /**
   * Menerbitkan tagihan untuk paket & masa langganan yang dipilih. Dulu tidak
   * ada tombol yang memanggilnya: memilih paket hanya memulai masa coba, dan
   * setelah masa coba habis pemilik tidak punya jalan untuk membayar.
   */
  buatTagihan: () => api.post<TagihanLangganan>('/subscription/invoices'),

  tagihan: () => api.get<Halaman<TagihanLangganan>>('/subscription/invoices', {
    query: { limit: 50 },
  }),

  /** Wajib Idempotency-Key: ini pembayaran uang sungguhan. */
  bayar: (
    input: { invoice_id: string; amount: number; method: string; reference?: string },
    kunci: string,
  ) => api.post<TagihanLangganan>('/subscription-payments', input, { idempotencyKey: kunci }),
}
