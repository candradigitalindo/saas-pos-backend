import { api, type Halaman } from '@/lib/api-client'
import type { RingkasanPenjualan,
  DiskonTagihan,
  GerakanKas,
  ItemTagihan,
  MetodeBayar,
  Shift,
  TagihanTerbuka,
  Transaksi,
} from '@/bersama/tipe/pos'
import type { Produk, SaldoStok } from '@/bersama/tipe/katalog'

export interface ItemCheckout {
  product_id: string
  variant_id?: string
  qty: string
  discount_amount?: number
  note?: string
}

export interface BayarCheckout {
  method: MetodeBayar
  amount: number
  reference?: string
}

export interface InputCheckout {
  /** ULID dibuat klien saat mode offline; server membuatnya bila kosong. */
  id?: string
  outlet_id: string
  shift_id?: string
  customer_id?: string
  order_type?: 'dine_in' | 'takeaway' | 'delivery' | 'pickup'
  order_discount?: number
  note?: string
  client_created_at?: string
  /** Tagihan terbuka yang dilunasi transaksi ini (ditutup server di transaksi yang sama). */
  open_bill_id?: string
  items: ItemCheckout[]
  payments: BayarCheckout[]
}

/** Isi PUT /open-bills/:id — tagihan utuh; base_version 0 = baru. */
export interface InputTagihan {
  outlet_id: string
  label: string
  items: ItemTagihan[]
  order_discount?: DiskonTagihan
  base_version: number
}

export const kasirApi = {
  // ── Shift ────────────────────────────────────────────────────────────────
  bukaShift: (outlet_id: string, opening_cash: number, note?: string) =>
    api.post<Shift>('/shifts/open', { outlet_id, opening_cash, note }),

  tutupShift: (id: string, counted_cash: number, note?: string) =>
    api.post<Shift>(`/shifts/${id}/close`, { counted_cash, note }),

  /**
   * Serah terima: tutup shift berjalan + buka yang baru, SATU transaksi.
   *
   * Bukan dua panggilan dari sini: ada batasan satu shift terbuka per outlet,
   * jadi gagal di tengah meninggalkan kasir tanpa shift terbuka sama sekali.
   */
  serahTerimaShift: (
    id: string,
    uangDihitung: number,
    modalDitinggal?: number,
    catatan?: string,
  ) =>
    api.post<{ ditutup: Shift; dibuka: Shift }>(`/shifts/${id}/handover`, {
      counted_cash: uangDihitung,
      opening_cash: modalDitinggal,
      note: catatan,
    }),

  daftarShift: (outlet_id?: string, page = 1, limit = 20) =>
    api.get<Halaman<Shift>>('/shifts', { query: { outlet_id, page, limit } }),

  /** Rincian kas ikut dihitung di sini, termasuk untuk shift yang masih terbuka. */
  shift: (id: string) => api.get<Shift>(`/shifts/${id}`),

  // ── Katalog untuk grid kasir ─────────────────────────────────────────────
  produk: (q?: string, category_id?: string, page = 1, limit = 100) =>
    api.get<Halaman<Produk>>('/products', {
      query: { q, category_id, is_active: 'true', page, limit },
    }),

  stok: (outlet_id: string, limit = 100) =>
    api.get<Halaman<SaldoStok>>('/stocks', { query: { outlet_id, limit } }),

  // ── Tagihan terbuka ──────────────────────────────────────────────────────
  daftarTagihan: (outlet_id: string) => api.get<TagihanTerbuka[]>('/open-bills', { query: { outlet_id } }),
  simpanTagihan: (id: string, input: InputTagihan) => api.put<TagihanTerbuka>(`/open-bills/${id}`, input),
  batalkanTagihan: (id: string, base_version: number) =>
    api.post<null>(`/open-bills/${id}/cancel`, { base_version }),

  // ── Transaksi ────────────────────────────────────────────────────────────
  /**
   * Checkout. `kunci` WAJIB dan harus dibuat SEKALI saat pengguna menekan
   * Bayar — kunci yang sama dipakai untuk semua percobaan ulang. Inilah yang
   * membuat tombol tertekan dua kali tidak menghasilkan dua transaksi.
   */
  bayar: (input: InputCheckout, kunci: string) =>
    api.post<Transaksi>('/sales', input, { idempotencyKey: kunci }),

  daftarTransaksi: (
    filter: {
      outlet_id?: string
      business_date?: string
      status?: string
      shift_id?: string
      /** Potongan nomor nota. */
      search?: string
      /** cash | qris | credit | … */
      method?: string
    },
    page = 1,
    limit = 20,
  ) => api.get<Halaman<Transaksi>>('/sales', { query: { ...filter, page, limit } }),

  ringkasanHari: (outlet_id: string | undefined, business_date: string) =>
    api.get<RingkasanPenjualan>('/sales/day-summary', { query: { outlet_id, business_date } }),

  transaksi: (id: string) => api.get<Transaksi>(`/sales/${id}`),

  batalkan: (id: string, reason: string) =>
    api.post<Transaksi>(`/sales/${id}/void`, { reason }),

  retur: (id: string, reason?: string) =>
    api.post<Transaksi>(`/sales/${id}/refund`, { reason }),

  // ── Kas laci ─────────────────────────────────────────────────────────────
  catatKas: (input: {
    outlet_id: string
    shift_id?: string
    direction: 'in' | 'out'
    amount: number
    reason: string
  }) => api.post<GerakanKas>('/cash-movements', input),

  daftarKas: (shift_id?: string, page = 1, limit = 50) =>
    api.get<Halaman<GerakanKas>>('/cash-movements', { query: { shift_id, page, limit } }),
}
