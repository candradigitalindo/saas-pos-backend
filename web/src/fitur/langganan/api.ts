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

/** Konfirmasi pembayaran dari tenant — menunggu diverifikasi staf platform. */
export interface KonfirmasiBayar {
  id: string
  invoice_id: string
  invoice_number?: string
  amount: number
  method: string
  reference: string
  note?: string
  status: 'pending' | 'approved' | 'rejected'
  reject_reason?: string
  created_at: string
  reviewed_at?: string
}

/** Rekening tujuan pembayaran langganan. */
export interface InfoBayar {
  bank_name: string
  account_number: string
  account_holder?: string
}

export interface RingkasanLangganan {
  subscription: Langganan
  open_invoice?: TagihanLangganan
  /** Konfirmasi TERBARU untuk tagihan terbuka (menunggu / ditolak). */
  payment_claim?: KonfirmasiBayar
  /** Kosong bila rekening tujuan belum dikonfigurasi di server. */
  payment_instructions?: InfoBayar
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

  /**
   * KONFIRMASI pembayaran ("sudah saya transfer"). Paket baru aktif setelah
   * staf keuangan platform memverifikasinya — tenant tidak lagi bisa menandai
   * tagihannya sendiri lunas. Wajib Idempotency-Key: tombol "Kirim" yang
   * tertekan dua kali tidak membuat dua konfirmasi.
   */
  konfirmasi: (
    input: { invoice_id: string; amount: number; method: string; reference: string; note?: string },
    kunci: string,
  ) =>
    api.post<KonfirmasiBayar>('/subscription-payment-claims', input, { idempotencyKey: kunci }),
}
