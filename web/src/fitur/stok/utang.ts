import { formatTanggal } from '@/bersama/util/tanggal'

export type NadaJatuhTempo = 'lewat' | 'hariIni' | 'dekat' | 'nanti' | 'tanpa'

/** Batas "sebentar lagi": jatuh tempo dalam seminggu. */
export const HARI_DEKAT = 7

const hari = (iso: string) => Date.UTC(+iso.slice(0, 4), +iso.slice(5, 7) - 1, +iso.slice(8, 10)) / 86_400_000

/**
 * Keadaan jatuh tempo satu utang, dibanding tanggal usaha hari ini (keduanya
 * YYYY-MM-DD — dibandingkan sebagai TANGGAL, bukan jam, supaya "hari ini"
 * tetap hari ini sampai toko tutup buku).
 */
export function statusJatuhTempo(jatuhTempo: string | undefined, hariIni: string): { nada: NadaJatuhTempo; teks: string } {
  if (!jatuhTempo) return { nada: 'tanpa', teks: 'tanpa jatuh tempo' }
  const selisih = Math.round(hari(jatuhTempo) - hari(hariIni))
  if (selisih < 0) return { nada: 'lewat', teks: `lewat ${-selisih} hari` }
  if (selisih === 0) return { nada: 'hariIni', teks: 'jatuh tempo hari ini' }
  if (selisih <= HARI_DEKAT) return { nada: 'dekat', teks: selisih === 1 ? 'jatuh tempo besok' : `${selisih} hari lagi` }
  return { nada: 'nanti', teks: `jatuh tempo ${formatTanggal(jatuhTempo + 'T12:00:00Z')}` }
}

/** YYYY-MM-DD n hari setelah `dari`. */
export function tambahHari(dari: string, n: number): string {
  const d = new Date(Date.UTC(+dari.slice(0, 4), +dari.slice(5, 7) - 1, +dari.slice(8, 10) + n))
  return d.toISOString().slice(0, 10)
}
