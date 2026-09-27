/** Bentuk data auth — disalin dari structs/auth.go & structs/user.go. */

export type { Pengguna, Toko, Usaha } from '@/bersama/tipe/organisasi'
import type { Pengguna, Toko, Usaha } from '@/bersama/tipe/organisasi'

export interface HasilAuth {
  access_token: string
  refresh_token: string
  token_type: string
  expires_in: number
  user: Pengguna
}

export interface ProfilSaya {
  user: Pengguna
  tenant: Usaha
  permissions: string[]
  outlet_ids: string[]
  /** Rincian cabang pada outlet_ids (pajak, biaya layanan, zona waktu). */
  outlets?: Toko[]
  /**
   * Paket langganan yang SEDANG BERLAKU (bukan sekadar yang dipilih). Opsional:
   * profil yang tersimpan dari versi lama belum memuatnya.
   */
  plan?: PaketBerlaku
}

/** structs.PlanEntitlementResponse */
export interface PaketBerlaku {
  code: string
  name: string
  /** 'trial' | 'active' | 'past_due' — atau 'none' untuk paket Gratis. */
  status: string
  active_until?: string
  features: Record<string, boolean>
  max_outlets: number | null
  max_users: number | null
  max_products: number | null
  /** Fitur yang terkunci → nama paket termurah yang membukanya. */
  upgrade_for?: Record<string, string>
}

export interface HasilDaftar {
  tenant: Usaha
  outlet: Toko
  auth: HasilAuth
}

/** Jenis usaha yang diterima backend (binding oneof). */
export const JENIS_USAHA = [
  { nilai: 'retail', label: 'Toko / Warung' },
  { nilai: 'fnb', label: 'Makanan & Minuman' },
  { nilai: 'service', label: 'Jasa' },
  { nilai: 'wholesale', label: 'Grosir' },
  { nilai: 'other', label: 'Lainnya' },
] as const
