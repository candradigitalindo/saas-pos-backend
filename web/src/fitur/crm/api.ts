import { api, type Halaman } from '@/lib/api-client'

export interface TahapPipeline {
  id: string
  name: string
  sort_order: number
  probability: string
  is_won: boolean
  is_lost: boolean
}

export interface Pipeline {
  id: string
  name: string
  kind: string
  is_default: boolean
  stages: TahapPipeline[]
}

export interface Prospek {
  id: string
  pipeline_id: string
  stage_id: string
  customer_id?: string
  lead_source_id?: string
  owner_id: string
  title: string
  value: number
  expected_close_date?: string
  status: string
  lost_reason?: string
  closed_at?: string
  created_at: string
  updated_at: string
}

export interface Aktivitas {
  id: string
  kind: 'call' | 'chat' | 'meeting' | 'visit' | 'task' | 'note'
  subject: string
  body?: string
  owner_id: string
  customer_id?: string
  deal_id?: string
  due_at?: string
  completed_at?: string
  status: string
  created_at: string
}

export interface Kunjungan {
  id: string
  visit_plan_id?: string
  customer_id: string
  owner_id: string
  checkin_at?: string
  checkout_at?: string
  checkin_lat?: string
  checkin_lng?: string
  photo_url?: string
  result: string
  no_order_reason?: string
  sale_id?: string
  business_date: string
}

export const crmApi = {
  pipeline: () => api.get<Pipeline[]>('/pipelines'),

  daftarProspek: (page = 1, limit = 50) =>
    api.get<Halaman<Prospek>>('/deals', { query: { page, limit } }),

  buatProspek: (input: {
    title: string
    value?: number
    customer_id?: string
    expected_close_date?: string
  }) => api.post<Prospek>('/deals', input),

  ubahProspek: (id: string, input: { stage_id?: string; value?: number; title?: string }) =>
    api.put<Prospek>(`/deals/${id}`, input),

  menang: (id: string) => api.post<Prospek>(`/deals/${id}/win`, {}),
  kalah: (id: string, lost_reason: string) =>
    api.post<Prospek>(`/deals/${id}/lose`, { lost_reason }),

  daftarAktivitas: (page = 1, limit = 50) =>
    api.get<Halaman<Aktivitas>>('/activities', { query: { page, limit } }),

  buatAktivitas: (input: {
    kind: string
    subject: string
    body?: string
    deal_id?: string
    customer_id?: string
    due_at?: string
  }) => api.post<Aktivitas>('/activities', input),

  selesaikanAktivitas: (id: string) => api.post<Aktivitas>(`/activities/${id}/complete`, {}),

  // ── Kunjungan lapangan ───────────────────────────────────────────────────
  daftarKunjungan: (page = 1, limit = 50) =>
    api.get<Halaman<Kunjungan>>('/visits', { query: { page, limit } }),

  /**
   * Check-in kunjungan. `id` dibuat KLIEN (ULID) supaya operasi yang sama bisa
   * diantre saat offline lalu dikirim lewat /sync/push tanpa jadi dua baris —
   * sales lapangan bekerja di jalan, dan sinyal di jalan tidak bisa diandalkan.
   */
  checkin: (input: {
    id: string
    customer_id: string
    checkin_at: string
    checkin_lat?: string
    checkin_lng?: string
  }) => api.post<Kunjungan>('/visits', input),

  checkout: (
    id: string,
    input: { result: string; no_order_reason?: string; checkout_at?: string },
  ) => api.post<Kunjungan>(`/visits/${id}/checkout`, input),
}
