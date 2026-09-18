import { api, BASIS_API } from '@/lib/api-client'
import { ambilSesi } from '@/lib/penyimpanan-sesi'

/** Delapan besaran agregat yang dipakai seluruh laporan (structs/report.go). */
export interface Total {
  sales_count: number
  gross_amount: number
  discount_amount: number
  tax_amount: number
  net_amount: number
  cost_amount: number
  fee_amount: number
  gross_profit: number
}

export interface EmberKanal extends Total {
  /** Kosong = kasir langsung. */
  channel_id: string
}

export interface Dashboard {
  date: string
  outlet_id?: string
  today: Total
  month_to_date: Total
  by_channel: EmberKanal[]
}

export interface BarisLaporan extends Total {
  /** tanggal | channel_id | nama kasir | metode bayar, tergantung group_by. */
  key: string
}

export interface LaporanPenjualan {
  from: string
  to: string
  group_by: string
  rows: BarisLaporan[]
  totals: Total
}

export interface BarisUntung {
  channel_id: string
  omzet: number
  modal: number
  biaya_kanal: number
  laba_bersih: number
}

export interface LaporanUntung {
  from: string
  to: string
  by_channel: BarisUntung[]
  totals: BarisUntung
}

export type Pengelompokan = 'day' | 'hour' | 'channel' | 'cashier' | 'payment'

export const laporanApi = {
  dashboard: (outlet_id?: string, date?: string) =>
    api.get<Dashboard>('/reports/dashboard', { query: { outlet_id, date } }),

  penjualan: (from: string, to: string, group_by: Pengelompokan, outlet_id?: string) =>
    api.get<LaporanPenjualan>('/reports/sales', {
      query: { from, to, group_by, outlet_id },
    }),

  untung: (from: string, to: string, outlet_id?: string) =>
    api.get<LaporanUntung>('/reports/profit', { query: { from, to, outlet_id } }),

  /**
   * Unduh CSV.
   *
   * Endpoint ini mengembalikan berkas, bukan amplop JSON, jadi tidak lewat
   * api-client — tetapi tetap memakai token dari penyimpanan yang sama.
   */
  async unduhCSV(
    type: 'sales' | 'profit' | 'dashboard',
    q: { from?: string; to?: string; group_by?: string; outlet_id?: string; date?: string },
  ): Promise<void> {
    const p = new URLSearchParams({ type, format: 'csv' })
    for (const [k, v] of Object.entries(q)) if (v) p.set(k, v)

    const token = ambilSesi('tenant')?.access_token
    const res = await fetch(`${BASIS_API}/reports/export?${p}`, {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    })
    if (!res.ok) throw new Error('Laporan belum bisa diunduh. Coba lagi sebentar lagi.')

    const blob = await res.blob()
    const namaBerkas =
      res.headers.get('Content-Disposition')?.match(/filename="(.+?)"/)?.[1] ??
      `laporan-${type}.csv`

    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = namaBerkas
    a.click()
    URL.revokeObjectURL(url)
  },
}
