import { useLiveQuery } from 'dexie-react-hooks'
import type { HargaKhusus, Kemasan, Produk, TingkatGrosir, VarianProduk } from '@/bersama/tipe/katalog'
import { db, type HargaProdukLokal, type KemasanLokal, type ProdukLokal, type VarianLokal } from './db'
import Decimal from 'decimal.js'
import { bandingQty } from '@/bersama/util/desimal'
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
    const [semuaProduk, satuan, stok, semuaVarian] = await Promise.all([
      db.produk.filter((p) => p.is_active).toArray(),
      db.satuan.toArray(),
      outletId ? db.stok.where('outlet_id').equals(outletId).toArray() : [],
      db.varian.filter((v) => v.is_active).toArray(),
    ])
    const varianPer = kelompokkanVarian(semuaVarian)
    const { grosir: grosirPer, khusus: khususPer } = await hargaPerProduk()
    const kemasanPer = kelompokkanKemasan(await db.kemasan.toArray())

    const namaSatuan = new Map(satuan.map((s) => [s.id, s.name]))
    const kunci = cari.trim().toLowerCase()

    let cocok = kunci ? semuaProduk.filter((p) => p.cari.includes(kunci)) : semuaProduk
    if (kategoriId) cocok = cocok.filter((p) => p.category_id === kategoriId)

    const produk = cocok
      .sort((a, b) => a.name.localeCompare(b.name, 'id'))
      .map((p) =>
        keProduk(p, namaSatuan.get(p.unit_id), varianPer.get(p.id), grosirPer.get(p.id), khususPer.get(p.id), {
          baris: kemasanPer.get(p.id),
          namaSatuan,
        }),
      )

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

/**
 * Cari varian dari barcode/SKU-nya — satu pindaian langsung masuk sebagai
 * "Kopi (Besar)" tanpa dialog pilih varian. Varian nonaktif atau barang
 * induk yang nonaktif dianggap tidak ketemu.
 */
export async function varianDariBarcode(
  kode: string,
): Promise<{ produk: Produk; varian: VarianProduk } | undefined> {
  const bersih = kode.trim()
  if (!bersih) return undefined
  const v =
    (await db.varian.where('barcode').equals(bersih).first()) ?? (await db.varian.where('sku').equals(bersih).first())
  if (!v || !v.is_active) return undefined
  const produk = await produkLokal(v.product_id)
  if (!produk || !produk.is_active) return undefined
  return { produk, varian: keVarian(v) }
}

/** Satu produk (beserta varian aktifnya) dari cache, untuk struk offline. */
export async function produkLokal(id: string): Promise<Produk | undefined> {
  const p = await db.produk.get(id)
  if (!p) return undefined
  const [s, varian, harga, kemasan, satuan] = await Promise.all([
    db.satuan.get(p.unit_id),
    db.varian.where('product_id').equals(id).toArray(),
    hargaPerProduk(id),
    db.kemasan.where('product_id').equals(id).toArray(),
    db.satuan.toArray(),
  ])
  return keProduk(
    p,
    s?.name,
    kelompokkanVarian(varian.filter((v) => v.is_active)).get(id),
    harga.grosir.get(id),
    harga.khusus.get(id),
    { baris: kemasan, namaSatuan: new Map(satuan.map((x) => [x.id, x.name])) },
  )
}

/**
 * Cari kemasan dari barcodenya (barcode dus) — satu pindaian langsung masuk
 * sebagai "1 dus". Barang induk yang nonaktif dianggap tidak ketemu.
 */
export async function kemasanDariBarcode(kode: string): Promise<{ produk: Produk; kemasan: Kemasan } | undefined> {
  const bersih = kode.trim()
  if (!bersih) return undefined
  const k = await db.kemasan.where('barcode').equals(bersih).first()
  if (!k) return undefined
  const produk = await produkLokal(k.product_id)
  const kemasan = produk?.packagings?.find((x) => x.id === k.id)
  if (!produk || !produk.is_active || !kemasan) return undefined
  return { produk, kemasan }
}

function kelompokkanKemasan(daftar: KemasanLokal[]): Map<string, KemasanLokal[]> {
  const peta = new Map<string, KemasanLokal[]>()
  for (const k of daftar) peta.set(k.product_id, [...(peta.get(k.product_id) ?? []), k])
  return peta
}

/** Kemasan lokal → bentuk katalog: nama satuan + harga berlaku, isi terkecil dulu. */
function keKemasan(baris: KemasanLokal[] | undefined, hargaJual: number, namaSatuan: Map<string, string>): Kemasan[] {
  return (baris ?? [])
    .map((k) => ({
      id: k.id,
      unit_id: k.unit_id,
      unit_name: namaSatuan.get(k.unit_id) ?? 'kemasan',
      conversion: k.conversion,
      sell_price: k.sell_price,
      // Sama dengan server (ProductUnit.HargaKemasan): isi × harga jual, dibulatkan.
      price: k.sell_price ?? new Decimal(k.conversion).mul(hargaJual).toDecimalPlaces(0, Decimal.ROUND_HALF_UP).toNumber(),
      barcode: k.barcode ?? undefined,
    }))
    .sort((a, b) => new Decimal(a.conversion).cmp(b.conversion))
}

/**
 * Harga dari daftar harga, per barang (baris tanpa varian) — sama dengan yang
 * dipakai server saat checkout:
 *   - grosir: tingkat per jumlah di daftar DEFAULT, jumlah terkecil dulu;
 *   - khusus: harga di daftar khusus (member/reseller) yang masih ada.
 */
async function hargaPerProduk(
  produkId?: string,
): Promise<{ grosir: Map<string, TingkatGrosir[]>; khusus: Map<string, HargaKhusus[]> }> {
  const grosir = new Map<string, TingkatGrosir[]>()
  const khusus = new Map<string, HargaKhusus[]>()
  const daftar = new Map((await db.daftarHarga.toArray()).map((l) => [l.id, l]))
  if (daftar.size === 0) return { grosir, khusus }
  const baris: HargaProdukLokal[] = produkId
    ? await db.hargaProduk.where('product_id').equals(produkId).toArray()
    : await db.hargaProduk.toArray()
  for (const h of baris) {
    const l = daftar.get(h.price_list_id)
    if (!l || h.variant_id) continue
    if (l.is_default) {
      grosir.set(h.product_id, [...(grosir.get(h.product_id) ?? []), { min_qty: h.min_qty, price: h.price }])
    } else {
      khusus.set(h.product_id, [
        ...(khusus.get(h.product_id) ?? []),
        { price_list_id: h.price_list_id, price: h.price, min_qty: h.min_qty },
      ])
    }
  }
  for (const t of grosir.values()) t.sort((a, b) => bandingQty(a.min_qty, b.min_qty))
  return { grosir, khusus }
}

/**
 * Satu varian dari cache, APA PUN statusnya — untuk struk offline, supaya
 * varian yang baru dinonaktifkan di tengah transaksi tetap tercetak benar.
 */
export async function varianLokal(id: string): Promise<VarianProduk | undefined> {
  const v = await db.varian.get(id)
  return v ? keVarian(v) : undefined
}

/** Varian aktif per barang, termurah dulu (urutan yang sama dengan menu antar). */
function kelompokkanVarian(daftar: VarianLokal[]): Map<string, VarianProduk[]> {
  const peta = new Map<string, VarianProduk[]>()
  for (const v of daftar) peta.set(v.product_id, [...(peta.get(v.product_id) ?? []), keVarian(v)])
  for (const vs of peta.values()) vs.sort((a, b) => a.price_delta - b.price_delta || a.name.localeCompare(b.name, 'id'))
  return peta
}

function keVarian(v: VarianLokal): VarianProduk {
  return {
    id: v.id,
    product_id: v.product_id,
    name: v.name,
    price_delta: v.price_delta,
    sku: v.sku ?? undefined,
    barcode: v.barcode ?? undefined,
    is_active: v.is_active,
  }
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

function keProduk(
  p: ProdukLokal,
  unitName?: string,
  varian?: VarianProduk[],
  grosir?: TingkatGrosir[],
  khusus?: HargaKhusus[],
  kemasan?: { baris?: KemasanLokal[]; namaSatuan: Map<string, string> },
): Produk {
  const daftarKemasan = kemasan ? keKemasan(kemasan.baris, p.sell_price, kemasan.namaSatuan) : []
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
    varian: varian?.length ? varian : undefined,
    wholesale_prices: grosir?.length ? grosir : undefined,
    special_prices: khusus?.length ? khusus : undefined,
    packagings: daftarKemasan.length ? daftarKemasan : undefined,
    created_at: '',
    updated_at: '',
  }
}
