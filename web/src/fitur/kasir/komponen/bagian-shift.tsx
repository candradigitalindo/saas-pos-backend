import { useLiveQuery } from 'dexie-react-hooks'
import { Check, ChevronRight, RotateCcw, Send, TriangleAlert, X } from 'lucide-react'
import { Tombol } from '@/bersama/ui/tombol'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { IZIN } from '@/lib/izin'
import { db } from '@/lib/offline/db'
import { useSinkron } from '@/lib/offline/mesin'
import { formatRupiah } from '@/bersama/util/uang'
import { formatJam, formatTanggalAkrab, formatTanggalJam, keDate, tanggalISO } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import type { Shift } from '@/bersama/tipe/pos'
import { rincianShift, useGerakanKas } from '../hooks'
import { DaftarCaraBayar } from './daftar-cara-bayar'

/**
 * Potongan yang dipakai bersama Tutup Shift & Ganti Shift: rumus laci,
 * rincian shift (dialog), hasil hitung satu baris, dan penjagaan penjualan
 * yang belum terkirim.
 */

export function TombolRincian({ onKlik, className }: { onKlik: () => void; className?: string }) {
  return (
    <button
      type="button"
      onClick={onKlik}
      className={cn(
        '-mr-2 flex min-h-11 shrink-0 items-center gap-0.5 rounded-kontrol px-2 text-label font-medium text-utama hover:bg-sorot',
        className,
      )}
    >
      Rincian shift
      <ChevronRight className="h-4 w-4" aria-hidden />
    </button>
  )
}

// ── Rincian (dibuka bila diminta) ───────────────────────────────────────────

export function DialogRincianShift({
  shift,
  kasirLama,
  onTutup,
}: {
  shift: Shift
  kasirLama: string
  onTutup: () => void
}) {
  const p = shift.sales
  return (
    <Dialog open onOpenChange={(o) => !o && onTutup()}>
      <IsiDialog
        judul={`Shift ${kasirLama}`}
        keterangan={`Sejak ${formatTanggalJam(shift.opened_at)} · ${lamaSejak(shift.opened_at)}`}
      >
        {p && (
          <section className="flex flex-col gap-3">
            <dl className="grid grid-cols-2 gap-2">
              <div className="rounded-kontrol bg-permukaan-2 p-3">
                <dt className="text-keterangan text-teks-redup">Penjualan</dt>
                <dd className="text-judul-kartu font-bold tabular-nums text-teks-utama">
                  {formatRupiah(p.sales_total)}
                </dd>
              </div>
              <div className="rounded-kontrol bg-permukaan-2 p-3">
                <dt className="text-keterangan text-teks-redup">Transaksi</dt>
                <dd className="text-judul-kartu font-bold tabular-nums text-teks-utama">
                  {p.sales_count}
                </dd>
                {p.sales_count > 0 && (
                  <dd className="text-keterangan tabular-nums text-teks-redup">
                    rata-rata {formatRupiah(p.average_sale)}
                  </dd>
                )}
              </div>
            </dl>
            {p.sales_count > 0 ? (
              <DaftarCaraBayar data={p.by_method} />
            ) : (
              <p className="text-label text-teks-redup">Belum ada penjualan di shift ini.</p>
            )}
            {(p.returns_count > 0 || p.canceled_count > 0) && (
              <p className="flex flex-wrap gap-x-4 gap-y-1 text-label text-teks-sekunder">
                {p.returns_count > 0 && (
                  <span className="inline-flex items-center gap-1.5">
                    <RotateCcw className="h-4 w-4" aria-hidden />
                    Retur {p.returns_count}× · −{formatRupiah(p.returns_total)}
                  </span>
                )}
                {p.canceled_count > 0 && (
                  <span className="inline-flex items-center gap-1.5">
                    <X className="h-4 w-4" aria-hidden />
                    Dibatalkan {p.canceled_count}× · {formatRupiah(p.canceled_total)}
                  </span>
                )}
              </p>
            )}
          </section>
        )}

        <section className="flex flex-col gap-2 border-t border-garis pt-3">
          <h3 className="font-semibold text-teks-utama">Uang yang seharusnya ada di laci</h3>
          <RumusLaci shift={shift} />
          {p?.by_method.some((m) => m.method !== 'cash') && (
            <p className="text-keterangan text-teks-redup">
              Hanya uang tunai yang masuk laci; QRIS, transfer, dan kasbon tidak ikut.
            </p>
          )}
        </section>

        <AksiDialog>
          <Tombol jenis="kedua" onClick={onTutup}>
            Tutup
          </Tombol>
        </AksiDialog>
      </IsiDialog>
    </Dialog>
  )
}

/**
 * Rumus dibuka apa adanya — kasir paham dari mana angkanya, bukan disuruh
 * percaya. Alasan uang masuk/keluar ditulis di bawah angkanya: "keluar
 * Rp 75.000" saja memancing pertanyaan yang tidak bisa dijawab kasir berikutnya.
 */
export function RumusLaci({ shift }: { shift: Shift }) {
  const { boleh } = useSesi()
  const { modalAwal, penjualanTunai, kasMasuk, kasKeluar, seharusnya } = rincianShift(shift)
  const adaGerakan = kasMasuk > 0 || kasKeluar > 0
  const { daftar } = useGerakanKas(adaGerakan && boleh(IZIN.cashMovement) ? shift.id : undefined)
  const alasan = (arah: 'in' | 'out') =>
    (daftar.data?.data ?? [])
      .filter((g) => g.direction === arah)
      .map((g) => g.reason)
      .join(', ')

  return (
    <dl className="flex flex-col gap-2 text-label">
      <BarisRumus label="Modal awal" nilai={formatRupiah(modalAwal)} />
      <BarisRumus label="Penjualan tunai" nilai={formatRupiah(penjualanTunai)} />
      {kasMasuk > 0 && (
        <BarisRumus label="Uang masuk lain" nilai={formatRupiah(kasMasuk)} alasan={alasan('in')} />
      )}
      {kasKeluar > 0 && (
        <BarisRumus
          label="Uang keluar"
          nilai={`−${formatRupiah(kasKeluar)}`}
          alasan={alasan('out')}
        />
      )}
      <div className="mt-1 flex items-baseline justify-between gap-3 border-t border-garis pt-3">
        <dt className="font-semibold text-teks-utama">Seharusnya</dt>
        <dd className="text-judul font-bold tabular-nums text-teks-utama">
          {formatRupiah(seharusnya)}
        </dd>
      </div>
    </dl>
  )
}

function BarisRumus({ label, nilai, alasan }: { label: string; nilai: string; alasan?: string }) {
  return (
    <div>
      <div className="flex items-baseline justify-between gap-3">
        <dt className="text-teks-sekunder">{label}</dt>
        <dd className="shrink-0 tabular-nums text-teks-utama">{nilai}</dd>
      </div>
      {alasan && <p className="line-clamp-2 text-keterangan text-teks-redup">{alasan}</p>}
    </div>
  )
}

// ── Potongan formulir ───────────────────────────────────────────────────────

/**
 * Pas → hijau. Selisih → jingga yang menenangkan, bukan merah yang menuduh.
 * Satu baris di bawah kolom (menggantikan kalimat bantuannya), bukan kotak
 * tiga baris.
 */
export function HasilHitung({ selisih }: { selisih: number }) {
  return (
    <p aria-live="polite" className="-mt-1.5 flex items-start gap-1.5 text-keterangan">
      {selisih === 0 ? (
        <>
          <Check className="h-4 w-4 shrink-0 text-hijau-800" aria-hidden />
          <span className="font-semibold text-hijau-800">Pas — sama dengan yang seharusnya.</span>
        </>
      ) : (
        <>
          <TriangleAlert className="h-4 w-4 shrink-0 text-jingga-700" aria-hidden />
          <span className="text-teks-sekunder">
            <strong className="font-semibold text-jingga-700">
              {selisih > 0 ? 'Lebih' : 'Kurang'} {formatRupiah(Math.abs(selisih))}
            </strong>{' '}
            · selisih kecil itu wajar.
          </span>
        </>
      )}
    </p>
  )
}

export function BarisRingkas({
  label,
  nilai,
  nada,
}: {
  label: string
  nilai: string
  nada?: 'pas' | 'selisih'
}) {
  return (
    <div className="flex items-baseline justify-between gap-3">
      <dt className="text-teks-sekunder">{label}</dt>
      <dd
        className={cn(
          'shrink-0 tabular-nums',
          nada === 'pas'
            ? 'font-semibold text-hijau-800'
            : nada === 'selisih'
              ? 'font-semibold text-jingga-700'
              : 'text-teks-utama',
        )}
      >
        {nilai}
      </dd>
    </div>
  )
}

/** "5 jam 12 menit", "2 hari 3 jam" — sudah berapa lama shift berjalan. */
export function lamaSejak(iso: string, kini = Date.now()): string {
  const menit = Math.max(0, Math.floor((kini - keDate(iso).getTime()) / 60_000))
  const jam = Math.floor(menit / 60)
  if (jam >= 24) return `${Math.floor(jam / 24)} hari ${jam % 24} jam`
  if (jam > 0) return `${jam} jam ${menit % 60} menit`
  return `${menit} menit`
}

export const layarLebar = () =>
  typeof window !== 'undefined' && !!window.matchMedia?.('(min-width: 1024px)').matches

/** "09.22" bila hari ini, "Kemarin 21.10" / "18 Sep 2026 09.22" bila lebih lama. */
export function waktuMulai(shift: Shift): string {
  if (shift.business_date === tanggalISO()) return formatJam(shift.opened_at)
  return `${formatTanggalAkrab(shift.opened_at)} ${formatJam(shift.opened_at)}`
}

/**
 * Penjualan offline yang BELUM terkirim menahan tutup/serah terima shift:
 * server menolak penjualan untuk shift yang sudah ditutup ("shift tidak
 * terbuka"), jadi transaksinya jatuh ke "perlu diperiksa" — dan laci
 * terhitung kurang karena penjualannya belum sampai server.
 */
export function useJualanBelumTerkirim(): number {
  return (
    useLiveQuery(
      () =>
        db.antrean
          .where('status')
          .anyOf('menunggu', 'mengirim')
          .filter((o) => o.op === 'sale.create')
          .count(),
      [],
    ) ?? 0
  )
}

export function PeringatanBelumTerkirim({ jumlah }: { jumlah: number }) {
  const { kirimSekarang } = useSinkron()
  return (
    <div role="alert" className="flex flex-col gap-2 rounded-kontrol border border-jingga-600 bg-permukaan-2 px-3 py-2.5">
      <p className="text-label text-jingga-700">
        <strong className="font-semibold">{jumlah} penjualan belum terkirim.</strong> Kirim dulu supaya laci dihitung
        dari semua penjualan — penjualan yang terkirim setelah shift ditutup akan ditolak.
      </p>
      {/* type="button": peringatan ini tinggal di DALAM formulir tutup/serah
          terima — tanpa itu menekannya ikut mengirim formulir. */}
      <Tombol type="button" jenis="kedua" ukuran="padat" onClick={() => void kirimSekarang()} className="self-start">
        <Send className="h-4 w-4" aria-hidden />
        Kirim Sekarang
      </Tombol>
    </div>
  )
}
