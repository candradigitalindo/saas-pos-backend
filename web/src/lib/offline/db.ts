import Dexie, { type EntityTable } from 'dexie'
import type { TagihanTerbuka } from '@/bersama/tipe/pos'

/**
 * Penyimpanan lokal untuk mode offline.
 *
 * Bentuk data di sini mengikuti apa yang DIKEMBALIKAN `GET /sync/pull`, yaitu
 * model mentah backend — bukan DTO layar. Bedanya nyata dan harus diingat:
 * produk hasil pull TIDAK punya `unit_name`/`category_name` (itu hasil join di
 * DTO), dan `category_id`/`sku`/`barcode` bisa bernilai null.
 *
 * Karena itu nama satuan dirangkai lokal dari tabel `satuan`.
 */

export interface ProdukLokal {
  id: string
  category_id: string | null
  unit_id: string
  name: string
  sku: string | null
  barcode: string | null
  sell_price: number
  cost_price: number
  track_stock: boolean
  min_stock: string
  is_active: boolean
  image_url: string
  sync_version: number
  /** Huruf kecil dari nama+sku+barcode — dipakai mencari tanpa memindai semua. */
  cari: string
}

/** Varian barang hasil pull (model mentah; sku/barcode bisa null). */
export interface VarianLokal {
  id: string
  product_id: string
  name: string
  sku: string | null
  barcode: string | null
  price_delta: number
  is_active: boolean
  sync_version: number
}

/**
 * Salinan lokal tagihan terbuka (open bill). Sumber kebenarannya server —
 * salinan ini supaya daftar tagihan tetap tampil & bisa diubah saat offline.
 * `tertunda` = ada perubahan lokal yang belum sampai server (di antrean).
 */
export interface TagihanLokal extends TagihanTerbuka {
  tertunda?: boolean
}

/** Kemasan barang hasil pull (product_units). */
export interface KemasanLokal {
  id: string
  product_id: string
  unit_id: string
  conversion: string
  sell_price: number | null
  barcode: string | null
  sync_version: number
}

/** Daftar harga (price_lists). Yang is_default memuat harga grosir per jumlah. */
export interface DaftarHargaLokal {
  id: string
  name: string
  kind: string
  is_default: boolean
  sync_version: number
}

/** Satu harga pada daftar harga (product_prices) — tingkat per jumlah minimal. */
export interface HargaProdukLokal {
  id: string
  product_id: string
  variant_id: string | null
  price_list_id: string
  min_qty: string
  price: number
  sync_version: number
}

export interface KategoriLokal {
  id: string
  name: string
  sort_order: number
  sync_version: number
}

export interface SatuanLokal {
  id: string
  name: string
  sync_version: number
}

export interface PelangganLokal {
  id: string
  name: string
  phone: string | null
  credit_limit: number
  /** Daftar harga khusus (member/reseller); null = harga umum. */
  price_list_id?: string | null
  sync_version: number
}

export interface StokLokal {
  /** outlet_id + '|' + product_id + '|' + variant_id */
  kunci: string
  outlet_id: string
  product_id: string
  variant_id: string
  qty: string
  reserved_qty: string
}

/** Status satu operasi di antrean kirim. */
export type StatusAntrean = 'menunggu' | 'mengirim' | 'terkirim' | 'perlu-diperiksa'

export interface AntreanOperasi {
  /** ULID entitas, dibuat klien. Juga dipakai sebagai kunci idempotensi. */
  id: string
  op: 'sale.create' | 'visit.upsert' | 'open_bill.upsert' | 'open_bill.cancel'
  payload: unknown
  status: StatusAntrean
  dibuatPada: number
  percobaan: number
  /** Alasan gagal, apa adanya dari server — ditampilkan di halaman pemeriksaan. */
  alasan?: string
  /** Ringkasan untuk ditampilkan tanpa membuka payload. */
  ringkasan: string
  nominal: number
}

export interface Meta {
  kunci: string
  nilai: string | number
}

class DBOffline extends Dexie {
  produk!: EntityTable<ProdukLokal, 'id'>
  varian!: EntityTable<VarianLokal, 'id'>
  tagihan!: EntityTable<TagihanLokal, 'id'>
  daftarHarga!: EntityTable<DaftarHargaLokal, 'id'>
  hargaProduk!: EntityTable<HargaProdukLokal, 'id'>
  kemasan!: EntityTable<KemasanLokal, 'id'>
  kategori!: EntityTable<KategoriLokal, 'id'>
  satuan!: EntityTable<SatuanLokal, 'id'>
  pelanggan!: EntityTable<PelangganLokal, 'id'>
  stok!: EntityTable<StokLokal, 'kunci'>
  antrean!: EntityTable<AntreanOperasi, 'id'>
  meta!: EntityTable<Meta, 'kunci'>

  constructor() {
    super('pos-umkm')

    // Versi lama tetap dideklarasikan: perangkat yang sudah memakai v1 butuh
    // jalur naik yang jelas, dan Dexie memindahkan datanya sendiri selama
    // rantainya utuh. Menghapus baris ini akan mengosongkan katalog offline
    // milik kasir yang sudah terpasang.
    this.version(1).stores({
      produk: 'id, name, cari, category_id, is_active',
      kategori: 'id, name',
      satuan: 'id, name',
      pelanggan: 'id, name, phone',
      stok: 'kunci, product_id, outlet_id',
      antrean: 'id, status, dibuatPada',
      meta: 'kunci',
    })

    // v2 menambah indeks barcode & sku supaya pemindai kasir bisa mencari
    // tanpa memindai seluruh tabel.
    this.version(2).stores({
      produk: 'id, name, cari, category_id, is_active, barcode, sku',
      kategori: 'id, name',
      satuan: 'id, name',
      pelanggan: 'id, name, phone',
      stok: 'kunci, product_id, outlet_id',
      antrean: 'id, status, dibuatPada',
      meta: 'kunci',
    })

    // v3 menambah varian barang (pilihan harga di kasir; dicari juga lewat
    // barcode/SKU-nya). Tabel lain tidak berubah. Kursor dibuang supaya tarikan
    // berikutnya mengambil ulang semuanya — varian yang sudah ada di server
    // sebelum perangkat ini diperbarui ikut terbawa.
    this.version(3)
      .stores({
        produk: 'id, name, cari, category_id, is_active, barcode, sku',
        varian: 'id, product_id, barcode, sku',
        kategori: 'id, name',
        satuan: 'id, name',
        pelanggan: 'id, name, phone',
        stok: 'kunci, product_id, outlet_id',
        antrean: 'id, status, dibuatPada',
        meta: 'kunci',
      })
      .upgrade((tx) => tx.table('meta').delete('kursor-sync'))

    // v4 menambah salinan tagihan terbuka (open bill) per cabang.
    this.version(4).stores({
      produk: 'id, name, cari, category_id, is_active, barcode, sku',
      varian: 'id, product_id, barcode, sku',
      tagihan: 'id, outlet_id',
      kategori: 'id, name',
      satuan: 'id, name',
      pelanggan: 'id, name, phone',
      stok: 'kunci, product_id, outlet_id',
      antrean: 'id, status, dibuatPada',
      meta: 'kunci',
    })

    // v5 menyimpan daftar harga & harga per jumlah (harga grosir) supaya
    // pratinjau keranjang dan struk offline memakai harga yang sama dengan
    // server. Kursor dibuang: harga yang sudah ada di server ikut tertarik.
    this.version(5)
      .stores({
        produk: 'id, name, cari, category_id, is_active, barcode, sku',
        varian: 'id, product_id, barcode, sku',
        tagihan: 'id, outlet_id',
        daftarHarga: 'id',
        hargaProduk: 'id, product_id, price_list_id',
        kategori: 'id, name',
        satuan: 'id, name',
        pelanggan: 'id, name, phone',
        stok: 'kunci, product_id, outlet_id',
        antrean: 'id, status, dibuatPada',
        meta: 'kunci',
      })
      .upgrade((tx) => tx.table('meta').delete('kursor-sync'))

    // v6: pelanggan membawa daftar harga khususnya. Skema tabel tidak berubah;
    // kursor dibuang supaya pelanggan lama ditarik ulang lengkap dengan
    // price_list_id-nya.
    this.version(6)
      .stores({
        produk: 'id, name, cari, category_id, is_active, barcode, sku',
        varian: 'id, product_id, barcode, sku',
        tagihan: 'id, outlet_id',
        daftarHarga: 'id',
        hargaProduk: 'id, product_id, price_list_id',
        kategori: 'id, name',
        satuan: 'id, name',
        pelanggan: 'id, name, phone',
        stok: 'kunci, product_id, outlet_id',
        antrean: 'id, status, dibuatPada',
        meta: 'kunci',
      })
      .upgrade((tx) => tx.table('meta').delete('kursor-sync'))

    // v7: kemasan barang (dus isi 40) — dijual & dipindai kasir offline.
    this.version(7)
      .stores({
        produk: 'id, name, cari, category_id, is_active, barcode, sku',
        varian: 'id, product_id, barcode, sku',
        kemasan: 'id, product_id, barcode',
        tagihan: 'id, outlet_id',
        daftarHarga: 'id',
        hargaProduk: 'id, product_id, price_list_id',
        kategori: 'id, name',
        satuan: 'id, name',
        pelanggan: 'id, name, phone',
        stok: 'kunci, product_id, outlet_id',
        antrean: 'id, status, dibuatPada',
        meta: 'kunci',
      })
      .upgrade((tx) => tx.table('meta').delete('kursor-sync'))
  }
}

export const db = new DBOffline()

const KUNCI_KURSOR = 'kursor-sync'
const KUNCI_PERANGKAT = 'device-id'

export async function ambilKursor(): Promise<number> {
  const m = await db.meta.get(KUNCI_KURSOR)
  return typeof m?.nilai === 'number' ? m.nilai : 0
}

export async function simpanKursor(kursor: number): Promise<void> {
  await db.meta.put({ kunci: KUNCI_KURSOR, nilai: kursor })
}

/**
 * Id perangkat, dibuat sekali lalu menetap. Backend memakainya untuk mengenali
 * asal kiriman batch.
 */
export async function idPerangkat(): Promise<string> {
  const m = await db.meta.get(KUNCI_PERANGKAT)
  if (typeof m?.nilai === 'string') return m.nilai
  const baru = `web-${crypto.randomUUID()}`
  await db.meta.put({ kunci: KUNCI_PERANGKAT, nilai: baru })
  return baru
}

/** Membuang seluruh data lokal — dipakai saat keluar dari akun. */
export async function kosongkanDB(): Promise<void> {
  await Promise.all([
    db.produk.clear(),
    db.varian.clear(),
    db.tagihan.clear(),
    db.daftarHarga.clear(),
    db.hargaProduk.clear(),
    db.kemasan.clear(),
    db.kategori.clear(),
    db.satuan.clear(),
    db.pelanggan.clear(),
    db.stok.clear(),
    db.antrean.clear(),
    db.meta.clear(),
  ])
}

export function kunciStok(outletId: string, productId: string, variantId = ''): string {
  return `${outletId}|${productId}|${variantId}`
}

/** Teks pencarian yang disimpan bersama produk supaya pencarian tidak perlu memindai. */
export function teksCari(p: { name: string; sku?: string | null; barcode?: string | null }): string {
  return [p.name, p.sku ?? '', p.barcode ?? ''].join(' ').toLowerCase()
}
