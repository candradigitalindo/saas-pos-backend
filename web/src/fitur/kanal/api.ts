import { api, type Halaman } from '@/lib/api-client'

/** Kinerja kanal dalam `days` hari terakhir (hanya pesanan selesai; batal terpisah). */
export interface StatistikKanal {
  days: number
  order_count: number
  /** Sebelum komisi. */
  gross_amount: number
  fee_amount: number
  /** Yang benar-benar diterima toko. */
  net_amount: number
  canceled_count: number
  last_order_at?: string
}

export interface Kanal {
  id: string
  outlet_id: string
  kind: 'pos' | 'marketplace' | 'delivery_app' | 'conversation'
  provider: string
  name: string
  merchant_ref?: string
  /** Tarif komisi kanal, string desimal: "0.20" = 20%. */
  commission_rate: string
  price_list_id?: string
  integration_mode: 'manual' | 'csv' | 'api'
  is_active: boolean
  created_at: string
  /** Sambungan API milik tenant: none | connected | error. */
  connection_status: 'none' | 'connected' | 'error'
  /** Hanya pada daftar kanal. */
  stats?: StatistikKanal
}

export interface PesananKanal {
  id: string
  channel_id: string
  sale_id: string
  external_order_id: string
  external_status?: string
  buyer_name?: string
  buyer_phone?: string
  shipping_address?: string
  courier?: string
  tracking_no?: string
  gross_amount: number
  fee_amount: number
  net_amount: number
  created_at: string
  /** Hanya pada daftar pesanan (dari penjualannya). */
  receipt_no?: string
  sale_status?: string
  occurred_at?: string
  items?: { product_name: string; qty: string; unit_name: string; line_total: number }[]
}

/** Penyedia yang bisa disambungkan dengan kredensial milik tenant sendiri. */
export interface PenyediaKanal {
  code: string
  name: string
  kind: string
  /** false = adaptornya belum dibuat; tetap bisa manual/CSV. */
  available: boolean
  docs_url: string
  steps: string[]
  fields: { key: string; label: string; help?: string; secret: boolean; optional?: boolean }[]
  capabilities: string[]
  note?: string
}

/** GET /channels/:id/connection — rahasia hanya pratinjau 4 karakter terakhir. */
export interface SambunganKanal {
  provider?: string
  status: 'none' | 'connected' | 'error'
  checked_at?: string
  error?: string
  fields: Record<string, { set: boolean; value?: string; preview?: string }>
  webhook_url?: string
  webhook_values?: { label: string; value: string }[]
  last_event_at?: string
  /** Dari tes yang baru dijalankan, mis. nama bisnis & nomor. */
  info?: string
}

/** Hasil impor CSV laporan kanal (services.ChannelOrderImportResult). */
export interface HasilImporKanal {
  imported: number
  /** Sudah tercatat pada impor sebelumnya — dilewati, tidak dobel. */
  skipped: number
  failed: number
  errors?: string[]
}

export const kanalApi = {
  /** Server mengirim seluruh kanal sebagai larik biasa, BUKAN halaman. */
  daftar: () => api.get<Kanal[]>('/channels'),

  buat: (input: {
    outlet_id: string
    kind: string
    provider: string
    name: string
    commission_rate?: string
    integration_mode?: string
  }) => api.post<Kanal>('/channels', input),

  ubah: (id: string, input: Record<string, unknown>) => api.put<Kanal>(`/channels/${id}`, input),

  daftarPesanan: (channel_id?: string, page = 1, limit = 20) =>
    api.get<Halaman<PesananKanal>>('/channel-orders', { query: { channel_id, page, limit } }),

  /**
   * Entri pesanan manual — untuk pesanan yang datang lewat WhatsApp atau
   * Instagram. Adaptor API per-provider belum ada (menunggu kemitraan), jadi
   * inilah jalur utamanya, bukan jalan darurat.
   */
  catatPesanan: (input: {
    channel_id: string
    external_order_id: string
    buyer_name?: string
    buyer_phone?: string
    items: { product_id: string; qty: string; unit_price?: number }[]
  }) => api.post<PesananKanal>('/channel-orders', input),

  batalkan: (id: string, reason: string) =>
    api.post<PesananKanal>(`/channel-orders/${id}/cancel`, { reason }),

  penyedia: () => api.get<PenyediaKanal[]>('/channel-providers'),
  sambungan: (id: string) => api.get<SambunganKanal>(`/channels/${id}/connection`),
  /** Isian rahasia yang dikosongkan = tetap memakai nilai tersimpan. */
  simpanSambungan: (id: string, provider: string, fields: Record<string, string>) =>
    api.put<SambunganKanal>(`/channels/${id}/connection`, { provider, fields }),
  tesSambungan: (id: string) => api.post<SambunganKanal>(`/channels/${id}/connection/test`),
  putusSambungan: (id: string) => api.hapus<null>(`/channels/${id}/connection`),

  /** Impor laporan harian dari marketplace sebagai CSV. */
  imporPesanan: (channelId: string, csv: string) =>
    api.postMentah<HasilImporKanal>(`/channels/${channelId}/orders/import`, csv, 'text/csv'),
}
