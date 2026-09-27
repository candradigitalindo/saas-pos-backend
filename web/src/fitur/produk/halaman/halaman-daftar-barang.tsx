import { useState } from 'react'
import { Link } from 'react-router-dom'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { ChevronLeft, ChevronRight, Package, Plus, Search, Upload } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { SegmenPilihan } from '@/bersama/ui/segmen'
import { FotoBarang } from '@/bersama/komponen/foto-barang'
import { KeadaanGagal, KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { LencanaStatus } from '@/bersama/komponen/lencana-status'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { useKategori } from '@/bersama/hooks/use-katalog'
import { GalatAPI } from '@/lib/api-client'
import { IZIN } from '@/lib/izin'
import { formatRupiah } from '@/bersama/util/uang'
import { formatSisaStok, stokPerluDicocokkan } from '@/bersama/util/desimal'
import { kelasPetak, petaWarnaKategori } from '@/bersama/util/warna-kategori'
import { cn } from '@/bersama/util/cn'
import type { Produk, SaldoStok } from '@/bersama/tipe/katalog'
import { stokApi } from '@/fitur/stok/api'
import { produkApi } from '../api'

/** Barang per halaman. */
const PER_HALAMAN = 25

/**
 * Daftar barang.
 *
 * Layar lebar memakai tabel, layar kecil memakai kartu bertumpuk — komponen
 * yang sama, tampilan berbeda. BUKAN tabel yang digeser ke samping: kolom
 * penting akan hilang di luar layar HP.
 *
 * Isinya mengikuti daftar produk di aplikasi SaaS kasir pada umumnya: gambar
 * mini (foto, atau petak berwarna kategori), SKU di bawah nama, margin, dan
 * STOK di toko yang sedang dipakai — sebelumnya pemilik harus pindah ke
 * halaman Stok hanya untuk tahu barang yang sedang ia ubah harganya masih ada
 * atau tidak. Saring kategori & nomor halaman ada di atas & bawah tabel.
 */
export function HalamanDaftarBarang() {
  const { boleh, tokoAktif } = useSesi()
  const [cari, setCari] = useState('')
  const [kategori, setKategori] = useState('semua')
  const [halaman, setHalaman] = useState(1)

  const q = useQuery({
    queryKey: ['produk', cari, kategori, halaman],
    queryFn: () =>
      produkApi.daftar(
        cari || undefined,
        kategori === 'semua' ? undefined : kategori,
        halaman,
        PER_HALAMAN,
      ),
    staleTime: 30_000,
    // Tabel tidak berkedip kosong setiap pindah halaman.
    placeholderData: keepPreviousData,
  })
  const daftar = q.data?.data ?? []
  const total = q.data?.total ?? 0
  const jumlahHalaman = Math.max(1, Math.ceil(total / PER_HALAMAN))

  const kat = useKategori()
  const semuaKategori = kat.data?.data ?? []
  const warna = petaWarnaKategori(semuaKategori)

  // Saldo HANYA untuk barang di halaman ini (product_ids), bukan seluruh toko.
  const bolehStok = boleh(IZIN.stockView) && !!tokoAktif
  const ids = daftar.map((p) => p.id)
  const stok = useQuery({
    queryKey: ['stok-barang', tokoAktif, ids.join(',')],
    queryFn: () => stokApi.saldo(tokoAktif!, false, 1, PER_HALAMAN, { produk: ids }),
    enabled: bolehStok && ids.length > 0,
    staleTime: 30_000,
  })
  const petaStok = new Map((stok.data?.data ?? []).map((s) => [s.product_id, s]))

  const gantiSaring = (f: () => void) => {
    f()
    setHalaman(1)
  }
  const bolehUbah = boleh(IZIN.productEdit)

  return (
    <div className="flex flex-col gap-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-judul font-bold text-teks-utama">Barang</h1>
          {q.data && (
            <p className="text-label text-teks-sekunder">
              {total.toLocaleString('id-ID')} barang
              {kategori !== 'semua' || cari ? ' cocok' : ''}
            </p>
          )}
        </div>
        <div className="flex gap-2">
          {boleh(IZIN.productImport) && (
            <Tombol jenis="kedua" asChild>
              <Link to="/barang/impor">
                <Upload className="h-5 w-5" aria-hidden />
                Impor
              </Link>
            </Tombol>
          )}
          {bolehUbah && (
            <Tombol asChild>
              <Link to="/barang/baru">
                <Plus className="h-5 w-5" aria-hidden />
                Tambah Barang
              </Link>
            </Tombol>
          )}
        </div>
      </header>

      <div className="flex flex-col gap-3">
        <div className="relative">
          <Search
            className="pointer-events-none absolute left-3 top-1/2 h-5 w-5 -translate-y-1/2 text-teks-redup"
            aria-hidden
          />
          <input
            type="search"
            value={cari}
            onChange={(e) => gantiSaring(() => setCari(e.target.value))}
            placeholder="Cari nama, SKU, atau barcode…"
            aria-label="Cari barang"
            className="h-12 w-full rounded-kontrol border border-garis bg-permukaan pl-10 pr-3 text-isi text-teks-utama placeholder:text-teks-redup"
          />
        </div>
        {semuaKategori.length > 1 && (
          <SegmenPilihan
            label="Saring kategori"
            nilai={kategori}
            onPilih={(v) => gantiSaring(() => setKategori(v))}
            pilihan={[
              ['semua', 'Semua'] as const,
              ...semuaKategori.map((k) => [k.id, k.name] as const),
            ]}
          />
        )}
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
          judul={cari || kategori !== 'semua' ? 'Barang tidak ditemukan' : 'Belum ada barang di sini'}
          penjelasan={
            cari
              ? `Tidak ada barang bernama "${cari}". Coba kata lain.`
              : kategori !== 'semua'
                ? 'Belum ada barang di kategori ini.'
                : 'Tambahkan barang dulu supaya bisa mulai berjualan.'
          }
          aksi={
            cari || kategori !== 'semua'
              ? {
                  label: 'Tampilkan semua barang',
                  onKlik: () =>
                    gantiSaring(() => {
                      setCari('')
                      setKategori('semua')
                    }),
                }
              : undefined
          }
        />
      ) : (
        <>
          {/* Layar kecil: kartu bertumpuk. */}
          <ul className="flex flex-col gap-2 lg:hidden">
            {daftar.map((p) => (
              <li key={p.id}>
                <Kartu className="flex items-center gap-3 p-3">
                  <FotoBarang
                    nama={p.name}
                    url={p.image_url}
                    kecil
                    kelasWarna={kelasPetak(warna, p.category_id)}
                    className="w-12"
                  />
                  <div className="min-w-0 flex-1">
                    <p className="truncate font-semibold text-teks-utama">{p.name}</p>
                    {/* Harga + stok. Kategori tidak diulang di sini: warna
                        petaknya dan penyaring di atas sudah menyebutnya, dan
                        baris HP tidak muat tiga keterangan. */}
                    {/* Membungkus, tidak dipotong: di HP 320px elipsis dulu
                        memakan harga & sisa stok — informasi terpenting baris ini. */}
                    <p className="text-keterangan tabular-nums text-teks-redup">
                      <span className="whitespace-nowrap">{formatRupiah(p.sell_price)}</span>
                      {bolehStok && p.track_stock ? (
                        <span className="whitespace-nowrap"> · {teksStok(petaStok.get(p.id), p)}</span>
                      ) : (
                        p.category_name && ` · ${p.category_name}`
                      )}
                    </p>
                  </div>
                  {bolehUbah && (
                    <Tombol jenis="kedua" ukuran="padat" asChild>
                      <Link to={`/barang/${p.id}`} aria-label={`Ubah ${p.name}`}>
                        Ubah
                      </Link>
                    </Tombol>
                  )}
                </Kartu>
              </li>
            ))}
          </ul>

          {/* Layar lebar: tabel. */}
          <div className="hidden overflow-hidden rounded-kartu border border-garis bg-permukaan shadow-kartu lg:block">
            <table className="w-full">
              <thead>
                <tr className="border-b border-garis bg-permukaan-2/60 text-left text-label text-teks-sekunder">
                  <th scope="col" className="px-4 py-3 font-medium">Barang</th>
                  <th scope="col" className="px-3 py-3 font-medium">Kategori</th>
                  <th scope="col" className="px-3 py-3 text-right font-medium">Harga jual</th>
                  <th scope="col" className="px-3 py-3 text-right font-medium">Modal</th>
                  <th scope="col" className="px-3 py-3 text-right font-medium">Margin</th>
                  {bolehStok && (
                    <th scope="col" className="px-3 py-3 text-right font-medium">Stok</th>
                  )}
                  <th scope="col" className="px-4 py-3">
                    <span className="sr-only">Aksi</span>
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-garis">
                {daftar.map((p) => (
                  <tr key={p.id} className="hover:bg-permukaan-2/50">
                    <td className="px-4 py-2.5">
                      <div className="flex items-center gap-3">
                        <FotoBarang
                          nama={p.name}
                          url={p.image_url}
                          kecil
                          kelasWarna={kelasPetak(warna, p.category_id)}
                          className="w-10"
                        />
                        <div className="min-w-0">
                          <p className="flex flex-wrap items-center gap-2 font-medium text-teks-utama">
                            {p.name}
                            {!p.is_active && <LencanaStatus nada="netral" anak="Tidak dijual" />}
                          </p>
                          {(p.sku || p.barcode) && (
                            <p className="text-keterangan tabular-nums text-teks-redup">
                              {p.sku ? `SKU ${p.sku}` : `Barcode ${p.barcode}`}
                            </p>
                          )}
                        </div>
                      </div>
                    </td>
                    <td className="px-3 py-2.5">
                      {p.category_name ? (
                        <span
                          className={cn(
                            'inline-flex rounded-full px-2.5 py-0.5 text-keterangan font-medium',
                            kelasPetak(warna, p.category_id),
                          )}
                        >
                          {p.category_name}
                        </span>
                      ) : (
                        <span className="text-teks-redup">—</span>
                      )}
                    </td>
                    <td className="px-3 py-2.5 text-right font-medium tabular-nums text-teks-utama">
                      {formatRupiah(p.sell_price)}
                    </td>
                    <td className="px-3 py-2.5 text-right tabular-nums text-teks-sekunder">
                      {formatRupiah(p.cost_price)}
                    </td>
                    <td className="px-3 py-2.5 text-right tabular-nums">
                      <Margin jual={p.sell_price} modal={p.cost_price} />
                    </td>
                    {bolehStok && (
                      <td className="px-3 py-2.5 text-right">
                        <SelStok produk={p} saldo={petaStok.get(p.id)} memuat={stok.isLoading} />
                      </td>
                    )}
                    <td className="px-4 py-2.5 text-right">
                      {bolehUbah && (
                        <Tombol jenis="teks" ukuran="padat" asChild>
                          <Link to={`/barang/${p.id}`} aria-label={`Ubah ${p.name}`}>
                            Ubah
                          </Link>
                        </Tombol>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {jumlahHalaman > 1 && (
            <nav
              aria-label="Halaman daftar barang"
              className="flex flex-wrap items-center justify-between gap-3"
            >
              <p className="text-label tabular-nums text-teks-sekunder">
                {((halaman - 1) * PER_HALAMAN + 1).toLocaleString('id-ID')}–
                {Math.min(halaman * PER_HALAMAN, total).toLocaleString('id-ID')} dari{' '}
                {total.toLocaleString('id-ID')} barang
              </p>
              <div className="flex gap-2">
                <Tombol
                  jenis="kedua"
                  ukuran="padat"
                  disabled={halaman <= 1}
                  onClick={() => setHalaman((h) => h - 1)}
                >
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

/**
 * Margin = (jual − modal) ÷ jual. Ditulis dalam persen karena itu yang
 * dibandingkan antarbarang; margin negatif (jual rugi) diberi warna bahaya
 * DAN tanda minus — bukan warna saja.
 */
function Margin({ jual, modal }: { jual: number; modal: number }) {
  if (jual <= 0) return <span className="text-teks-redup">—</span>
  const persen = Math.round(((jual - modal) / jual) * 100)
  return (
    <span className={cn('font-medium', persen < 0 ? 'text-bahaya-teks' : 'text-teks-sekunder')}>
      {persen < 0 ? `−${Math.abs(persen)}` : persen}%
    </span>
  )
}

/** Teks stok ringkas untuk kartu HP. */
function teksStok(saldo: SaldoStok | undefined, p: Produk): string {
  if (!saldo) return 'stok belum dicatat'
  return formatSisaStok(saldo.qty, saldo.unit_name || p.unit_name)
}

/**
 * Sel stok di tabel: angka sisa, dengan lencana bila habis/menipis/minus.
 * Barang yang stoknya tidak dilacak (jasa, racikan) ditulis apa adanya —
 * bukan "0", yang akan terbaca sebagai habis.
 */
function SelStok({
  produk,
  saldo,
  memuat,
}: {
  produk: Produk
  saldo?: SaldoStok
  memuat: boolean
}) {
  if (!produk.track_stock) {
    return <span className="text-keterangan text-teks-redup">Tidak dilacak</span>
  }
  if (memuat) return <span className="text-teks-redup">…</span>
  if (!saldo) return <span className="text-keterangan text-teks-redup">Belum dicatat</span>

  const qty = Number.parseFloat(saldo.qty)
  if (stokPerluDicocokkan(saldo.qty)) {
    return <LencanaStatus nada="bahaya" anak="Perlu dicocokkan" />
  }
  if (qty <= 0) return <LencanaStatus nada="bahaya" anak="Habis" />
  return (
    <span
      className={cn(
        'tabular-nums',
        saldo.low ? 'font-semibold text-jingga-700' : 'text-teks-utama',
      )}
    >
      {formatSisaStok(saldo.qty, saldo.unit_name || produk.unit_name)}
    </span>
  )
}
