/** Bentuk data kasir dari backend (structs/pos.go, structs/sale.go). */

export interface Shift {
  id: string
  outlet_id: string
  status: 'open' | 'closed'
  opened_by: string
  closed_by?: string
  opened_at: string
  closed_at?: string
  business_date: string
  opening_cash: number
  expected_cash: number
  counted_cash?: number
  difference?: number
  note?: string
  /** Rincian pembentuk expected_cash, diisi pada GET /shifts/:id. */
  cash_sales?: number
  cash_in?: number
  cash_out?: number
}

export interface GerakanKas {
  id: string
  outlet_id: string
  shift_id: string
  direction: 'in' | 'out'
  amount: number
  reason: string
  occurred_at: string
  business_date: string
}

/** Metode bayar yang diterima backend. 'credit' = kasbon. */
export type MetodeBayar = 'cash' | 'qris' | 'transfer' | 'card' | 'ewallet' | 'credit'

export interface ItemTransaksi {
  id: string
  product_id: string
  variant_id?: string
  product_name: string
  unit_name: string
  qty: string
  unit_price: number
  unit_cost: number
  discount_amount: number
  tax_amount: number
  line_total: number
  note?: string
}

export interface PembayaranTransaksi {
  id: string
  method: MetodeBayar
  amount: number
  reference?: string
  fee_amount: number
  paid_at: string
}

export interface Transaksi {
  id: string
  outlet_id: string
  shift_id?: string
  customer_id?: string
  receipt_no: string
  order_type: string
  status: string
  subtotal: number
  discount_amount: number
  tax_amount: number
  service_amount: number
  rounding_amount: number
  total: number
  paid_amount: number
  change_amount: number
  cost_total: number
  gross_profit: number
  return_of_sale_id?: string
  note?: string
  occurred_at: string
  business_date: string
  voided_at?: string
  void_reason?: string
  /** Hanya di daftar riwayat (GET /sales). */
  customer_name?: string
  cashier_name?: string
  /** Hanya di daftar riwayat: nota yang diretur (baris retur) / nota returnya (penjualan asal). */
  return_of_receipt_no?: string
  returned_by_receipt_no?: string
  items?: ItemTransaksi[]
  payments?: PembayaranTransaksi[]
  created_at: string
}

/**
 * Ringkasan penjualan satu hari (GET /sales/day-summary) atau satu shift
 * (`sales` pada GET /shifts/:id).
 */
export interface RingkasanPenjualan {
  /** Hanya pada ringkasan harian. */
  business_date?: string
  sales_count: number
  sales_total: number
  average_sale: number
  returns_count: number
  /** Positif: nilai yang dikembalikan ke pembeli. */
  returns_total: number
  canceled_count: number
  canceled_total: number
  /** Penjualan − retur. */
  net_total: number
  /** Uang masuk per cara bayar; tunai sudah dikurangi kembalian. */
  by_method: { method: string; count: number; amount: number }[]
}

/** Ringkasan belanja pelanggan — hanya ada di daftar (GET /customers). */
export interface StatistikPelanggan {
  visit_count: number
  /** Belanja bersih: retur mengurangi, void tidak ikut. */
  total_spent: number
  last_visit_at?: string
  /** Hanya dikirim kepada pemegang izin receivable.manage. */
  receivable_outstanding?: number
}

export interface Pelanggan {
  id: string
  code?: string
  name: string
  phone?: string
  email?: string
  address?: string
  type: string
  credit_limit: number
  note?: string
  created_at: string
  updated_at: string
  stats?: StatistikPelanggan
}
