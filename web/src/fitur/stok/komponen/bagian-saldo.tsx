import type { LucideIcon } from 'lucide-react'
import { Ban, CircleCheck, Clock, Hourglass, TriangleAlert } from 'lucide-react'
import { LencanaStatus } from '@/bersama/komponen/lencana-status'
import { formatQty, formatQtySatuan } from '@/bersama/util/desimal'
import { formatLaluHari } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import type { SaldoStok } from '@/bersama/tipe/katalog'
import type { KeadaanStok } from '../api'
import { HARI_MENDESAK, cukupHari, isiMeter, teksCukup, type Keadaan } from '../keadaan-stok'

/**
 * Potongan tampilan satu saldo stok — dipakai daftar Stok dan Riwayat Stok
 * satu barang, supaya keadaan, sisa, dan "laku/cukup" terbaca sama di keduanya.
 */

/**
 * Tiap keadaan: label, ikon, warna ikon, warna bilah, dan kalimat kosongnya.
 * Ikon + teks + warna — tidak pernah warna saja.
 */
export const KEADAAN: Record<
  KeadaanStok,
  { label: string; ikon: LucideIcon; teks: string; bilah: string; kosong: [string, string] }
> = {
  safe: {
    label: 'Aman',
    ikon: CircleCheck,
    teks: 'text-hijau-700',
    bilah: 'bg-hijau-600',
    kosong: ['Belum ada barang yang aman', 'Semua barang sedang di bawah batas minimumnya.'],
  },
  low: {
    label: 'Hampir habis',
    ikon: Clock,
    teks: 'text-jingga-700',
    bilah: 'bg-jingga-400',
    kosong: ['Tidak ada yang hampir habis', 'Semua stok masih di atas batas yang Anda tentukan. Bagus.'],
  },
  out: {
    label: 'Habis',
    ikon: Ban,
    teks: 'text-bahaya-teks',
    bilah: 'bg-bahaya/60',
    kosong: ['Tidak ada barang yang habis', 'Semua barang masih ada stoknya.'],
  },
  negative: {
    label: 'Perlu dicocokkan',
    ikon: TriangleAlert,
    teks: 'text-bahaya-teks',
    bilah: 'bg-bahaya-teks',
    kosong: ['Semua catatan cocok', 'Tidak ada barang yang terjual melebihi catatan stoknya.'],
  },
  idle: {
    label: 'Tidak laku 30 hari',
    ikon: Hourglass,
    teks: 'text-teks-sekunder',
    bilah: 'bg-teks-redup',
    kosong: ['Semua barang laku', 'Setiap barang yang ada stoknya terjual dalam 30 hari terakhir.'],
  },
}

export function LencanaKeadaan({ keadaan }: { keadaan: Keadaan }) {
  switch (keadaan) {
    case 'negative':
      return <LencanaStatus nada="bahaya" anak="Perlu dicocokkan" ikon={TriangleAlert} />
    case 'out':
      return <LencanaStatus nada="bahaya" anak="Habis" />
    case 'low':
      return <LencanaStatus nada="menunggu" anak="Hampir habis" />
    default:
      return <LencanaStatus nada="berhasil" anak="Aman" />
  }
}

/** Sisa dalam satu kata-angka: "146 pcs" / "Habis". Minus dijelaskan terpisah. */
export function teksSisa(s: SaldoStok, k: Keadaan): string {
  return k === 'out' || k === 'negative' ? 'Habis' : formatQtySatuan(s.qty, s.unit_name)
}

/**
 * Keterangan penjualan: "laku 12 pcs · cukup ±5 hari", atau kapan terakhir
 * laku bila 30 hari ini tidak ada yang terjual. Bagian "cukup" ditandai
 * mendesak bila ≤ 3 hari — walau sisanya masih di atas batas minimum.
 */
export function Laku({ s, k, sebaris = false }: { s: SaldoStok; k: Keadaan; sebaris?: boolean }) {
  if (k === 'negative') {
    return (
      <span className="text-bahaya-teks">
        tercatat {formatQtySatuan(s.qty, s.unit_name).replace('-', '−')} — hitung ulang barangnya
      </span>
    )
  }
  const laku = Number.parseFloat(s.sold_30d ?? '0') || 0
  if (laku <= 0) {
    return (
      <span>
        {s.last_sold_at ? `terakhir laku ${formatLaluHari(s.last_sold_at).toLowerCase()}` : 'belum pernah terjual'}
      </span>
    )
  }
  const hari = cukupHari(s)
  const mendesak = hari !== null && hari <= HARI_MENDESAK
  const cukup = hari !== null && (
    <span className={cn(mendesak && 'font-semibold text-jingga-700')}>
      {mendesak && <Clock className="mr-0.5 inline h-3.5 w-3.5 -translate-y-px" aria-hidden />}
      {teksCukup(hari)}
    </span>
  )
  if (!sebaris) {
    return (
      <>
        <span className="block font-medium tabular-nums text-teks-utama">{formatQtySatuan(s.sold_30d ?? '0', s.unit_name)}</span>
        {cukup && <span className="block">{cukup}</span>}
      </>
    )
  }
  return (
    <span>
      laku {formatQty(s.sold_30d ?? '0')} sebulan{cukup && <> · {cukup}</>}
    </span>
  )
}

/** Meteran sisa terhadap batas minimum; garis tipis di tengah = batasnya. */
export function Meter({ s, k, className }: { s: SaldoStok; k: Keadaan; className?: string }) {
  const isi = isiMeter(s)
  // Minus: meteran kosong tidak menambah apa pun pada "Perlu dicocokkan".
  if (isi === null || k === 'negative') return null
  return (
    <span className={cn('relative block h-1.5 overflow-hidden rounded-full bg-permukaan-2', className)} aria-hidden>
      <span
        className={cn('absolute inset-y-0 left-0 rounded-full', KEADAAN[k].bilah)}
        style={{ width: `${isi > 0 ? Math.max(isi * 100, 6) : 0}%` }}
      />
      <span className="absolute inset-y-0 left-1/2 w-0.5 -translate-x-1/2 bg-teks-redup/70" />
    </span>
  )
}
