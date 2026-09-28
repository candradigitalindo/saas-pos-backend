import { useState } from 'react'
import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { ChevronDown, ClipboardList } from 'lucide-react'
import { Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { Tombol } from '@/bersama/ui/tombol'
import { LencanaStatus } from '@/bersama/komponen/lencana-status'
import { KeadaanGagal } from '@/bersama/komponen/keadaan-kosong'
import { Kerangka, KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { GalatAPI } from '@/lib/api-client'
import { formatRupiah } from '@/bersama/util/uang'
import { formatQty } from '@/bersama/util/desimal'
import { formatTanggalJam } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import { stokApi, type Opname } from '../api'

const PER_HALAMAN = 20

/** "−Rp 18.000" / "+Rp 12.000" — tanda di depan, bukan di dalam angka. */
export function formatSelisihRupiah(n: number): string {
  if (n === 0) return formatRupiah(0)
  return `${n < 0 ? '−' : '+'}${formatRupiah(Math.abs(n))}`
}

/**
 * Riwayat hitung fisik toko ini, terbaru dulu: berapa barang dihitung, berapa
 * yang berubah, perkiraan nilai selisihnya, kapan & siapa. Ketuk untuk
 * rincian per barang (catatan → hitungan).
 *
 * Nilainya perubahan NILAI STOK (catatan minus dihitung nol) dengan harga
 * modal SEKARANG — gerakan opname tidak menyimpan harga modal saat itu, jadi
 * ditulis "≈".
 */
export function DialogRiwayatHitung({ terbuka, onTutup }: { terbuka: boolean; onTutup: () => void }) {
  const { tokoAktif } = useSesi()
  const [buka, setBuka] = useState<string | null>(null)

  const q = useInfiniteQuery({
    queryKey: ['opname', tokoAktif],
    queryFn: ({ pageParam }) => stokApi.daftarOpname(tokoAktif!, pageParam, PER_HALAMAN),
    initialPageParam: 1,
    getNextPageParam: (akhir, semua) => (semua.length * PER_HALAMAN < akhir.total ? semua.length + 1 : undefined),
    enabled: terbuka && !!tokoAktif,
  })
  const daftar = q.data?.pages.flatMap((h) => h.data) ?? []

  return (
    <Dialog open={terbuka} onOpenChange={(o) => !o && onTutup()}>
      <IsiDialog
        judul="Riwayat hitung fisik"
        keterangan="Angka rupiah = perubahan nilai stok, menurut harga modal sekarang."
        className="sm:max-w-xl"
      >
        {q.isLoading ? (
          <KerangkaBaris jumlah={4} />
        ) : q.isError ? (
          <KeadaanGagal
            pesan={q.error instanceof GalatAPI ? q.error.pesan : 'Riwayat belum bisa dimuat.'}
            onCobaLagi={() => q.refetch()}
          />
        ) : daftar.length === 0 ? (
          <div className="flex flex-col items-center gap-2 py-8 text-center">
            <ClipboardList className="h-8 w-8 text-teks-redup" aria-hidden />
            <p className="text-isi text-teks-sekunder">Belum pernah ada hitung fisik di toko ini.</p>
          </div>
        ) : (
          <>
            <ul className="-mx-1 divide-y divide-garis">
              {daftar.map((o) => (
                <li key={o.id}>
                  <BarisSesi o={o} terbuka={buka === o.id} onKlik={() => setBuka(buka === o.id ? null : o.id)} />
                </li>
              ))}
            </ul>
            {q.hasNextPage && (
              <Tombol jenis="kedua" lebarPenuh memuat={q.isFetchingNextPage} onClick={() => q.fetchNextPage()}>
                Muat lebih banyak
              </Tombol>
            )}
          </>
        )}
      </IsiDialog>
    </Dialog>
  )
}

function BarisSesi({ o, terbuka, onKlik }: { o: Opname; terbuka: boolean; onKlik: () => void }) {
  const selesai = o.status === 'posted'
  return (
    <div>
      <button
        type="button"
        onClick={onKlik}
        aria-expanded={terbuka}
        className="flex w-full items-start gap-3 rounded-kontrol px-1 py-3 text-left hover:bg-permukaan-2/60"
      >
        <span className="min-w-0 flex-1">
          <span className="flex flex-wrap items-center gap-2">
            <span className="font-semibold text-teks-utama">{o.item_count} barang dihitung</span>
            {!selesai && <LencanaStatus nada="netral" anak="Tidak selesai" />}
          </span>
          <span className="block text-label text-teks-sekunder">
            {o.changed_count ? `${o.changed_count} berubah` : 'semua cocok dengan catatan'}
          </span>
          <span className="block text-keterangan text-teks-redup">
            {formatTanggalJam(o.counted_at || o.created_at)}
            {o.created_by_name && ` · ${o.created_by_name}`}
          </span>
        </span>
        <span className="flex shrink-0 items-center gap-1">
          <span
            className={cn(
              'font-bold tabular-nums',
              o.value_diff < 0 ? 'text-bahaya-teks' : o.value_diff > 0 ? 'text-hijau-700' : 'text-teks-redup',
            )}
          >
            {o.value_diff ? `≈ ${formatSelisihRupiah(o.value_diff)}` : '—'}
          </span>
          <ChevronDown
            className={cn('h-4 w-4 text-teks-redup transition-transform', terbuka && 'rotate-180')}
            aria-hidden
          />
        </span>
      </button>
      {terbuka && <RincianSesi id={o.id} />}
    </div>
  )
}

function RincianSesi({ id }: { id: string }) {
  const q = useQuery({ queryKey: ['opname', 'rinci', id], queryFn: () => stokApi.opname(id) })
  if (q.isLoading) return <Kerangka className="mb-3 h-16 w-full" />
  if (q.isError || !q.data) return <p className="pb-3 text-label text-bahaya-teks">Rincian belum bisa dimuat.</p>
  // Yang berubah dulu — itulah yang dicari saat membuka riwayat ini.
  const items = [...(q.data.items ?? [])].sort(
    (a, b) => Number(Number.parseFloat(b.diff_qty) !== 0) - Number(Number.parseFloat(a.diff_qty) !== 0),
  )
  return (
    <ul className="mb-3 flex flex-col gap-1.5 rounded-kontrol bg-permukaan-2/60 px-3 py-2.5">
      {items.map((it) => {
        const selisih = Number.parseFloat(it.diff_qty)
        return (
          <li key={it.id} className="flex items-baseline justify-between gap-3 text-label">
            <span className={cn('min-w-0', selisih === 0 ? 'text-teks-sekunder' : 'text-teks-utama')}>
              {it.product_name}
              <span className="block text-keterangan tabular-nums text-teks-redup">
                catatan {formatQty(it.system_qty).replace('-', '−')} → dihitung {formatQty(it.counted_qty)}{' '}
                {it.unit_name}
              </span>
            </span>
            <span
              className={cn(
                'shrink-0 font-semibold tabular-nums',
                selisih < 0 ? 'text-bahaya-teks' : selisih > 0 ? 'text-hijau-700' : 'text-teks-redup',
              )}
            >
              {selisih === 0 ? 'cocok' : `${selisih > 0 ? '+' : '−'}${formatQty(String(Math.abs(selisih)))}`}
            </span>
          </li>
        )
      })}
    </ul>
  )
}
