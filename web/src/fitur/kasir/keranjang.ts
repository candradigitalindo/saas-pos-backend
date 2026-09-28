/**
 * Keranjang belanja.
 *
 * Aturan yang mengikat berkas ini:
 *   - HARGA TIDAK DIKIRIM KLIEN. Server memakai harga dari master produk
 *     (§13.1 backend). Harga di sini hanya untuk ditampilkan.
 *   - Total yang dihitung di sini adalah PRATINJAU. Angka yang dicetak di
 *     struk selalu diambil dari balasan server.
 *   - Jumlah barang string desimal, tidak pernah Number.
 *   - Satu baris = satu barang + satu varian. "Kopi (Besar)" dan "Kopi
 *     (Kecil)" dua baris; qtyDari menjumlah semuanya per barang karena stok
 *     dihitung di tingkat barang.
 */
import { useCallback, useMemo, useState } from 'react'
import type { Produk, VarianProduk } from '@/bersama/tipe/katalog'
import { bandingQty, kurangQty, qtyKosong, tambahQty } from '@/bersama/util/desimal'
import { hitungTotal, type AturanHarga, type RincianTotal } from '@/bersama/util/total'

export interface BarisKeranjang {
  /** id barang, atau "id barang|id varian" — dipakai ubahQty/hapus. */
  kunci: string
  produk: Produk
  varian?: VarianProduk
  qty: string
  /** Diskon per baris dalam rupiah bulat. Butuh izin sale.discount. */
  diskon: number
  catatan?: string
}

export interface Keranjang {
  baris: BarisKeranjang[]
  /** Jumlah jenis barang (bukan total qty) — untuk lencana di tombol. */
  jumlahBaris: number
  /**
   * Total yang harus dibayar — dihitung PERSIS seperti server, termasuk pajak
   * eksklusif dan biaya layanan cabang (lihat total.ts). Struk tetap memakai
   * angka balasan server.
   */
  pratinjauTotal: number
  /** Rincian di balik total: subtotal, diskon, pajak, biaya layanan. */
  rincian: RincianTotal
  tambah: (p: Produk, qty?: string, varian?: VarianProduk) => void
  ubahQty: (kunci: string, qty: string) => void
  ubahDiskon: (kunci: string, diskon: number) => void
  hapus: (kunci: string) => void
  kosongkan: () => void
  /** Total qty satu barang di keranjang, semua variannya dijumlah. */
  qtyDari: (produkId: string) => string
}

export function kunciBaris(produkId: string, varianId?: string): string {
  return varianId ? `${produkId}|${varianId}` : produkId
}

/** Harga satuan pratinjau: harga jual + selisih varian. */
export function hargaBaris(b: Pick<BarisKeranjang, 'produk' | 'varian'>): number {
  return b.produk.sell_price + (b.varian?.price_delta ?? 0)
}

/** "Kopi (Besar)" — sama dengan nama yang dicetak server di struk. */
export function namaBaris(b: Pick<BarisKeranjang, 'produk' | 'varian'>): string {
  return b.varian ? `${b.produk.name} (${b.varian.name})` : b.produk.name
}

/**
 * @param aturan pengaturan harga cabang aktif (pajak, biaya layanan). Tanpa
 *   aturan (profil belum termuat) total dihitung tanpa pajak dan layanan.
 */
export function useKeranjang(aturan?: AturanHarga): Keranjang {
  const [baris, setBaris] = useState<BarisKeranjang[]>([])

  const tambah = useCallback((p: Produk, qty = '1', varian?: VarianProduk) => {
    const kunci = kunciBaris(p.id, varian?.id)
    setBaris((lama) => {
      const ada = lama.find((b) => b.kunci === kunci)
      if (!ada) return [...lama, { kunci, produk: p, varian, qty, diskon: 0 }]
      return lama.map((b) =>
        b.kunci === kunci ? { ...b, qty: tambahQty(b.qty, qty) } : b,
      )
    })
  }, [])

  const ubahQty = useCallback((kunci: string, qty: string) => {
    setBaris((lama) =>
      // Jumlah nol berarti barangnya dibatalkan — barisnya hilang, bukan
      // tertinggal sebagai "0 pcs" yang membingungkan saat membaca ulang.
      qtyKosong(qty)
        ? lama.filter((b) => b.kunci !== kunci)
        : lama.map((b) => (b.kunci === kunci ? { ...b, qty } : b)),
    )
  }, [])

  const ubahDiskon = useCallback((kunci: string, diskon: number) => {
    setBaris((lama) =>
      lama.map((b) => (b.kunci === kunci ? { ...b, diskon } : b)),
    )
  }, [])

  const hapus = useCallback((kunci: string) => {
    setBaris((lama) => lama.filter((b) => b.kunci !== kunci))
  }, [])

  const kosongkan = useCallback(() => setBaris([]), [])

  const qtyDari = useCallback(
    (produkId: string) =>
      baris.filter((b) => b.produk.id === produkId).reduce((t, b) => tambahQty(t, b.qty), '0'),
    [baris],
  )

  const rincian = useMemo(() => hitungTotal(barisHitung(baris), aturan), [baris, aturan])

  return {
    baris,
    jumlahBaris: baris.length,
    pratinjauTotal: rincian.total,
    rincian,
    tambah,
    ubahQty,
    ubahDiskon,
    hapus,
    kosongkan,
    qtyDari,
  }
}

/** Baris keranjang dalam bentuk yang dipakai hitungTotal. */
export function barisHitung(baris: BarisKeranjang[]) {
  return baris.map((b) => ({ harga: hargaBaris(b), qty: b.qty, diskon: b.diskon }))
}

/** Menyusun item checkout dari keranjang. Perhatikan: TANPA harga. */
export function itemUntukCheckout(baris: BarisKeranjang[]) {
  return baris.map((b) => ({
    product_id: b.produk.id,
    ...(b.varian ? { variant_id: b.varian.id } : {}),
    qty: b.qty,
    ...(b.diskon > 0 ? { discount_amount: b.diskon } : {}),
    ...(b.catatan ? { note: b.catatan } : {}),
  }))
}

/** Apakah stok cukup untuk qty yang diminta. Dipakai menandai kartu "HABIS". */
export function stokCukup(stok: string | undefined, qty: string): boolean {
  if (stok === undefined) return true // barang tanpa lacak stok
  return bandingQty(stok, qty) >= 0
}

export { kurangQty }
