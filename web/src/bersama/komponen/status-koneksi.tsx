import { Check, CloudOff } from 'lucide-react'
import { useOnline } from '@/bersama/hooks/use-online'
import { cn } from '@/bersama/util/cn'

/**
 * Indikator koneksi — kecil, tenang, dan selalu menyebut bahwa jualan tetap
 * bisa jalan. Jumlah antrean diisi mesin sinkronisasi pada tahap offline (U3).
 */
export function StatusKoneksi({
  menunggu = 0,
  className,
}: {
  menunggu?: number
  className?: string
}) {
  const online = useOnline()

  if (menunggu > 0) {
    return (
      <p className={cn('flex items-center gap-1.5 text-keterangan text-jingga-700', className)}>
        <CloudOff className="h-4 w-4" aria-hidden />
        {menunggu} transaksi menunggu dikirim
      </p>
    )
  }

  if (!online) {
    return (
      <p className={cn('flex items-center gap-1.5 text-keterangan text-jingga-700', className)}>
        <CloudOff className="h-4 w-4" aria-hidden />
        Belum ada internet — tetap bisa jualan seperti biasa
      </p>
    )
  }

  return (
    <p className={cn('flex items-center gap-1.5 text-keterangan text-hijau-700', className)}>
      <Check className="h-4 w-4" aria-hidden />
      Semua data tersimpan
    </p>
  )
}
