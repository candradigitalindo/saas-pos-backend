import { createBrowserRouter, Navigate, RouterProvider } from 'react-router-dom'
import { LayoutKosong } from './layouts/layout-kosong'
import { LayoutToko } from './layouts/layout-toko'
import { ButuhIzin, ButuhMasuk, TamuSaja } from './penjaga'
import { HalamanMasuk } from '@/fitur/auth/halaman/halaman-masuk'
import { HalamanDaftar } from '@/fitur/auth/halaman/halaman-daftar'
import { HalamanSelamatDatang } from '@/fitur/onboarding/halaman/halaman-selamat-datang'
import { HalamanBeranda } from '@/fitur/beranda/halaman/halaman-beranda'
import { HalamanLainnya } from '@/fitur/beranda/halaman/halaman-lainnya'
import { HalamanKasir } from '@/fitur/kasir/halaman/halaman-kasir'
import { HalamanTutupShift } from '@/fitur/kasir/halaman/halaman-tutup-shift'
import { HalamanRiwayat } from '@/fitur/kasir/halaman/halaman-riwayat'
import { HalamanKas } from '@/fitur/kasir/halaman/halaman-kas'
import { HalamanDaftarBarang } from '@/fitur/produk/halaman/halaman-daftar-barang'
import { HalamanFormBarang } from '@/fitur/produk/halaman/halaman-form-barang'
import { HalamanImpor } from '@/fitur/produk/halaman/halaman-impor'
import { HalamanMaster } from '@/fitur/produk/halaman/halaman-master'
import { HalamanStok } from '@/fitur/stok/halaman/halaman-stok'
import { HalamanKartuStok } from '@/fitur/stok/halaman/halaman-kartu-stok'
import { HalamanKoreksiStok } from '@/fitur/stok/halaman/halaman-koreksi-stok'
import { HalamanBarangMasuk } from '@/fitur/stok/halaman/halaman-barang-masuk'
import { KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { Compass } from 'lucide-react'
import { IZIN } from '@/lib/izin'

/**
 * Rute aplikasi toko.
 *
 * Realm mitra (/mitra/*) dan panel internal (/panel/*) sengaja BUKAN bagian
 * dari pohon ini: tokennya berbeda dan ditolak silang oleh backend, jadi
 * layout, penyimpanan sesi, dan menunya juga dipisah.
 */
const router = createBrowserRouter([
  {
    element: (
      <TamuSaja>
        <LayoutKosong />
      </TamuSaja>
    ),
    children: [
      { path: '/masuk', element: <HalamanMasuk /> },
      { path: '/daftar', element: <HalamanDaftar /> },
    ],
  },
  {
    path: '/selamat-datang',
    element: (
      <ButuhMasuk>
        <HalamanSelamatDatang />
      </ButuhMasuk>
    ),
  },
  // Kasir sengaja DI LUAR LayoutToko: mode fokus satu layar penuh, tanpa
  // navigasi. Semakin sedikit pintu, semakin kecil peluang tersesat saat
  // antrean panjang (ui/04-PETA-LAYAR.md).
  {
    path: '/kasir',
    element: (
      <ButuhMasuk>
        <ButuhIzin izin={[IZIN.saleCreate]}>
          <HalamanKasir />
        </ButuhIzin>
      </ButuhMasuk>
    ),
  },
  {
    element: (
      <ButuhMasuk>
        <LayoutToko />
      </ButuhMasuk>
    ),
    children: [
      { index: true, element: <HalamanBeranda /> },
      { path: '/lainnya', element: <HalamanLainnya /> },
      {
        path: '/kasir/tutup-shift',
        element: (
          <ButuhIzin izin={[IZIN.shiftClose]}>
            <HalamanTutupShift />
          </ButuhIzin>
        ),
      },
      {
        path: '/kasir/riwayat',
        element: (
          <ButuhIzin izin={[IZIN.saleCreate]}>
            <HalamanRiwayat />
          </ButuhIzin>
        ),
      },
      {
        path: '/kasir/kas',
        element: (
          <ButuhIzin izin={[IZIN.cashMovement]}>
            <HalamanKas />
          </ButuhIzin>
        ),
      },
      {
        path: '/barang',
        element: (
          <ButuhIzin izin={[IZIN.productView]}>
            <HalamanDaftarBarang />
          </ButuhIzin>
        ),
      },
      {
        path: '/barang/baru',
        element: (
          <ButuhIzin izin={[IZIN.productEdit]}>
            <HalamanFormBarang />
          </ButuhIzin>
        ),
      },
      {
        path: '/barang/impor',
        element: (
          <ButuhIzin izin={[IZIN.productImport]}>
            <HalamanImpor />
          </ButuhIzin>
        ),
      },
      {
        path: '/barang/master',
        element: (
          <ButuhIzin izin={[IZIN.productEdit]}>
            <HalamanMaster />
          </ButuhIzin>
        ),
      },
      {
        // Setelah /barang/baru dan /barang/impor supaya keduanya tidak tertelan
        // sebagai id barang.
        path: '/barang/:id',
        element: (
          <ButuhIzin izin={[IZIN.productEdit]}>
            <HalamanFormBarang />
          </ButuhIzin>
        ),
      },
      {
        path: '/stok',
        element: (
          <ButuhIzin izin={[IZIN.stockView]}>
            <HalamanStok />
          </ButuhIzin>
        ),
      },
      {
        path: '/stok/kartu/:productId',
        element: (
          <ButuhIzin izin={[IZIN.stockView]}>
            <HalamanKartuStok />
          </ButuhIzin>
        ),
      },
      {
        path: '/stok/koreksi',
        element: (
          <ButuhIzin izin={[IZIN.stockAdjust]}>
            <HalamanKoreksiStok />
          </ButuhIzin>
        ),
      },
      {
        path: '/stok/masuk',
        element: (
          <ButuhIzin izin={[IZIN.stockAdjust]}>
            <HalamanBarangMasuk />
          </ButuhIzin>
        ),
      },
      {
        path: '/laporan',
        element: (
          <ButuhIzin izin={[IZIN.reportView]}>
            <BelumDibangun nama="Laporan" tahap="U4" />
          </ButuhIzin>
        ),
      },
    ],
  },
  { path: '*', element: <Navigate to="/" replace /> },
])

export function Rute() {
  return <RouterProvider router={router} />
}

/**
 * Penanda tahap roadmap yang belum dikerjakan. Ada supaya menu tidak pernah
 * membawa ke layar kosong tanpa penjelasan — "tidak ada jalan buntu" berlaku
 * juga untuk aplikasi yang masih dibangun.
 */
function BelumDibangun({ nama, tahap }: { nama: string; tahap: string }) {
  return (
    <KeadaanKosong
      ikon={Compass}
      judul={`${nama} sedang disiapkan`}
      penjelasan={`Layar ini dibangun pada tahap ${tahap}. Menunya sudah terpasang supaya alurnya bisa diuji lebih dulu.`}
    />
  )
}
