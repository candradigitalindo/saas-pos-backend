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
  /**
   * Harga grosir per jumlah, jumlah terkecil dulu. Dari API: GET satu barang;
   * di kasir: dari data lokal (daftar harga default).
   */
  wholesale_prices?: TingkatGrosir[]
  /**
   * Harga khusus per daftar harga (member/reseller). Dari API: satu harga per
   * daftar; di kasir: dari data lokal (min_qty ikut, bawaannya 1).
   */
  special_prices?: HargaKhusus[]
  /** Kemasan jual/beli (dus isi 40). Dari API: GET satu barang; kasir: data lokal. */
  packagings?: Kemasan[]
  created_at: string
  updated_at: string
}

/**
 * Kemasan barang: "dus isi 40". Stok & laporan tetap dalam satuan dasar
 * barang; kemasan punya harga sendiri (sell_price) atau isi × harga jual.
 */
export interface Kemasan {
  id: string
  unit_id: string
  unit_name: string
  /** Isi dalam satuan dasar (desimal string, > 1). */
  conversion: string
  /** Harga tersendiri; kosong = isi × harga jual. */
  sell_price?: number | null
  /** Harga kemasan yang berlaku. */
  price: number
  barcode?: string
}

/** Harga barang pada satu daftar harga khusus. */
export interface HargaKhusus {
  price_list_id: string
  price: number
  /** Jumlah minimal (desimal string); tidak ada = 1. */
  min_qty?: string
}

/** Daftar harga khusus (member, reseller). */
export interface DaftarHarga {
  id: string
  name: string
  kind: string
}

/** Satu tingkat harga grosir: beli minimal min_qty → harga satuan price. */
export interface TingkatGrosir {
  /** Desimal string, > 1. */
  min_qty: string
  price: number
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
