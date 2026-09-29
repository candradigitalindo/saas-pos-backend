import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ChevronDown, HandCoins, PackagePlus, TriangleAlert } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { KeadaanGagal, KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { Kerangka } from '@/bersama/komponen/kerangka'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { IZIN } from '@/lib/izin'
import { GalatAPI } from '@/lib/api-client'
import { formatRupiah } from '@/bersama/util/uang'
import { formatTanggal } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import { stokApi, type Pembelian, type RingkasanUtang } from '../api'
import { statusJatuhTempo, type NadaJatuhTempo } from '../utang'
import { DialogBayarUtang } from '../komponen/dialog-bayar-utang'

const WARNA_TEMPO: Record<NadaJatuhTempo, string> = {
  lewat: 'font-semibold text-bahaya-teks',
  hariIni: 'font-semibold text-jingga-700',
  dekat: 'text-jingga-700',
  nanti: 'text-teks-redup',
  tanpa: 'text-teks-redup',
}

/**
 * Utang pemasok (000047): barang masuk yang belum lunas, dikelompokkan per
 * pemasok — terbesar dulu — dengan jatuh tempo terdekat di tiap kelompok.
 * "Bayar" melunasi sebagian atau sekaligus; sumber uangnya dipilih tiap kali
 * (laci kasir atau uang lain).
 *
 * Utang muncul dari layar Barang Masuk ("Sebagian" / "Belum bayar").
 * Pembelian sebelum fitur ini ada dianggap lunas (migrasi 000047).
 */
export function HalamanUtangPemasok() {
  const { tokoAktif, boleh } = useSesi()
  const navigate = useNavigate()
  const [bayar, setBayar] = useState<Pembelian | null>(null)

  const q = useQuery({
    queryKey: ['utang', tokoAktif, 'ringkasan'],
    queryFn: () => stokApi.ringkasanUtang(tokoAktif!),
    enabled: !!tokoAktif,
  })
  const r = q.data

  return (
    <div className="flex w-full max-w-3xl flex-col gap-4">
      <header className="flex items-center justify-between gap-3">
        <div className="min-w-0">
          <h1 className="text-judul font-bold text-teks-utama">Utang Pemasok</h1>
          <p className="text-label text-teks-sekunder">Barang masuk yang belum lunas. Bayar sebagian atau sekaligus.</p>
        </div>
        {boleh(IZIN.stockAdjust) && (
          <Tombol jenis="kedua" asChild className="shrink-0 px-3 sm:px-5">
            <Link to="/stok/masuk" aria-label="Catat barang masuk">
              <PackagePlus className="h-5 w-5" aria-hidden />
              <span className="hidden sm:inline">Barang Masuk</span>
            </Link>
          </Tombol>
        )}
      </header>

      {q.isLoading ? (
        <Kerangka className="h-64 w-full rounded-kartu" />
      ) : q.isError ? (
        <KeadaanGagal
          pesan={q.error instanceof GalatAPI ? q.error.pesan : 'Utang pemasok belum bisa dimuat.'}
          onCobaLagi={() => q.refetch()}
        />
      ) : !r || r.count === 0 ? (
        <KeadaanKosong
          ikon={HandCoins}
          judul="Tidak ada utang ke pemasok"
          penjelasan='Barang masuk yang dicatat "Sebagian" atau "Belum bayar" muncul di sini sampai lunas.'
          aksi={boleh(IZIN.stockAdjust) ? { label: 'Catat Barang Masuk', onKlik: () => navigate('/stok/masuk') } : undefined}
        />
      ) : (
        <>
          <Kartu className="flex flex-wrap items-end justify-between gap-3 p-4 sm:p-5">
            <div>
              <p className="text-keterangan font-medium text-teks-sekunder">Total utang</p>
              <p className="text-judul font-extrabold tabular-nums text-teks-utama">{formatRupiah(r.outstanding)}</p>
              <p className="text-keterangan text-teks-redup">
                {r.count} nota · {r.suppliers.length} pemasok
              </p>
            </div>
            <div className="flex flex-wrap gap-2">
              {r.overdue_count > 0 && (
                <p className="flex items-center gap-1.5 rounded-full bg-bahaya-teks/10 px-3 py-1.5 text-label font-semibold text-bahaya-teks">
                  <TriangleAlert className="h-4 w-4" aria-hidden />
                  {r.overdue_count} lewat · {formatRupiah(r.overdue_amount)}
                </p>
              )}
              {r.due_soon_count > 0 && (
                <p className="rounded-full bg-permukaan-2 px-3 py-1.5 text-label font-semibold text-jingga-700">
                  {r.due_soon_count} jatuh tempo ≤ {r.due_soon_days} hari · {formatRupiah(r.due_soon_amount)}
                </p>
              )}
            </div>
          </Kartu>

          <ul className="flex flex-col gap-3">
            {r.suppliers.map((s) => (
              <li key={s.supplier_id || '-'}>
                <KartuPemasok s={s} hariIni={r.today} terbukaAwal={r.suppliers.length <= 3} onBayar={setBayar} />
              </li>
            ))}
          </ul>
        </>
      )}

      <DialogBayarUtang pembelian={bayar} onTutup={() => setBayar(null)} />
    </div>
  )
}

function KartuPemasok({
  s,
  hariIni,
  terbukaAwal,
  onBayar,
}: {
  s: RingkasanUtang['suppliers'][number]
  hariIni: string
  terbukaAwal: boolean
  onBayar: (p: Pembelian) => void
}) {
  const { tokoAktif } = useSesi()
  const [buka, setBuka] = useState(terbukaAwal)
  const tempo = statusJatuhTempo(s.nearest_due, hariIni)
  const q = useQuery({
    queryKey: ['utang', tokoAktif, 'nota', s.supplier_id || '-'],
    queryFn: () => stokApi.daftarPembelian(tokoAktif!, 1, 100, { belumLunas: true, pemasok: s.supplier_id || '-' }),
    enabled: !!tokoAktif && buka,
  })

  return (
    <Kartu className="overflow-hidden">
      <button
        type="button"
        onClick={() => setBuka((v) => !v)}
        aria-expanded={buka}
        className="flex w-full items-start gap-3 p-4 text-left hover:bg-permukaan-2/40"
      >
        <span className="min-w-0 flex-1">
          <span className="block truncate text-judul-kartu font-semibold text-teks-utama">
            {s.supplier_name || 'Tanpa pemasok'}
          </span>
          <span className="block text-keterangan text-teks-sekunder">
            {s.count} nota ·{' '}
            {s.overdue_count > 0 ? (
              <span className={WARNA_TEMPO.lewat}>{s.overdue_count} lewat jatuh tempo</span>
            ) : (
              <span className={WARNA_TEMPO[tempo.nada]}>{tempo.nada === 'tanpa' ? 'tanpa jatuh tempo' : tempo.teks}</span>
            )}
          </span>
        </span>
        <span className="flex shrink-0 items-center gap-1.5">
          <span className="text-judul-kartu font-extrabold tabular-nums text-teks-utama">{formatRupiah(s.outstanding)}</span>
          <ChevronDown
            className={cn('h-5 w-5 text-teks-redup transition-transform', buka && 'rotate-180')}
            aria-hidden
          />
        </span>
      </button>

      {buka && (
        <div className="border-t border-garis">
          {q.isLoading ? (
            <Kerangka className="m-4 h-16" />
          ) : (
            <ul className="divide-y divide-garis">
              {(q.data?.data ?? []).map((p) => (
                <BarisNota key={p.id} p={p} hariIni={hariIni} onBayar={() => onBayar(p)} />
              ))}
            </ul>
          )}
        </div>
      )}
    </Kartu>
  )
}

function BarisNota({ p, hariIni, onBayar }: { p: Pembelian; hariIni: string; onBayar: () => void }) {
  const { boleh } = useSesi()
  const tempo = statusJatuhTempo(p.due_date, hariIni)
  const nama = p.item_names ?? []
  const lebih = p.item_count - nama.length
  return (
    <li className="flex flex-wrap items-center gap-x-3 gap-y-2 px-4 py-3">
      <div className="min-w-0 flex-1">
        <p className="text-label font-medium text-teks-utama">
          {formatTanggal(p.occurred_at)}
          {p.invoice_no && <span className="text-teks-redup"> · nota {p.invoice_no}</span>}
        </p>
        <p className="truncate text-keterangan text-teks-sekunder">
          {nama.join(', ')}
          {lebih > 0 && ` +${lebih} lagi`}
        </p>
        <p className={cn('text-keterangan', WARNA_TEMPO[tempo.nada])}>{tempo.teks}</p>
      </div>
      <div className="flex items-center gap-3">
        <p className="text-right tabular-nums">
          <span className="block font-bold text-teks-utama">{formatRupiah(p.outstanding)}</span>
          {p.paid_amount > 0 && (
            <span className="block text-keterangan text-teks-redup">dari {formatRupiah(p.total)}</span>
          )}
        </p>
        {boleh(IZIN.stockAdjust) && (
          <Tombol ukuran="padat" onClick={onBayar}>
            Bayar
          </Tombol>
        )}
      </div>
    </li>
  )
}
