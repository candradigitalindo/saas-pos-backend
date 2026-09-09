import { Minus, Plus } from 'lucide-react'
import { cn } from '@/bersama/util/cn'
import { formatQty, kurangQty, tambahQty } from '@/bersama/util/desimal'

/**
 * Pengubah jumlah dengan tombol − dan + besar.
 *
 * Alasannya bukan estetika: mengetik angka di HP sambil berdiri di depan rak
 * itu susah, dan salah ketik pada jumlah barang berarti stok salah.
 */
export function StepperJumlah({
  nilai,
  onNilai,
  langkah = 1,
  minimal = '0',
  satuan,
  besar,
  label = 'Jumlah',
}: {
  nilai: string
  onNilai: (qty: string) => void
  langkah?: number
  minimal?: string
  satuan?: string
  besar?: boolean
  label?: string
}) {
  const ukuran = besar ? 'h-16 w-16' : 'h-12 w-12'

  return (
    <div className="flex items-center gap-2" role="group" aria-label={label}>
      <button
        type="button"
        onClick={() => onNilai(kurangQty(nilai, langkah, minimal))}
        aria-label={`Kurangi ${label.toLowerCase()}`}
        className={cn(
          ukuran,
          'flex shrink-0 items-center justify-center rounded-kontrol border',
          'border-garis bg-permukaan text-teks-utama hover:bg-permukaan-2',
        )}
      >
        <Minus className="h-5 w-5" aria-hidden />
      </button>

      <div
        className={cn(
          'flex min-w-16 flex-1 items-center justify-center gap-1 font-bold tabular-nums',
          besar ? 'text-judul' : 'text-judul-kartu',
        )}
        aria-live="polite"
      >
        {formatQty(nilai)}
        {satuan && <span className="text-label font-normal text-teks-redup">{satuan}</span>}
      </div>

      <button
        type="button"
        onClick={() => onNilai(tambahQty(nilai, langkah))}
        aria-label={`Tambah ${label.toLowerCase()}`}
        className={cn(
          ukuran,
          'flex shrink-0 items-center justify-center rounded-kontrol border',
          'border-garis bg-permukaan text-teks-utama hover:bg-permukaan-2',
        )}
      >
        <Plus className="h-5 w-5" aria-hidden />
      </button>
    </div>
  )
}
