import { api } from '@/lib/api-client'
import type { HasilAuth, HasilDaftar, ProfilSaya } from './tipe'

export interface DaftarInput {
  business_name: string
  business_type: string
  phone: string
  outlet_name: string
  timezone?: string
  referral_code?: string
  owner: {
    name: string
    username: string
    email: string
    password: string
    device_name?: string
  }
}

export const authApi = {
  masuk: (username: string, password: string) =>
    api.post<HasilAuth>(
      '/auth/login',
      { username, password, device_name: namaPerangkat() },
      { tanpaToken: true },
    ),

  daftar: (input: DaftarInput) =>
    api.post<HasilDaftar>(
      '/auth/register',
      { ...input, owner: { ...input.owner, device_name: namaPerangkat() } },
      { tanpaToken: true },
    ),

  keluar: (refresh_token: string) => api.post<null>('/auth/logout', { refresh_token }),

  saya: () => api.get<ProfilSaya>('/me'),
}

/** Label perangkat supaya pemilik mengenali sesinya di daftar perangkat aktif. */
function namaPerangkat(): string {
  const ua = navigator.userAgent
  if (/Android/i.test(ua)) return 'HP Android'
  if (/iPhone/i.test(ua)) return 'iPhone'
  if (/iPad/i.test(ua)) return 'iPad'
  if (/Macintosh/i.test(ua)) return 'Mac'
  if (/Windows/i.test(ua)) return 'Komputer Windows'
  return 'Perangkat'
}
