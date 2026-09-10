import { api, type Halaman } from '@/lib/api-client'

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
}

export const kanalApi = {
  daftar: () => api.get<Halaman<Kanal>>('/channels', { query: { limit: 100 } }),

  buat: (input: {
    outlet_id: string
    kind: string
    provider: string
    name: string
    commission_rate?: string
    integration_mode?: string
  }) => api.post<Kanal>('/channels', input),

  ubah: (id: string, input: Record<string, unknown>) => api.put<Kanal>(`/channels/${id}`, input),

  hapus: (id: string) => api.hapus<null>(`/channels/${id}`),

  daftarPesanan: (channel_id?: string, page = 1, limit = 25) =>
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

  ubahStatus: (id: string, external_status: string) =>
    api.post<PesananKanal>(`/channel-orders/${id}/status`, { external_status }),

  batalkan: (id: string, reason: string) =>
    api.post<PesananKanal>(`/channel-orders/${id}/cancel`, { reason }),

  /** Impor laporan harian dari marketplace sebagai CSV. */
  imporPesanan: (channelId: string, csv: string) =>
    api.postMentah<{ total: number; imported: number; failed: number }>(
      `/channels/${channelId}/orders/import`,
      csv,
      'text/csv',
    ),
}
