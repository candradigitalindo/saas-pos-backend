import { useEffect, useMemo, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  ChevronLeft,
  ChevronRight,
  Clock,
  MessageCircle,
  ReceiptText,
  RotateCcw,
  Search,
  UserRound,
  X,
} from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { Kolom } from '@/bersama/ui/kolom'
import { SegmenPilihan } from '@/bersama/ui/segmen'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { KeadaanKosong, KeadaanGagal } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris, KerangkaKartuAngka } from '@/bersama/komponen/kerangka'
import { KartuSorotan } from '@/bersama/komponen/kartu-sorotan'
import { LencanaTransaksi } from '@/bersama/komponen/lencana-status'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { GalatAPI } from '@/lib/api-client'
import { IZIN } from '@/lib/izin'
import { formatRupiah } from '@/bersama/util/uang'
import { formatQty } from '@/bersama/util/desimal'
import { formatJam, formatTanggalPanjang, geserTanggal, tanggalISO } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import type { RingkasanPenjualan, Transaksi } from '@/bersama/tipe/pos'
import { kasirApi } from '../api'
import { useRingkasanHari, useRiwayatBertahap } from '../hooks'
import { NAMA_JENIS_PESANAN, ikonMetode, namaMetode } from '../label-transaksi'
import { DaftarCaraBayar } from '../komponen/daftar-cara-bayar'
import { DialogKirimWA } from '../komponen/dialog-kirim-wa'

const POLA_TANGGAL = /^\d{4}-\d{2}-\d{2}$/

type Saring = 'semua' | 'cash' | 'qris' | 'credit' | 'canceled' | 'returned'
const SARINGAN: [Saring, string][] = [
  ['semua', 'Semua'],
  ['cash', 'Tunai'],
  ['qris', 'QRIS'],
  ['credit', 'Kasbon'],
  ['canceled', 'Dibatalkan'],
  ['returned', 'Retur'],
]

/**
 * Riwayat penjualan per hari.
 *
 * Dulu hanya daftar nomor nota, jam, dan total — isi belanja, cara bayar,
 * pelanggan, dan kasirnya tidak terlihat, hari ramai terpotong di 20
 * transaksi tanpa tanda, dan tidak ada angka ringkasan sama sekali. Pemilik
 * yang menelusuri "yang bayar QRIS tadi siang" harus menebak dari jamnya.
 *
 * Kini: ringkasan hari (uang masuk bersih, per cara bayar, retur & batal),
 * pencarian nomor nota, saringan cara bayar/status, daftar yang menyebut isi
 * belanjanya, dan detail nota lengkap dengan aksi batal/retur.
 *
 * Tanggalnya ikut di alamat (`?tanggal=2026-09-18`): tautan dari "Transaksi
 * terakhir" di Beranda membuka hari nota itu, dan memuat ulang halaman tidak
 * melempar kembali ke hari ini. `&cari=NOTA` mengisi kotak cari.
 */
export function HalamanRiwayat() {
  const { tokoAktif, boleh } = useSesi()
  const [params, setParams] = useSearchParams()
  const hariIni = tanggalISO()
  const kemarin = geserTanggal(hariIni, -1)
  const dariAlamat = params.get('tanggal')
  const tanggal = dariAlamat && POLA_TANGGAL.test(dariAlamat) ? dariAlamat : hariIni
  const setTanggal = (t: string) => {
    if (!POLA_TANGGAL.test(t) || t > hariIni) return
    setParams(t === hariIni ? {} : { tanggal: t }, { replace: true })
  }

  // `?cari=` membuka langsung satu nota (tautan dari halaman pelanggan).
  const [ketikCari, setKetikCari] = useState(() => params.get('cari') ?? '')
  const [cari, setCari] = useState(ketikCari)
  useEffect(() => {
    const w = setTimeout(() => setCari(ketikCari.trim()), 300)
    return () => clearTimeout(w)
  }, [ketikCari])
  const [saring, setSaring] = useState<Saring>('semua')
  const [dipilih, setDipilih] = useState<Transaksi | null>(null)
  const [dibatalkan, setDibatalkan] = useState<Transaksi | null>(null)
  const [diretur, setDiretur] = useState<Transaksi | null>(null)

  const metode = saring === 'cash' || saring === 'qris' || saring === 'credit' ? saring : undefined
  const status = saring === 'canceled' || saring === 'returned' ? saring : undefined
  const q = useRiwayatBertahap({
    outlet_id: tokoAktif,
    business_date: tanggal,
    search: cari || undefined,
    method: metode,
    status,
  })
  const ringkasan = useRingkasanHari(tokoAktif, tanggal)
  const pembanding = useRingkasanHari(tokoAktif, geserTanggal(tanggal, -1))

  const baris = useMemo(() => q.data?.pages.flatMap((h) => h.data) ?? [], [q.data])
  const jumlahSemua = q.data?.pages[0]?.total ?? 0
  const adaSaringan = !!cari || saring !== 'semua'

  return (
    <div className="flex flex-col gap-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div className="min-w-0">
          <h1 className="text-judul font-bold text-teks-utama">Riwayat Penjualan</h1>
          <p className="text-label text-teks-sekunder">
            {tanggal === hariIni
              ? `Hari ini · ${formatTanggalPanjang(tanggal)}`
              : formatTanggalPanjang(tanggal)}
          </p>
        </div>
        <PemilihTanggal tanggal={tanggal} hariIni={hariIni} kemarin={kemarin} onPilih={setTanggal} />
      </header>

      <RingkasanHari
        data={ringkasan.data}
        pembanding={pembanding.data}
        memuat={ringkasan.isLoading}
        hariIni={tanggal === hariIni}
      />

      {/* Cari & saring. Kotak cari mengambil sisa lebar; saringan menggeser
          mendatar di HP (pola SegmenPilihan). */}
      <div className="flex flex-col gap-3 lg:flex-row lg:items-center">
        <div className="relative lg:w-72">
          <Search
            className="pointer-events-none absolute left-3 top-1/2 h-5 w-5 -translate-y-1/2 text-teks-redup"
            aria-hidden
          />
          <input
            type="search"
            value={ketikCari}
            onChange={(e) => setKetikCari(e.target.value)}
            placeholder="Cari nomor nota…"
            aria-label="Cari nomor nota"
            className="h-12 w-full rounded-kontrol border border-garis bg-permukaan pl-10 pr-3 text-isi text-teks-utama placeholder:text-teks-redup focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-utama"
          />
        </div>
        <SegmenPilihan label="Saring transaksi" nilai={saring} onPilih={setSaring} pilihan={SARINGAN} />
      </div>

      {q.isLoading ? (
        <KerangkaBaris jumlah={5} />
      ) : q.isError ? (
        <KeadaanGagal
          pesan={q.error instanceof GalatAPI ? q.error.pesan : 'Daftar transaksi belum bisa dimuat.'}
          onCobaLagi={() => q.refetch()}
        />
      ) : baris.length === 0 ? (
        adaSaringan ? (
          <KeadaanKosong
            ikon={Search}
            judul="Tidak ada transaksi yang cocok"
            penjelasan={
              cari
                ? `Tidak ada nota bernomor "${cari}" pada tanggal ini dengan saringan yang dipilih.`
                : 'Tidak ada transaksi dengan saringan ini pada tanggal ini.'
            }
            aksi={{
              label: 'Hapus Saringan',
              onKlik: () => {
                setKetikCari('')
                setCari('')
                setSaring('semua')
              },
            }}
          />
        ) : (
          <KosongHariIni
            hariIni={tanggal === hariIni}
            bolehJual={boleh(IZIN.saleCreate)}
            onKemarin={() => setTanggal(geserTanggal(tanggal, -1))}
          />
        )
      ) : (
        <section className="flex flex-col gap-2" aria-label="Daftar transaksi">
          <p className="text-keterangan text-teks-redup">
            Menampilkan {baris.length} dari {jumlahSemua} transaksi
            {adaSaringan ? ' yang cocok' : ''} · ketuk untuk melihat rinciannya
          </p>

          {/* HP & tablet: kartu. Layar lebar: tabel dengan kolom tetap. */}
          <ul className="flex flex-col gap-2 lg:hidden">
            {baris.map((t) => (
              <li key={t.id}>
                <KartuTransaksi transaksi={t} onBuka={() => setDipilih(t)} />
              </li>
            ))}
          </ul>
          <TabelTransaksi baris={baris} onBuka={setDipilih} />

          {q.hasNextPage && (
            <Tombol
              jenis="kedua"
              className="self-center"
              memuat={q.isFetchingNextPage}
              labelMemuat="Memuat…"
              onClick={() => q.fetchNextPage()}
            >
              Muat {Math.min(30, jumlahSemua - baris.length)} transaksi lagi
            </Tombol>
          )}
        </section>
      )}

      {dipilih && (
        <DialogDetail
          transaksi={dipilih}
          bolehBatalkan={boleh(IZIN.saleVoid)}
          bolehRetur={boleh(IZIN.saleRefund)}
          onTutup={() => setDipilih(null)}
          onBatalkan={() => {
            setDibatalkan(dipilih)
            setDipilih(null)
          }}
          onRetur={() => {
            setDiretur(dipilih)
            setDipilih(null)
          }}
        />
      )}
      {dibatalkan && <DialogBatalkan transaksi={dibatalkan} onTutup={() => setDibatalkan(null)} />}
      {diretur && <DialogRetur transaksi={diretur} onTutup={() => setDiretur(null)} />}
    </div>
  )
}

// ── Kepala: tanggal ─────────────────────────────────────────────────────────

function PemilihTanggal({
  tanggal,
  hariIni,
  kemarin,
  onPilih,
}: {
  tanggal: string
  hariIni: string
  kemarin: string
  onPilih: (t: string) => void
}) {
  const pintas = (t: string, label: string) => (
    <button
      type="button"
      aria-pressed={tanggal === t}
      onClick={() => onPilih(t)}
      className={cn(
        'h-12 shrink-0 rounded-full px-4 text-label font-semibold pointer-fine:h-10',
        tanggal === t
          ? 'bg-sorot text-utama'
          : 'text-teks-sekunder hover:bg-permukaan-2 hover:text-teks-utama',
      )}
    >
      {label}
    </button>
  )
  return (
    <div className="flex flex-wrap items-center gap-2">
      <div className="flex items-center gap-1 rounded-full bg-permukaan-2 p-1">
        {pintas(hariIni, 'Hari ini')}
        {pintas(kemarin, 'Kemarin')}
      </div>
      <div className="flex items-center gap-1.5">
        <Tombol
          jenis="kedua"
          ukuran="ikon"
          aria-label="Hari sebelumnya"
          onClick={() => onPilih(geserTanggal(tanggal, -1))}
        >
          <ChevronLeft className="h-5 w-5" aria-hidden />
        </Tombol>
        <input
          type="date"
          aria-label="Tanggal"
          value={tanggal}
          max={hariIni}
          onChange={(e) => onPilih(e.target.value)}
          className="h-12 rounded-kontrol border border-garis bg-permukaan px-3 text-isi tabular-nums text-teks-utama focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-utama"
        />
        <Tombol
          jenis="kedua"
          ukuran="ikon"
          aria-label="Hari berikutnya"
          disabled={tanggal >= hariIni}
          onClick={() => onPilih(geserTanggal(tanggal, 1))}
        >
          <ChevronRight className="h-5 w-5" aria-hidden />
        </Tombol>
      </div>
    </div>
  )
}

// ── Ringkasan hari ──────────────────────────────────────────────────────────

/**
 * Tiga kartu: uang masuk bersih (dibanding hari sebelumnya), uang masuk per
 * cara bayar, serta retur & batal — yang terakhir disebut TERBUKA, bukan
 * diam-diam dikurangkan dari angka bersih.
 */
function RingkasanHari({
  data,
  pembanding,
  memuat,
  hariIni,
}: {
  data?: RingkasanPenjualan
  pembanding?: RingkasanPenjualan
  memuat: boolean
  hariIni: boolean
}) {
  if (memuat) {
    return (
      <div className="grid gap-3 lg:grid-cols-3">
        <KerangkaKartuAngka />
        <KerangkaKartuAngka />
        <KerangkaKartuAngka />
      </div>
    )
  }
  if (!data) return null
  // Retur & batal hanya ditampilkan bila ada: dua baris "0×" hanya memanjangkan
  // layar HP sebelum daftar transaksinya muncul.
  const adaKoreksi = data.returns_count + data.canceled_count > 0

  return (
    <div className="grid gap-3 lg:grid-cols-3">
      <KartuSorotan
        label={hariIni ? 'Uang masuk bersih hari ini' : 'Uang masuk bersih'}
        nilai={data.net_total}
        pembanding={pembanding?.net_total}
        labelPembanding="dibanding hari sebelumnya"
        rincian={[
          { label: 'Transaksi', nilai: String(data.sales_count) },
          { label: 'Rata-rata', nilai: formatRupiah(data.average_sale) },
        ]}
      />

      <Kartu className={cn('flex flex-col gap-3 p-5', !adaKoreksi && 'lg:col-span-2')}>
        <p className="text-label font-semibold text-teks-utama">Per cara bayar</p>
        <DaftarCaraBayar data={data.by_method} />
      </Kartu>

      {adaKoreksi && (
        <Kartu className="flex flex-col gap-3 p-5">
          <p className="text-label font-semibold text-teks-utama">Retur & batal</p>
          <dl className="flex flex-col gap-2 text-label">
            <div className="flex items-center justify-between gap-3">
              <dt className="flex items-center gap-2 text-teks-sekunder">
                <RotateCcw className="h-4 w-4 shrink-0" aria-hidden />
                Retur · {data.returns_count}×
              </dt>
              <dd className="font-semibold tabular-nums text-teks-utama">
                {data.returns_total > 0 ? `−${formatRupiah(data.returns_total)}` : formatRupiah(0)}
              </dd>
            </div>
            <div className="flex items-center justify-between gap-3">
              <dt className="flex items-center gap-2 text-teks-sekunder">
                <X className="h-4 w-4 shrink-0" aria-hidden />
                Dibatalkan · {data.canceled_count}×
              </dt>
              <dd className="font-semibold tabular-nums text-teks-utama">
                {formatRupiah(data.canceled_total)}
              </dd>
            </div>
          </dl>
          <p className="mt-auto text-keterangan text-teks-redup">
            Uang masuk bersih = penjualan {formatRupiah(data.sales_total)} dikurangi retur. Transaksi
            yang dibatalkan tidak dihitung.
          </p>
        </Kartu>
      )}
    </div>
  )
}

function KosongHariIni({
  hariIni,
  bolehJual,
  onKemarin,
}: {
  hariIni: boolean
  bolehJual: boolean
  onKemarin: () => void
}) {
  return (
    <div className="flex flex-col items-center gap-3 rounded-kartu border border-dashed border-garis px-6 py-12 text-center">
      <ReceiptText className="h-10 w-10 text-teks-redup" aria-hidden />
      <p className="text-judul-kartu font-semibold text-teks-utama">
        {hariIni ? 'Belum ada transaksi hari ini' : 'Tidak ada penjualan pada tanggal ini'}
      </p>
      <p className="max-w-sm text-isi text-teks-sekunder">
        {hariIni
          ? 'Transaksi dari layar Kasir muncul di sini begitu terjual — lengkap dengan isi belanja dan cara bayarnya.'
          : 'Coba hari sebelumnya, atau pilih tanggal lain di kalender.'}
      </p>
      <div className="flex flex-wrap justify-center gap-2">
        {hariIni && bolehJual && (
          <Tombol asChild>
            <Link to="/kasir">Buka Kasir</Link>
          </Tombol>
        )}
        <Tombol jenis="kedua" onClick={onKemarin}>
          <ChevronLeft className="h-5 w-5" aria-hidden />
          Hari sebelumnya
        </Tombol>
      </div>
    </div>
  )
}

// ── Daftar ──────────────────────────────────────────────────────────────────

/** "2× Kopi Hitam, 1× Gula Pasir +2 lainnya". */
function ringkasBarang(t: Transaksi): string {
  const items = t.items ?? []
  if (items.length === 0) return '—'
  const tampil = items.slice(0, 2).map((i) => `${formatQty(i.qty.replace('-', ''))}× ${i.product_name}`)
  return items.length > 2 ? `${tampil.join(', ')} +${items.length - 2} lainnya` : tampil.join(', ')
}

function CaraBayar({ transaksi }: { transaksi: Transaksi }) {
  const metode = [...new Set((transaksi.payments ?? []).map((p) => p.method))]
  if (metode.length === 0) return <span className="text-teks-redup">—</span>
  return (
    <span className="inline-flex flex-wrap items-center gap-1.5">
      {metode.map((m) => {
        const Ikon = ikonMetode(m)
        return (
          <span
            key={m}
            className="inline-flex items-center gap-1 rounded-full bg-permukaan-2 px-2 py-0.5 text-keterangan font-medium text-teks-sekunder"
          >
            <Ikon className="h-3.5 w-3.5" aria-hidden />
            {namaMetode(m)}
          </span>
        )
      })}
    </span>
  )
}

/**
 * Status untuk lencana. Penjualan KASBON berstatus "completed" di server, tapi
 * lencana "Lunas" di sebelahnya bohong — itu utang. Ditampilkan "Kasbon".
 * Begitu juga penjualan yang sudah diretur: statusnya tetap "completed"
 * (returnya baris sendiri), tapi uangnya sudah kembali ke pembeli.
 */
const statusTampil = (t: Transaksi) => {
  if (t.voided_at) return 'void'
  if (t.returned_by_receipt_no) return 'refunded'
  if (t.status === 'completed' && (t.payments ?? []).some((p) => p.method === 'credit')) return 'credit'
  return t.status
}

/** Retur dan penjualan asalnya saling menyebut nomor nota. */
function TautanRetur({ transaksi: t }: { transaksi: Transaksi }) {
  const nota = t.return_of_receipt_no ?? t.returned_by_receipt_no
  if (!nota) return null
  return (
    <p className="mt-1 inline-flex items-center gap-1 text-keterangan text-teks-redup">
      <RotateCcw className="h-3.5 w-3.5 shrink-0" aria-hidden />
      {t.return_of_receipt_no ? 'Retur nota' : 'Diretur lewat nota'}{' '}
      <span className="tabular-nums">{nota}</span>
    </p>
  )
}

function KartuTransaksi({ transaksi: t, onBuka }: { transaksi: Transaksi; onBuka: () => void }) {
  const redup = t.status === 'canceled' || !!t.voided_at
  return (
    <button
      type="button"
      onClick={onBuka}
      className="w-full rounded-kartu border border-garis bg-permukaan p-4 text-left shadow-kartu transition-colors hover:border-utama/40 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-utama"
    >
      <div className="flex items-start justify-between gap-3">
        {/* Nomor nota UTUH di baris sendiri: ujungnya ("-0009") yang membedakan
            satu nota dari yang lain — dulu justru bagian itu yang terpotong. */}
        <div className="min-w-0">
          <p className="flex items-center gap-2 font-semibold text-teks-utama">
            <Clock className="h-4 w-4 shrink-0 text-teks-redup" aria-hidden />
            {formatJam(t.occurred_at)}
          </p>
          <p className="text-keterangan tabular-nums text-teks-redup">{t.receipt_no}</p>
        </div>
        <p
          className={cn(
            'shrink-0 text-judul-kartu font-bold tabular-nums',
            redup ? 'text-teks-redup line-through' : 'text-teks-utama',
          )}
        >
          {formatRupiah(t.total)}
        </p>
      </div>
      <p className="mt-1.5 line-clamp-2 text-label text-teks-sekunder">{ringkasBarang(t)}</p>
      <TautanRetur transaksi={t} />
      <div className="mt-2.5 flex flex-wrap items-center gap-x-3 gap-y-1.5 text-keterangan text-teks-redup">
        {/* Baris retur tidak punya pembayaran — tanpa tanda "—" yang yatim. */}
        {(t.payments ?? []).length > 0 && <CaraBayar transaksi={t} />}
        {t.customer_name && (
          <span className="inline-flex items-center gap-1">
            <UserRound className="h-3.5 w-3.5" aria-hidden />
            {t.customer_name}
          </span>
        )}
        {t.cashier_name && <span>Kasir {t.cashier_name}</span>}
        {statusTampil(t) !== 'completed' && statusTampil(t) !== 'credit' && (
          <LencanaTransaksi status={statusTampil(t)} />
        )}
      </div>
    </button>
  )
}

function TabelTransaksi({ baris, onBuka }: { baris: Transaksi[]; onBuka: (t: Transaksi) => void }) {
  return (
    <Kartu className="hidden overflow-hidden lg:block">
      <table className="w-full text-label">
        <thead className="border-b border-garis bg-permukaan-2 text-left text-keterangan font-semibold uppercase tracking-wide text-teks-redup">
          <tr>
            <th className="px-4 py-3 font-semibold">Waktu</th>
            <th className="px-4 py-3 font-semibold">Barang</th>
            <th className="px-4 py-3 font-semibold">Cara bayar</th>
            <th className="px-4 py-3 font-semibold">Pelanggan · Kasir</th>
            <th className="px-4 py-3 font-semibold">Status</th>
            <th className="px-4 py-3 text-right font-semibold">Total</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-garis">
          {baris.map((t) => {
            const redup = t.status === 'canceled' || !!t.voided_at
            return (
              <tr
                key={t.id}
                tabIndex={0}
                onClick={() => onBuka(t)}
                onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), onBuka(t))}
                aria-label={`Nota ${t.receipt_no}, ${formatRupiah(t.total)} — lihat rincian`}
                className="cursor-pointer align-top transition-colors hover:bg-permukaan-2 focus-visible:bg-sorot focus-visible:outline-none"
              >
                <td className="whitespace-nowrap px-4 py-3">
                  <p className="font-semibold text-teks-utama">{formatJam(t.occurred_at)}</p>
                  <p className="text-keterangan tabular-nums text-teks-redup">{t.receipt_no}</p>
                </td>
                <td className="max-w-80 px-4 py-3 text-teks-sekunder">
                  <p className="line-clamp-2">{ringkasBarang(t)}</p>
                  <TautanRetur transaksi={t} />
                </td>
                <td className="px-4 py-3">
                  <CaraBayar transaksi={t} />
                </td>
                <td className="px-4 py-3 text-teks-sekunder">
                  <p className="text-teks-utama">{t.customer_name ?? '—'}</p>
                  {t.cashier_name && <p className="text-keterangan text-teks-redup">{t.cashier_name}</p>}
                </td>
                <td className="px-4 py-3">
                  <LencanaTransaksi status={statusTampil(t)} />
                </td>
                <td
                  className={cn(
                    'whitespace-nowrap px-4 py-3 text-right font-bold tabular-nums',
                    redup ? 'text-teks-redup line-through' : 'text-teks-utama',
                  )}
                >
                  {formatRupiah(t.total)}
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </Kartu>
  )
}

// ── Detail nota ─────────────────────────────────────────────────────────────

/** Catatan retur dari server berbentuk "Retur <nota asal> <alasan>" — ambil alasannya. */
const alasanRetur = (t: Transaksi) => {
  const catatan = (t.note ?? '').trim()
  const awalan = t.return_of_receipt_no ? `Retur ${t.return_of_receipt_no}` : ''
  return awalan && catatan.startsWith(awalan) ? catatan.slice(awalan.length).trim() : catatan
}

/**
 * Rincian satu nota — semua angka dari server, tidak dihitung ulang. Aksi:
 * BATALKAN (salah input, hari yang sama) dan RETUR (barang dikembalikan
 * pembeli). Dulu retur tidak punya tombol di mana pun walau API-nya ada.
 */
function DialogDetail({
  transaksi: t,
  bolehBatalkan,
  bolehRetur,
  onTutup,
  onBatalkan,
  onRetur,
}: {
  transaksi: Transaksi
  bolehBatalkan: boolean
  bolehRetur: boolean
  onTutup: () => void
  onBatalkan: () => void
  onRetur: () => void
}) {
  // Penjualan yang sudah diretur tidak bisa diretur/dibatalkan lagi (server
  // menolak) — tombolnya tidak ditawarkan.
  const selesai = t.status === 'completed' && !t.voided_at && !t.returned_by_receipt_no
  // Struk yang dibatalkan tidak ditawarkan untuk dikirim; yang sudah diretur
  // tetap boleh (halaman struk digital tidak menyembunyikan statusnya).
  const bisaDikirim = t.status === 'completed' && !t.voided_at
  const [kirimWA, setKirimWA] = useState(false)
  const baris = (label: string, nilai: number, tebal = false) =>
    nilai !== 0 || tebal ? (
      <div className={cn('flex items-baseline justify-between gap-3', tebal && 'font-bold text-teks-utama')}>
        <dt className={tebal ? '' : 'text-teks-sekunder'}>{label}</dt>
        <dd className="tabular-nums">{formatRupiah(nilai)}</dd>
      </div>
    ) : null

  return (
    <Dialog open onOpenChange={(o) => !o && onTutup()}>
      <IsiDialog judul={`Nota ${t.receipt_no}`}>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-label text-teks-sekunder">
          <span>
            {formatTanggalPanjang(t.business_date)} · {formatJam(t.occurred_at)}
          </span>
          <LencanaTransaksi status={statusTampil(t)} />
        </div>

        <dl className="grid grid-cols-2 gap-x-4 gap-y-2 rounded-kontrol bg-permukaan-2 p-3 text-label">
          <div>
            <dt className="text-keterangan text-teks-redup">Kasir</dt>
            <dd className="text-teks-utama">{t.cashier_name ?? '—'}</dd>
          </div>
          <div>
            <dt className="text-keterangan text-teks-redup">Pelanggan</dt>
            <dd className="text-teks-utama">{t.customer_name ?? 'Umum'}</dd>
          </div>
          <div>
            <dt className="text-keterangan text-teks-redup">Jenis pesanan</dt>
            <dd className="text-teks-utama">{NAMA_JENIS_PESANAN[t.order_type] ?? t.order_type}</dd>
          </div>
          <div>
            <dt className="text-keterangan text-teks-redup">Cara bayar</dt>
            <dd>
              <CaraBayar transaksi={t} />
            </dd>
          </div>
        </dl>

        {(t.voided_at || t.void_reason) && (
          <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
            Dibatalkan{t.void_reason ? `: ${t.void_reason}` : ''}. Tidak dihitung sebagai penjualan.
          </p>
        )}
        {t.status === 'returned' && (
          <p className="rounded-kontrol bg-permukaan-2 px-3 py-2 text-label text-teks-sekunder">
            Retur
            {t.return_of_receipt_no && (
              <>
                {' '}atas nota <strong className="tabular-nums text-teks-utama">{t.return_of_receipt_no}</strong>
              </>
            )}
            : uang {formatRupiah(Math.abs(t.total))} dikembalikan ke pembeli dan stoknya masuk lagi.
            {alasanRetur(t) && <span className="mt-1 block">Alasan: {alasanRetur(t)}</span>}
          </p>
        )}
        {t.returned_by_receipt_no && (
          <p className="rounded-kontrol border border-garis bg-permukaan-2 px-3 py-2 text-label text-teks-sekunder">
            Sudah diretur lewat nota{' '}
            <strong className="tabular-nums text-teks-utama">{t.returned_by_receipt_no}</strong>. Uangnya
            sudah dikembalikan, jadi transaksi ini tidak bisa diretur atau dibatalkan lagi.
          </p>
        )}

        <ul className="flex flex-col divide-y divide-garis">
          {(t.items ?? []).map((i) => (
            <li key={i.id} className="flex items-start justify-between gap-3 py-2.5">
              <div className="min-w-0">
                <p className="font-medium text-teks-utama">{i.product_name}</p>
                <p className="text-keterangan tabular-nums text-teks-redup">
                  {formatQty(i.qty)} {i.unit_name} × {formatRupiah(i.unit_price)}
                  {i.discount_amount > 0 && ` − diskon ${formatRupiah(i.discount_amount)}`}
                </p>
                {i.note && <p className="break-words text-keterangan italic text-teks-sekunder">“{i.note}”</p>}
              </div>
              <p className="shrink-0 font-semibold tabular-nums text-teks-utama">{formatRupiah(i.line_total)}</p>
            </li>
          ))}
        </ul>

        <dl className="flex flex-col gap-1.5 border-t border-garis pt-3 text-label">
          {baris('Subtotal', t.subtotal)}
          {baris('Diskon', -t.discount_amount)}
          {baris('Pajak', t.tax_amount)}
          {baris('Biaya layanan', t.service_amount)}
          {baris('Pembulatan', t.rounding_amount)}
          {baris('Total', t.total, true)}
          {(t.payments ?? []).map((p) => (
            <div key={p.id} className="flex items-baseline justify-between gap-3">
              <dt className="text-teks-sekunder">Dibayar {namaMetode(p.method).toLowerCase()}</dt>
              <dd className="tabular-nums text-teks-sekunder">{formatRupiah(p.amount)}</dd>
            </div>
          ))}
          {t.change_amount > 0 && baris('Kembalian', t.change_amount)}
        </dl>

        {(selesai || bisaDikirim) && (
          <AksiDialog>
            {/* Kirim ulang struk — pembeli yang minta belakangan, atau yang
                lupa dikirimi saat bayar. */}
            {bisaDikirim && (
              <Tombol jenis="kedua" onClick={() => setKirimWA(true)}>
                <MessageCircle className="h-5 w-5" aria-hidden />
                Kirim WhatsApp
              </Tombol>
            )}
            {selesai && bolehRetur && (
              <Tombol jenis="kedua" onClick={onRetur}>
                <RotateCcw className="h-5 w-5" aria-hidden />
                Retur
              </Tombol>
            )}
            {selesai && bolehBatalkan && (
              <Tombol jenis="kedua" onClick={onBatalkan}>
                Batalkan
              </Tombol>
            )}
          </AksiDialog>
        )}
      </IsiDialog>
      {kirimWA && <DialogKirimWA transaksi={t} onTutup={() => setKirimWA(false)} />}
    </Dialog>
  )
}

/**
 * Pembatalan adalah aksi berat dan tidak bisa dibalik, jadi dialognya
 * MENJELASKAN AKIBATNYA DENGAN ANGKA — bukan sekadar bertanya "Anda yakin?".
 */
function DialogBatalkan({ transaksi, onTutup }: { transaksi: Transaksi; onTutup: () => void }) {
  const qc = useQueryClient()
  const toast = useToast()
  const [alasan, setAlasan] = useState('')
  const [galat, setGalat] = useState<string | null>(null)

  const batal = useMutation({
    mutationFn: () => kasirApi.batalkan(transaksi.id, alasan.trim()),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['riwayat-transaksi'] })
      qc.invalidateQueries({ queryKey: ['shift-aktif'] })
      qc.invalidateQueries({ queryKey: ['stok-kasir'] })
      toast.berhasil(`Transaksi ${transaksi.receipt_no} dibatalkan.`)
      onTutup()
    },
    // 409 punya beberapa sebab (sudah dibatalkan, sudah diretur, kasbonnya
    // sudah dicicil) — kalimat dari server yang menyebut sebab sebenarnya.
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan. Coba lagi.'),
  })

  return (
    <Dialog open onOpenChange={(o) => !o && !batal.isPending && onTutup()}>
      <IsiDialog judul={`Batalkan transaksi ${transaksi.receipt_no}?`}>
        <div className="flex flex-col gap-2 text-isi text-teks-sekunder">
          <p className="text-label">
            Untuk transaksi yang <strong className="text-teks-utama">salah dicatat</strong>. Bila
            pembeli mengembalikan barangnya, pakai Retur.
          </p>
          {transaksi.items && transaksi.items.length > 0 && (
            <p>
              Stok{' '}
              {transaksi.items
                .map((i) => `${formatQty(i.qty)} ${i.unit_name} ${i.product_name}`)
                .join(', ')}{' '}
              akan dikembalikan.
            </p>
          )}
          <p>
            Omzet hari itu turun{' '}
            <strong className="tabular-nums text-teks-utama">{formatRupiah(transaksi.total)}</strong>.
            Transaksinya tetap tercatat di riwayat sebagai &ldquo;dibatalkan&rdquo;.
          </p>
        </div>

        <Kolom
          label="Alasan pembatalan"
          placeholder="Contoh: salah pilih barang"
          value={alasan}
          onChange={(e) => setAlasan(e.target.value)}
          bantuan="Minimal 3 huruf. Dicatat supaya bisa ditelusuri nanti."
          required
        />

        {galat && (
          <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
            {galat}
          </p>
        )}

        <AksiDialog>
          <Tombol
            jenis="bahaya"
            memuat={batal.isPending}
            labelMemuat="Membatalkan…"
            disabled={alasan.trim().length < 3}
            onClick={() => batal.mutate()}
          >
            Ya, Batalkan Transaksi
          </Tombol>
          <Tombol jenis="kedua" onClick={onTutup} disabled={batal.isPending}>
            Jangan jadi
          </Tombol>
        </AksiDialog>
      </IsiDialog>
    </Dialog>
  )
}

/**
 * Retur penuh: barang kembali ke stok, uangnya dikembalikan ke pembeli, dan
 * tercatat sebagai baris retur bernilai negatif (nota aslinya tetap ada).
 */
function DialogRetur({ transaksi, onTutup }: { transaksi: Transaksi; onTutup: () => void }) {
  const qc = useQueryClient()
  const toast = useToast()
  const [alasan, setAlasan] = useState('')
  const [galat, setGalat] = useState<string | null>(null)

  const retur = useMutation({
    mutationFn: () => kasirApi.retur(transaksi.id, alasan.trim() || undefined),
    onSuccess: (r) => {
      qc.invalidateQueries({ queryKey: ['riwayat-transaksi'] })
      qc.invalidateQueries({ queryKey: ['shift-aktif'] })
      qc.invalidateQueries({ queryKey: ['stok-kasir'] })
      toast.berhasil(`Retur ${transaksi.receipt_no} tercatat (${r.receipt_no}).`)
      onTutup()
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan. Coba lagi.'),
  })

  return (
    <Dialog open onOpenChange={(o) => !o && !retur.isPending && onTutup()}>
      <IsiDialog judul={`Retur nota ${transaksi.receipt_no}?`}>
        <div className="flex flex-col gap-2 text-isi text-teks-sekunder">
          <p>
            Kembalikan{' '}
            <strong className="tabular-nums text-teks-utama">{formatRupiah(transaksi.total)}</strong> ke
            pembeli. Semua barangnya kembali ke stok.
          </p>
          <p className="text-label">
            Nota ini tetap ada; retur dicatat sebagai nota baru bernilai minus, jadi laporan
            menunjukkan keduanya.
          </p>
        </div>
        <Kolom
          label="Alasan retur (boleh kosong)"
          placeholder="Contoh: barang rusak"
          value={alasan}
          onChange={(e) => setAlasan(e.target.value)}
        />
        {galat && (
          <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
            {galat}
          </p>
        )}
        <AksiDialog>
          <Tombol memuat={retur.isPending} labelMemuat="Memproses…" onClick={() => retur.mutate()}>
            <RotateCcw className="h-5 w-5" aria-hidden />
            Retur {formatRupiah(transaksi.total)}
          </Tombol>
          <Tombol jenis="kedua" onClick={onTutup} disabled={retur.isPending}>
            Jangan jadi
          </Tombol>
        </AksiDialog>
      </IsiDialog>
    </Dialog>
  )
}
