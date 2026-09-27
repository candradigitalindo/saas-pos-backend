import type { LucideIcon } from 'lucide-react'
import { Ban, Check, CircleDot, CloudOff, Clock, RotateCcw, TriangleAlert } from 'lucide-react'
import { cn } from '@/bersama/util/cn'

/**
 * Lencana status: SELALU ikon + teks + warna, tidak pernah warna saja.
 *
 * 1 dari 12 pria buta warna merah-hijau. Lencana yang hanya berbeda warnanya
 * tidak terbaca oleh mereka sama sekali.
 */
export type NadaStatus = 'berhasil' | 'menunggu' | 'bahaya' | 'netral' | 'info'

const NADA: Record<NadaStatus, { kelas: string; ikon: LucideIcon }> = {
  berhasil: { kelas: 'bg-hijau-800/15 text-hijau-800', ikon: Check },
  menunggu: { kelas: 'bg-permukaan-2 text-jingga-700', ikon: Clock },
  bahaya: { kelas: 'bg-bahaya-teks/15 text-bahaya-teks', ikon: Ban },
  netral: { kelas: 'bg-permukaan-2 text-teks-sekunder', ikon: CircleDot },
  info: { kelas: 'bg-info-teks/15 text-info-teks', ikon: CloudOff },
}

export function LencanaStatus({
  nada,
  anak,
  ikon,
  className,
}: {
  nada: NadaStatus
  anak: string
  ikon?: LucideIcon
  className?: string
}) {
  const { kelas, ikon: IkonBawaan } = NADA[nada]
  const Ikon = ikon ?? IkonBawaan
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-full px-2 py-0.5',
        'text-keterangan font-medium',
        kelas,
        className,
      )}
    >
      <Ikon className="h-3.5 w-3.5" aria-hidden />
      {anak}
    </span>
  )
}

/**
 * Peta status transaksi backend → lencana berbahasa manusia.
 *
 * Status yang benar-benar dikirim server untuk penjualan: `completed`,
 * `canceled` (void), `returned` (baris retur, bernilai negatif), dan `open`
 * (tagihan meja yang belum dibayar). Dua yang terakhir dulu tidak dipetakan,
 * sehingga riwayat menampilkan kata Inggris mentah "returned".
 */
export function LencanaTransaksi({ status }: { status: string }) {
  switch (status) {
    case 'paid':
    case 'completed':
      return <LencanaStatus nada="berhasil" anak="Lunas" />
    case 'void':
    case 'voided':
    case 'canceled':
      return <LencanaStatus nada="bahaya" anak="Dibatalkan" />
    case 'returned':
      // Baris retur bukan kegagalan — uang dikembalikan dengan sengaja.
      return <LencanaStatus nada="netral" anak="Retur" ikon={RotateCcw} />
    case 'open':
      return <LencanaStatus nada="menunggu" anak="Belum dibayar" />
    case 'refunded':
      return <LencanaStatus nada="bahaya" anak="Diretur" ikon={TriangleAlert} />
    case 'partial':
      return <LencanaStatus nada="menunggu" anak="Bayar sebagian" />
    case 'credit':
    case 'unpaid':
      return <LencanaStatus nada="menunggu" anak="Kasbon" />
    case 'draft':
      return <LencanaStatus nada="netral" anak="Draf" />
    default:
      return <LencanaStatus nada="netral" anak={status} />
  }
}
