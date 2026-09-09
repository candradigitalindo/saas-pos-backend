import { lazy, Suspense, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ArrowDown, ArrowUp, Download, Lightbulb, Minus } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { KeadaanGagal, KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { Kerangka, KerangkaKartuAngka } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { useOnline } from '@/bersama/hooks/use-online'
import { GalatAPI } from '@/lib/api-client'
import { IZIN } from '@/lib/izin'
import { formatRupiah, persenSelisih } from '@/bersama/util/uang'
import { tanggalISO } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import { BarChart3, CloudOff } from 'lucide-react'
import { laporanApi, type Pengelompokan } from '../api'
import { BatangKanal, type BarisBatang } from '../komponen/batang-kanal'

// Recharts hanya diunduh oleh yang benar-benar membuka laporan.
const GrafikHarian = lazy(async () => ({
  default: (await import('../komponen/grafik-harian')).GrafikHarian,
}))

type Rentang = 'hari-ini' | '7-hari' | '30-hari' | 'bulan-ini'

const RENTANG: Record<Rentang, string> = {
  'hari-ini': 'Hari ini',
  '7-hari': '7 hari terakhir',
  '30-hari': '30 hari terakhir',
  'bulan-ini': 'Bulan ini',
}

/**
 * Laporan pemilik — jawab dulu, rinci belakangan (ui/05-ALUR-UTAMA.md §6).
 *
 * Angka besarnya adalah UNTUNG, bukan omzet: itu pertanyaan yang sebenarnya.
 * Rumusnya ditulis terbuka sebagai pengurangan bertingkat supaya pemilik paham
 * dari mana angkanya, bukan disuruh percaya.
 */
export function HalamanLaporan() {
  const { tokoAktif, boleh } = useSesi()
  const online = useOnline()
  const toast = useToast()
  const [rentang, setRentang] = useState<Rentang>('hari-ini')
  const [kelompok, setKelompok] = useState<Pengelompokan>('day')

  const { dari, sampai } = useMemo(() => hitungRentang(rentang), [rentang])
  const bolehLihatUntung = boleh(IZIN.reportProfit)

  const penjualan = useQuery({
    queryKey: ['laporan-penjualan', dari, sampai, kelompok, tokoAktif],
    queryFn: () => laporanApi.penjualan(dari, sampai, kelompok, tokoAktif),
    enabled: online,
  })

  // Tab "Untung" tidak muncul sama sekali tanpa izin report.profit — pemilik
  // sering ingin kasir melihat omzet tapi TIDAK melihat margin.
  const untung = useQuery({
    queryKey: ['laporan-untung', dari, sampai, tokoAktif],
    queryFn: () => laporanApi.untung(dari, sampai, tokoAktif),
    enabled: online && bolehLihatUntung,
  })

  // Pembanding: periode sebelumnya dengan panjang yang sama.
  const sebelumnya = useQuery({
    queryKey: ['laporan-sebelum', dari, sampai, tokoAktif],
    queryFn: () => {
      const { dari: d, sampai: s } = periodeSebelum(dari, sampai)
      return laporanApi.penjualan(d, s, 'day', tokoAktif)
    },
    enabled: online,
  })

  if (!online) {
    return (
      <KeadaanKosong
        ikon={CloudOff}
        judul="Laporan butuh internet"
        penjelasan="Angka laporan dihitung di server supaya selalu tepat. Sambungkan ke internet untuk melihatnya — kasir tetap bisa dipakai seperti biasa."
      />
    )
  }

  const totalSekarang = penjualan.data?.totals
  const totalSebelum = sebelumnya.data?.totals

  return (
    <div className="flex flex-col gap-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-judul font-bold text-teks-utama">Laporan</h1>

        <div className="flex flex-wrap gap-2">
          {(Object.keys(RENTANG) as Rentang[]).map((r) => (
            <button
              key={r}
              type="button"
              onClick={() => setRentang(r)}
              aria-pressed={rentang === r}
              className={cn(
                'h-10 rounded-full border px-3 text-label font-medium',
                rentang === r
                  ? 'border-utama bg-sorot text-utama'
                  : 'border-garis bg-permukaan text-teks-sekunder hover:bg-permukaan-2',
              )}
            >
              {RENTANG[r]}
            </button>
          ))}
        </div>
      </header>

      {penjualan.isLoading ? (
        <KerangkaKartuAngka />
      ) : penjualan.isError ? (
        <KeadaanGagal
          pesan={
            penjualan.error instanceof GalatAPI
              ? penjualan.error.pesan
              : 'Laporan belum bisa dimuat.'
          }
          onCobaLagi={() => penjualan.refetch()}
        />
      ) : (
        <>
          {/* Angka pertama yang dilihat: untung bila berhak, omzet bila tidak. */}
          <AngkaSorotan
            label={bolehLihatUntung ? 'Untung bersih' : 'Uang masuk'}
            nilai={
              bolehLihatUntung
                ? (untung.data?.totals.laba_bersih ?? 0)
                : (totalSekarang?.net_amount ?? 0)
            }
            pembanding={
              bolehLihatUntung
                ? undefined
                : totalSebelum?.net_amount
            }
            labelPembanding={labelPembanding(rentang)}
            memuat={bolehLihatUntung && untung.isLoading}
          />

          {/* Rumusnya terbuka: pemilik melihat dari mana angkanya. */}
          {bolehLihatUntung && untung.data && (
            <Kartu className="p-4">
              <dl className="flex flex-col gap-1.5 text-label">
                <BarisRumus label="Uang masuk" nilai={untung.data.totals.omzet} />
                <BarisRumus label="Modal barang" nilai={-untung.data.totals.modal} />
                {untung.data.totals.biaya_kanal > 0 && (
                  <BarisRumus
                    label="Biaya kanal"
                    nilai={-untung.data.totals.biaya_kanal}
                  />
                )}
                <div className="mt-1 flex items-baseline justify-between border-t border-garis pt-2">
                  <dt className="font-semibold text-teks-utama">Untung bersih</dt>
                  <dd className="text-judul-kartu font-bold tabular-nums text-teks-utama">
                    {formatRupiah(untung.data.totals.laba_bersih)}
                  </dd>
                </div>
              </dl>
            </Kartu>
          )}

          <div className="grid gap-3 sm:grid-cols-3">
            <KartuKecil
              label="Jumlah transaksi"
              nilai={(totalSekarang?.sales_count ?? 0).toLocaleString('id-ID')}
            />
            <KartuKecil
              label="Rata-rata belanja"
              nilai={formatRupiah(
                totalSekarang && totalSekarang.sales_count > 0
                  ? Math.trunc(totalSekarang.net_amount / totalSekarang.sales_count)
                  : 0,
              )}
            />
            <KartuKecil
              label="Diskon diberikan"
              nilai={formatRupiah(totalSekarang?.discount_amount ?? 0)}
            />
          </div>

          {/* Dari mana penjualannya */}
          {bolehLihatUntung && (untung.data?.by_channel.length ?? 0) > 0 && (
            <Kartu className="p-4">
              <BatangKanal
                judul="Dari mana penjualannya?"
                baris={barisKanal(untung.data!.by_channel)}
              />
              <Wawasan
                baris={untung.data!.by_channel.map((c) => ({
                  nama: namaKanal(c.channel_id),
                  omzet: c.omzet,
                  laba: c.laba_bersih,
                }))}
              />
            </Kartu>
          )}

          {/* Rincian: grafik harian atau tabel pengelompokan lain */}
          <Kartu className="flex flex-col gap-3 p-4">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h2 className="text-judul-kartu font-semibold text-teks-utama">
                Rincian
              </h2>
              <div className="flex flex-wrap gap-2">
                {(
                  [
                    ['day', 'Per hari'],
                    ['payment', 'Cara bayar'],
                    ['cashier', 'Kasir'],
                    ['channel', 'Kanal'],
                  ] as [Pengelompokan, string][]
                ).map(([nilai, label]) => (
                  <button
                    key={nilai}
                    type="button"
                    onClick={() => setKelompok(nilai)}
                    aria-pressed={kelompok === nilai}
                    className={cn(
                      'h-9 rounded-full border px-3 text-keterangan font-medium',
                      kelompok === nilai
                        ? 'border-utama bg-sorot text-utama'
                        : 'border-garis bg-permukaan text-teks-sekunder hover:bg-permukaan-2',
                    )}
                  >
                    {label}
                  </button>
                ))}
              </div>
            </div>

            {(penjualan.data?.rows.length ?? 0) === 0 ? (
              <KeadaanKosong
                ikon={BarChart3}
                judul="Belum ada penjualan di periode ini"
                penjelasan="Coba pilih rentang tanggal yang lain, atau mulai berjualan dulu lewat layar Kasir."
              />
            ) : kelompok === 'day' ? (
              <Suspense fallback={<Kerangka className="h-56 w-full" />}>
                <GrafikHarian rows={penjualan.data!.rows} />
              </Suspense>
            ) : (
              <BatangKanal
                baris={penjualan.data!.rows.map((r) => ({
                  kunci: r.key,
                  label: labelBaris(kelompok, r.key),
                  nilai: r.net_amount,
                  keterangan: `${r.sales_count}×`,
                }))}
              />
            )}
          </Kartu>

          {boleh(IZIN.reportExport) && (
            <Tombol
              jenis="kedua"
              onClick={async () => {
                try {
                  await laporanApi.unduhCSV('sales', {
                    from: dari,
                    to: sampai,
                    group_by: kelompok,
                    outlet_id: tokoAktif,
                  })
                } catch (e) {
                  toast.gagal(e instanceof Error ? e.message : 'Gagal mengunduh.')
                }
              }}
              className="self-start"
            >
              <Download className="h-5 w-5" aria-hidden />
              Unduh Excel
            </Tombol>
          )}
        </>
      )}
    </div>
  )
}

function AngkaSorotan({
  label,
  nilai,
  pembanding,
  labelPembanding,
  memuat,
}: {
  label: string
  nilai: number
  pembanding?: number
  labelPembanding: string
  memuat?: boolean
}) {
  if (memuat) return <KerangkaKartuAngka />

  const persen = pembanding === undefined ? null : persenSelisih(nilai, pembanding)
  const naik = persen !== null && persen > 0
  const turun = persen !== null && persen < 0
  const Panah = naik ? ArrowUp : turun ? ArrowDown : Minus

  return (
    <Kartu className="p-4">
      <p className="text-label font-medium text-teks-sekunder">{label}</p>
      <p
        className={cn(
          'mt-1 text-angka font-extrabold tabular-nums',
          nilai < 0 ? 'text-bahaya-teks' : 'text-teks-utama',
        )}
      >
        {formatRupiah(nilai)}
      </p>
      {persen !== null && (
        <p
          className={cn(
            'mt-1 flex items-center gap-1 text-keterangan font-medium',
            persen === 0 ? 'text-teks-redup' : naik ? 'text-hijau-700' : 'text-bahaya-teks',
          )}
        >
          <Panah className="h-4 w-4" aria-hidden />
          {Math.abs(persen)}% {labelPembanding}
        </p>
      )}
    </Kartu>
  )
}

function KartuKecil({ label, nilai }: { label: string; nilai: string }) {
  return (
    <Kartu className="p-4">
      <p className="text-keterangan text-teks-sekunder">{label}</p>
      <p className="mt-1 text-judul-kartu font-bold tabular-nums text-teks-utama">
        {nilai}
      </p>
    </Kartu>
  )
}

function BarisRumus({ label, nilai }: { label: string; nilai: number }) {
  return (
    <div className="flex items-baseline justify-between">
      <dt className="text-teks-sekunder">{label}</dt>
      <dd className="tabular-nums text-teks-utama">{formatRupiah(nilai)}</dd>
    </div>
  )
}

/**
 * Baris 💡 hanya muncul kalau memang ada pola yang layak disebut. Kalau
 * dipaksakan setiap hari, akan diabaikan (ui/05-ALUR-UTAMA.md §6).
 */
function Wawasan({
  baris,
}: {
  baris: { nama: string; omzet: number; laba: number }[]
}) {
  if (baris.length < 2) return null

  const ramai = [...baris].sort((a, b) => b.omzet - a.omzet)[0]!
  if (ramai.omzet <= 0) return null

  const marginRamai = ramai.laba / ramai.omzet
  const lain = baris.filter((b) => b.nama !== ramai.nama && b.omzet > 0)
  if (lain.length === 0) return null

  const marginTerbaik = Math.max(...lain.map((b) => b.laba / b.omzet))
  // Disebut hanya bila selisih marginnya benar-benar mencolok.
  if (marginTerbaik - marginRamai < 0.1) return null

  return (
    <p className="mt-3 flex items-start gap-2 rounded-kontrol bg-jingga-100 px-3 py-2 text-label text-jingga-700">
      <Lightbulb className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
      {ramai.nama} paling ramai, tapi setelah dipotong biaya, untungnya paling
      tipis dibanding kanal lain.
    </p>
  )
}

// ── Bantuan ────────────────────────────────────────────────────────────────

function hitungRentang(r: Rentang): { dari: string; sampai: string } {
  const kini = new Date()
  const sampai = tanggalISO(kini)
  if (r === 'hari-ini') return { dari: sampai, sampai }
  if (r === 'bulan-ini') {
    return { dari: `${sampai.slice(0, 7)}-01`, sampai }
  }
  const hari = r === '7-hari' ? 6 : 29
  return { dari: tanggalISO(new Date(kini.getTime() - hari * 86_400_000)), sampai }
}

function periodeSebelum(dari: string, sampai: string): { dari: string; sampai: string } {
  const d = new Date(`${dari}T00:00:00Z`)
  const s = new Date(`${sampai}T00:00:00Z`)
  const panjang = Math.max(1, Math.round((s.getTime() - d.getTime()) / 86_400_000) + 1)
  const sBaru = new Date(d.getTime() - 86_400_000)
  const dBaru = new Date(sBaru.getTime() - (panjang - 1) * 86_400_000)
  const iso = (x: Date) => x.toISOString().slice(0, 10)
  return { dari: iso(dBaru), sampai: iso(sBaru) }
}

function labelPembanding(r: Rentang): string {
  return r === 'hari-ini' ? 'dibanding kemarin' : 'dibanding periode sebelumnya'
}

function namaKanal(id: string): string {
  return id ? `Kanal ${id.slice(0, 6)}` : 'Kasir langsung'
}

function barisKanal(
  rows: { channel_id: string; omzet: number; laba_bersih: number }[],
): BarisBatang[] {
  const total = rows.reduce((j, r) => j + r.omzet, 0)
  return rows
    .filter((r) => r.omzet > 0)
    .sort((a, b) => b.omzet - a.omzet)
    .map((r) => ({
      kunci: r.channel_id || 'langsung',
      label: namaKanal(r.channel_id),
      nilai: r.omzet,
      keterangan: total > 0 ? `${Math.round((r.omzet / total) * 100)}%` : undefined,
    }))
}

function labelBaris(kelompok: Pengelompokan, kunci: string): string {
  if (kelompok === 'payment') {
    const nama: Record<string, string> = {
      cash: 'Tunai',
      qris: 'QRIS',
      transfer: 'Transfer',
      card: 'Kartu',
      ewallet: 'Dompet digital',
      credit: 'Kasbon',
    }
    return nama[kunci] ?? kunci
  }
  if (kelompok === 'channel') return namaKanal(kunci)
  return kunci || '—'
}
