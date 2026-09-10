import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Boxes, ChevronRight, PackagePlus, Receipt, TriangleAlert } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { KartuAngka } from '@/bersama/komponen/kartu-angka'
import { KerangkaKartuAngka } from '@/bersama/komponen/kerangka'
import { StatusKoneksi } from '@/bersama/komponen/status-koneksi'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { useSinkron } from '@/lib/offline/mesin'
import { api } from '@/lib/api-client'
import { IZIN } from '@/lib/izin'
import { tanggalISO } from '@/bersama/util/tanggal'
import { formatSisaStok } from '@/bersama/util/desimal'
import { cn } from '@/bersama/util/cn'
import type { Halaman } from '@/lib/api-client'
import type { SaldoStok } from '@/bersama/tipe/katalog'
import { laporanApi } from '../api'

/**
 * Beranda pemilik.
 *
 * Sasarannya: "hari ini untung berapa" terjawab dalam NOL ketukan. Karena itu
 * angkanya ada di layar pertama, bukan di balik menu Laporan.
 */
export function HalamanBeranda() {
  const { profil, boleh, tokoAktif } = useSesi()
  const sinkron = useSinkron()
  const nama = profil?.user.name?.split(' ')[0] ?? ''

  const bolehLihatLaporan = boleh(IZIN.reportView)
  const bolehLihatUntung = boleh(IZIN.reportProfit)

  const hariIni = tanggalISO()
  const kemarin = tanggalISO(new Date(Date.now() - 86_400_000))

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

  return (
    <div className="flex flex-col gap-4">
      <header className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-judul font-bold text-teks-utama">
            {sapaan()}
            {nama && `, ${nama}`}
          </h1>
          <p className="text-label text-teks-sekunder">{profil?.tenant.business_name}</p>
        </div>
      </header>

      <StatusKoneksi menunggu={sinkron.menunggu} />

      {sinkron.perluDiperiksa > 0 && (
        <Link to="/kasir/belum-terkirim">
          <Kartu className="flex items-center gap-3 border-jingga-600 bg-jingga-100 p-4">
            <TriangleAlert className="h-5 w-5 shrink-0 text-jingga-700" aria-hidden />
            <p className="flex-1 text-label font-medium text-jingga-700">
              {sinkron.perluDiperiksa} transaksi perlu diperiksa
            </p>
            <ChevronRight className="h-5 w-5 text-jingga-700" aria-hidden />
          </Kartu>
        </Link>
      )}

      {bolehLihatLaporan &&
        (dashboard.isLoading ? (
          <div className="grid gap-3 sm:grid-cols-2">
            <KerangkaKartuAngka />
            <KerangkaKartuAngka />
          </div>
        ) : (
          <div className="grid gap-3 sm:grid-cols-2">
            <KartuAngka
              label="Uang masuk hari ini"
              nilai={hari?.net_amount ?? 0}
              pembanding={kmr?.net_amount}
            />
            {bolehLihatUntung ? (
              <KartuAngka
                label="Untung kotor hari ini"
                nilai={hari?.gross_profit ?? 0}
                pembanding={kmr?.gross_profit}
              />
            ) : (
              <KartuAngka
                label="Jumlah transaksi hari ini"
                nilai={hari?.sales_count ?? 0}
                pembanding={kmr?.sales_count}
                uang={false}
              />
            )}
          </div>
        ))}

      {pintasan.length > 0 && (
        <section aria-label="Pintasan" className="grid grid-cols-2 gap-3 sm:grid-cols-3">
          {pintasan.map((p) => (
            <Link
              key={p.ke}
              to={p.ke}
              className={cn(
                'flex min-h-24 flex-col items-center justify-center gap-2 rounded-kartu',
                'border border-garis bg-permukaan p-4 text-center shadow-kartu',
                'hover:bg-permukaan-2',
              )}
            >
              <p.ikon className="h-7 w-7 text-utama" aria-hidden />
              <span className="text-label font-semibold text-teks-utama">{p.label}</span>
            </Link>
          ))}
        </section>
      )}

      {(stokMenipis.data?.data.length ?? 0) > 0 && (
        <section className="flex flex-col gap-2">
          <h2 className="text-judul-kartu font-semibold text-teks-utama">
            Hampir habis
          </h2>
          <Kartu className="divide-y divide-garis">
            {stokMenipis.data?.data.map((s) => (
              <Link
                key={s.product_id}
                to="/stok"
                className="flex min-h-14 items-center justify-between gap-3 px-4 hover:bg-permukaan-2"
              >
                <span className="min-w-0 truncate text-isi text-teks-utama">
                  {s.product_name}
                </span>
                <span className="shrink-0 text-label tabular-nums text-jingga-700">
                  {formatSisaStok(s.qty, s.unit_name)}
                </span>
              </Link>
            ))}
          </Kartu>
        </section>
      )}

      {bolehLihatLaporan && (hari?.sales_count ?? 0) === 0 && !dashboard.isLoading && (
        <Kartu className="p-4">
          <p className="text-label text-teks-sekunder">
            Belum ada penjualan hari ini. Angkanya akan muncul di sini begitu ada
            transaksi pertama.
          </p>
        </Kartu>
      )}
    </div>
  )
}

function sapaan(): string {
  const jam = new Date().getHours()
  if (jam < 11) return 'Selamat pagi'
  if (jam < 15) return 'Selamat siang'
  if (jam < 18) return 'Selamat sore'
  return 'Selamat malam'
}
