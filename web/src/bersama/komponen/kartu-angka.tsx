import { ArrowDown, ArrowUp, Minus } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { formatRupiah, persenSelisih } from '@/bersama/util/uang'
import { cn } from '@/bersama/util/cn'

/**
 * Kartu angka untuk beranda.
 *
 * Urutan sengaja: label kecil dulu, baru angka besar. Dan SELALU ada
 * pembanding — angka tanpa pembanding tidak memberi tahu apa pun
 * (ui/01-PRINSIP-DESAIN.md §4).
 *
 * Perbandingan ditandai ikon panah, bukan hanya warna.
 */
export function KartuAngka({
  label,
  nilai,
  pembanding,
  labelPembanding = 'dibanding kemarin',
  keterangan,
  uang = true,
  /** true bila naik itu BURUK (mis. jumlah retur) — arah panah tetap, warnanya dibalik. */
  naikItuBuruk = false,
}: {
  label: string
  nilai: number
  pembanding?: number
  labelPembanding?: string
  keterangan?: string
  uang?: boolean
  naikItuBuruk?: boolean
}) {
  const persen = pembanding === undefined ? null : persenSelisih(nilai, pembanding)
  const naik = persen !== null && persen > 0
  const turun = persen !== null && persen < 0
  const bagus = naik ? !naikItuBuruk : turun ? naikItuBuruk : true

  const Panah = naik ? ArrowUp : turun ? ArrowDown : Minus

  return (
    <Kartu className="p-4">
      <p className="text-label font-medium text-teks-sekunder">{label}</p>
      <p className="mt-1 text-angka font-extrabold tabular-nums text-teks-utama">
        {uang ? formatRupiah(nilai) : nilai.toLocaleString('id-ID')}
      </p>

      {persen !== null ? (
        <p
          className={cn(
            'mt-1 flex items-center gap-1 text-keterangan font-medium',
            persen === 0
              ? 'text-teks-redup'
              : bagus
                ? 'text-hijau-700'
                : 'text-bahaya-teks',
          )}
        >
          <Panah className="h-4 w-4" aria-hidden />
          {Math.abs(persen)}% {labelPembanding}
        </p>
      ) : (
        /* Angka tanpa pembanding tidak memberi tahu apa pun (ui/01 §4). Kalau
           pembandingnya memang belum ada, KATAKAN — jangan tinggalkan angka
           telanjang yang tampak seperti baris yang lupa dimuat. */
        <p className="mt-1 text-keterangan text-teks-redup">
          {keterangan ?? (pembanding === undefined ? '' : `Belum ada data pembanding ${labelPembanding.replace(/^dibanding /, '')}`)}
        </p>
      )}
    </Kartu>
  )
}
