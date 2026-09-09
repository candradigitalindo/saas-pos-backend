import { createBrowserRouter, Navigate, RouterProvider } from 'react-router-dom'
import { LayoutKosong } from './layouts/layout-kosong'
import { LayoutToko } from './layouts/layout-toko'
import { ButuhIzin, ButuhMasuk, TamuSaja } from './penjaga'
import { HalamanMasuk } from '@/fitur/auth/halaman/halaman-masuk'
import { HalamanDaftar } from '@/fitur/auth/halaman/halaman-daftar'
import { HalamanSelamatDatang } from '@/fitur/onboarding/halaman/halaman-selamat-datang'
import { HalamanBeranda } from '@/fitur/beranda/halaman/halaman-beranda'
import { HalamanLainnya } from '@/fitur/beranda/halaman/halaman-lainnya'
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
        path: '/kasir',
        element: (
          <ButuhIzin izin={[IZIN.saleCreate]}>
            <BelumDibangun nama="Kasir" tahap="U1" />
          </ButuhIzin>
        ),
      },
      {
        path: '/barang',
        element: (
          <ButuhIzin izin={[IZIN.productView]}>
            <BelumDibangun nama="Barang" tahap="U2" />
          </ButuhIzin>
        ),
      },
      {
        path: '/stok',
        element: (
          <ButuhIzin izin={[IZIN.stockView]}>
            <BelumDibangun nama="Stok" tahap="U2" />
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
