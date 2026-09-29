import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import {
  Boxes,
  CalendarDays,
  ChevronRight,
  PackagePlus,
  Receipt,
  TrendingUp,
  TriangleAlert,
} from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { SegmenPilihan } from '@/bersama/ui/segmen'
import { KartuAngka } from '@/bersama/komponen/kartu-angka'
import { KartuBagian, TautanBagian } from '@/bersama/komponen/kartu-bagian'
import { KartuSorotan } from '@/bersama/komponen/kartu-sorotan'
import { KerangkaKartuAngka } from '@/bersama/komponen/kerangka'
import { StatusKoneksi } from '@/bersama/komponen/status-koneksi'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { useSinkron } from '@/lib/offline/mesin'
import { PeringatanUtang } from '@/fitur/stok/komponen/peringatan-utang'
import { PeringatanKasbon } from '@/fitur/pelanggan/komponen/peringatan-kasbon'
import { api } from '@/lib/api-client'
import { IZIN } from '@/lib/izin'
import { formatRupiah } from '@/bersama/util/uang'
import { formatHariPanjang, tanggalISO } from '@/bersama/util/tanggal'
import { formatSisaStok } from '@/bersama/util/desimal'
import { cn } from '@/bersama/util/cn'
import type { Halaman } from '@/lib/api-client'
import type { SaldoStok } from '@/bersama/tipe/katalog'
import { laporanApi } from '../api'
import { KartuTren, type PanjangPeriode } from '../komponen/kartu-tren'
import { DaftarTerlaris } from '../komponen/daftar-terlaris'
import { TransaksiTerakhir } from '../komponen/transaksi-terakhir'

/** Berapa hari terakhir yang digambar di grafik mungil kartu sorotan. */
const HARI_RIWAYAT = 7

/** Kunci localStorage pilihan periode bagian "Penjualan" (7/30 hari). */
const KUNCI_PERIODE = 'beranda.periode'

/**
 * Periode yang terakhir dipilih di perangkat ini. Hanya kenyamanan: bila
 * penyimpanan diblokir (mode privat, data situs dihapus) kembali ke 7 hari.
 */
function periodeTersimpan(): PanjangPeriode {
  try {
    return localStorage.getItem(KUNCI_PERIODE) === '30' ? 30 : 7
  } catch {
    return 7
  }
}

function simpanPeriode(h: PanjangPeriode) {
  try {
    localStorage.setItem(KUNCI_PERIODE, String(h))
  } catch {
    /* tidak tersimpan — pilihan tetap berlaku sampai halaman ditutup */
  }
}

/**
 * Beranda pemilik.
 *
 * Sasarannya: "hari ini untung berapa" terjawab dalam NOL ketukan. Karena itu
 * angkanya ada di layar pertama, bukan di balik menu Laporan.
 *
 * Susunannya mengikuti satu urutan: uang masuk hari ini disorot sendirian di
 * kartu berwarna penuh, sisanya angka pendamping yang berjajar setara. Versi
 * sebelumnya menaruh semuanya di kartu putih seragam — tidak ada yang menonjol,
 * dan satu-satunya warna kuat di layar adalah merah "turun" dan jingga "habis".
 * Lihat KartuSorotan untuk alasan lengkapnya.
 *
 * Di bawahnya bagian yang lazim di dasbor SaaS kasir: tren uang masuk 7/30
 * hari dengan pembanding periode sebelumnya, barang terlaris, transaksi
 * terakhir, dan stok hampir habis — masing-masing kartu berjudul sendiri
 * (KartuBagian). Tanpa itu, Beranda di hari yang belum ada penjualannya hanya
 * berisi deretan "Rp 0" dan tidak memberi tahu apa pun; kini hari yang sepi
 * tetap menunjukkan bagaimana pekan ini berjalan.
 */
export function HalamanBeranda() {
  const { profil, boleh, tokoAktif, rincianToko } = useSesi()
  const [periode, setPeriode] = useState<PanjangPeriode>(periodeTersimpan)
  const gantiPeriode = (h: PanjangPeriode) => {
    setPeriode(h)
    simpanPeriode(h)
  }
  const sinkron = useSinkron()
  const nama = profil?.user.name?.split(' ')[0] ?? ''

  const bolehLihatLaporan = boleh(IZIN.reportView)
  const bolehLihatUntung = boleh(IZIN.reportProfit)

  const hariIni = tanggalISO()
  const kemarin = tanggalISO(new Date(Date.now() - 86_400_000))
  const awalRiwayat = tanggalISO(new Date(Date.now() - (HARI_RIWAYAT - 1) * 86_400_000))

  const dashboard = useQuery({
    queryKey: ['dashboard', tokoAktif, hariIni],
    queryFn: () => laporanApi.dashboard(tokoAktif, hariIni),
    enabled: bolehLihatLaporan,
  })

  // Pembanding kemarin — angka tanpa pembanding tidak memberi tahu apa pun.
  const dashboardKemarin = useQuery({
    queryKey: ['dashboard', tokoAktif, kemarin],
    queryFn: () => laporanApi.dashboard(tokoAktif, kemarin),
    enabled: bolehLihatLaporan,
  })

  // Tujuh hari terakhir, untuk grafik mungil di kartu sorotan. "Turun 67%"
  // terhadap SATU hari kemarin gampang menyesatkan — kemarin bisa saja hari
  // luar biasa. Bentuk sepekan menempatkannya pada tempatnya.
  const riwayat = useQuery({
    queryKey: ['riwayat-harian', tokoAktif, awalRiwayat, hariIni],
    queryFn: () => laporanApi.penjualan(awalRiwayat, hariIni, 'day', tokoAktif),
    enabled: bolehLihatLaporan,
  })

  const stokMenipis = useQuery({
    queryKey: ['stok-menipis', tokoAktif],
    queryFn: () =>
      api.get<Halaman<SaldoStok>>('/stocks', {
        query: { outlet_id: tokoAktif, low: 'true', limit: 5 },
      }),
    enabled: boleh(IZIN.stockView) && !!tokoAktif,
  })

  const pintasan = [
    { ke: '/kasir', label: 'Jual barang', ikon: Receipt, izin: [IZIN.saleCreate] },
    { ke: '/stok/masuk', label: 'Barang masuk', ikon: PackagePlus, izin: [IZIN.stockAdjust] },
    { ke: '/stok', label: 'Lihat stok', ikon: Boxes, izin: [IZIN.stockView] },
  ].filter((p) => boleh(...p.izin))

  const hari = dashboard.data?.today
  const kmr = dashboardKemarin.data?.today
  const bulan = dashboard.data?.month_to_date

  // Hari tanpa penjualan tidak dikirim backend. Dibiarkan bolong, grafiknya
  // berbohong — hari sepi akan terlihat seperti hari yang tidak ada.
  const deretHarian = deretTujuhHari(riwayat.data?.rows, HARI_RIWAYAT)

  const bolehLihatTransaksi = boleh(IZIN.saleCreate)
  const bolehLihatStok = boleh(IZIN.stockView)
  const adaStokMenipis = (stokMenipis.data?.data.length ?? 0) > 0
  // Toko lebih dari satu: sebut toko yang sedang ditampilkan angkanya.
  const banyakToko = (profil?.outlets?.length ?? 0) > 1

  return (
    <div className="flex flex-col gap-4">
      <header>
        {/* Tanggal lengkap di atas sapaan — konteks "angka hari apa ini"
            yang di dasbor mana pun selalu ada di kepala halaman. */}
        <p className="text-label font-medium text-teks-redup">
          {formatHariPanjang(new Date(), rincianToko?.timezone)}
        </p>
        <h1 className="text-judul font-bold text-teks-utama">
          {sapaan()}
          {nama && `, ${nama}`}
        </h1>
        <p className="text-label text-teks-sekunder">
          {profil?.tenant.business_name}
          {banyakToko && rincianToko && rincianToko.name !== profil?.tenant.business_name
            ? ` · ${rincianToko.name}`
            : ''}
        </p>
      </header>

      {/* Di layar lebar status yang sama sudah ada di kaki navigasi samping —
          dua kalimat "Semua data tersimpan" di satu layar hanya derau. */}
      <StatusKoneksi menunggu={sinkron.menunggu} className="lg:hidden" />

      {sinkron.perluDiperiksa > 0 && (
        <Link to="/kasir/belum-terkirim">
          <Kartu className="flex items-center gap-3 border-jingga-600 bg-permukaan-2 p-4">
            <TriangleAlert className="h-5 w-5 shrink-0 text-jingga-700" aria-hidden />
            <p className="flex-1 text-label font-medium text-jingga-700">
              {sinkron.perluDiperiksa} transaksi perlu diperiksa
            </p>
            <ChevronRight className="h-5 w-5 text-jingga-700" aria-hidden />
          </Kartu>
        </Link>
      )}

      {/* Muncul hanya saat ada utang pemasok / kasbon pelanggan yang lewat
          atau segera jatuh tempo. */}
      <PeringatanUtang />
      <PeringatanKasbon />

      {bolehLihatLaporan &&
        (dashboard.isLoading ? (
          <div className="grid gap-3 sm:grid-cols-2">
            <KerangkaKartuAngka />
            <KerangkaKartuAngka />
          </div>
        ) : (
          <>
            {/* Susunan dasbor: satu angka disorot di kiri dengan konteksnya
                sendiri, angka pendamping bertumpuk di kolom kanan. Di HP
                semuanya menumpuk — dan karena dua angka pendukung sudah ikut di
                kaki kartu sorotan, tumpukannya tetap pendek. */}
            <div className="grid gap-3 lg:grid-cols-3">
              <KartuSorotan
                className="lg:col-span-2"
                label="Uang masuk hari ini"
                nilai={hari?.net_amount ?? 0}
                pembanding={kmr?.net_amount}
                riwayat={deretHarian}
                labelRiwayat={`Uang masuk ${HARI_RIWAYAT} hari terakhir`}
                rincian={[
                  {
                    label: 'Transaksi',
                    nilai: (hari?.sales_count ?? 0).toLocaleString('id-ID'),
                  },
                  {
                    label: 'Rata-rata belanja',
                    nilai: formatRupiah(rataBelanja(hari?.net_amount, hari?.sales_count)),
                  },
                ]}
                aksi={
                  <Link
                    to="/laporan"
                    className="-m-2 flex min-h-12 items-center gap-1 rounded-kontrol p-2 text-keterangan font-semibold underline-offset-4 hover:underline"
                  >
                    Laporan
                    <ChevronRight className="h-4 w-4" aria-hidden />
                  </Link>
                }
              />

              <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-1">
                {bolehLihatUntung && (
                  <KartuAngka
                    ringkas
                    ikon={TrendingUp}
                    label="Untung kotor hari ini"
                    nilai={hari?.gross_profit ?? 0}
                    pembanding={kmr?.gross_profit}
                    labelPembanding="dari kemarin"
                  />
                )}
                <KartuAngka
                  ringkas
                  ikon={CalendarDays}
                  label="Uang masuk bulan ini"
                  nilai={bulan?.net_amount ?? 0}
                  keterangan={`${(bulan?.sales_count ?? 0).toLocaleString('id-ID')} transaksi sejak tanggal 1`}
                />
              </div>
            </div>
          </>
        ))}

      {pintasan.length > 0 && (
        <section aria-label="Pintasan" className="grid gap-3 sm:grid-cols-3">
          {pintasan.map((p) => (
            <Link
              key={p.ke}
              to={p.ke}
              className={cn(
                'group flex min-h-16 items-center gap-3 rounded-kartu border border-garis',
                'bg-permukaan px-4 shadow-kartu transition-colors hover:bg-permukaan-2',
              )}
            >
              <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-kontrol bg-utama/10 text-utama">
                <p.ikon className="h-5 w-5" aria-hidden />
              </span>
              <span className="flex-1 text-label font-semibold text-teks-utama">{p.label}</span>
              <ChevronRight
                className="h-5 w-5 shrink-0 text-teks-redup transition-transform group-hover:translate-x-0.5"
                aria-hidden
              />
            </Link>
          ))}
        </section>
      )}

      {bolehLihatLaporan && (
        <section aria-labelledby="judul-penjualan" className="flex flex-col gap-3 pt-2">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h2 id="judul-penjualan" className="text-judul-kartu font-bold text-teks-utama">
              Penjualan
            </h2>
            <SegmenPilihan
              label="Periode"
              nilai={String(periode) as '7' | '30'}
              onPilih={(v) => gantiPeriode(v === '30' ? 30 : 7)}
              pilihan={[['7', '7 hari'], ['30', '30 hari']] as const}
            />
          </div>
          {/* Tiga kolom baru mulai 1280px. Di 1024px navigasi samping memakan
              256px, dan sepertiga sisanya tinggal ±236px — judul "Barang
              terlaris" patah jadi dua baris dan keterangannya jadi tiga. */}
          <div className="grid gap-3 xl:grid-cols-3">
            <div className="min-w-0 xl:col-span-2">
              <KartuTren
                tokoAktif={tokoAktif}
                hariIni={hariIni}
                hari={periode}
                onGantiHari={gantiPeriode}
              />
            </div>
            <DaftarTerlaris tokoAktif={tokoAktif} hariIni={hariIni} hari={periode} />
          </div>
        </section>
      )}

      {(bolehLihatTransaksi || (bolehLihatStok && adaStokMenipis)) && (
        <div
          className={cn(
            'grid gap-3',
            // Berdampingan hanya bila keduanya ada; sendirian, satu kartu
            // mengambil lebar penuh alih-alih menyisakan lubang di kanan.
            bolehLihatTransaksi && bolehLihatStok && adaStokMenipis && 'md:grid-cols-2 xl:grid-cols-3',
          )}
        >
          {bolehLihatTransaksi && (
            <div className={cn('min-w-0', adaStokMenipis && bolehLihatStok && 'xl:col-span-2')}>
              <TransaksiTerakhir tokoAktif={tokoAktif} zona={rincianToko?.timezone} />
            </div>
          )}

          {bolehLihatStok && adaStokMenipis && (
            <KartuBagian
              judul="Hampir habis"
              keterangan="Saatnya belanja ulang"
              aksi={<TautanBagian ke="/stok">Lihat semua</TautanBagian>}
              isiClassName="px-0 pb-1 pt-2 sm:px-0"
            >
              <ul className="divide-y divide-garis border-t border-garis">
                {stokMenipis.data?.data.map((s) => {
                  // Yang benar-benar nol diberi lencana; yang tinggal sedikit
                  // cukup teks. Tanpa pembedaan ini, lima baris jingga berderet
                  // membuat "tinggal 4" terbaca segenting "habis" — dan kalau
                  // semuanya mendesak, tidak ada yang mendesak.
                  const habis = Number(s.qty) <= 0
                  return (
                    <li key={s.product_id}>
                      <Link
                        to="/stok"
                        className="flex min-h-12 items-center justify-between gap-3 px-4 hover:bg-permukaan-2 sm:px-5"
                      >
                        <span className="min-w-0 truncate text-label text-teks-utama">
                          {s.product_name}
                        </span>
                        <span
                          className={cn(
                            'shrink-0 text-label tabular-nums',
                            habis
                              ? 'rounded-full bg-permukaan-2 px-2 py-0.5 font-semibold text-jingga-700'
                              : 'text-teks-sekunder',
                          )}
                        >
                          {formatSisaStok(s.qty, s.unit_name)}
                        </span>
                      </Link>
                    </li>
                  )
                })}
              </ul>
            </KartuBagian>
          )}
        </div>
      )}
    </div>
  )
}

/**
 * Menyusun deret nilai harian sepanjang `jumlah` hari terakhir, hari ini di
 * posisi terakhir. Hari yang tidak dikirim backend diisi nol, bukan dilewati.
 */
function deretTujuhHari(
  rows: { key: string; net_amount: number }[] | undefined,
  jumlah: number,
): number[] | undefined {
  if (!rows) return undefined
  const perTanggal = new Map(rows.map((r) => [r.key, r.net_amount]))
  return Array.from({ length: jumlah }, (_, i) => {
    const t = tanggalISO(new Date(Date.now() - (jumlah - 1 - i) * 86_400_000))
    return perTanggal.get(t) ?? 0
  })
}

/** Rata-rata belanja per transaksi; nol transaksi berarti nol, bukan NaN. */
function rataBelanja(uangMasuk?: number, jumlah?: number): number {
  if (!uangMasuk || !jumlah) return 0
  return Math.round(uangMasuk / jumlah)
}

function sapaan(): string {
  const jam = new Date().getHours()
  if (jam < 11) return 'Selamat pagi'
  if (jam < 15) return 'Selamat siang'
  if (jam < 18) return 'Selamat sore'
  return 'Selamat malam'
}
