import { lazy, Suspense, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ChevronRight, CloudOff, Download, HandCoins, PackageOpen, ShoppingBasket, Wallet } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { useToast } from '@/bersama/komponen/toast'
import { SegmenPilihan } from '@/bersama/ui/segmen'
import { KartuAngka } from '@/bersama/komponen/kartu-angka'
import { KeadaanGagal, KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { Kerangka, KerangkaKartuAngka } from '@/bersama/komponen/kerangka'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { useOnline } from '@/bersama/hooks/use-online'
import { GalatAPI } from '@/lib/api-client'
import { IZIN } from '@/lib/izin'
import { formatRupiah } from '@/bersama/util/uang'
import { formatQtySatuan } from '@/bersama/util/desimal'
import { tanggalISO } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import { laporanApi } from '../api'
import { daftarTanggal } from '../deret'
import { hitungRentang, KolomTanggal, RENTANG, type Rentang } from '../rentang'
import { TabLaporan } from '../komponen/tab-laporan'

// Recharts hanya diunduh oleh yang benar-benar membuka laporan.
const GrafikHarian = lazy(async () => ({
  default: (await import('../komponen/grafik-harian')).GrafikHarian,
}))

/**
 * Laporan belanja & utang pemasok — sisi UANG KELUAR, pasangan Laporan
 * Penjualan.
 *
 * Tiga pertanyaan yang dijawab berurutan:
 * 1. Berapa yang dibelanjakan, dan seberapa besar dibanding uang masuk?
 *    (Persentase itu yang bermakna; "belanja naik 20%" belum tentu buruk —
 *    bisa jadi stok untuk penjualan yang juga naik.)
 * 2. Berapa uang yang benar-benar KELUAR ke pemasok — dari laci kasir atau
 *    uang lain — dan berapa utang yang masih tersisa?
 * 3. Ke pemasok mana dan untuk barang apa uangnya pergi?
 *
 * Nota dihitung menurut tanggal usaha notanya; pembayaran menurut tanggal
 * bayarnya — pelunasan hari ini untuk nota bulan lalu tetap uang keluar hari
 * ini.
 */
export function HalamanLaporanBelanja() {
  const { tokoAktif, boleh } = useSesi()
  const online = useOnline()
  const toast = useToast()
  const [mengunduh, setMengunduh] = useState<string | null>(null)
  const unduh = async (tipe: 'purchases' | 'purchase_payments') => {
    setMengunduh(tipe)
    try {
      await laporanApi.unduhCSV(tipe, { from: dari, to: sampai, outlet_id: tokoAktif })
    } catch (e) {
      toast.gagal(e instanceof Error ? e.message : 'Gagal mengunduh.')
    } finally {
      setMengunduh(null)
    }
  }
  const [rentang, setRentang] = useState<Rentang>('30-hari')
  const [dariPilih, setDariPilih] = useState(() => tanggalISO())
  const [sampaiPilih, setSampaiPilih] = useState(() => tanggalISO())
  const { dari, sampai } = useMemo(() => hitungRentang(rentang, dariPilih, sampaiPilih), [rentang, dariPilih, sampaiPilih])

  const q = useQuery({
    queryKey: ['laporan-belanja', dari, sampai, tokoAktif],
    queryFn: () => laporanApi.belanja(dari, sampai, tokoAktif),
    enabled: online,
  })
  // Pembanding yang bermakna: uang masuk di rentang yang sama.
  const masuk = useQuery({
    queryKey: ['laporan-penjualan', dari, sampai, 'day', tokoAktif],
    queryFn: () => laporanApi.penjualan(dari, sampai, 'day', tokoAktif),
    enabled: online && boleh(IZIN.reportView),
  })

  if (!boleh(IZIN.stockView)) {
    return (
      <KeadaanKosong
        ikon={ShoppingBasket}
        judul="Laporan belanja butuh izin stok"
        penjelasan="Laporan ini memuat nilai pembelian. Minta pemilik menambahkan izin melihat stok ke peran Anda."
      />
    )
  }
  if (!online) {
    return (
      <KeadaanKosong
        ikon={CloudOff}
        judul="Laporan butuh internet"
        penjelasan="Angka laporan dihitung di server supaya selalu tepat. Sambungkan ke internet untuk melihatnya."
      />
    )
  }

  const r = q.data
  const t = r?.totals
  const uangMasuk = masuk.data?.totals.net_amount
  const persenMasuk = t && uangMasuk ? Math.round((t.belanja / uangMasuk) * 100) : null
  const deret = r ? isiHari(r.days, dari, sampai) : []
  const maksPemasok = Math.max(1, ...(r?.suppliers ?? []).map((s) => s.belanja))
  const maksBarang = Math.max(1, ...(r?.products ?? []).map((b) => b.nilai))

  return (
    <div className="flex flex-col gap-4">
      <TabLaporan />
      <header className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-judul font-bold text-teks-utama">Laporan Belanja</h1>
        <SegmenPilihan
          label="Rentang tanggal"
          nilai={rentang}
          onPilih={setRentang}
          pilihan={(Object.keys(RENTANG) as Rentang[]).map((x) => [x, RENTANG[x]] as const)}
        />
      </header>

      {rentang === 'pilih' && (
        <Kartu className="flex flex-wrap items-end gap-3 p-4">
          <KolomTanggal label="Dari tanggal" nilai={dariPilih} onUbah={setDariPilih} />
          <KolomTanggal label="Sampai tanggal" nilai={sampaiPilih} onUbah={setSampaiPilih} />
        </Kartu>
      )}

      {q.isLoading ? (
        <div className="grid gap-3 lg:grid-cols-3">
          <KerangkaKartuAngka />
          <KerangkaKartuAngka />
          <KerangkaKartuAngka />
        </div>
      ) : q.isError || !r || !t ? (
        <KeadaanGagal
          pesan={q.error instanceof GalatAPI ? q.error.pesan : 'Laporan belanja belum bisa dimuat.'}
          onCobaLagi={() => q.refetch()}
        />
      ) : t.nota_count === 0 && t.dibayar === 0 ? (
        <KeadaanKosong
          ikon={PackageOpen}
          judul="Belum ada belanja di rentang ini"
          penjelasan="Barang masuk yang dicatat akan muncul di sini — berapa yang dibelanjakan, ke pemasok mana, dan sisa utangnya."
        />
      ) : (
        <>
          <div className="grid gap-3 lg:grid-cols-3">
            <KartuAngka
              ikon={ShoppingBasket}
              label="Belanja"
              nilai={t.belanja}
              keterangan={
                `${t.nota_count.toLocaleString('id-ID')} nota` +
                (persenMasuk !== null ? ` · ${persenMasuk}% dari uang masuk (${formatRupiah(uangMasuk!)})` : '')
              }
            />
            <KartuAngka
              ringkas
              ikon={Wallet}
              label="Uang keluar ke pemasok"
              nilai={t.dibayar}
              keterangan={`${formatRupiah(t.dari_laci)} dari laci · ${formatRupiah(t.dari_lain)} uang lain`}
            />
            <Link to="/stok/utang" className="rounded-kartu focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-utama">
              <KartuAngka
                ringkas
                ikon={HandCoins}
                label="Utang saat ini"
                nilai={t.utang_kini}
                keterangan={
                  t.sisa_nota > 0
                    ? `${formatRupiah(t.sisa_nota)} dari nota periode ini belum lunas`
                    : 'Nota periode ini lunas semua'
                }
                className="h-full"
              />
            </Link>
          </div>

          {deret.length > 1 && (
            <Kartu className="flex flex-col gap-3 p-4 sm:p-5">
              <h2 className="text-judul-kartu font-semibold text-teks-utama">Belanja per hari</h2>
              <Suspense fallback={<Kerangka className="h-56 w-full" />}>
                <GrafikHarian rows={deret} labelNilai="Belanja" />
              </Suspense>
            </Kartu>
          )}

          <div className="grid gap-3 lg:grid-cols-2">
            <Kartu className="flex flex-col gap-1 p-4 sm:p-5">
              <h2 className="text-judul-kartu font-semibold text-teks-utama">Per pemasok</h2>
              <ul className="flex flex-col divide-y divide-garis">
                {r.suppliers.map((s) => {
                  const isi = (
                    <>
                      <span className="flex items-baseline justify-between gap-3">
                        <span className="min-w-0 truncate font-medium text-teks-utama">
                          {s.supplier_name || 'Tanpa pemasok'}
                        </span>
                        <span className="shrink-0 font-semibold tabular-nums text-teks-utama">
                          {formatRupiah(s.belanja)}
                        </span>
                      </span>
                      <Batang persen={(s.belanja / maksPemasok) * 100} />
                      <span className="flex justify-between gap-3 text-keterangan text-teks-redup">
                        <span>{s.nota_count} nota</span>
                        <span className={cn(s.sisa_nota > 0 && 'font-semibold text-jingga-700')}>
                          {s.sisa_nota > 0 ? `sisa ${formatRupiah(s.sisa_nota)}` : 'lunas'}
                        </span>
                      </span>
                    </>
                  )
                  return (
                    <li key={s.supplier_id || '-'}>
                      {s.supplier_id ? (
                        <Link
                          to={`/pemasok/${s.supplier_id}`}
                          className="flex flex-col gap-1.5 rounded-kontrol py-2.5 hover:bg-permukaan-2/50"
                        >
                          {isi}
                        </Link>
                      ) : (
                        <div className="flex flex-col gap-1.5 py-2.5">{isi}</div>
                      )}
                    </li>
                  )
                })}
              </ul>
            </Kartu>

            <Kartu className="flex flex-col gap-1 p-4 sm:p-5">
              <h2 className="text-judul-kartu font-semibold text-teks-utama">Barang dengan belanja terbesar</h2>
              <ul className="flex flex-col divide-y divide-garis">
                {r.products.map((b) => (
                  <li key={b.product_id} className="flex flex-col gap-1.5 py-2.5">
                    <span className="flex items-baseline justify-between gap-3">
                      <span className="min-w-0 truncate font-medium text-teks-utama">{b.product_name}</span>
                      <span className="shrink-0 font-semibold tabular-nums text-teks-utama">{formatRupiah(b.nilai)}</span>
                    </span>
                    <Batang persen={(b.nilai / maksBarang) * 100} warna="bg-grafik-2" />
                    <span className="text-keterangan tabular-nums text-teks-redup">
                      {formatQtySatuan(b.qty, b.unit_name)} masuk
                    </span>
                  </li>
                ))}
              </ul>
              <Link
                to="/pemasok"
                className="mt-1 flex min-h-11 items-center gap-1 self-start text-label font-medium text-utama hover:underline"
              >
                Lihat pemasok
                <ChevronRight className="h-4 w-4" aria-hidden />
              </Link>
            </Kartu>
          </div>

          {/* Untuk pembukuan / akuntan: dua berkas, satu per pertanyaan —
              "barang apa dibeli berapa" dan "uang apa keluar kapan". */}
          {boleh(IZIN.reportExport) && (
            <div className="flex flex-wrap gap-2">
              <Tombol jenis="kedua" memuat={mengunduh === 'purchases'} onClick={() => unduh('purchases')}>
                <Download className="h-5 w-5" aria-hidden />
                Unduh nota belanja (Excel)
              </Tombol>
              <Tombol jenis="kedua" memuat={mengunduh === 'purchase_payments'} onClick={() => unduh('purchase_payments')}>
                <Download className="h-5 w-5" aria-hidden />
                Unduh pembayaran (Excel)
              </Tombol>
            </div>
          )}
        </>
      )}
    </div>
  )
}

function Batang({ persen, warna = 'bg-grafik-1' }: { persen: number; warna?: string }) {
  return (
    <span className="block h-1.5 overflow-hidden rounded-full bg-permukaan-2" aria-hidden>
      <span className={cn('block h-full rounded-full', warna)} style={{ width: `${Math.max(persen, 2)}%` }} />
    </span>
  )
}

/**
 * Deret harian lengkap: hari tanpa belanja diisi nol, bukan dilewati — grafik
 * yang bolong berbohong tentang hari-hari sepi.
 */
function isiHari(days: { date: string; belanja: number }[], dari: string, sampai: string) {
  const per = new Map(days.map((d) => [d.date, d.belanja]))
  return daftarTanggal(dari, sampai).map((tgl) => ({ key: tgl, net_amount: per.get(tgl) ?? 0 }))
}
