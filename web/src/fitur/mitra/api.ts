import { api } from '@/lib/api-client'

/**
 * Portal mitra — REALM TERPISAH.
 *
 * Token tenant ditolak di sini dan sebaliknya, jadi setiap panggilan wajib
 * menyebut `realm: 'mitra'` supaya api-client mengambil token dari penyimpanan
 * yang benar. Salah realm bukan sekadar 401: itu berarti mitra melihat data
 * toko, atau sebaliknya.
 */
const MITRA = { realm: 'mitra' } as const

// Catatan penting: endpoint daftar di realm mitra mengembalikan ARRAY POLOS,
// bukan amplop paginasi seperti rute tenant. Bentuknya memang berbeda, dan
// menyamakannya di sini hanya akan menyembunyikan perbedaan itu dari layar.

export interface Mitra {
  id: string
  tier_name: string
  kind: string
  name: string
  phone?: string
  email?: string
  region?: string
  referral_code: string
  status: string
  verified_at?: string
  joined_at?: string
}

export interface PenggunaMitra {
  id: string
  name: string
  email: string
  phone?: string
}

export interface AuthMitra {
  access_token: string
  expires_at: string
  partner: Mitra
  user: PenggunaMitra
}

export interface Pencairan {
  id: string
  period_start: string
  period_end: string
  gross_amount: number
  clawback_amount: number
  tax_amount: number
  net_amount: number
  status: string
  transfer_proof_url?: string
  tax_slip_url?: string
  paid_at?: string
}

export interface DashboardMitra {
  leads_by_status: Record<string, number>
  merchants_total: number
  merchants_active: number
  /** Komisi yang belum disetujui — belum bisa dicairkan. */
  commission_held: number
  commission_approved: number
  commission_paid: number
  last_payout?: Pencairan
}

export interface Prospek {
  id: string
  business_name: string
  contact_name?: string
  phone?: string
  city?: string
  status: string
  converted_tenant_id?: string
  attribution_expires_at: string
  created_at: string
}

/**
 * Merchant binaan — SENGAJA hanya lima field.
 *
 * Blueprint G.8: tidak pernah omzet, produk, harga, pelanggan, atau transaksi.
 * Batas ini ditampilkan permanen di layarnya supaya mitra tahu datanya memang
 * sedikit karena aturan, bukan karena aplikasinya rusak.
 */
export interface MerchantBinaan {
  tenant_id: string
  business_name: string
  subscription_status: string
  current_period_end?: string
  is_active: boolean
  attributed_at: string
  activated: boolean
}

export interface KomisiMitra {
  id: string
  referral_id: string
  subscription_invoice_id: string
  period_month: string
  base_amount: number
  rate: string
  amount: number
  status: string
  payout_id?: string
}

export const mitraApi = {
  masuk: (email: string, password: string) =>
    api.post<AuthMitra>('/partner/auth/login', { email, password }, { tanpaToken: true }),

  saya: () => api.get<{ partner: Mitra; user: PenggunaMitra }>('/partner/me', MITRA),

  dashboard: () => api.get<DashboardMitra>('/partner/dashboard', MITRA),

  daftarProspek: () => api.get<Prospek[]>('/partner/leads', MITRA),

  daftarkanProspek: (input: {
    business_name: string
    contact_name?: string
    phone: string
    city?: string
    note?: string
  }) => api.post<Prospek>('/partner/leads', input, MITRA),

  merchantBinaan: () => api.get<MerchantBinaan[]>('/partner/merchants', MITRA),

  komisi: () => api.get<KomisiMitra[]>('/partner/commissions', MITRA),

  pencairan: () => api.get<Pencairan[]>('/partner/payouts', MITRA),
}
