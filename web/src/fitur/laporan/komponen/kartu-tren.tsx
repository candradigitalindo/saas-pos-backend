import { lazy, Suspense } from 'react'
import { useQuery } from '@tanstack/react-query'
import { BarChart3 } from 'lucide-react'
import { KartuBagian } from '@/bersama/komponen/kartu-bagian'
import { LencanaSelisih } from '@/bersama/komponen/kartu-angka'
import { Kerangka } from '@/bersama/komponen/kerangka'
import { Tombol } from '@/bersama/ui/tombol'
import { formatRupiah, persenSelisih } from '@/bersama/util/uang'
import { laporanApi } from '../api'
import {
  labelHariPendek,
  labelRentang,
  lengkapiHari,
  periodeBerpembanding,
  ringkasPeriode,
} from '../deret'

// Recharts diunduh setelah angka-angkanya tampil, bukan menahan Beranda.
const GrafikHarian = lazy(async () => ({
  default: (await import('./grafik-harian')).GrafikHarian,
}))

export type PanjangPeriode = 7 | 30

/**
 * Tren uang masuk beberapa hari terakhir — grafik batang per hari, total
 * periodenya, dan perbandingannya dengan periode sepanjang itu tepat sebelumnya.
 *
 * Kartu sorotan di atasnya menjawab "hari ini berapa"; kartu ini menjawab
 * pertanyaan berikutnya yang selalu menyusul: "ramai atau sepi, dibanding
 * biasanya?". Satu hari tidak bisa menjawabnya — kemarin bisa saja hari libur.
 *
 * Hari tanpa penjualan digambar sebagai batang nol (lengkapiHari), dan periode
 * yang sama sekali kosong tidak digambar sebagai grafik datar: ia diberi
 * kalimat, plus jalan pintas ke periode yang lebih panjang.
 */
export function KartuTren({
  tokoAktif,
  hariIni,
  hari,
  onGantiHari,
}: {
  tokoAktif?: string
  hariIni: string
  hari: PanjangPeriode
  onGantiHari: (h: PanjangPeriode) => void
}) {
  const p = periodeBerpembanding(hariIni, hari)

  // Satu permintaan untuk dua periode — lihat periodeBerpembanding.
  const q = useQuery({
    queryKey: ['tren-harian', tokoAktif, p.dariLalu, p.sampai],
    queryFn: () => laporanApi.penjualan(p.dariLalu, p.sampai, 'day', tokoAktif),
  })

  const deret = lengkapiHari(q.data?.rows, p.dari, p.sampai)
  const kini = ringkasPeriode(deret)
  const lalu = ringkasPeriode(lengkapiHari(q.data?.rows, p.dariLalu, p.sampaiLalu))
  const persen = persenSelisih(kini.uangMasuk, lalu.uangMasuk)
  const kosong = kini.transaksi === 0 && kini.uangMasuk === 0

  return (
    <KartuBagian
      judul="Uang masuk"
      keterangan={`${hari} hari terakhir · ${labelRentang(p.dari, p.sampai)}`}
    >
      {q.isLoading ? (
        <div className="flex flex-col gap-3">
          <Kerangka className="h-9 w-48" />
          <Kerangka className="h-56 w-full" />
        </div>
      ) : q.isError ? (
        <p className="py-8 text-center text-label text-teks-sekunder">
          Grafik belum bisa dimuat. Coba lagi sebentar lagi.
        </p>
      ) : kosong ? (
        <div className="flex flex-col items-center gap-2 py-8 text-center">
          <BarChart3 className="h-10 w-10 text-teks-redup" aria-hidden />
          <p className="text-isi font-semibold text-teks-utama">
            Belum ada penjualan dalam {hari} hari terakhir
          </p>
          <p className="max-w-sm text-label text-teks-sekunder">
            {lalu.uangMasuk > 0
              ? `${hari} hari sebelumnya ada ${formatRupiah(lalu.uangMasuk)} uang masuk.`
              : 'Angkanya akan muncul di sini begitu ada transaksi.'}
          </p>
          {hari === 7 && (
            <Tombol jenis="kedua" ukuran="padat" className="mt-1" onClick={() => onGantiHari(30)}>
              Lihat 30 hari terakhir
            </Tombol>
          )}
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
            <p className="text-judul font-extrabold tabular-nums text-teks-utama">
              {formatRupiah(kini.uangMasuk)}
            </p>
            {persen !== null ? (
              <LencanaSelisih persen={persen} label={`dari ${hari} hari sebelumnya`} />
            ) : (
              <span className="text-keterangan text-teks-redup">
                Belum ada pembanding dari {hari} hari sebelumnya
              </span>
            )}
          </div>

          <Suspense fallback={<Kerangka className="h-56 w-full" />}>
            <GrafikHarian rows={deret} />
          </Suspense>

          <dl className="grid grid-cols-3 gap-3 border-t border-garis pt-3">
            <Angka label="Transaksi" nilai={kini.transaksi.toLocaleString('id-ID')} />
            <Angka label="Rata-rata/hari" nilai={formatRupiah(kini.rataPerHari)} />
            <Angka
              label="Hari teramai"
              nilai={kini.hariTeramai ? labelHariPendek(kini.hariTeramai.tanggal) : '—'}
            />
          </dl>
        </div>
      )}
    </KartuBagian>
  )
}

function Angka({ label, nilai }: { label: string; nilai: string }) {
  return (
    <div className="min-w-0">
      <dt className="truncate text-keterangan text-teks-redup">{label}</dt>
      <dd className="truncate text-label font-semibold tabular-nums text-teks-utama">{nilai}</dd>
    </div>
  )
}
