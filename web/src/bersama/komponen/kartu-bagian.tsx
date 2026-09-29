import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { ChevronRight } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { cn } from '@/bersama/util/cn'

/**
 * Kartu satu bagian dasbor: kepala (judul, keterangan, aksi) DI DALAM kartu,
 * lalu isinya.
 *
 * Pola ini yang dipakai dasbor SaaS pada umumnya, dan alasannya praktis:
 * judul yang melayang di atas kartu (versi Beranda sebelumnya) terbaca sebagai
 * milik halaman, bukan milik kartunya — begitu dua kartu berdampingan di layar
 * lebar, tidak jelas judul mana untuk kartu mana. Di dalam kartu, setiap
 * bagian utuh sendiri dan bisa dipindah-susun tanpa kehilangan judulnya.
 */
export function KartuBagian({
  judul,
  keterangan,
  aksi,
  children,
  className,
  isiClassName,
}: {
  judul: string
  keterangan?: ReactNode
  /** Kendali di kanan kepala: tautan "Lihat semua", segmen, dst. */
  aksi?: ReactNode
  children: ReactNode
  className?: string
  isiClassName?: string
}) {
  return (
    <Kartu className={cn('flex min-w-0 flex-col', className)}>
      <div className="flex items-start justify-between gap-3 px-4 pt-4 sm:px-5">
        <div className="min-w-0">
          <h2 className="text-judul-kartu font-semibold text-teks-utama">{judul}</h2>
          {keterangan && <p className="text-keterangan text-teks-redup">{keterangan}</p>}
        </div>
        {aksi && <div className="-my-2 shrink-0">{aksi}</div>}
      </div>
      <div className={cn('flex-1 px-4 pb-4 pt-3 sm:px-5', isiClassName)}>{children}</div>
    </Kartu>
  )
}

/** Tautan kecil "Lihat semua ›" untuk kepala KartuBagian; target sentuh 48px. */
export function TautanBagian({ ke, children }: { ke: string; children: ReactNode }) {
  return (
    <Link
      to={ke}
      className="flex min-h-12 items-center gap-1 rounded-kontrol px-2 text-label font-semibold text-utama underline-offset-4 hover:underline"
    >
      {children}
      <ChevronRight className="h-4 w-4" aria-hidden />
    </Link>
  )
}
