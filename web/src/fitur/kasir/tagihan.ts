/**
 * Tagihan terbuka (open bill / tahan transaksi) di sisi kasir.
 *
 * Sumber kebenarannya SERVER (dibagi semua perangkat toko). Dexie menyimpan
 * salinannya supaya daftar tetap tampil dan bisa diubah saat offline:
 *
 *   - Online: simpan/batal langsung ke server, salinan lokal diperbarui dari
 *     balasannya. Versi yang basi ditolak server (409) — pesannya diteruskan
 *     apa adanya ke kasir, bukan diam-diam menimpa pesanan perangkat lain.
 *   - Offline: perubahan diantre (op open_bill.upsert / open_bill.cancel) dan
 *     salinan lokal ditandai `tertunda`. Setiap operasi punya id sendiri, jadi
 *     satu tagihan boleh diubah berkali-kali selama offline.
 *   - Menyegarkan daftar dari server TIDAK menimpa tagihan yang perubahannya
 *     masih di antrean, dan menyembunyikan tagihan yang sudah dibayar di
 *     perangkat ini tapi penjualannya belum terkirim.
 */
import { useQuery } from '@tanstack/react-query'
import { useLiveQuery } from 'dexie-react-hooks'
import { ulid } from 'ulid'
import { GalatAPI } from '@/lib/api-client'
import { antrekan } from '@/lib/offline/antrean'
import { db, type TagihanLokal } from '@/lib/offline/db'
import { produkLokal, varianLokal } from '@/lib/offline/katalog-lokal'
import { useOnline } from '@/bersama/hooks/use-online'
import type { DiskonTagihan, ItemTagihan } from '@/bersama/tipe/pos'
import { tambahQty } from '@/bersama/util/desimal'
import { kasirApi, type InputTagihan } from './api'
import { kunciBaris, type BarisKeranjang, type DiskonTransaksi, type PelangganKeranjang } from './keranjang'

/** Tagihan yang sedang dimuat di keranjang. */
export interface TagihanAktif {
  id: string
  version: number
  label: string
}

const SELANG_SEGAR_MS = 20_000

/**
 * Daftar tagihan terbuka satu cabang, dibaca dari Dexie (jadi tetap ada saat
 * offline) dan disegarkan dari server tiap 20 detik selama online — pelayan
 * di HP lain bisa menambah tagihan kapan saja.
 */
export function useTagihanTerbuka(outletId?: string) {
  const online = useOnline()
  const segar = useQuery({
    queryKey: ['tagihan-terbuka', outletId],
    queryFn: () => segarkanTagihan(outletId!),
    enabled: !!outletId && online,
    refetchInterval: SELANG_SEGAR_MS,
  })
  const daftar = useLiveQuery(
    async () =>
      outletId
        ? (await db.tagihan.where('outlet_id').equals(outletId).toArray()).sort((a, b) =>
            b.updated_at.localeCompare(a.updated_at),
          )
        : [],
    [outletId],
  )
  return { daftar: daftar ?? [], memuat: daftar === undefined, segarkan: () => segar.refetch() }
}

/** Menarik daftar dari server lalu menyelaraskan salinan lokal cabang itu. */
export async function segarkanTagihan(outletId: string): Promise<number> {
  const server = await kasirApi.daftarTagihan(outletId)
  const { diubahLokal, dibayarLokal } = await tagihanDiAntrean()
  await db.transaction('rw', db.tagihan, async () => {
    const lokal = await db.tagihan.where('outlet_id').equals(outletId).toArray()
    const hapus = lokal.filter((l) => !diubahLokal.has(l.id) || dibayarLokal.has(l.id)).map((l) => l.id)
    await db.tagihan.bulkDelete(hapus)
    await db.tagihan.bulkPut(server.filter((s) => !diubahLokal.has(s.id) && !dibayarLokal.has(s.id)))
  })
  return server.length
}

/** Tagihan yang masih punya perubahan / pembayaran di antrean offline. */
async function tagihanDiAntrean(): Promise<{ diubahLokal: Set<string>; dibayarLokal: Set<string> }> {
  const ops = await db.antrean.where('status').anyOf('menunggu', 'mengirim').toArray()
  const diubahLokal = new Set<string>()
  const dibayarLokal = new Set<string>()
  for (const o of ops) {
    const p = o.payload as { id?: string; open_bill_id?: string }
    if ((o.op === 'open_bill.upsert' || o.op === 'open_bill.cancel') && p.id) diubahLokal.add(p.id)
    if (o.op === 'sale.create' && p.open_bill_id) dibayarLokal.add(p.open_bill_id)
  }
  return { diubahLokal, dibayarLokal }
}

/**
 * Menyimpan keranjang sebagai tagihan (baru bila `aktif` kosong).
 * @returns tagihan tersimpan + apakah ia baru diantre (offline).
 */
export async function simpanTagihan(a: {
  outletId: string
  aktif: TagihanAktif | null
  label: string
  baris: BarisKeranjang[]
  diskonTransaksi: DiskonTransaksi | null
  /** Pratinjau total, untuk ringkasan antrean. */
  nominal: number
  namaPengguna?: string
  /** Pelanggan keranjang — ikut disimpan supaya harga khususnya terbawa. */
  pelangganId?: string
}): Promise<{ tagihan: TagihanLokal; diantre: boolean }> {
  const id = a.aktif?.id ?? ulid()
  const input: InputTagihan = {
    outlet_id: a.outletId,
    label: a.label.trim(),
    ...(a.pelangganId ? { customer_id: a.pelangganId } : {}),
    items: barisKeItemTagihan(a.baris),
    order_discount: diskonKeTagihan(a.diskonTransaksi),
    base_version: a.aktif?.version ?? 0,
  }
  try {
    const t = await kasirApi.simpanTagihan(id, input)
    await db.tagihan.put(t)
    return { tagihan: t, diantre: false }
  } catch (e) {
    if (!(e instanceof GalatAPI) || !e.bisaDiantre) throw e
    await antrekan({
      id: ulid(),
      op: 'open_bill.upsert',
      payload: { id, ...input },
      ringkasan: `Tagihan ${input.label} (${a.baris.length} barang)`,
      nominal: a.nominal,
    })
    const lama = await db.tagihan.get(id)
    const sekarang = new Date().toISOString()
    const lokal: TagihanLokal = {
      id,
      outlet_id: a.outletId,
      label: input.label,
      customer_id: a.pelangganId,
      order_type: lama?.order_type ?? 'dine_in',
      items: input.items,
      order_discount: input.order_discount,
      status: 'open',
      version: input.base_version + 1,
      created_by: lama?.created_by ?? '',
      created_by_name: lama?.created_by_name ?? a.namaPengguna,
      updated_by_name: a.namaPengguna,
      created_at: lama?.created_at ?? sekarang,
      updated_at: sekarang,
      tertunda: true,
    }
    await db.tagihan.put(lokal)
    return { tagihan: lokal, diantre: true }
  }
}

/** Membatalkan tagihan; offline → diantre. Salinan lokal langsung dibuang. */
export async function batalkanTagihan(t: TagihanLokal): Promise<{ diantre: boolean }> {
  try {
    await kasirApi.batalkanTagihan(t.id, t.version)
    await db.tagihan.delete(t.id)
    return { diantre: false }
  } catch (e) {
    if (!(e instanceof GalatAPI) || !e.bisaDiantre) throw e
    await antrekan({
      id: ulid(),
      op: 'open_bill.cancel',
      payload: { id: t.id, base_version: t.version },
      ringkasan: `Batalkan tagihan ${t.label}`,
      nominal: 0,
    })
    await db.tagihan.delete(t.id)
    return { diantre: true }
  }
}

/**
 * Pelanggan (dari data lokal) dalam bentuk keranjang, lengkap dengan nama
 * daftar harga khususnya. Pelanggan yang tidak ada lagi → null (umum).
 */
export async function pelangganKeranjang(id?: string): Promise<PelangganKeranjang | null> {
  if (!id) return null
  const p = await db.pelanggan.get(id)
  if (!p) return null
  const daftar = p.price_list_id ? await db.daftarHarga.get(p.price_list_id) : undefined
  return {
    id: p.id,
    name: p.name,
    phone: p.phone,
    price_list_id: daftar && !daftar.is_default ? daftar.id : null,
    nama_daftar: daftar && !daftar.is_default ? daftar.name : undefined,
  }
}

/** Keranjang → baris tagihan (tanpa harga; persen tetap persen). */
export function barisKeItemTagihan(baris: BarisKeranjang[]): ItemTagihan[] {
  return baris.map((b) => ({
    product_id: b.produk.id,
    ...(b.varian ? { variant_id: b.varian.id } : {}),
    qty: b.qty,
    ...(b.diskonPersen !== undefined && b.diskonPersen > 0
      ? { discount_percent: b.diskonPersen }
      : b.diskon > 0
        ? { discount_amount: b.diskon }
        : {}),
    ...(b.catatan?.trim() ? { note: b.catatan.trim() } : {}),
  }))
}

export function diskonKeTagihan(d: DiskonTransaksi | null): DiskonTagihan | undefined {
  if (!d || d.nilai <= 0) return undefined
  return { kind: d.jenis === 'persen' ? 'percent' : 'nominal', value: d.nilai }
}

export function tagihanKeDiskon(d?: DiskonTagihan): DiskonTransaksi | null {
  if (!d || d.value <= 0) return null
  return { jenis: d.kind === 'percent' ? 'persen' : 'nominal', nilai: d.value }
}

/**
 * Baris tagihan → baris keranjang, memakai katalog lokal. Barang yang sudah
 * tidak ada di katalog dilewati dan dihitung, supaya kasir diberi tahu —
 * bukan tagihannya gagal dibuka seluruhnya.
 */
export async function itemTagihanKeBaris(items: ItemTagihan[]): Promise<{ baris: BarisKeranjang[]; hilang: number }> {
  const baris: BarisKeranjang[] = []
  let hilang = 0
  for (const it of items) {
    const produk = await produkLokal(it.product_id)
    const varian = it.variant_id ? await varianLokal(it.variant_id) : undefined
    if (!produk || (it.variant_id && !varian)) {
      hilang++
      continue
    }
    const kunci = kunciBaris(produk.id, varian?.id)
    const ada = baris.find((b) => b.kunci === kunci)
    if (ada) {
      // Baris ganda (seharusnya tidak terjadi dari keranjang) digabung.
      ada.qty = tambahQty(ada.qty, it.qty)
      continue
    }
    baris.push({
      kunci,
      produk,
      varian,
      qty: it.qty,
      diskon: it.discount_amount ?? 0,
      diskonPersen: it.discount_percent,
      catatan: it.note,
    })
  }
  return { baris, hilang }
}
