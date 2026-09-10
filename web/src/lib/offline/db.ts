import Dexie, { type EntityTable } from 'dexie'

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
  op: 'sale.create' | 'visit.upsert'
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
