import { geserTanggal, tanggalKeUTC as keUTC } from '@/bersama/util/tanggal'
import type { BarisLaporan } from './api'

/**
 * Hitungan tanggal & deret harian untuk grafik laporan.
 *
 * Tanggal di sini SELALU berbentuk "YYYY-MM-DD" (hari usaha dari server), dan
 * aritmetikanya dikerjakan di UTC. Bukan kehalusan: dengan `new Date(...)`
 * lokal, perangkat yang zonanya punya pergantian jam musim panas bisa
 * melompati atau menggandakan satu tanggal — dan grafik tujuh hari mendadak
 * berisi enam batang.
 */

export { geserTanggal }

/** Semua tanggal dari `dari` s.d. `sampai` (inklusif), berurutan. */
export function daftarTanggal(dari: string, sampai: string): string[] {
  const keluar: string[] = []
  // Batas 400 langkah: rentang terbalik atau salah ketik tidak boleh membuat
  // perulangan tanpa ujung.
  for (let t = dari, i = 0; t <= sampai && i < 400; t = geserTanggal(t, 1), i++) {
    keluar.push(t)
  }
  return keluar
}

/** Baris kosong untuk satu tanggal — semua besaran nol. */
function barisNol(key: string): BarisLaporan {
  return {
    key,
    sales_count: 0,
    gross_amount: 0,
    discount_amount: 0,
    tax_amount: 0,
    net_amount: 0,
    cost_amount: 0,
    fee_amount: 0,
    gross_profit: 0,
  }
}

/**
 * Melengkapi deret harian: tanggal yang tidak dikirim server (tidak ada
 * penjualan) diisi baris bernilai nol, bukan dilewati.
 *
 * Server hanya mengirim hari yang ADA penjualannya. Digambar apa adanya, tiga
 * hari berjualan dalam sebulan tampil sebagai tiga batang berdempetan — hari
 * sepi lenyap dari grafik dan bulan yang lesu terlihat ramai.
 */
export function lengkapiHari(
  rows: BarisLaporan[] | undefined,
  dari: string,
  sampai: string,
): BarisLaporan[] {
  const perTanggal = new Map((rows ?? []).map((r) => [r.key, r]))
  return daftarTanggal(dari, sampai).map((t) => perTanggal.get(t) ?? barisNol(t))
}

export interface RingkasanPeriode {
  /** Uang masuk seluruh periode. */
  uangMasuk: number
  transaksi: number
  /** Uang masuk dibagi jumlah HARI (termasuk hari tanpa penjualan). */
  rataPerHari: number
  /** Hari dengan uang masuk terbesar; undefined bila tak ada penjualan. */
  hariTeramai?: { tanggal: string; uangMasuk: number }
}

/** Ringkasan deret harian yang SUDAH dilengkapi (lihat lengkapiHari). */
export function ringkasPeriode(deret: BarisLaporan[]): RingkasanPeriode {
  let uangMasuk = 0
  let transaksi = 0
  let teramai: BarisLaporan | undefined
  for (const r of deret) {
    uangMasuk += r.net_amount
    transaksi += r.sales_count
    if (r.net_amount > 0 && (!teramai || r.net_amount > teramai.net_amount)) teramai = r
  }
  return {
    uangMasuk,
    transaksi,
    rataPerHari: deret.length ? Math.round(uangMasuk / deret.length) : 0,
    hariTeramai: teramai && { tanggal: teramai.key, uangMasuk: teramai.net_amount },
  }
}

/**
 * Periode `hari` hari yang berakhir `sampai`, BESERTA periode sepanjang itu
 * tepat sebelumnya — untuk "naik 12% dari 7 hari sebelumnya".
 *
 * Keduanya diambil dengan SATU permintaan (rentang ganda) lalu dibelah di sini,
 * supaya dua angka yang dibandingkan pasti berasal dari data yang sama.
 */
export function periodeBerpembanding(sampai: string, hari: number) {
  const dari = geserTanggal(sampai, -(hari - 1))
  const sampaiLalu = geserTanggal(dari, -1)
  const dariLalu = geserTanggal(sampaiLalu, -(hari - 1))
  return { dari, sampai, dariLalu, sampaiLalu }
}

/** "21–27 Sep" (sebulan) atau "29 Agu – 27 Sep" (melintasi bulan). */
export function labelRentang(dari: string, sampai: string): string {
  const f = (iso: string, o: Intl.DateTimeFormatOptions) =>
    new Intl.DateTimeFormat('id-ID', { ...o, timeZone: 'UTC' }).format(keUTC(iso))
  if (dari.slice(0, 7) === sampai.slice(0, 7)) {
    return `${f(dari, { day: 'numeric' })}–${f(sampai, { day: 'numeric', month: 'short' })}`
  }
  return `${f(dari, { day: 'numeric', month: 'short' })} – ${f(sampai, { day: 'numeric', month: 'short' })}`
}

/** "Jum, 18 Sep" — hari & tanggal pendek. */
export function labelHariPendek(iso: string): string {
  return new Intl.DateTimeFormat('id-ID', {
    weekday: 'short',
    day: 'numeric',
    month: 'short',
    timeZone: 'UTC',
  }).format(keUTC(iso))
}
