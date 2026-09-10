import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Boxes, PackagePlus, SlidersHorizontal } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { LencanaStatus } from '@/bersama/komponen/lencana-status'
import { KeadaanGagal, KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { GalatAPI } from '@/lib/api-client'
import { IZIN } from '@/lib/izin'
import { formatQtySatuan, formatSisaStok, stokPerluDicocokkan } from '@/bersama/util/desimal'
import { cn } from '@/bersama/util/cn'
import { stokApi } from '../api'

/** Saldo stok per toko, dengan penyaring "hampir habis". */
export function HalamanStok() {
  const { tokoAktif, boleh } = useSesi()
  const [hanyaMenipis, setHanyaMenipis] = useState(false)

  const q = useQuery({
    queryKey: ['stok', tokoAktif, hanyaMenipis],
    queryFn: () => stokApi.saldo(tokoAktif!, hanyaMenipis),
    enabled: !!tokoAktif,
    staleTime: 30_000,
  })

  const daftar = q.data?.data ?? []

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

      <div className="flex gap-2">
        <TombolSaring aktif={!hanyaMenipis} onKlik={() => setHanyaMenipis(false)}>
          Semua barang
        </TombolSaring>
        <TombolSaring aktif={hanyaMenipis} onKlik={() => setHanyaMenipis(true)}>
          Hampir habis
        </TombolSaring>
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
          judul={hanyaMenipis ? 'Tidak ada barang yang hampir habis' : 'Belum ada stok tercatat'}
          penjelasan={
            hanyaMenipis
              ? 'Semua stok masih di atas batas yang Anda tentukan. Bagus.'
              : 'Catat barang masuk atau isi stok awal supaya saldonya muncul di sini.'
          }
          aksi={
            hanyaMenipis
              ? { label: 'Lihat semua barang', onKlik: () => setHanyaMenipis(false) }
              : undefined
          }
        />
      ) : (
        <ul className="flex flex-col gap-2">
          {daftar.map((s) => (
            <li key={`${s.product_id}-${s.variant_id ?? ''}`}>
              <Kartu className="flex items-center justify-between gap-3 p-4">
                <div className="min-w-0">
                  <p className="truncate font-semibold text-teks-utama">
                    {s.product_name}
                  </p>
                  <p className="text-keterangan tabular-nums text-teks-redup">
                    {formatSisaStok(s.qty, s.unit_name)}
                    {Number.parseFloat(s.min_stock) > 0 &&
                      ` · batas ${formatQtySatuan(s.min_stock, s.unit_name)}`}
                  </p>
                </div>

                <div className="flex shrink-0 items-center gap-2">
                  {/* Ikon + teks + warna, bukan warna saja. */}
                  {stokPerluDicocokkan(s.qty) ? (
                    <LencanaStatus nada="bahaya" anak="Perlu dicocokkan" />
                  ) : Number.parseFloat(s.qty) <= 0 ? (
                    <LencanaStatus nada="bahaya" anak="Habis" />
                  ) : s.low ? (
                    <LencanaStatus nada="menunggu" anak="Hampir habis" />
                  ) : (
                    <LencanaStatus nada="berhasil" anak="Aman" />
                  )}
                  <Tombol jenis="teks" ukuran="padat" asChild>
                    <Link to={`/stok/kartu/${s.product_id}`}>Riwayat</Link>
                  </Tombol>
                </div>
              </Kartu>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

function TombolSaring({
  aktif,
  onKlik,
  children,
}: {
  aktif: boolean
  onKlik: () => void
  children: React.ReactNode
}) {
  return (
    <button
      type="button"
      onClick={onKlik}
      aria-pressed={aktif}
      className={cn(
        'h-12 rounded-full border px-4 text-label font-medium',
        aktif
          ? 'border-utama bg-sorot text-utama'
          : 'border-garis bg-permukaan text-teks-sekunder hover:bg-permukaan-2',
      )}
    >
      {children}
    </button>
  )
}
