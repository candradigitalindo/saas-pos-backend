import { useState } from 'react'
import { Link } from 'react-router-dom'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import type { LucideIcon } from 'lucide-react'
import {
  Ban,
  Boxes,
  ChevronLeft,
  ChevronRight,
  CircleCheck,
  Clock,
  PackagePlus,
  Search,
  SlidersHorizontal,
  TriangleAlert,
} from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { SegmenPilihan } from '@/bersama/ui/segmen'
import { LencanaStatus } from '@/bersama/komponen/lencana-status'
import { KeadaanGagal, KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { Kerangka, KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { GalatAPI } from '@/lib/api-client'
import { IZIN } from '@/lib/izin'
import { formatRupiah } from '@/bersama/util/uang'
import { formatQtySatuan, formatSisaStok, stokPerluDicocokkan } from '@/bersama/util/desimal'
import { cn } from '@/bersama/util/cn'
import type { SaldoStok } from '@/bersama/tipe/katalog'
import { stokApi, type RingkasanStok } from '../api'

/** Baris per halaman — batas atas server. */
const PER_HALAMAN = 100

/**
 * Saldo stok per toko.
 *
 * Di atas daftar ada RINGKASAN seluruh barang: berapa yang aman, hampir habis,
 * habis, dan perlu dicocokkan, plus nilai stok menurut harga modal. Angka ini
 * dihitung server atas semua barang (GET /stocks/summary) — bukan dari halaman
 * yang kebetulan termuat, yang di toko dengan 300 barang hanya 100 baris
 * pertama.
 *
 * Pencarian juga di server (?q=) untuk alasan yang sama: dulu tidak ada kotak
 * cari sama sekali, dan menyaring di sisi klien akan diam-diam melewatkan
 * barang di halaman berikutnya.
 */
export function HalamanStok() {
  const { tokoAktif, boleh } = useSesi()
  const [hanyaMenipis, setHanyaMenipis] = useState(false)
  const [cari, setCari] = useState('')
  const [halaman, setHalaman] = useState(1)

  const ringkasan = useQuery({
    queryKey: ['stok-ringkasan', tokoAktif],
    queryFn: () => stokApi.ringkasan(tokoAktif!),
    enabled: !!tokoAktif,
    staleTime: 30_000,
  })

  const q = useQuery({
    queryKey: ['stok', tokoAktif, hanyaMenipis, cari, halaman],
    queryFn: () =>
      stokApi.saldo(tokoAktif!, hanyaMenipis, halaman, PER_HALAMAN, { cari: cari.trim() }),
    enabled: !!tokoAktif,
    staleTime: 30_000,
    placeholderData: keepPreviousData,
  })

  const daftar = q.data?.data ?? []
  const total = q.data?.total ?? 0
  const jumlahHalaman = Math.max(1, Math.ceil(total / PER_HALAMAN))
  const gantiSaring = (f: () => void) => {
    f()
    setHalaman(1)
  }

  return (
    <div className="flex flex-col gap-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-judul font-bold text-teks-utama">Stok</h1>
        <div className="flex gap-2">
          {boleh(IZIN.stockAdjust) && (
            <>
              <Tombol jenis="kedua" asChild>
                <Link to="/stok/koreksi">
                  <SlidersHorizontal className="h-5 w-5" aria-hidden />
                  Koreksi
                </Link>
              </Tombol>
              <Tombol asChild>
                <Link to="/stok/masuk">
                  <PackagePlus className="h-5 w-5" aria-hidden />
                  Barang Masuk
                </Link>
              </Tombol>
            </>
          )}
        </div>
      </header>

      <Ringkasan
        data={ringkasan.data}
        memuat={ringkasan.isLoading}
        onLihatMenipis={() => gantiSaring(() => setHanyaMenipis(true))}
      />

      <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
        <div className="relative flex-1">
          <Search
            className="pointer-events-none absolute left-3 top-1/2 h-5 w-5 -translate-y-1/2 text-teks-redup"
            aria-hidden
          />
          <input
            type="search"
            value={cari}
            onChange={(e) => gantiSaring(() => setCari(e.target.value))}
            placeholder="Cari nama, SKU, atau barcode…"
            aria-label="Cari barang di stok"
            className="h-12 w-full rounded-kontrol border border-garis bg-permukaan pl-10 pr-3 text-isi text-teks-utama placeholder:text-teks-redup"
          />
        </div>
        <SegmenPilihan
          label="Saring stok"
          nilai={hanyaMenipis ? 'menipis' : 'semua'}
          onPilih={(v) => gantiSaring(() => setHanyaMenipis(v === 'menipis'))}
          pilihan={[
            ['semua', 'Semua barang'],
            ['menipis', 'Perlu belanja'],
          ]}
        />
      </div>

      {q.isLoading ? (
        <KerangkaBaris jumlah={5} />
      ) : q.isError ? (
        <KeadaanGagal
          pesan={q.error instanceof GalatAPI ? q.error.pesan : 'Saldo stok belum bisa dimuat.'}
          onCobaLagi={() => q.refetch()}
        />
      ) : daftar.length === 0 ? (
        <KeadaanKosong
          ikon={Boxes}
          judul={
            cari
              ? 'Barang tidak ditemukan'
              : hanyaMenipis
                ? 'Tidak ada barang yang perlu dibeli'
                : 'Belum ada stok tercatat'
          }
          penjelasan={
            cari
              ? `Tidak ada barang bernama "${cari}" di stok toko ini.`
              : hanyaMenipis
                ? 'Semua stok masih di atas batas yang Anda tentukan. Bagus.'
                : 'Catat barang masuk atau isi stok awal supaya saldonya muncul di sini.'
          }
          aksi={
            cari || hanyaMenipis
              ? {
                  label: 'Lihat semua barang',
                  onKlik: () =>
                    gantiSaring(() => {
                      setCari('')
                      setHanyaMenipis(false)
                    }),
                }
              : undefined
          }
        />
      ) : (
        <>
          {/* HP & tablet: kartu bertumpuk. */}
          <ul className="flex flex-col gap-2 lg:hidden">
            {daftar.map((s) => (
              <li key={`${s.product_id}-${s.variant_id ?? ''}`}>
                {/* Lencana di bawah nama, bukan di sampingnya: berdampingan
                    dengan lencana DAN tombol, nama barang di HP 390px tinggal
                    "Air Mine…". */}
                <Kartu className="flex items-center gap-3 p-4">
                  <div className="min-w-0 flex-1">
                    <p className="truncate font-semibold text-teks-utama">{s.product_name}</p>
                    <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1">
                      <LencanaSaldo saldo={s} />
                      <span className="text-keterangan tabular-nums text-teks-redup">
                        {formatSisaStok(s.qty, s.unit_name)}
                        {Number.parseFloat(s.min_stock) > 0 &&
                          ` · batas ${formatQtySatuan(s.min_stock, s.unit_name)}`}
                      </span>
                    </div>
                  </div>
                  <Tombol jenis="teks" ukuran="padat" asChild>
                    <Link to={`/stok/kartu/${s.product_id}`} aria-label={`Riwayat stok ${s.product_name}`}>
                      Riwayat
                    </Link>
                  </Tombol>
                </Kartu>
              </li>
            ))}
          </ul>

          {/* Layar lebar: tabel — sisa & batas dalam kolom sendiri supaya bisa
              dibandingkan sekali lirik, tidak dibaca dari satu kalimat. */}
          <div className="hidden overflow-hidden rounded-kartu border border-garis bg-permukaan shadow-kartu lg:block">
            <table className="w-full">
              <thead>
                <tr className="border-b border-garis bg-permukaan-2/60 text-left text-label text-teks-sekunder">
                  <th scope="col" className="px-4 py-3 font-medium">Barang</th>
                  <th scope="col" className="px-3 py-3 text-right font-medium">Sisa</th>
                  <th scope="col" className="px-3 py-3 text-right font-medium">Batas minimum</th>
                  <th scope="col" className="px-3 py-3 font-medium">Keadaan</th>
                  <th scope="col" className="px-4 py-3">
                    <span className="sr-only">Aksi</span>
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-garis">
                {daftar.map((s) => (
                  <tr key={`${s.product_id}-${s.variant_id ?? ''}`} className="hover:bg-permukaan-2/50">
                    <td className="px-4 py-3 font-medium text-teks-utama">{s.product_name}</td>
                    <td className="px-3 py-3 text-right font-semibold tabular-nums text-teks-utama">
                      {formatSisaStok(s.qty, s.unit_name)}
                    </td>
                    <td className="px-3 py-3 text-right tabular-nums text-teks-sekunder">
                      {Number.parseFloat(s.min_stock) > 0
                        ? formatQtySatuan(s.min_stock, s.unit_name)
                        : '—'}
                    </td>
                    <td className="px-3 py-3">
                      <LencanaSaldo saldo={s} />
                    </td>
                    <td className="px-4 py-3 text-right">
                      <Tombol jenis="teks" ukuran="padat" asChild>
                        <Link to={`/stok/kartu/${s.product_id}`} aria-label={`Riwayat stok ${s.product_name}`}>
                          Riwayat
                        </Link>
                      </Tombol>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {jumlahHalaman > 1 && (
            <nav aria-label="Halaman daftar stok" className="flex flex-wrap items-center justify-between gap-3">
              <p className="text-label tabular-nums text-teks-sekunder">
                Halaman {halaman} dari {jumlahHalaman} · {total.toLocaleString('id-ID')} barang
              </p>
              <div className="flex gap-2">
                <Tombol jenis="kedua" ukuran="padat" disabled={halaman <= 1} onClick={() => setHalaman((h) => h - 1)}>
                  <ChevronLeft className="h-4 w-4" aria-hidden />
                  Sebelumnya
                </Tombol>
                <Tombol
                  jenis="kedua"
                  ukuran="padat"
                  disabled={halaman >= jumlahHalaman}
                  onClick={() => setHalaman((h) => h + 1)}
                >
                  Berikutnya
                  <ChevronRight className="h-4 w-4" aria-hidden />
                </Tombol>
              </div>
            </nav>
          )}
        </>
      )}
    </div>
  )
}

/** Ikon + teks + warna, bukan warna saja. */
function LencanaSaldo({ saldo }: { saldo: SaldoStok }) {
  if (stokPerluDicocokkan(saldo.qty)) return <LencanaStatus nada="bahaya" anak="Perlu dicocokkan" />
  if (Number.parseFloat(saldo.qty) <= 0) return <LencanaStatus nada="bahaya" anak="Habis" />
  if (saldo.low) return <LencanaStatus nada="menunggu" anak="Hampir habis" />
  return <LencanaStatus nada="berhasil" anak="Aman" />
}

/**
 * Empat hitungan keadaan + nilai stok. Keadaan yang butuh tindakan (hampir
 * habis, habis, minus) bisa ditekan untuk langsung menyaring daftarnya.
 */
function Ringkasan({
  data,
  memuat,
  onLihatMenipis,
}: {
  data?: RingkasanStok
  memuat: boolean
  onLihatMenipis: () => void
}) {
  if (memuat) {
    return (
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-5">
        {Array.from({ length: 5 }, (_, i) => (
          <Kerangka key={i} className="h-21" />
        ))}
      </div>
    )
  }
  if (!data || data.total === 0) return null

  return (
    <div className="grid grid-cols-2 gap-3 lg:grid-cols-5">
      <Angka ikon={CircleCheck} label="Aman" nilai={data.safe} nada="berhasil" />
      <Angka ikon={Clock} label="Hampir habis" nilai={data.low} nada="menunggu" onKlik={data.low ? onLihatMenipis : undefined} />
      <Angka ikon={Ban} label="Habis" nilai={data.out} nada="bahaya" onKlik={data.out ? onLihatMenipis : undefined} />
      <Angka
        ikon={TriangleAlert}
        label="Perlu dicocokkan"
        nilai={data.negative}
        nada="bahaya"
        onKlik={data.negative ? onLihatMenipis : undefined}
      />
      <Kartu className="col-span-2 flex flex-col justify-center p-4 lg:col-span-1">
        <p className="text-keterangan font-medium text-teks-sekunder">Nilai stok (harga modal)</p>
        <p className="mt-1 text-judul-kartu font-extrabold tabular-nums text-teks-utama">
          {formatRupiah(data.stock_value)}
        </p>
      </Kartu>
    </div>
  )
}

const NADA_ANGKA = {
  berhasil: 'text-hijau-700',
  menunggu: 'text-jingga-700',
  bahaya: 'text-bahaya-teks',
} as const

function Angka({
  ikon: Ikon,
  label,
  nilai,
  nada,
  onKlik,
}: {
  ikon: LucideIcon
  label: string
  nilai: number
  nada: keyof typeof NADA_ANGKA
  onKlik?: () => void
}) {
  // Nol ditulis redup: "0 habis" adalah kabar baik, tidak perlu berwarna.
  const isi = (
    <>
      <p className="flex items-center gap-1.5 text-keterangan font-medium text-teks-sekunder">
        <Ikon className={cn('h-4 w-4', nilai ? NADA_ANGKA[nada] : 'text-teks-redup')} aria-hidden />
        {label}
      </p>
      <p
        className={cn(
          'mt-1 text-judul font-extrabold tabular-nums',
          nilai ? 'text-teks-utama' : 'text-teks-redup',
        )}
      >
        {nilai.toLocaleString('id-ID')}
      </p>
    </>
  )
  if (!onKlik) return <Kartu className="p-4">{isi}</Kartu>
  return (
    <button
      type="button"
      onClick={onKlik}
      aria-label={`${label}: ${nilai}. Tampilkan barang yang perlu dibeli`}
      className="rounded-kartu border border-garis bg-permukaan p-4 text-left shadow-kartu transition-colors hover:border-utama/40 hover:bg-permukaan-2/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-utama"
    >
      {isi}
    </button>
  )
}
