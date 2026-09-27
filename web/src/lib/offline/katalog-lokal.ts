import { useLiveQuery } from 'dexie-react-hooks'
import type { Produk } from '@/bersama/tipe/katalog'
import { db, type ProdukLokal } from './db'
import { petaWarnaKategori } from '@/bersama/util/warna-kategori'

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
  /** id kategori → kelas warna petak, dihitung dari SEMUA kategori. */
  warnaKategori: Map<string, string>
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
    const semuaKategori = await db.kategori.toArray()
    const kategori = semuaKategori
      .filter((k) => idTerpakai.has(k.id))
      .sort((a, b) => a.sort_order - b.sort_order || a.name.localeCompare(b.name, 'id'))
      .map((k) => ({ id: k.id, nama: k.name }))
    // Warna petak dihitung dari SEMUA kategori, bukan hanya yang terpakai —
    // daftar Barang juga menghitungnya dari semua, jadi satu kategori
    // berwarna sama di kedua layar (lihat warna-kategori.ts).
    const warnaKategori = petaWarnaKategori(semuaKategori)

    return { produk, petaStok, kategori, warnaKategori, kosong: semuaProduk.length === 0 }
  }, [cari, outletId, kategoriId])

  return {
    produk: hasil?.produk ?? [],
    petaStok: hasil?.petaStok ?? new Map(),
    kategori: hasil?.kategori ?? [],
    warnaKategori: hasil?.warnaKategori ?? new Map<string, string>(),
    kosong: hasil?.kosong ?? false,
    memuat: hasil === undefined,
  }
}

/**
 * Cari satu barang dari barcode-nya.
 *
 * Dibaca dari Dexie, bukan dari server: memindai barang di kasir harus tetap
 * jalan saat sinyal mati, dan perjalanan ke server per pindaian terlalu lambat
 * untuk antrean.
 */
export async function produkDariBarcode(kode: string): Promise<Produk | undefined> {
  const bersih = kode.trim()
  if (!bersih) return undefined

  const cocok =
    (await db.produk.where('barcode').equals(bersih).first()) ??
    // Sebagian barcode ritel dicetak dengan nol di depan yang tidak ikut
    // terbaca pemindai (EAN-13 vs UPC-A 12 digit).
    (await db.produk.filter((p) => !!p.barcode && p.barcode.replace(/^0+/, '') === bersih.replace(/^0+/, '')).first()) ??
    // Terakhir: SKU, karena banyak warung menempel kode sendiri.
    (await db.produk.where('sku').equals(bersih).first())

  if (!cocok || !cocok.is_active) return undefined
  const satuan = await db.satuan.get(cocok.unit_id)
  return keProduk(cocok, satuan?.name)
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
