import { formatRupiah } from '@/bersama/util/uang'
import { formatTanggal } from '@/bersama/util/tanggal'
import { statusJatuhTempo } from '@/fitur/stok/utang'

/** Satu kasbon di teks tagihan. */
export interface BarisTagihan {
  /** Tanggal usaha nota (YYYY-MM-DD). */
  tanggal?: string
  nota?: string
  sisa: number
  jatuhTempo?: string
}

/** Paling banyak sekian kasbon dirinci; sisanya disebut jumlahnya saja. */
const RINCI_MAKS = 5

const tgl = (iso: string) => formatTanggal(iso + 'T12:00:00Z')

/**
 * Teks tagihan WhatsApp ke pelanggan — sopan, pendek, dan menyebut nota
 * serta jatuh temponya supaya pelanggan bisa mencocokkan sendiri. Dikirim
 * dari WhatsApp PEMILIK (wa.me), bukan nomor platform: pelanggan warung
 * mengenal nomor tokonya, dan pemilik bisa menyunting kalimatnya dulu.
 *
 * Tanpa rincian (dari daftar Kasbon), cukup total & jumlah notanya.
 */
export function teksTagihan(p: {
  nama: string
  toko: string
  total: number
  hariIni: string
  /** Jumlah nota belum lunas (dipakai bila `rincian` kosong). */
  jumlahNota?: number
  /** Jatuh tempo terdekat (dipakai bila `rincian` kosong). */
  jatuhTempo?: string
  rincian?: BarisTagihan[]
}): string {
  const salam = `Halo ${p.nama}, ini dari ${p.toko}. Mengingatkan kasbon yang belum dibayar`
  const penutup = 'Bisa dibayar langsung di toko atau lewat transfer. Terima kasih 🙏'

  if (!p.rincian?.length) {
    const nota = p.jumlahNota && p.jumlahNota > 1 ? ` (${p.jumlahNota} nota)` : ''
    return `${salam}: ${formatRupiah(p.total)}${nota}${keteranganTempo(p.jatuhTempo, p.hariIni)}.\n\n${penutup}`
  }

  const baris = p.rincian.slice(0, RINCI_MAKS).map((k, i) => {
    const asal = [k.tanggal && tgl(k.tanggal), k.nota && `nota ${k.nota}`].filter(Boolean).join(', ')
    return `${i + 1}. ${asal ? `${asal} — ` : ''}${formatRupiah(k.sisa)}${keteranganTempo(k.jatuhTempo, p.hariIni)}`
  })
  const lebih = p.rincian.length - RINCI_MAKS
  if (lebih > 0) baris.push(`… dan ${lebih} nota lainnya`)
  return `${salam}:\n${baris.join('\n')}\n\nTotal: ${formatRupiah(p.total)}\n\n${penutup}`
}

/** ", jatuh tempo 26 Sep 2026" / ", sudah lewat jatuh tempo 4 hari" / "". */
function keteranganTempo(jatuhTempo: string | undefined, hariIni: string): string {
  if (!jatuhTempo) return ''
  const s = statusJatuhTempo(jatuhTempo, hariIni)
  if (s.nada === 'lewat') return `, sudah lewat jatuh tempo ${s.teks.replace('lewat ', '')}`
  if (s.nada === 'hariIni') return ', jatuh tempo hari ini'
  return `, jatuh tempo ${tgl(jatuhTempo)}`
}
