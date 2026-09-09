import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Package, Plus, Search, Upload } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { KeadaanGagal, KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { GalatAPI } from '@/lib/api-client'
import { IZIN } from '@/lib/izin'
import { formatRupiah } from '@/bersama/util/uang'
import { useQuery } from '@tanstack/react-query'
import { produkApi } from '../api'

/**
 * Daftar barang.
 *
 * Layar lebar memakai tabel, layar kecil memakai kartu bertumpuk — komponen
 * yang sama, tampilan berbeda. BUKAN tabel yang digeser ke samping: kolom
 * penting akan hilang di luar layar HP.
 */
export function HalamanDaftarBarang() {
  const { boleh } = useSesi()
  const [cari, setCari] = useState('')

  const q = useQuery({
    queryKey: ['produk', cari],
    queryFn: () => produkApi.daftar(cari || undefined),
    staleTime: 30_000,
  })

  const daftar = q.data?.data ?? []

  return (
    <div className="flex flex-col gap-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-judul font-bold text-teks-utama">Barang</h1>
        <div className="flex gap-2">
          {boleh(IZIN.productImport) && (
            <Tombol jenis="kedua" asChild>
              <Link to="/barang/impor">
                <Upload className="h-5 w-5" aria-hidden />
                Impor
              </Link>
            </Tombol>
          )}
          {boleh(IZIN.productEdit) && (
            <Tombol asChild>
              <Link to="/barang/baru">
                <Plus className="h-5 w-5" aria-hidden />
                Tambah Barang
              </Link>
            </Tombol>
          )}
        </div>
      </header>

      <div className="relative">
        <Search
          className="pointer-events-none absolute left-3 top-1/2 h-5 w-5 -translate-y-1/2 text-teks-redup"
          aria-hidden
        />
        <input
          type="search"
          value={cari}
          onChange={(e) => setCari(e.target.value)}
          placeholder="Cari nama, SKU, atau barcode…"
          aria-label="Cari barang"
          className="h-12 w-full rounded-kontrol border border-garis bg-permukaan pl-10 pr-3 text-isi text-teks-utama placeholder:text-teks-redup"
        />
      </div>

      {q.isLoading ? (
        <KerangkaBaris jumlah={5} />
      ) : q.isError ? (
        <KeadaanGagal
          pesan={q.error instanceof GalatAPI ? q.error.pesan : 'Daftar barang belum bisa dimuat.'}
          onCobaLagi={() => q.refetch()}
        />
      ) : daftar.length === 0 ? (
        <KeadaanKosong
          ikon={Package}
          judul={cari ? 'Barang tidak ditemukan' : 'Belum ada barang di sini'}
          penjelasan={
            cari
              ? `Tidak ada barang bernama "${cari}". Coba kata lain.`
              : 'Tambahkan barang dulu supaya bisa mulai berjualan.'
          }
          aksi={
            cari
              ? { label: 'Hapus pencarian', onKlik: () => setCari('') }
              : undefined
          }
        />
      ) : (
        <>
          {/* Layar kecil: kartu bertumpuk. */}
          <ul className="flex flex-col gap-2 lg:hidden">
            {daftar.map((p) => (
              <li key={p.id}>
                <Kartu className="flex items-center justify-between gap-3 p-4">
                  <div className="min-w-0">
                    <p className="truncate font-semibold text-teks-utama">{p.name}</p>
                    <p className="text-keterangan tabular-nums text-teks-redup">
                      {formatRupiah(p.sell_price)}
                      {p.category_name && ` · ${p.category_name}`}
                    </p>
                  </div>
                  {boleh(IZIN.productEdit) && (
                    <Tombol jenis="kedua" ukuran="padat" asChild>
                      <Link to={`/barang/${p.id}`}>Ubah</Link>
                    </Tombol>
                  )}
                </Kartu>
              </li>
            ))}
          </ul>

          {/* Layar lebar: tabel. */}
          <div className="hidden overflow-hidden rounded-kartu border border-garis bg-permukaan lg:block">
            <table className="w-full">
              <thead>
                <tr className="border-b border-garis text-left text-label text-teks-sekunder">
                  <th scope="col" className="p-3 font-medium">Nama</th>
                  <th scope="col" className="p-3 font-medium">Kategori</th>
                  <th scope="col" className="p-3 text-right font-medium">Harga jual</th>
                  <th scope="col" className="p-3 text-right font-medium">Harga modal</th>
                  <th scope="col" className="p-3 font-medium">Satuan</th>
                  <th scope="col" className="p-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-garis">
                {daftar.map((p) => (
                  <tr key={p.id}>
                    <td className="p-3 font-medium text-teks-utama">
                      {p.name}
                      {!p.is_active && (
                        <span className="ml-2 text-keterangan text-teks-redup">
                          (tidak dijual)
                        </span>
                      )}
                    </td>
                    <td className="p-3 text-teks-sekunder">{p.category_name ?? '—'}</td>
                    <td className="p-3 text-right tabular-nums text-teks-utama">
                      {formatRupiah(p.sell_price)}
                    </td>
                    <td className="p-3 text-right tabular-nums text-teks-sekunder">
                      {formatRupiah(p.cost_price)}
                    </td>
                    <td className="p-3 text-teks-sekunder">{p.unit_name ?? '—'}</td>
                    <td className="p-3 text-right">
                      {boleh(IZIN.productEdit) && (
                        <Tombol jenis="teks" ukuran="padat" asChild>
                          <Link to={`/barang/${p.id}`}>Ubah</Link>
                        </Tombol>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {(q.data?.total ?? 0) > daftar.length && (
            <p className="text-center text-keterangan text-teks-redup">
              Menampilkan {daftar.length} dari {q.data?.total} barang. Gunakan
              pencarian untuk mempersempit.
            </p>
          )}
        </>
      )}
    </div>
  )
}
