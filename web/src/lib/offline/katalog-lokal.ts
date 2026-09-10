import { useLiveQuery } from 'dexie-react-hooks'
import type { Produk } from '@/bersama/tipe/katalog'
import { db, type ProdukLokal } from './db'

/**
 * Katalog kasir dibaca dari Dexie, BUKAN dari server tiap ketikan.
 *
 * Dua alasan, dua-duanya dari dokumen:
 *   - kasir wajib tetap bisa menjual saat internet mati;
 *   - sasaran cari produk < 100 ms tidak mungkin dicapai lewat jaringan.
 *
 * Karena itu layar kasir tidak pernah "beralih mode" saat sinyal hilang —
 * sumber datanya memang selalu lokal, dan mesin sinkronisasi yang menyegarkan
 * di belakang.
 */

/** Produk lokal + nama satuan, dirangkai lokal karena pull tidak mengirimnya. */
export interface HasilKatalogLokal {
  produk: Produk[]
  petaStok: Map<string, string>
  /** Kategori yang benar-benar dipakai barang aktif. */
  kategori: { id: string; nama: string }[]
  /** true bila Dexie masih kosong — belum pernah menarik master data. */
  kosong: boolean
  memuat: boolean
}

export function useKatalogLokal(
  cari: string,
  outletId?: string,
  kategoriId?: string,
): HasilKatalogLokal {
  const hasil = useLiveQuery(async () => {
    const [semuaProduk, satuan, stok] = await Promise.all([
      db.produk.filter((p) => p.is_active).toArray(),
      db.satuan.toArray(),
      outletId ? db.stok.where('outlet_id').equals(outletId).toArray() : [],
    ])

    const namaSatuan = new Map(satuan.map((s) => [s.id, s.name]))
    const kunci = cari.trim().toLowerCase()

    let cocok = kunci ? semuaProduk.filter((p) => p.cari.includes(kunci)) : semuaProduk
    if (kategoriId) cocok = cocok.filter((p) => p.category_id === kategoriId)

    const produk = cocok
      .sort((a, b) => a.name.localeCompare(b.name, 'id'))
      .map((p) => keProduk(p, namaSatuan.get(p.unit_id)))

    const petaStok = new Map<string, string>()
    for (const s of stok) petaStok.set(s.product_id, s.qty)

    // Kategori yang benar-benar dipakai barang aktif — bukan seluruh katalog,
    // supaya kasir tidak melihat tab kategori yang isinya selalu kosong.
    const idTerpakai = new Set(semuaProduk.map((p) => p.category_id).filter(Boolean))
    const kategori = (await db.kategori.toArray())
      .filter((k) => idTerpakai.has(k.id))
      .sort((a, b) => a.sort_order - b.sort_order || a.name.localeCompare(b.name, 'id'))
      .map((k) => ({ id: k.id, nama: k.name }))

    return { produk, petaStok, kategori, kosong: semuaProduk.length === 0 }
  }, [cari, outletId, kategoriId])

  return {
    produk: hasil?.produk ?? [],
    petaStok: hasil?.petaStok ?? new Map(),
    kategori: hasil?.kategori ?? [],
    kosong: hasil?.kosong ?? false,
    memuat: hasil === undefined,
  }
}

/** Satu produk dari cache, untuk membangun struk offline. */
export async function produkLokal(id: string): Promise<Produk | undefined> {
  const p = await db.produk.get(id)
  if (!p) return undefined
  const s = await db.satuan.get(p.unit_id)
  return keProduk(p, s?.name)
}

/**
 * Menyesuaikan stok lokal setelah penjualan offline.
 *
 * Hasilnya PERKIRAAN, dan layar offline menyebutnya begitu: server tetap
 * pemegang kebenaran, dan stok minus tetap diterima lalu ditandai untuk
 * ditinjau (sesuai aturan backend). Yang penting kasir tidak melihat angka stok
 * yang jelas-jelas basi setelah ia sendiri baru saja menjualnya.
 */
export async function kurangiStokLokal(
  outletId: string,
  baris: { productId: string; qty: string }[],
): Promise<void> {
  await db.transaction('rw', db.stok, async () => {
    for (const b of baris) {
      const kunci = `${outletId}|${b.productId}|`
      const ada = await db.stok.get(kunci)
      if (!ada) continue
      const sisa = Number.parseFloat(ada.qty) - Number.parseFloat(b.qty)
      await db.stok.update(kunci, { qty: String(Number.isFinite(sisa) ? sisa : ada.qty) })
    }
  })
}

function keProduk(p: ProdukLokal, unitName?: string): Produk {
  return {
    id: p.id,
    name: p.name,
    category_id: p.category_id ?? undefined,
    unit_id: p.unit_id,
    unit_name: unitName,
    sku: p.sku ?? undefined,
    barcode: p.barcode ?? undefined,
    sell_price: p.sell_price,
    cost_price: p.cost_price,
    track_stock: p.track_stock,
    min_stock: p.min_stock,
    is_active: p.is_active,
    image_url: p.image_url,
    created_at: '',
    updated_at: '',
  }
}
