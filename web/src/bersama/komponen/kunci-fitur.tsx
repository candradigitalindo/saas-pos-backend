import { Link } from 'react-router-dom'
import { Lock } from 'lucide-react'
import { Tombol } from '@/bersama/ui/tombol'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { IZIN } from '@/lib/izin'
import type { KodeFitur } from '@/lib/fitur'
import { cn } from '@/bersama/util/cn'

/**
 * Pemberitahuan fitur terkunci paket — di atas halaman fitur itu.
 *
 * Halamannya tetap terbuka dan datanya tetap terbaca (server hanya menutup
 * tindakan BARU); tombol "tambah" disembunyikan pemanggilnya, dan kartu ini
 * menjelaskan kenapa, dengan nama paket yang membukanya.
 *
 * Ajakan "Lihat paket" hanya untuk yang berhak mengurus langganan
 * (billing.manage). Kasir diberi tahu siapa yang bisa memutuskannya — tombol
 * yang membawanya ke layar yang tidak boleh ia buka hanya membingungkan.
 *
 * Tidak merender apa pun bila fiturnya termasuk paket.
 */
export function BannerKunciFitur({
  fitur,
  nama,
  penjelasan,
  className,
}: {
  fitur: KodeFitur
  /** Nama fitur dalam kalimat, mis. "Kanal online". */
  nama: string
  /** Apa yang tetap bisa dilakukan tanpa fitur ini. */
  penjelasan: string
  className?: string
}) {
  const { punyaFitur, paketUntuk, boleh } = useSesi()
  if (punyaFitur(fitur)) return null
  const paket = paketUntuk(fitur)

  return (
    <div
      role="status"
      className={cn(
        'flex flex-col gap-3 rounded-kartu border border-garis bg-permukaan p-4 shadow-kartu sm:flex-row sm:items-center',
        className,
      )}
    >
      <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-sorot text-hijau-800">
        <Lock className="h-5 w-5" aria-hidden />
      </span>
      <div className="min-w-0 flex-1">
        <p className="font-semibold text-teks-utama">
          {nama} {paket ? `tersedia mulai paket ${paket}` : 'tidak termasuk paket Anda'}
        </p>
        <p className="text-label text-teks-sekunder">
          {penjelasan}
          {!boleh(IZIN.billingManage) && ' Minta pemilik usaha menaikkan paket bila perlu.'}
        </p>
      </div>
      {boleh(IZIN.billingManage) && (
        <Tombol asChild jenis="kedua" ukuran="padat" className="shrink-0">
          <Link to="/langganan">Lihat paket</Link>
        </Tombol>
      )}
    </div>
  )
}
