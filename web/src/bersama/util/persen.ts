/**
 * Tarif di server disimpan sebagai PECAHAN ("0.11" = 11%), tapi pemilik warung
 * berpikir dalam PERSEN dan mengetik dengan koma ("7,5"). Dua fungsi ini
 * menerjemahkan bolak-balik lewat decimal.js — tanpa float, jadi 7,5% tidak
 * pernah berubah jadi 0.07500000000000001.
 */
import Decimal from 'decimal.js'

/** Pola persen yang diterima: 0–99, opsional 1–2 angka desimal (koma atau titik). */
const POLA_PERSEN = /^\d{1,2}([.,]\d{1,2})?$/

/**
 * "10" → "0.1", "7,5" → "0.075", "" → "0". null bila bukan persen yang sah
 * (huruf, negatif, ≥ 100, atau lebih dari dua desimal) — server menolak
 * tarif ≥ 1 dan lebih dari 4 desimal pecahan, dan batas di sini setara.
 */
export function persenKePecahan(teks: string): string | null {
  const t = teks.trim()
  if (t === '') return '0'
  if (!POLA_PERSEN.test(t)) return null
  return new Decimal(t.replace(',', '.')).div(100).toString()
}

/** "0.1" → "10", "0.075" → "7,5", "0" / "" → "" (kolom kosong = tidak ada). */
export function pecahanKePersen(pecahan: string | undefined): string {
  if (!pecahan) return ''
  let p: Decimal
  try {
    p = new Decimal(pecahan).mul(100)
  } catch {
    return ''
  }
  return p.isZero() ? '' : p.toString().replace('.', ',')
}
