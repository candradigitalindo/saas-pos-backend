import { tanggalISO } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'

/**
 * Rentang tanggal laporan — dipakai bersama Laporan Penjualan & Laporan
 * Belanja supaya pilihan, hitungan, dan kolom tanggalnya sama persis.
 */
export type Rentang = 'hari-ini' | '7-hari' | '30-hari' | 'bulan-ini' | 'pilih'

export const RENTANG: Record<Rentang, string> = {
  'hari-ini': 'Hari ini',
  '7-hari': '7 hari terakhir',
  '30-hari': '30 hari terakhir',
  'bulan-ini': 'Bulan ini',
  pilih: 'Pilih tanggal',
}

export function hitungRentang(
  r: Rentang,
  dariPilih: string,
  sampaiPilih: string,
): { dari: string; sampai: string } {
  const kini = new Date()
  const sampai = tanggalISO(kini)
  if (r === 'pilih') {
    // Dibalik bila terbalik, bukan ditolak: orang sering mengisi kolom kedua
    // lebih dulu, dan menolak isian yang maksudnya jelas cuma menghalangi.
    return dariPilih <= sampaiPilih
      ? { dari: dariPilih, sampai: sampaiPilih }
      : { dari: sampaiPilih, sampai: dariPilih }
  }
  if (r === 'hari-ini') return { dari: sampai, sampai }
  if (r === 'bulan-ini') {
    return { dari: `${sampai.slice(0, 7)}-01`, sampai }
  }
  const hari = r === '7-hari' ? 6 : 29
  return { dari: tanggalISO(new Date(kini.getTime() - hari * 86_400_000)), sampai }
}

export function periodeSebelum(dari: string, sampai: string): { dari: string; sampai: string } {
  const d = new Date(`${dari}T00:00:00Z`)
  const s = new Date(`${sampai}T00:00:00Z`)
  const panjang = Math.max(1, Math.round((s.getTime() - d.getTime()) / 86_400_000) + 1)
  const sBaru = new Date(d.getTime() - 86_400_000)
  const dBaru = new Date(sBaru.getTime() - (panjang - 1) * 86_400_000)
  const iso = (x: Date) => x.toISOString().slice(0, 10)
  return { dari: iso(dBaru), sampai: iso(sBaru) }
}

export function labelPembanding(r: Rentang): string {
  return r === 'hari-ini' ? 'dibanding kemarin' : 'dibanding periode sebelumnya'
}

/** Kolom tanggal: label di atas, bukan placeholder (ui/02 — Kolom isian). */
export function KolomTanggal({
  label,
  nilai,
  onUbah,
}: {
  label: string
  nilai: string
  onUbah: (v: string) => void
}) {
  return (
    <label className="flex flex-1 flex-col gap-1">
      <span className="text-keterangan font-medium text-teks-sekunder">{label}</span>
      <input
        type="date"
        value={nilai}
        max={tanggalISO()}
        onChange={(e) => onUbah(e.target.value)}
        className={cn(
          'h-12 w-full rounded-kontrol border border-garis bg-permukaan px-3',
          'text-isi tabular-nums text-teks-utama',
          'focus:border-utama focus:outline-none focus:ring-2 focus:ring-utama/30',
        )}
      />
    </label>
  )
}
