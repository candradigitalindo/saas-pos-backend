import { lazy, Suspense, type ComponentType, type ReactNode } from 'react'
import { createBrowserRouter, Navigate, RouterProvider } from 'react-router-dom'
import { LayoutKosong } from './layouts/layout-kosong'
import { LayoutToko } from './layouts/layout-toko'
import { ButuhIzin, ButuhMasuk, TamuSaja } from './penjaga'
import { HalamanMasuk } from '@/fitur/auth/halaman/halaman-masuk'
import { HalamanDaftar } from '@/fitur/auth/halaman/halaman-daftar'
import { HalamanBeranda } from '@/fitur/beranda/halaman/halaman-beranda'
import { KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { Compass } from 'lucide-react'
import { IZIN } from '@/lib/izin'

/**
 * Rute dipecah per berkas supaya muat pertama di 3G tetap di bawah tiga detik:
 * pemilik warung yang membuka Beranda tidak perlu ikut mengunduh modul gaji.
 *
 * Masuk, Daftar, dan Beranda sengaja TIDAK dipecah — ketiganya pasti dibutuhkan
 * di detik-detik pertama, dan memecahnya justru menambah satu perjalanan bolak-balik.
 */
const HalamanKasir = muat(() => import('@/fitur/kasir/halaman/halaman-kasir'), 'HalamanKasir')
const HalamanTutupShift = muat(() => import('@/fitur/kasir/halaman/halaman-tutup-shift'), 'HalamanTutupShift')
const HalamanRiwayat = muat(() => import('@/fitur/kasir/halaman/halaman-riwayat'), 'HalamanRiwayat')
const HalamanKas = muat(() => import('@/fitur/kasir/halaman/halaman-kas'), 'HalamanKas')
const HalamanPerluDiperiksa = muat(() => import('@/fitur/kasir/halaman/halaman-perlu-diperiksa'), 'HalamanPerluDiperiksa')
const HalamanLainnya = muat(() => import('@/fitur/beranda/halaman/halaman-lainnya'), 'HalamanLainnya')
const HalamanSelamatDatang = muat(() => import('@/fitur/onboarding/halaman/halaman-selamat-datang'), 'HalamanSelamatDatang')
const HalamanDaftarBarang = muat(() => import('@/fitur/produk/halaman/halaman-daftar-barang'), 'HalamanDaftarBarang')
const HalamanFormBarang = muat(() => import('@/fitur/produk/halaman/halaman-form-barang'), 'HalamanFormBarang')
const HalamanImpor = muat(() => import('@/fitur/produk/halaman/halaman-impor'), 'HalamanImpor')
const HalamanMaster = muat(() => import('@/fitur/produk/halaman/halaman-master'), 'HalamanMaster')
const HalamanStok = muat(() => import('@/fitur/stok/halaman/halaman-stok'), 'HalamanStok')
const HalamanKartuStok = muat(() => import('@/fitur/stok/halaman/halaman-kartu-stok'), 'HalamanKartuStok')
const HalamanKoreksiStok = muat(() => import('@/fitur/stok/halaman/halaman-koreksi-stok'), 'HalamanKoreksiStok')
const HalamanBarangMasuk = muat(() => import('@/fitur/stok/halaman/halaman-barang-masuk'), 'HalamanBarangMasuk')

/** React.lazy untuk modul yang mengekspor komponen bernama, bukan default. */
function muat<N extends string>(
  impor: () => Promise<Record<N, ComponentType>>,
  nama: N,
) {
  return lazy(async () => ({ default: (await impor())[nama] as ComponentType }))
}

/**
 * Rute kasir DIPRAMUAT begitu aplikasi menganggur.
 *
 * Kasir harus terbuka seketika, dan yang lebih penting: berkasnya harus sudah
 * ada di perangkat SEBELUM sinyal hilang. Kasir yang menunggu unduhan di tengah
 * antrean adalah kegagalan, bukan kelambatan.
 */
export function pramuatKasir(): void {
  const jalan = () => {
    void import('@/fitur/kasir/halaman/halaman-kasir')
    void import('@/fitur/kasir/halaman/halaman-tutup-shift')
  }
  if (typeof window.requestIdleCallback === 'function') window.requestIdleCallback(jalan)
  else window.setTimeout(jalan, 2000)
}

/** Menunggu berkas rute selesai diunduh — kerangka, bukan layar kosong. */
function Tunggu({ children }: { children: ReactNode }) {
  return (
    <Suspense
      fallback={
        <div className="p-4">
          <KerangkaBaris jumlah={4} />
        </div>
      }
    >
      {children}
    </Suspense>
  )
}

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
        <Tunggu>
              <HalamanSelamatDatang />
            </Tunggu>
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
          <Tunggu>
              <HalamanKasir />
            </Tunggu>
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
      { path: '/lainnya', element: <Tunggu>
              <HalamanLainnya />
            </Tunggu> },
      {
        path: '/kasir/tutup-shift',
        element: (
          <ButuhIzin izin={[IZIN.shiftClose]}>
            <Tunggu>
              <HalamanTutupShift />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/kasir/riwayat',
        element: (
          <ButuhIzin izin={[IZIN.saleCreate]}>
            <Tunggu>
              <HalamanRiwayat />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/kasir/kas',
        element: (
          <ButuhIzin izin={[IZIN.cashMovement]}>
            <Tunggu>
              <HalamanKas />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/kasir/belum-terkirim',
        element: (
          <ButuhIzin izin={[IZIN.saleCreate]}>
            <Tunggu>
              <HalamanPerluDiperiksa />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/barang',
        element: (
          <ButuhIzin izin={[IZIN.productView]}>
            <Tunggu>
              <HalamanDaftarBarang />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/barang/baru',
        element: (
          <ButuhIzin izin={[IZIN.productEdit]}>
            <Tunggu>
              <HalamanFormBarang />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/barang/impor',
        element: (
          <ButuhIzin izin={[IZIN.productImport]}>
            <Tunggu>
              <HalamanImpor />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/barang/master',
        element: (
          <ButuhIzin izin={[IZIN.productEdit]}>
            <Tunggu>
              <HalamanMaster />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        // Setelah /barang/baru dan /barang/impor supaya keduanya tidak tertelan
        // sebagai id barang.
        path: '/barang/:id',
        element: (
          <ButuhIzin izin={[IZIN.productEdit]}>
            <Tunggu>
              <HalamanFormBarang />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/stok',
        element: (
          <ButuhIzin izin={[IZIN.stockView]}>
            <Tunggu>
              <HalamanStok />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/stok/kartu/:productId',
        element: (
          <ButuhIzin izin={[IZIN.stockView]}>
            <Tunggu>
              <HalamanKartuStok />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/stok/koreksi',
        element: (
          <ButuhIzin izin={[IZIN.stockAdjust]}>
            <Tunggu>
              <HalamanKoreksiStok />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/stok/masuk',
        element: (
          <ButuhIzin izin={[IZIN.stockAdjust]}>
            <Tunggu>
              <HalamanBarangMasuk />
            </Tunggu>
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
