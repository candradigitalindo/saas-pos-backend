import { api } from '@/lib/api-client'
import {
  ambilKursor,
  db,
  kunciStok,
  simpanKursor,
  teksCari,
  type KategoriLokal,
  type PelangganLokal,
  type ProdukLokal,
  type SatuanLokal,
  type StokLokal,
} from './db'

/**
 * Bentuk apa adanya dari GET /sync/pull — model backend, bukan DTO layar.
 *
 * Setiap daftar bisa datang sebagai `null`, BUKAN `[]`: Go memarshal irisan nil
 * jadi `null`. Itu keadaan normal untuk warung baru yang belum punya kategori
 * atau pelanggan — persis pengguna pertama yang lahir dari alur onboarding.
 * Backend sudah diperbaiki mengirim `[]`, tapi tipe di sini tetap mengakui
 * `null` supaya klien lama tidak pecah saat bicara dengan server lama.
 */
export interface PerubahanTarik {
  cursor: number
  has_more: boolean
  safety_lag: number
  categories: null | {
    id: string
    name: string
    sort_order: number
    sync_version: number
  }[]
  units: null | { id: string; name: string; sync_version: number }[]
  products: null | {
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
  }[]
  customers: null | {
    id: string
    name: string
    phone: string | null
    credit_limit: number
    sync_version: number
  }[]
  stocks: null | {
    outlet_id: string
    product_id: string
    variant_id: string
    qty: string
    reserved_qty: string
  }[]
  /** Batu nisan penghapusan: { "products": ["id1", …], … } */
  deleted: null | Record<string, string[]>
}

/**
 * Menarik master data ke penyimpanan lokal.
 *
 * Dua hal yang mudah salah dan sengaja ditangani di sini:
 *
 *  1. Kursor yang disimpan adalah `cursor` DIKURANGI `safety_lag`. Backend
 *     memintanya begitu karena nomor sync_version dialokasikan sebelum transaksi
 *     selesai — menyimpan kursor mentah bisa melewatkan baris yang commit
 *     belakangan. Menarik ulang sedikit data jauh lebih murah daripada kehilangan
 *     satu barang diam-diam.
 *
 *  2. `has_more` ditarik sampai habis dalam satu jalan, supaya kasir tidak
 *     berjualan dengan katalog setengah.
 */
export async function tarikMasterData(outletId: string): Promise<{ halaman: number }> {
  let halaman = 0

  for (;;) {
    const sejak = await ambilKursor()
    const mentah = await api.get<PerubahanTarik>('/sync/pull', {
      query: { since: sejak, outlet_id: outletId, limit: 500 },
    })
    // Dinormalkan SEKALI di sini: daftar yang null jadi array kosong, supaya
    // sisa fungsi ini tidak perlu memeriksa null di tiap baris.
    const p = {
      ...mentah,
      categories: mentah.categories ?? [],
      units: mentah.units ?? [],
      products: mentah.products ?? [],
      customers: mentah.customers ?? [],
      stocks: mentah.stocks ?? [],
      deleted: mentah.deleted ?? {},
    }
    halaman++

    await db.transaction(
      'rw',
      [db.produk, db.kategori, db.satuan, db.pelanggan, db.stok, db.meta],
      async () => {
        if (p.categories.length) {
          await db.kategori.bulkPut(
            p.categories.map<KategoriLokal>((c) => ({
              id: c.id,
              name: c.name,
              sort_order: c.sort_order,
              sync_version: c.sync_version,
            })),
          )
        }

        if (p.units.length) {
          await db.satuan.bulkPut(
            p.units.map<SatuanLokal>((u) => ({
              id: u.id,
              name: u.name,
              sync_version: u.sync_version,
            })),
          )
        }

        if (p.products.length) {
          await db.produk.bulkPut(
            p.products.map<ProdukLokal>((x) => ({ ...x, cari: teksCari(x) })),
          )
        }

        if (p.customers.length) {
          await db.pelanggan.bulkPut(
            p.customers.map<PelangganLokal>((c) => ({
              id: c.id,
              name: c.name,
              phone: c.phone,
              credit_limit: c.credit_limit,
              sync_version: c.sync_version,
            })),
          )
        }

        // Stok adalah SNAPSHOT per outlet, bukan delta — ditimpa apa adanya.
        if (p.stocks.length) {
          await db.stok.bulkPut(
            p.stocks.map<StokLokal>((s) => ({
              kunci: kunciStok(s.outlet_id, s.product_id, s.variant_id),
              outlet_id: s.outlet_id,
              product_id: s.product_id,
              variant_id: s.variant_id,
              qty: s.qty,
              reserved_qty: s.reserved_qty,
            })),
          )
        }

        await terapkanPenghapusan(p.deleted)

        // Lihat catatan (1) di atas: kursor disimpan dengan jeda aman.
        const aman = Math.max(0, p.cursor - p.safety_lag)
        await simpanKursor(aman)
      },
    )

    if (!p.has_more) break
    // Halaman penuh tapi kursor tidak maju berarti jeda aman menahan kita di
    // tempat; berhenti daripada menarik halaman yang sama selamanya.
    if (p.cursor <= sejak) break
  }

  return { halaman }
}

async function terapkanPenghapusan(deleted: Record<string, string[]>): Promise<void> {
  for (const [tabel, ids] of Object.entries(deleted)) {
    if (!ids.length) continue
    switch (tabel) {
      case 'products':
        await db.produk.bulkDelete(ids)
        // Saldo stok barang yang dihapus ikut dibuang supaya tidak jadi hantu.
        await db.stok.where('product_id').anyOf(ids).delete()
        break
      case 'categories':
        await db.kategori.bulkDelete(ids)
        break
      case 'units':
        await db.satuan.bulkDelete(ids)
        break
      case 'customers':
        await db.pelanggan.bulkDelete(ids)
        break
      default:
        // Tabel yang belum dipakai layar mana pun (varian, daftar harga)
        // sengaja diabaikan, bukan dianggap galat.
        break
    }
  }
}
