import type { SaldoStok } from '@/bersama/tipe/katalog'

/**
 * Tafsiran satu baris saldo stok — dipisah dari halaman supaya bisa diuji
 * tanpa merender apa pun.
 *
 * Batas keadaannya SAMA dengan server (repositories.stockStatusCond &
 * StockSummary): minus → habis → hampir habis (0 < sisa ≤ batas) → aman.
 */
export type Keadaan = 'negative' | 'out' | 'low' | 'safe'

export function keadaanSaldo(s: Pick<SaldoStok, 'qty' | 'min_stock'>): Keadaan {
  const qty = Number.parseFloat(s.qty) || 0
  if (qty < 0) return 'negative'
  if (qty === 0) return 'out'
  if (qty <= (Number.parseFloat(s.min_stock) || 0)) return 'low'
  return 'safe'
}

/** Jendela hitungan laku di server (stockWindow). */
export const HARI_JENDELA = 30

/** Rata-rata keluar per hari selama 30 hari terakhir; 0 bila tak ada data. */
export function lakuPerHari(s: Pick<SaldoStok, 'sold_30d'>): number {
  const n = Number.parseFloat(s.sold_30d ?? '0') || 0
  return n > 0 ? n / HARI_JENDELA : 0
}

/**
 * Perkiraan sisa stok cukup untuk berapa hari lagi, menurut laju 30 hari
 * terakhir. null bila tidak bisa diperkirakan (barang habis/minus, atau tidak
 * laku sama sekali — "cukup selamanya" bukan informasi).
 */
export function cukupHari(s: Pick<SaldoStok, 'qty' | 'sold_30d'>): number | null {
  const qty = Number.parseFloat(s.qty) || 0
  const laju = lakuPerHari(s)
  if (qty <= 0 || laju <= 0) return null
  return qty / laju
}

/**
 * "cukup ±2 hari" / "cukup ±3 minggu" / "cukup > 3 bulan". Pembulatannya
 * sengaja kasar: angka ini perkiraan dari rata-rata, dan "±17 hari" memberi
 * kesan ketelitian yang tidak dimilikinya.
 */
export function teksCukup(hari: number): string {
  if (hari < 1) return 'habis hari ini'
  if (hari < 14) return `cukup ±${Math.round(hari)} hari`
  if (hari < 60) return `cukup ±${Math.round(hari / 7)} minggu`
  if (hari <= 90) return `cukup ±${Math.round(hari / 30)} bulan`
  return 'cukup > 3 bulan'
}

/** Sisa ≤ 3 hari menurut laju jual: perlu belanja walau belum di bawah batas. */
export const HARI_MENDESAK = 3

/**
 * Isi meteran sisa-terhadap-batas, 0…1. Meterannya berskala DUA KALI batas
 * minimum, jadi garis batas selalu tepat di tengah: separuh terisi = pas di
 * batas. null bila batas belum diatur — tanpa batas, meteran tidak punya arti.
 */
export function isiMeter(s: Pick<SaldoStok, 'qty' | 'min_stock'>): number | null {
  const min = Number.parseFloat(s.min_stock) || 0
  if (min <= 0) return null
  const qty = Number.parseFloat(s.qty) || 0
  return Math.min(1, Math.max(0, qty / (min * 2)))
}

/** Belanja diperkirakan cukup untuk dua minggu penjualan. */
export const HARI_TARGET_BELANJA = 14

/**
 * Saran jumlah beli (satuan dasar) untuk barang di saran belanja: cukup untuk
 * dua minggu menurut laju jual, dan paling sedikit sampai dua kali batas
 * minimum. Saldo minus dianggap nol — catatannya memang belum cocok, dan rak
 * yang "minus" di sistem biasanya kosong di dunia nyata. Selalu ≥ 1 dan bulat:
 * orang membeli barang utuh.
 */
export function saranBeli(s: Pick<SaldoStok, 'qty' | 'min_stock' | 'sold_30d'>): number {
  const qty = Math.max(Number.parseFloat(s.qty) || 0, 0)
  const min = Number.parseFloat(s.min_stock) || 0
  const target = Math.max(min * 2, lakuPerHari(s) * HARI_TARGET_BELANJA)
  return Math.max(1, Math.ceil(target - qty))
}
