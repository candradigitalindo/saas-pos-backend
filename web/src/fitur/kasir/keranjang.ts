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
 *   - Diskon bisa nominal atau persen. Yang persen disimpan SEBAGAI persen dan
 *     nominalnya dihitung ulang setiap kali jumlah berubah — "10%" tetap 10%
 *     walau kasir menambah qty sesudahnya. Server hanya menerima nominal,
 *     jadi konversinya terjadi di itemUntukCheckout / diskonTransaksiNominal.
 *     Nominal selalu dijepit ke batas yang diterima server (diskon baris ≤
 *     nilai baris, diskon transaksi ≤ total baris) supaya pengurangan qty
 *     tidak membuat checkout ditolak.
 */
import { useCallback, useMemo, useState } from 'react'
import Decimal from 'decimal.js'
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
  /** Bila diisi (0–100), diskon baris = persen ini dari nilai baris; `diskon` diabaikan. */
  diskonPersen?: number
  /** Catatan untuk barang ini ("tanpa es"), ikut tercetak di struk. */
  catatan?: string
}

/** Diskon transaksi: nominal rupiah, atau persen dari belanja setelah diskon baris. */
export interface DiskonTransaksi {
  jenis: 'nominal' | 'persen'
  nilai: number
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
  /** Mengganti catatan & diskon satu baris sekaligus (dialog atur baris). */
  aturBaris: (kunci: string, isi: Pick<BarisKeranjang, 'catatan' | 'diskon' | 'diskonPersen'>) => void
  diskonTransaksi: DiskonTransaksi | null
  aturDiskonTransaksi: (d: DiskonTransaksi | null) => void
  /** Nominal diskon transaksi yang dikirim ke server (sudah dijepit). */
  diskonTransaksiNominal: number
  hapus: (kunci: string) => void
  kosongkan: () => void
  /** Total qty satu barang di keranjang, semua variannya dijumlah. */
  qtyDari: (produkId: string) => string
  /** Tagihan terbuka yang sedang dimuat — null untuk keranjang biasa. */
  tagihan: { id: string; version: number; label: string } | null
  /** Mengganti seluruh isi keranjang dengan isi sebuah tagihan. */
  muatTagihan: (
    tagihan: { id: string; version: number; label: string },
    baris: BarisKeranjang[],
    diskonTransaksi: DiskonTransaksi | null,
  ) => void
}

function bulat(d: Decimal): number {
  return d.toDecimalPlaces(0, Decimal.ROUND_HALF_UP).toNumber()
}

/** Nilai kotor baris = bulat(qty × harga), sama dengan server. */
export function kotorBaris(b: Pick<BarisKeranjang, 'produk' | 'varian' | 'qty'>): number {
  try {
    return bulat(new Decimal(b.qty || '0').mul(hargaBaris(b)))
  } catch {
    return 0
  }
}

/** Diskon baris yang berlaku (rupiah), sudah dijepit 0..nilai baris. */
export function diskonBaris(b: Pick<BarisKeranjang, 'produk' | 'varian' | 'qty' | 'diskon' | 'diskonPersen'>): number {
  const kotor = kotorBaris(b)
  const mentah =
    b.diskonPersen !== undefined ? bulat(new Decimal(kotor).mul(b.diskonPersen).div(100)) : b.diskon
  return Math.min(Math.max(0, mentah), kotor)
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
  const [diskonTransaksi, setDiskonTransaksi] = useState<DiskonTransaksi | null>(null)
  const [tagihan, setTagihan] = useState<Keranjang['tagihan']>(null)

  const muatTagihan = useCallback<Keranjang['muatTagihan']>((t, isi, diskon) => {
    setBaris(isi)
    setDiskonTransaksi(diskon)
    setTagihan(t)
  }, [])

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
      lama.map((b) => (b.kunci === kunci ? { ...b, diskon, diskonPersen: undefined } : b)),
    )
  }, [])

  const aturBaris = useCallback(
    (kunci: string, isi: Pick<BarisKeranjang, 'catatan' | 'diskon' | 'diskonPersen'>) => {
      setBaris((lama) =>
        lama.map((b) =>
          b.kunci === kunci
            ? { ...b, catatan: isi.catatan?.trim() || undefined, diskon: isi.diskon, diskonPersen: isi.diskonPersen }
            : b,
        ),
      )
    },
    [],
  )

  const hapus = useCallback((kunci: string) => {
    setBaris((lama) => lama.filter((b) => b.kunci !== kunci))
  }, [])

  const kosongkan = useCallback(() => {
    setBaris([])
    setDiskonTransaksi(null)
    setTagihan(null)
  }, [])

  const qtyDari = useCallback(
    (produkId: string) =>
      baris.filter((b) => b.produk.id === produkId).reduce((t, b) => tambahQty(t, b.qty), '0'),
    [baris],
  )

  const { rincian, diskonTransaksiNominal } = useMemo(() => {
    const hitung = barisHitung(baris)
    const nominal = nominalDiskonTransaksi(hitungTotal(hitung, aturan), diskonTransaksi)
    return { rincian: hitungTotal(hitung, aturan, nominal), diskonTransaksiNominal: nominal }
  }, [baris, aturan, diskonTransaksi])

  return {
    baris,
    jumlahBaris: baris.length,
    pratinjauTotal: rincian.total,
    rincian,
    tambah,
    ubahQty,
    ubahDiskon,
    aturBaris,
    diskonTransaksi,
    aturDiskonTransaksi: setDiskonTransaksi,
    diskonTransaksiNominal,
    hapus,
    kosongkan,
    qtyDari,
    tagihan,
    muatTagihan,
  }
}

/** Baris keranjang dalam bentuk yang dipakai hitungTotal. */
export function barisHitung(baris: BarisKeranjang[]) {
  return baris.map((b) => ({ harga: hargaBaris(b), qty: b.qty, diskon: diskonBaris(b) }))
}

/**
 * Nominal diskon transaksi dari rincian TANPA diskon transaksi. Persen dihitung
 * dari belanja setelah diskon baris (sebelum pajak & layanan); hasilnya dijepit
 * ke Σ total baris — batas yang ditegakkan server.
 */
export function nominalDiskonTransaksi(tanpaDiskon: RincianTotal, d: DiskonTransaksi | null): number {
  if (!d || d.nilai <= 0) return 0
  const dasar = tanpaDiskon.subtotal - tanpaDiskon.discount_amount
  const mentah = d.jenis === 'persen' ? bulat(new Decimal(dasar).mul(d.nilai).div(100)) : d.nilai
  const batas = tanpaDiskon.total - tanpaDiskon.service_amount
  return Math.min(Math.max(0, mentah), Math.max(0, batas))
}

/** Menyusun item checkout dari keranjang. Perhatikan: TANPA harga. */
export function itemUntukCheckout(baris: BarisKeranjang[]) {
  return baris.map((b) => ({
    product_id: b.produk.id,
    ...(b.varian ? { variant_id: b.varian.id } : {}),
    qty: b.qty,
    ...(diskonBaris(b) > 0 ? { discount_amount: diskonBaris(b) } : {}),
    ...(b.catatan?.trim() ? { note: b.catatan.trim() } : {}),
  }))
}

/** Apakah stok cukup untuk qty yang diminta. Dipakai menandai kartu "HABIS". */
export function stokCukup(stok: string | undefined, qty: string): boolean {
  if (stok === undefined) return true // barang tanpa lacak stok
  return bandingQty(stok, qty) >= 0
}

export { kurangQty }
