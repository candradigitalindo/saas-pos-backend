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
  items?: ItemTransaksi[]
  payments?: PembayaranTransaksi[]
  created_at: string
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
}
