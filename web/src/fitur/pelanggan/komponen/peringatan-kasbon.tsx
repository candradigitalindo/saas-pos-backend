import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ChevronRight, NotebookPen } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { IZIN } from '@/lib/izin'
import { formatRupiah } from '@/bersama/util/uang'
import { cn } from '@/bersama/util/cn'
import { pelangganApi } from '../api'

/** Ringkasan kasbon — kunci sama untuk layar Kasbon, Beranda, dan lencana menu. */
export function useRingkasanKasbon(aktif = true) {
  const { tokoAktif, boleh } = useSesi()
  return useQuery({
    queryKey: ['kasbon', 'ringkasan', tokoAktif],
    queryFn: () => pelangganApi.ringkasanKasbon(tokoAktif ?? undefined),
    enabled: aktif && boleh(IZIN.receivableManage),
    staleTime: 60_000,
  })
}

/**
 * Peringatan kasbon di Beranda — HANYA saat ada yang perlu ditagih: lewat
 * jatuh tempo, atau jatuh tempo dalam beberapa hari. Kasbon tanpa jatuh tempo
 * tidak mengganggu Beranda; daftar lengkapnya ada di layar Kasbon.
 */
export function PeringatanKasbon() {
  const q = useRingkasanKasbon()
  const r = q.data
  if (!r || (r.overdue_count === 0 && r.due_soon_count === 0)) return null
  const lewat = r.overdue_count > 0

  const bagian = [
    r.overdue_count > 0 && `${formatRupiah(r.overdue_amount)} lewat jatuh tempo`,
    r.due_soon_count > 0 &&
      `${formatRupiah(r.due_soon_amount)} jatuh tempo ${r.due_soon_days > 0 ? `≤ ${r.due_soon_days} hari` : 'hari ini'}`,
  ].filter(Boolean)

  return (
    <Link to="/kasbon">
      <Kartu
        className={cn(
          'flex items-center gap-3 p-4 transition-colors hover:brightness-[0.98]',
          lewat ? 'border-bahaya-teks/40 bg-bahaya-teks/5' : 'border-jingga-600 bg-permukaan-2',
        )}
      >
        <NotebookPen className={cn('h-5 w-5 shrink-0', lewat ? 'text-bahaya-teks' : 'text-jingga-700')} aria-hidden />
        <div className="min-w-0 flex-1">
          <p className={cn('text-label font-semibold', lewat ? 'text-bahaya-teks' : 'text-jingga-700')}>
            Kasbon:{' '}
            {lewat
              ? `${r.overdue_count} pelanggan lewat jatuh tempo — waktunya menagih`
              : `${r.due_soon_count} pelanggan segera jatuh tempo`}
          </p>
          <p className="text-keterangan text-teks-sekunder">{bagian.join(' · ')}</p>
        </div>
        <ChevronRight className="h-5 w-5 shrink-0 text-teks-redup" aria-hidden />
      </Kartu>
    </Link>
  )
}
