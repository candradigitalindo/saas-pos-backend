import { api, type Halaman } from '@/lib/api-client'
import type { Produk, SaldoStok } from '@/bersama/tipe/katalog'

export interface GerakanStok {
  id: string
  outlet_id: string
  product_id: string
  variant_id?: string
  kind: string
  qty_delta: string
  balance_after: string
  unit_cost: number
  ref_table?: string
  ref_id?: string
  reason?: string
  occurred_at: string
  business_date: string
}

export interface ItemPembelian {
  id: string
  product_id: string
  qty: string
  unit_cost: number
  line_total: number
}

export interface Pembelian {
  id: string
  outlet_id: string
  supplier_id?: string
  invoice_no?: string
  status: string
  subtotal: number
  discount_amount: number
  tax_amount: number
  total: number
  paid_amount: number
  occurred_at: string
  business_date: string
  items?: ItemPembelian[]
  created_at: string
}

export interface ItemOpname {
  id: string
  product_id: string
  variant_id?: string
  system_qty: string
  counted_qty: string
  diff_qty: string
}

export interface Opname {
  id: string
  outlet_id: string
  status: string
  note?: string
  counted_at?: string
  business_date: string
  items?: ItemOpname[]
  created_at: string
}

export interface ItemTransfer {
  id: string
  product_id: string
  variant_id?: string
  qty: string
}

export interface Transfer {
  id: string
  from_outlet_id: string
  to_outlet_id: string
  status: string
  note?: string
  sent_at?: string
  received_at?: string
  business_date: string
  items?: ItemTransfer[]
  created_at: string
}

/** GET /stocks/summary — hitungan atas SELURUH barang, bukan satu halaman. */
export interface RingkasanStok {
  total: number
  /** qty > batas minimum */
  safe: number
  /** 0 < qty ≤ batas minimum */
  low: number
  /** qty = 0 */
  out: number
  /** qty < 0 — catatan perlu dicocokkan */
  negative: number
  /** Σ max(qty, 0) × harga modal, rupiah */
  stock_value: number
}

export const stokApi = {
  saldo: (
    outlet_id: string,
    low?: boolean,
    page = 1,
    limit = 100,
    opsi: { cari?: string; produk?: string[] } = {},
  ) =>
    api.get<Halaman<SaldoStok>>('/stocks', {
      query: {
        outlet_id,
        low: low ? 'true' : undefined,
        q: opsi.cari || undefined,
        product_ids: opsi.produk?.length ? opsi.produk.join(',') : undefined,
        page,
        limit,
      },
    }),

  ringkasan: (outlet_id: string) =>
    api.get<RingkasanStok>('/stocks/summary', { query: { outlet_id } }),

  kartuStok: (product_id: string, outlet_id?: string, page = 1, limit = 50) =>
    api.get<Halaman<GerakanStok>>('/stock-movements', {
      query: { product_id, outlet_id, page, limit },
    }),

  /**
   * Koreksi stok. Kirim `new_qty` (set ke angka pasti, dipakai saat mengisi
   * saldo awal atau hasil hitung fisik) ATAU `delta` (geser naik/turun) —
   * tidak boleh dua-duanya. Wajib Idempotency-Key: kiriman ulang dengan kunci
   * yang sama tidak menggeser stok dua kali.
   */
  koreksi: (
    input: {
      outlet_id: string
      product_id: string
      new_qty?: string
      delta?: string
      reason: string
    },
    kunci: string,
  ) => api.post<GerakanStok>('/stock-adjustments', input, { idempotencyKey: kunci }),

  /** Barang masuk. Wajib Idempotency-Key: ini menciptakan stok DAN utang. */
  /** Satu barang lengkap dengan kemasannya — daftar barang tidak memuat kemasan. */
  barang: (id: string) => api.get<Produk>(`/products/${id}`),

  barangMasuk: (
    input: {
      outlet_id: string
      supplier_id?: string
      invoice_no?: string
      paid_amount?: number
      /** product_unit_id: kemasan yang dibeli (dus); qty & unit_cost per kemasan. */
      items: { product_id: string; product_unit_id?: string; qty: string; unit_cost: number }[]
    },
    kunci: string,
  ) => api.post<Pembelian>('/purchases', input, { idempotencyKey: kunci }),

  daftarPembelian: (page = 1, limit = 20) =>
    api.get<Halaman<Pembelian>>('/purchases', { query: { page, limit } }),

  pembelian: (id: string) => api.get<Pembelian>(`/purchases/${id}`),

  // ── Hitung fisik (opname) ────────────────────────────────────────────────
  //
  // Tiga langkah terpisah di backend, dan itu justru cocok dengan cara orang
  // bekerja: sesi dibuat dulu, hitungan disimpan sambil berjalan, baru
  // diposting di akhir. Petugas bisa berhenti di tengah tanpa kehilangan apa
  // pun yang sudah dihitung.
  buatOpname: (outlet_id: string, note?: string) =>
    api.post<Opname>('/stock-opnames', { outlet_id, note }),

  simpanHitungan: (id: string, items: { product_id: string; counted_qty: string }[]) =>
    api.post<Opname>(`/stock-opnames/${id}/items`, { items }),

  /** Menerapkan hasil hitungan ke stok. Tidak bisa dibatalkan. */
  postingOpname: (id: string) => api.post<Opname>(`/stock-opnames/${id}/post`, {}),

  daftarOpname: (page = 1, limit = 20) =>
    api.get<Halaman<Opname>>('/stock-opnames', { query: { page, limit } }),

  opname: (id: string) => api.get<Opname>(`/stock-opnames/${id}`),

  // ── Kirim barang antar toko ──────────────────────────────────────────────
  buatTransfer: (input: {
    from_outlet_id: string
    to_outlet_id: string
    note?: string
    items: { product_id: string; qty: string }[]
  }) => api.post<Transfer>('/stock-transfers', input),

  kirimTransfer: (id: string) => api.post<Transfer>(`/stock-transfers/${id}/send`, {}),

  terimaTransfer: (id: string) => api.post<Transfer>(`/stock-transfers/${id}/receive`, {}),

  daftarTransfer: (page = 1, limit = 20) =>
    api.get<Halaman<Transfer>>('/stock-transfers', { query: { page, limit } }),

  transfer: (id: string) => api.get<Transfer>(`/stock-transfers/${id}`),
}
