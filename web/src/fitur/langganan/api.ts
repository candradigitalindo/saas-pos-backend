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
  /** Paket yang dibayar tagihan ini. */
  plan_code?: string
  plan_name?: string
  /** "plan_change" = pindah paket; paketnya baru berpindah saat lunas. */
  kind: 'regular' | 'plan_change'
  /** Potongan sisa masa paket lama (sudah termasuk di discount_amount). */
  credit_amount: number
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

/** Pengembalian dana setelah berhenti — ditransfer staf keuangan dari panel. */
export interface Pengembalian {
  id: string
  amount: number
  months_used: number
  status: 'pending' | 'paid' | 'not_needed'
  destination_bank?: string
  destination_account?: string
  destination_holder?: string
  payout_reference?: string
  created_at: string
  paid_at?: string
}

/** Angka pengembalian bila berhenti SEKARANG. */
export interface PratinjauBerhenti {
  refund_amount: number
  months_used: number
  paid_amount: number
  term_months: number
  monthly_price: number
}

export interface InputBerhenti {
  reason?: string
  refund_bank?: string
  refund_account?: string
  refund_holder?: string
}

export interface RingkasanLangganan {
  subscription: Langganan
  open_invoice?: TagihanLangganan
  /** Pengembalian dana terbaru yang bernilai (setelah berhenti). */
  refund?: Pengembalian
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

  /** Membatalkan tagihan ganti paket yang belum dibayar (paket tidak berubah). */
  batalkanGanti: (invoiceId: string) =>
    api.post<TagihanLangganan>(`/subscription/invoices/${invoiceId}/void`, {}),

  pratinjauBerhenti: () => api.get<PratinjauBerhenti>('/subscription/cancel-preview'),

  /** Rekening tujuan wajib bila ada uang yang dikembalikan (lihat pratinjau). */
  berhenti: (input: InputBerhenti) =>
    api.post<{
      refund_amount: number
      earned_amount: number
      months_used: number
      subscription: Langganan
    }>('/subscription/cancel', input),

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
