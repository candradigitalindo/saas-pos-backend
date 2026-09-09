/** Bentuk data auth — disalin dari structs/auth.go & structs/user.go. */

export interface Pengguna {
  id: string
  name: string
  username: string
  email: string
  role_id?: string
  role_name: string
  role_ids?: string[]
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface Usaha {
  id: string
  business_name: string
  business_type: string
  owner_name: string
  phone: string
  email?: string
  status: string
  created_at: string
}

export interface Toko {
  id: string
  name: string
  type: string
  address?: string
  phone?: string
  timezone: string
  business_day_start: string
  currency: string
  tax_enabled: boolean
  tax_rate: string
  tax_inclusive: boolean
  service_charge_rate: string
  receipt_header?: string
  receipt_footer?: string
  is_active: boolean
}

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
