/**
 * Jumlah barang datang dari server sebagai string desimal ("1.5") supaya 0,5 kg
 * tetap presisi. Diolah dengan decimal.js, tidak pernah dengan Number, lalu
 * dikirim balik sebagai string.
 */
import Decimal from 'decimal.js'

export function tambahQty(qty: string, delta: number | string): string {
  return new Decimal(qty || '0').plus(delta).toString()
}

export function kurangQty(qty: string, delta: number | string, minimal = '0'): string {
  const hasil = new Decimal(qty || '0').minus(delta)
  const batas = new Decimal(minimal)
  return (hasil.lessThan(batas) ? batas : hasil).toString()
}

export function bandingQty(a: string, b: string): number {
  return new Decimal(a || '0').comparedTo(new Decimal(b || '0'))
}

export function qtyKosong(qty: string): boolean {
  return new Decimal(qty || '0').lessThanOrEqualTo(0)
}

/** "44" bukan "44.00"; "1.5" tetap "1,5" untuk mata Indonesia. */
export function formatQty(qty: string): string {
  const d = new Decimal(qty || '0')
  const teks = d.toDecimalPlaces(3).toString()
  return teks.replace('.', ',')
}

/** Teks jumlah + satuan, siap tampil: "44 pcs". */
export function formatQtySatuan(qty: string, satuan?: string): string {
  return satuan ? `${formatQty(qty)} ${satuan}` : formatQty(qty)
}

/**
 * Sisa stok dalam bahasa orang, cukup pendek untuk satu baris daftar.
 *
 * Stok minus itu SAH di backend — penjualan tidak pernah diblokir, dan
 * selisihnya ditandai untuk ditinjau. Tapi "sisa −2 pcs" tidak berarti apa-apa
 * bagi pemilik warung: yang ia butuh tahu adalah barangnya HABIS.
 *
 * Keterangan "catatan perlu dicocokkan" sengaja TIDAK disatukan di sini —
 * kalimatnya panjang dan di layar HP ia menggencet nama barang sampai tinggal
 * "Air …". Pakai `stokPerluDicocokkan()` di layar yang memang punya ruang.
 */
export function formatSisaStok(qty: string, satuan?: string): string {
  const n = new Decimal(qty || '0')
  if (n.greaterThan(0)) return `sisa ${formatQtySatuan(qty, satuan)}`
  return 'habis'
}

/**
 * true bila catatan stok minus — artinya barang sempat terjual melebihi
 * catatan, dan hitungan fisik perlu dijalankan.
 */
export function stokPerluDicocokkan(qty: string): boolean {
  return new Decimal(qty || '0').lessThan(0)
}
