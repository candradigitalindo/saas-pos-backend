/**
 * Entitas organisasi: usaha, toko, dan pengguna.
 *
 * Ditaruh di `bersama/` karena dipakai lintas modul — auth mengambilnya saat
 * masuk, pengaturan mengelolanya, dan stok memakainya untuk kirim antar toko.
 */

export interface Pengguna {
  id: string
  name: string
  username: string
  email: string
  role_id?: string
  role_name: string
  /** SELURUH peran yang dipegang. Izin efektifnya adalah gabungan. */
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

export interface Peran {
  id: string
  name: string
  description?: string
  /** true hanya untuk peran bawaan Pemilik — izin role.manage-nya terkunci. */
  is_system: boolean
  permission_codes?: string[]
  created_at: string
  updated_at: string
}

/** Satu entri katalog izin, untuk layar pengaturan peran. */
export interface KatalogIzin {
  code: string
  group_name: string
  description: string
}
