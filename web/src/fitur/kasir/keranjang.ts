/**
 * Keranjang belanja.
 *
 * Aturan yang mengikat berkas ini:
 *   - HARGA TIDAK DIKIRIM KLIEN. Server memakai harga dari master produk
 *     (§13.1 backend). Harga di sini hanya untuk ditampilkan.
 *   - Total yang dihitung di sini adalah PRATINJAU. Angka yang dicetak di
 *     struk selalu diambil dari balasan server.
 *   - Jumlah barang string desimal, tidak pernah Number.
 */
import { useCallback, useMemo, useState } from 'react'
import type { Produk } from '@/bersama/tipe/katalog'
import { bandingQty, kurangQty, qtyKosong, tambahQty } from '@/bersama/util/desimal'
import { pratinjauBaris } from '@/bersama/util/uang'

export interface BarisKeranjang {
  produk: Produk
  qty: string
  /** Diskon per baris dalam rupiah bulat. Butuh izin sale.discount. */
  diskon: number
  catatan?: string
}

export interface Keranjang {
  baris: BarisKeranjang[]
  /** Jumlah jenis barang (bukan total qty) — untuk lencana di tombol. */
  jumlahBaris: number
  /** Pratinjau total. Angka final tetap dari server. */
  pratinjauTotal: number
  tambah: (p: Produk, qty?: string) => void
  ubahQty: (produkId: string, qty: string) => void
  ubahDiskon: (produkId: string, diskon: number) => void
  hapus: (produkId: string) => void
  kosongkan: () => void
  qtyDari: (produkId: string) => string
}

export function useKeranjang(): Keranjang {
  const [baris, setBaris] = useState<BarisKeranjang[]>([])

  const tambah = useCallback((p: Produk, qty = '1') => {
    setBaris((lama) => {
      const ada = lama.find((b) => b.produk.id === p.id)
      if (!ada) return [...lama, { produk: p, qty, diskon: 0 }]
      return lama.map((b) =>
        b.produk.id === p.id ? { ...b, qty: tambahQty(b.qty, qty) } : b,
      )
    })
  }, [])

  const ubahQty = useCallback((produkId: string, qty: string) => {
    setBaris((lama) =>
      // Jumlah nol berarti barangnya dibatalkan — barisnya hilang, bukan
      // tertinggal sebagai "0 pcs" yang membingungkan saat membaca ulang.
      qtyKosong(qty)
        ? lama.filter((b) => b.produk.id !== produkId)
        : lama.map((b) => (b.produk.id === produkId ? { ...b, qty } : b)),
    )
  }, [])

  const ubahDiskon = useCallback((produkId: string, diskon: number) => {
    setBaris((lama) =>
      lama.map((b) => (b.produk.id === produkId ? { ...b, diskon } : b)),
    )
  }, [])

  const hapus = useCallback((produkId: string) => {
    setBaris((lama) => lama.filter((b) => b.produk.id !== produkId))
  }, [])

  const kosongkan = useCallback(() => setBaris([]), [])

  const qtyDari = useCallback(
    (produkId: string) => baris.find((b) => b.produk.id === produkId)?.qty ?? '0',
    [baris],
  )

  const pratinjauTotal = useMemo(
    () =>
      baris.reduce(
        (jml, b) => jml + pratinjauBaris(b.produk.sell_price, b.qty, b.diskon),
        0,
      ),
    [baris],
  )

  return {
    baris,
    jumlahBaris: baris.length,
    pratinjauTotal,
    tambah,
    ubahQty,
    ubahDiskon,
    hapus,
    kosongkan,
    qtyDari,
  }
}

/** Menyusun item checkout dari keranjang. Perhatikan: TANPA harga. */
export function itemUntukCheckout(baris: BarisKeranjang[]) {
  return baris.map((b) => ({
    product_id: b.produk.id,
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
