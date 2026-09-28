/**
 * Bentuk data katalog dari backend (structs/master_data.go, structs/pos.go).
 *
 * Ditaruh di `bersama/` karena dipakai lintas modul — kasir membaca katalog
 * untuk menjual, modul barang mengubahnya. Aturan "fitur tidak saling
 * mengimpor" tetap utuh.
 *
 * Ingat dua bentuk angka:
 *   - uang  → number, bilangan BULAT rupiah
 *   - jumlah → string desimal ("1.5"), supaya 0,5 kg tetap presisi
 */

export interface Produk {
  id: string
  name: string
  category_id?: string
  category_name?: string
  unit_id: string
  unit_name?: string
  sku?: string
  barcode?: string
  sell_price: number
  cost_price: number
  track_stock: boolean
  min_stock: string
  is_active: boolean
  image_url?: string
  /** Untuk menu aplikasi antar (GoFood/GrabFood). */
  description?: string
  /** Varian AKTIF (kasir: dari data lokal) — pilihan dengan selisih harga. */
  varian?: VarianProduk[]
  created_at: string
  updated_at: string
}

/**
 * Varian barang = pilihan dengan selisih harga (ukuran, es/panas, level
 * pedas). Stok tetap dihitung di tingkat barang.
 */
export interface VarianProduk {
  id: string
  product_id: string
  name: string
  /** Selisih terhadap harga jual barang; boleh negatif. */
  price_delta: number
  /** Harga jual barang + selisih (dari API). */
  price?: number
  sku?: string
  barcode?: string
  is_active: boolean
}

export interface Kategori {
  id: string
  parent_id?: string
  name: string
  sort_order: number
  created_at: string
  updated_at: string
}

export interface Satuan {
  id: string
  name: string
  base_unit_id?: string
  conversion: string
  allow_decimal: boolean
  created_at: string
  updated_at: string
}

export interface Pemasok {
  id: string
  name: string
  phone?: string
  address?: string
  note?: string
  created_at: string
  updated_at: string
}

export interface SaldoStok {
  outlet_id: string
  product_id: string
  variant_id?: string
  product_name: string
  unit_name: string
  qty: string
  reserved_qty: string
  min_stock: string
  low: boolean
}
