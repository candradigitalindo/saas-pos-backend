import { lazy, Suspense, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Download, Lightbulb } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { KeadaanGagal, KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { Kerangka, KerangkaKartuAngka } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { useOnline } from '@/bersama/hooks/use-online'
import { GalatAPI } from '@/lib/api-client'
import { IZIN } from '@/lib/izin'
import { formatRupiah } from '@/bersama/util/uang'
import { formatQtySatuan } from '@/bersama/util/desimal'
import { tanggalISO } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import { BarChart3, CloudOff, ReceiptText, TicketPercent, Wallet } from 'lucide-react'
import { api, type Halaman } from '@/lib/api-client'
import { laporanApi, type Pengelompokan } from '../api'
import { lengkapiHari } from '../deret'
import { BatangKanal, type BarisBatang } from '../komponen/batang-kanal'
import { GrafikJam } from '../komponen/grafik-jam'
import { KartuSorotan } from '@/bersama/komponen/kartu-sorotan'
import { KartuAngka } from '@/bersama/komponen/kartu-angka'
import { SegmenPilihan } from '@/bersama/ui/segmen'

// Recharts hanya diunduh oleh yang benar-benar membuka laporan.
const GrafikHarian = lazy(async () => ({
  default: (await import('../komponen/grafik-harian')).GrafikHarian,
}))

type Rentang = 'hari-ini' | '7-hari' | '30-hari' | 'bulan-ini' | 'pilih'

/**
 * Berapa barang yang digambar di rincian "Per barang". Toko kelontong bisa
 * punya ratusan barang; ratusan batang bukan lagi ringkasan. Sisanya ada di
 * berkas unduhan, yang memuat semuanya.
 */
const BATAS_BARANG = 20

const RENTANG: Record<Rentang, string> = {
  'hari-ini': 'Hari ini',
  '7-hari': '7 hari terakhir',
  '30-hari': '30 hari terakhir',
  'bulan-ini': 'Bulan ini',
  pilih: 'Pilih tanggal',
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

  // Tanggal pilihan sendiri. Bawaannya HARI INI supaya menekan "Pilih tanggal"
  // tidak pernah menghasilkan layar kosong yang membingungkan.
  const [dariPilih, setDariPilih] = useState(() => tanggalISO())
  const [sampaiPilih, setSampaiPilih] = useState(() => tanggalISO())

  const { dari, sampai } = useMemo(
    () => hitungRentang(rentang, dariPilih, sampaiPilih),
    [rentang, dariPilih, sampaiPilih],
  )
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

  // Nama kanal — laporan hanya membawa id-nya.
  const kanal = useQuery({
    queryKey: ['kanal-nama'],
    queryFn: () =>
      api.get<Halaman<{ id: string; name: string }>>('/channels', { query: { limit: 100 } }),
    enabled: online && boleh(IZIN.channelManage, IZIN.channelOrderAccept),
    retry: false,
    staleTime: 5 * 60_000,
  })
  const petaKanal = useMemo(
    () => new Map((kanal.data?.data ?? []).map((k) => [k.id, k.name])),
    [kanal.data],
  )

  // Pembanding: periode sebelumnya dengan panjang yang sama.
  const sebelumnya = useQuery({
    queryKey: ['laporan-sebelum', dari, sampai, tokoAktif],
    queryFn: () => {
      const { dari: d, sampai: s } = periodeSebelum(dari, sampai)
      return laporanApi.penjualan(d, s, 'day', tokoAktif)
    },
    enabled: online,
  })

  // Untung juga butuh pembanding — angka tanpa pembanding tidak memberi tahu
  // apa pun (ui/01 §4), dan itu berlaku untuk angka terpenting di layar ini.
  const untungSebelum = useQuery({
    queryKey: ['laporan-untung-sebelum', dari, sampai, tokoAktif],
    queryFn: () => {
      const { dari: d, sampai: s } = periodeSebelum(dari, sampai)
      return laporanApi.untung(d, s, tokoAktif)
    },
    enabled: online && bolehLihatUntung,
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

        <SegmenPilihan
          label="Rentang tanggal"
          nilai={rentang}
          onPilih={setRentang}
          pilihan={(Object.keys(RENTANG) as Rentang[]).map((r) => [r, RENTANG[r]] as const)}
        />
      </header>

      {rentang === 'pilih' && (
        <Kartu className="flex flex-wrap items-end gap-3 p-4">
          <KolomTanggal label="Dari tanggal" nilai={dariPilih} onUbah={setDariPilih} />
          <KolomTanggal label="Sampai tanggal" nilai={sampaiPilih} onUbah={setSampaiPilih} />
        </Kartu>
      )}

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
          {(() => {
            const nilaiSorot = bolehLihatUntung
              ? (untung.data?.totals.laba_bersih ?? 0)
              : (totalSekarang?.net_amount ?? 0)
            const bandingSorot = bolehLihatUntung
              ? untungSebelum.data?.totals.laba_bersih
              : totalSebelum?.net_amount

            if (bolehLihatUntung && untung.isLoading) return <KerangkaKartuAngka />

            return (
              <KartuSorotan
                label={bolehLihatUntung ? 'Untung bersih' : 'Uang masuk'}
                nilai={nilaiSorot}
                pembanding={bandingSorot}
                labelPembanding={labelPembanding(rentang)}
                // Rugi tidak duduk di atas kartu perayaan — lihat KartuSorotan.
                nada={nilaiSorot < 0 ? 'bahaya' : 'utama'}
              />
            )
          })()}

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

          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
            <KartuAngka
              ringkas
              ikon={ReceiptText}
              label="Jumlah transaksi"
              nilai={totalSekarang?.sales_count ?? 0}
              uang={false}
              keterangan="Transaksi selesai di periode ini"
            />
            <KartuAngka
              ringkas
              ikon={Wallet}
              label="Rata-rata belanja"
              nilai={
                totalSekarang && totalSekarang.sales_count > 0
                  ? Math.trunc(totalSekarang.net_amount / totalSekarang.sales_count)
                  : 0
              }
              keterangan="Uang masuk dibagi jumlah transaksi"
            />
            <KartuAngka
              ringkas
              ikon={TicketPercent}
              className="col-span-2 sm:col-span-1"
              label="Diskon diberikan"
              nilai={totalSekarang?.discount_amount ?? 0}
              keterangan="Potongan harga di periode ini"
            />
          </div>

          {/* Dari mana penjualannya */}
          {bolehLihatUntung && (untung.data?.by_channel.length ?? 0) > 0 && (
            <Kartu className="p-4">
              <BatangKanal
                judul="Dari mana penjualannya?"
                baris={barisKanal(untung.data!.by_channel, petaKanal)}
              />
              <Wawasan
                baris={untung.data!.by_channel.map((c) => ({
                  nama: namaKanal(c.channel_id, petaKanal),
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
              <SegmenPilihan
                label="Rincian menurut"
                nilai={kelompok}
                onPilih={setKelompok}
                pilihan={
                  [
                    ['day', 'Per hari'],
                    ['hour', 'Jam ramai'],
                    ['product', 'Per barang'],
                    ['payment', 'Cara bayar'],
                    ['cashier', 'Kasir'],
                    ['channel', 'Kanal'],
                  ] as const
                }
              />
            </div>

            {(penjualan.data?.rows.length ?? 0) === 0 ? (
              <KeadaanKosong
                ikon={BarChart3}
                judul="Belum ada penjualan di periode ini"
                penjelasan="Coba pilih rentang tanggal yang lain, atau mulai berjualan dulu lewat layar Kasir."
              />
            ) : kelompok === 'day' ? (
              <Suspense fallback={<Kerangka className="h-56 w-full" />}>
                {/* Hari tanpa penjualan tetap digambar (batang nol) — tanpa
                    itu tiga hari berjualan sebulan tampil berdempetan. */}
                <GrafikHarian rows={lengkapiHari(penjualan.data!.rows, dari, sampai)} />
              </Suspense>
            ) : kelompok === 'hour' ? (
              <GrafikJam rows={penjualan.data!.rows} />
            ) : kelompok === 'product' ? (
              <>
                <BatangKanal
                  satuWarna
                  baris={penjualan.data!.rows.slice(0, BATAS_BARANG).map((r) => ({
                    kunci: r.key,
                    label: r.label ?? r.key,
                    nilai: r.net_amount,
                    keterangan: formatQtySatuan(r.qty ?? '0', r.unit),
                  }))}
                />
                {penjualan.data!.rows.length > BATAS_BARANG && (
                  <p className="text-keterangan text-teks-redup">
                    {BATAS_BARANG} barang teratas dari {penjualan.data!.rows.length}. Daftar
                    lengkapnya ada di berkas unduhan.
                  </p>
                )}
              </>
            ) : (
              <BatangKanal
                baris={penjualan.data!.rows.map((r) => ({
                  kunci: r.key,
                  label: labelBaris(kelompok, r.key, petaKanal),
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
    <p className="mt-3 flex items-start gap-2 rounded-kontrol bg-permukaan-2 px-3 py-2 text-label text-jingga-700">
      <Lightbulb className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
      {ramai.nama} paling ramai, tapi setelah dipotong biaya, untungnya paling
      tipis dibanding kanal lain.
    </p>
  )
}

// ── Bantuan ────────────────────────────────────────────────────────────────

function hitungRentang(
  r: Rentang,
  dariPilih: string,
  sampaiPilih: string,
): { dari: string; sampai: string } {
  const kini = new Date()
  const sampai = tanggalISO(kini)
  if (r === 'pilih') {
    // Dibalik bila terbalik, bukan ditolak: orang sering mengisi kolom kedua
    // lebih dulu, dan menolak isian yang maksudnya jelas cuma menghalangi.
    return dariPilih <= sampaiPilih
      ? { dari: dariPilih, sampai: sampaiPilih }
      : { dari: sampaiPilih, sampai: dariPilih }
  }
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

/** Kolom tanggal: label di atas, bukan placeholder (ui/02 — Kolom isian). */
function KolomTanggal({
  label,
  nilai,
  onUbah,
}: {
  label: string
  nilai: string
  onUbah: (v: string) => void
}) {
  return (
    <label className="flex flex-1 flex-col gap-1">
      <span className="text-keterangan font-medium text-teks-sekunder">{label}</span>
      <input
        type="date"
        value={nilai}
        max={tanggalISO()}
        onChange={(e) => onUbah(e.target.value)}
        className={cn(
          'h-12 w-full rounded-kontrol border border-garis bg-permukaan px-3',
          'text-isi tabular-nums text-teks-utama',
          'focus:border-utama focus:outline-none focus:ring-2 focus:ring-utama/30',
        )}
      />
    </label>
  )
}

/**
 * Nama kanal untuk ditampilkan.
 *
 * Laporan hanya membawa `channel_id`, jadi namanya diambil dari daftar kanal.
 * Sebelum peta itu tersedia, JANGAN tampilkan potongan id seperti
 * "Kanal 01M24C" — bagi pemilik warung itu sama saja dengan tidak ada
 * keterangan, dan melanggar aturan "bahasa orang, bukan bahasa sistem".
 */
function namaKanal(id: string, peta?: Map<string, string>): string {
  if (!id) return 'Kasir langsung'
  return peta?.get(id) ?? 'Kanal online'
}

function barisKanal(
  rows: { channel_id: string; omzet: number; laba_bersih: number }[],
  peta: Map<string, string>,
): BarisBatang[] {
  const total = rows.reduce((j, r) => j + r.omzet, 0)
  return rows
    .filter((r) => r.omzet > 0)
    .sort((a, b) => b.omzet - a.omzet)
    .map((r) => ({
      kunci: r.channel_id || 'langsung',
      label: namaKanal(r.channel_id, peta),
      nilai: r.omzet,
      keterangan: total > 0 ? `${Math.round((r.omzet / total) * 100)}%` : undefined,
    }))
}

function labelBaris(
  kelompok: Pengelompokan,
  kunci: string,
  peta: Map<string, string>,
): string {
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
  if (kelompok === 'channel') return namaKanal(kunci, peta)
  return kunci || '—'
}
