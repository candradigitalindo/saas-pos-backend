import { api, type Halaman } from '@/lib/api-client'

/**
 * Panel internal penyedia SaaS — REALM KETIGA.
 *
 * Token panel ditolak di rute tenant maupun portal mitra, jadi setiap
 * panggilan menyebut realmnya sendiri.
 */
const PANEL = { realm: 'platform' } as const

/** Kemampuan, cerminan models/platform_admin.go. Menu dibentuk dari ini. */
export const KEMAMPUAN = {
  verifikasiMitra: 'partner.verify',
  keuanganMitra: 'partner.finance',
  sengketa: 'partner.dispute',
  kelolaAdmin: 'platform.admin',
  baca: 'platform.read',
  /** Memverifikasi konfirmasi pembayaran langganan (finance & superadmin). */
  verifikasiBayar: 'billing.verify',
} as const

export interface AdminPanel {
  id: string
  name: string
  email: string
  role: 'superadmin' | 'operator' | 'finance' | 'support'
  capabilities: string[]
  is_active: boolean
  last_login_at?: string
}

export interface AuthPanel {
  access_token: string
  expires_at: string
  admin: AdminPanel
}

export interface MitraPanel {
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

export interface PeristiwaOutbox {
  id: string
  tenant_id?: string
  topic: string
  status: string
  attempts: number
  last_error?: string
  available_at: string
  processed_at?: string
  created_at: string
}

/** Konfirmasi pembayaran langganan dari tenant, dengan konteksnya. */
export interface KonfirmasiBayarPanel {
  id: string
  invoice_id: string
  invoice_number: string
  amount: number
  method: string
  reference: string
  note?: string
  status: 'pending' | 'approved' | 'rejected'
  reject_reason?: string
  created_at: string
  reviewed_at?: string
  tenant_id: string
  business_name: string
  tenant_phone?: string
  invoice_total: number
  invoice_paid: number
  plan_name?: string
}

export interface HasilJalankanKomisi {
  from: string
  to: string
  referrals_seen: number
  activated: number
  computed: number
  clawed_back: number
  skipped_no_activation: number
}

export const panelApi = {
  masuk: (email: string, password: string) =>
    api.post<AuthPanel>('/platform/auth/login', { email, password }, { tanpaToken: true }),

  saya: () => api.get<AdminPanel>('/platform/me', PANEL),

  /** Berpaginasi — tidak seperti endpoint daftar lain di realm ini. */
  daftarMitra: () =>
    api.get<Halaman<MitraPanel>>('/platform/partners', { ...PANEL, query: { limit: 100 } }),

  // Verifikasi & pembekuan membalas data null; layar membaca ulang daftarnya.
  setujuiMitra: (id: string) => api.post<null>(`/platform/partners/${id}/approve`, {}, PANEL),
  bekukanMitra: (id: string, reason?: string) =>
    api.post<null>(`/platform/partners/${id}/suspend`, { reason }, PANEL),

  jalankanKomisi: (from: string, to: string) =>
    api.post<HasilJalankanKomisi>('/platform/partner-commissions/run', { from, to }, PANEL),

  /** Antrean notifikasi. Yang mati hanya terlihat di sini. */
  outbox: (status?: string) =>
    api.get<PeristiwaOutbox[]>('/platform/outbox', { ...PANEL, query: { status } }),

  ulangiOutbox: (id: string) => api.post<null>(`/platform/outbox/${id}/retry`, {}, PANEL),

  /** Konfirmasi pembayaran langganan. status: pending (bawaan) | approved | rejected | all. */
  konfirmasiBayar: (status?: string) =>
    api.get<KonfirmasiBayarPanel[]>('/platform/subscription-payment-claims', {
      ...PANEL,
      query: { status },
    }),
  /** Menyetujui = mencatat pembayaran & mengaktifkan paket tenant. */
  setujuiBayar: (id: string) =>
    api.post<unknown>(`/platform/subscription-payment-claims/${id}/approve`, {}, PANEL),
  tolakBayar: (id: string, reason: string) =>
    api.post<null>(`/platform/subscription-payment-claims/${id}/reject`, { reason }, PANEL),
}
