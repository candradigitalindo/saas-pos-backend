import { useState } from 'react'
import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { ChevronDown, PackageOpen } from 'lucide-react'
import { Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { Tombol } from '@/bersama/ui/tombol'
import { KeadaanGagal } from '@/bersama/komponen/keadaan-kosong'
import { Kerangka, KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { GalatAPI } from '@/lib/api-client'
import { formatRupiah } from '@/bersama/util/uang'
import { formatQty, formatQtySatuan } from '@/bersama/util/desimal'
import { formatTanggalJam } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import { stokApi, type Pembelian } from '../api'

const PER_HALAMAN = 20

/**
 * Riwayat barang masuk toko ini, terbaru dulu. Satu baris = satu nota:
 * pemasok, nomor nota, cuplikan barang, total, kapan & siapa yang mencatat.
 * Ketuk untuk membuka rinciannya (jumlah, satuan, harga beli per baris).
 *
 * Status bayar SENGAJA tidak ditampilkan: belum ada layar utang pemasok, dan
 * "belum lunas" yang tidak bisa dilunasi dari mana pun hanya membingungkan.
 */
export function DialogRiwayatMasuk({ terbuka, onTutup }: { terbuka: boolean; onTutup: () => void }) {
  const { tokoAktif } = useSesi()
  const [buka, setBuka] = useState<string | null>(null)

  const q = useInfiniteQuery({
    queryKey: ['pembelian', tokoAktif],
    queryFn: ({ pageParam }) => stokApi.daftarPembelian(tokoAktif!, pageParam, PER_HALAMAN),
    initialPageParam: 1,
    getNextPageParam: (akhir, semua) => (semua.length * PER_HALAMAN < akhir.total ? semua.length + 1 : undefined),
    enabled: terbuka && !!tokoAktif,
  })
  const daftar = q.data?.pages.flatMap((h) => h.data) ?? []
  const total = q.data?.pages[0]?.total ?? 0

  return (
    <Dialog open={terbuka} onOpenChange={(o) => !o && onTutup()}>
      <IsiDialog
        judul="Riwayat barang masuk"
        keterangan={total ? `${total.toLocaleString('id-ID')} nota di toko ini, terbaru dulu.` : undefined}
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
            <PackageOpen className="h-8 w-8 text-teks-redup" aria-hidden />
            <p className="text-isi text-teks-sekunder">Belum ada barang masuk yang dicatat di toko ini.</p>
          </div>
        ) : (
          <>
            <ul className="-mx-1 divide-y divide-garis">
              {daftar.map((p) => (
                <li key={p.id}>
                  <BarisNota p={p} terbuka={buka === p.id} onKlik={() => setBuka(buka === p.id ? null : p.id)} />
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

function BarisNota({ p, terbuka, onKlik }: { p: Pembelian; terbuka: boolean; onKlik: () => void }) {
  const nama = p.item_names ?? []
  const lebih = p.item_count - nama.length
  return (
    <div>
      <button
        type="button"
        onClick={onKlik}
        aria-expanded={terbuka}
        className="flex w-full items-start gap-3 rounded-kontrol px-1 py-3 text-left hover:bg-permukaan-2/60"
      >
        <span className="min-w-0 flex-1">
          <span className="flex flex-wrap items-baseline gap-x-2">
            <span className="font-semibold text-teks-utama">{p.supplier_name || 'Tanpa pemasok'}</span>
            {p.invoice_no && <span className="text-keterangan text-teks-redup">Nota {p.invoice_no}</span>}
          </span>
          <span className="block truncate text-label text-teks-sekunder">
            {nama.join(', ')}
            {lebih > 0 && ` +${lebih} lagi`}
          </span>
          <span className="block text-keterangan text-teks-redup">
            {formatTanggalJam(p.occurred_at)}
            {p.created_by_name && ` · ${p.created_by_name}`}
          </span>
        </span>
        <span className="flex shrink-0 items-center gap-1">
          <span className="text-right">
            <span className="block font-bold tabular-nums text-teks-utama">{formatRupiah(p.total)}</span>
            <span className="block text-keterangan text-teks-redup">{p.item_count} barang</span>
          </span>
          <ChevronDown
            className={cn('h-4 w-4 text-teks-redup transition-transform', terbuka && 'rotate-180')}
            aria-hidden
          />
        </span>
      </button>
      {terbuka && <RincianNota id={p.id} />}
    </div>
  )
}

function RincianNota({ id }: { id: string }) {
  const q = useQuery({ queryKey: ['pembelian', 'rinci', id], queryFn: () => stokApi.pembelian(id) })
  if (q.isLoading) return <Kerangka className="mb-3 h-16 w-full" />
  if (q.isError || !q.data) return <p className="pb-3 text-label text-bahaya-teks">Rincian belum bisa dimuat.</p>
  return (
    <ul className="mb-3 flex flex-col gap-1.5 rounded-kontrol bg-permukaan-2/60 px-3 py-2.5">
      {(q.data.items ?? []).map((it) => {
        const isi = Number.parseFloat(it.unit_conversion ?? '1')
        const kemasan = isi > 1
        return (
          <li key={it.id} className="flex items-baseline justify-between gap-3 text-label">
            <span className="min-w-0 text-teks-utama">
              {it.product_name}
              {it.variant_name && ` (${it.variant_name})`}
              <span className="block text-keterangan text-teks-redup">
                {formatQtySatuan(it.qty, it.unit_name || it.base_unit_name)}
                {kemasan && ` isi ${formatQty(String(isi))}`} @ {formatRupiah(it.unit_cost)}
              </span>
            </span>
            <span className="shrink-0 font-medium tabular-nums text-teks-utama">{formatRupiah(it.line_total)}</span>
          </li>
        )
      })}
    </ul>
  )
}
