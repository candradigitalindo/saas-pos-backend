import { api, type Halaman } from '@/lib/api-client'
import type { KatalogIzin, Peran, Pengguna, Toko } from '@/bersama/tipe/organisasi'

export interface InputToko {
  name: string
  type?: string
  address?: string
  phone?: string
  timezone?: string
  business_day_start?: string
}

export interface InputPengguna {
  name: string
  username: string
  email: string
  password: string
  role_id: string
  /** Peran TAMBAHAN. Izin efektif = gabungan peran utama + peran-peran ini. */
  role_ids?: string[]
  /** Cabang tempat staf boleh bekerja. Tidak dikirim saat membuat = semua cabang aktif. */
  outlet_ids?: string[]
}

export const pengaturanApi = {
  // ── Toko / cabang ────────────────────────────────────────────────────────
  daftarToko: () => api.get<Halaman<Toko>>('/outlets', { query: { limit: 100 } }),
  buatToko: (input: InputToko) => api.post<Toko>('/outlets', input),
  ubahToko: (id: string, input: Partial<InputToko>) => api.put<Toko>(`/outlets/${id}`, input),
  hapusToko: (id: string) => api.hapus<null>(`/outlets/${id}`),

  // ── Pengguna ─────────────────────────────────────────────────────────────
  daftarPengguna: () => api.get<Halaman<Pengguna>>('/users', { query: { limit: 100 } }),
  buatPengguna: (input: InputPengguna) => api.post<Pengguna>('/users', input),
  ubahPengguna: (
    id: string,
    input: Partial<Omit<InputPengguna, 'role_ids'>> & {
      role_ids?: string[]
      is_active?: boolean
    },
  ) => api.put<Pengguna>(`/users/${id}`, input),
  hapusPengguna: (id: string) => api.hapus<null>(`/users/${id}`),

  // ── Peran & hak akses ────────────────────────────────────────────────────
  daftarPeran: () => api.get<Halaman<Peran>>('/roles', { query: { limit: 100 } }),
  peran: (id: string) => api.get<Peran>(`/roles/${id}`),
  buatPeran: (name: string, description?: string, permission_codes?: string[]) =>
    api.post<Peran>('/roles', { name, description, permission_codes }),
  ubahPeran: (id: string, input: { name?: string; description?: string }) =>
    api.put<Peran>(`/roles/${id}`, input),
  /** Mengganti SELURUH pemetaan izin sebuah peran. */
  aturIzinPeran: (id: string, permission_codes: string[]) =>
    api.put<Peran>(`/roles/${id}/permissions`, { permission_codes }),
  hapusPeran: (id: string) => api.hapus<null>(`/roles/${id}`),

  /** Katalog izin lengkap, sudah berkelompok lewat `group_name`. */
  katalogIzin: () => api.get<KatalogIzin[]>('/permissions'),
}
