import { lazy, Suspense, type ComponentType, type ReactNode } from 'react'
import { createBrowserRouter, Navigate, RouterProvider } from 'react-router-dom'
import { LayoutKosong } from './layouts/layout-kosong'
import { LayoutToko } from './layouts/layout-toko'
import { ButuhIzin, ButuhMasuk, TamuSaja } from './penjaga'
import { HalamanMasuk } from '@/fitur/auth/halaman/halaman-masuk'
import { HalamanDaftar } from '@/fitur/auth/halaman/halaman-daftar'
import { HalamanBeranda } from '@/fitur/laporan/halaman/halaman-beranda'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { IZIN } from '@/lib/izin'
import { PenyediaSesiMitra, useSesiMitra } from '@/fitur/mitra/sesi-mitra'

/**
 * Rute dipecah per berkas supaya muat pertama di 3G tetap di bawah tiga detik:
 * pemilik warung yang membuka Beranda tidak perlu ikut mengunduh modul gaji.
 *
 * Masuk, Daftar, dan Beranda sengaja TIDAK dipecah — ketiganya pasti dibutuhkan
 * di detik-detik pertama, dan memecahnya justru menambah satu perjalanan bolak-balik.
 */
const HalamanKasir = muat(() => import('@/fitur/kasir/halaman/halaman-kasir'), 'HalamanKasir')
const HalamanTutupShift = muat(() => import('@/fitur/kasir/halaman/halaman-tutup-shift'), 'HalamanTutupShift')
const HalamanGantiShift = muat(() => import('@/fitur/kasir/halaman/halaman-ganti-shift'), 'HalamanGantiShift')
const HalamanRiwayat = muat(() => import('@/fitur/kasir/halaman/halaman-riwayat'), 'HalamanRiwayat')
const HalamanStrukPublik = muat(() => import('@/fitur/kasir/halaman/halaman-struk-publik'), 'HalamanStrukPublik')
const HalamanKas = muat(() => import('@/fitur/kasir/halaman/halaman-kas'), 'HalamanKas')
const HalamanPerluDiperiksa = muat(() => import('@/fitur/kasir/halaman/halaman-perlu-diperiksa'), 'HalamanPerluDiperiksa')
const HalamanLaporan = muat(() => import('@/fitur/laporan/halaman/halaman-laporan'), 'HalamanLaporan')
const HalamanLaporanBelanja = muat(() => import('@/fitur/laporan/halaman/halaman-laporan-belanja'), 'HalamanLaporanBelanja')
const HalamanLainnya = muat(() => import('./halaman/halaman-lainnya'), 'HalamanLainnya')
const HalamanSelamatDatang = muat(() => import('@/fitur/onboarding/halaman/halaman-selamat-datang'), 'HalamanSelamatDatang')
const HalamanDaftarBarang = muat(() => import('@/fitur/produk/halaman/halaman-daftar-barang'), 'HalamanDaftarBarang')
const HalamanFormBarang = muat(() => import('@/fitur/produk/halaman/halaman-form-barang'), 'HalamanFormBarang')
const HalamanImpor = muat(() => import('@/fitur/produk/halaman/halaman-impor'), 'HalamanImpor')
const HalamanMaster = muat(() => import('@/fitur/produk/halaman/halaman-master'), 'HalamanMaster')
const HalamanStok = muat(() => import('@/fitur/stok/halaman/halaman-stok'), 'HalamanStok')
const HalamanKartuStok = muat(() => import('@/fitur/stok/halaman/halaman-kartu-stok'), 'HalamanKartuStok')
const HalamanKoreksiStok = muat(() => import('@/fitur/stok/halaman/halaman-koreksi-stok'), 'HalamanKoreksiStok')
const HalamanBarangMasuk = muat(() => import('@/fitur/stok/halaman/halaman-barang-masuk'), 'HalamanBarangMasuk')
const HalamanOpname = muat(() => import('@/fitur/stok/halaman/halaman-opname'), 'HalamanOpname')
const HalamanTransfer = muat(() => import('@/fitur/stok/halaman/halaman-transfer'), 'HalamanTransfer')
const HalamanUtangPemasok = muat(() => import('@/fitur/stok/halaman/halaman-utang-pemasok'), 'HalamanUtangPemasok')
const HalamanPemasok = muat(() => import('@/fitur/pemasok/halaman/halaman-pemasok'), 'HalamanPemasok')
const HalamanRincianPemasok = muat(() => import('@/fitur/pemasok/halaman/halaman-rincian-pemasok'), 'HalamanRincianPemasok')
const HalamanPelanggan = muat(() => import('@/fitur/pelanggan/halaman/halaman-pelanggan'), 'HalamanPelanggan')
const HalamanKasbon = muat(() => import('@/fitur/pelanggan/halaman/halaman-kasbon'), 'HalamanKasbon')
const HalamanPengaturan = muat(() => import('@/fitur/pengaturan/halaman/halaman-pengaturan'), 'HalamanPengaturan')
const HalamanToko = muat(() => import('@/fitur/pengaturan/halaman/halaman-toko'), 'HalamanToko')
const HalamanPengguna = muat(() => import('@/fitur/pengaturan/halaman/halaman-pengguna'), 'HalamanPengguna')
const HalamanPeran = muat(() => import('@/fitur/pengaturan/halaman/halaman-peran'), 'HalamanPeran')
const HalamanKaryawan = muat(() => import('@/fitur/sdm/halaman/halaman-karyawan'), 'HalamanKaryawan')
const HalamanGaji = muat(() => import('@/fitur/sdm/halaman/halaman-gaji'), 'HalamanGaji')
const HalamanKanal = muat(() => import('@/fitur/kanal/halaman/halaman-kanal'), 'HalamanKanal')
const HalamanLangganan = muat(() => import('@/fitur/langganan/halaman/halaman-langganan'), 'HalamanLangganan')
const HalamanProspekCRM = muat(() => import('@/fitur/crm/halaman/halaman-prospek'), 'HalamanProspek')
const HalamanKunjungan = muat(() => import('@/fitur/crm/halaman/halaman-kunjungan'), 'HalamanKunjungan')

// Realm mitra: aplikasi terpisah dengan sesi, layout, dan menu sendiri.
const LayoutMitra = muat(() => import('./layouts/layout-mitra'), 'LayoutMitra')
const HalamanMasukMitra = muat(() => import('@/fitur/mitra/halaman/halaman-masuk-mitra'), 'HalamanMasukMitra')
const HalamanDashboardMitra = muat(() => import('@/fitur/mitra/halaman/halaman-dashboard-mitra'), 'HalamanDashboardMitra')
const HalamanProspekMitra = muat(() => import('@/fitur/mitra/halaman/halaman-prospek-mitra'), 'HalamanProspekMitra')
const HalamanKomisiMitra = muat(() => import('@/fitur/mitra/halaman/halaman-komisi-mitra'), 'HalamanKomisiMitra')

// Realm ketiga: panel internal penyedia SaaS. Desktop saja, tanpa offline.
const HalamanPanel = muat(() => import('@/fitur/panel/halaman/halaman-panel'), 'HalamanPanel')

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
  // Struk digital untuk PEMBELI — tanpa penjaga masuk/tamu: dibuka dari
  // tautan WhatsApp oleh siapa pun, termasuk yang sedang masuk sebagai kasir.
  {
    path: '/struk/:token',
    element: (
      <Tunggu>
        <HalamanStrukPublik />
      </Tunggu>
    ),
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
        // Serah terima butuh KEDUA izin: yang menyerahkan harus boleh menutup
        // shift lama sekaligus membuka yang baru.
        path: '/kasir/ganti-shift',
        element: (
          <ButuhIzin izin={[IZIN.shiftClose, IZIN.shiftOpen]}>
            <Tunggu>
              <HalamanGantiShift />
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
        path: '/stok/opname',
        element: (
          <ButuhIzin izin={[IZIN.stockOpname]}>
            <Tunggu>
              <HalamanOpname />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/stok/transfer',
        element: (
          <ButuhIzin izin={[IZIN.stockTransfer]}>
            <Tunggu>
              <HalamanTransfer />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/pemasok',
        element: (
          <ButuhIzin izin={[IZIN.productView]}>
            <Tunggu>
              <HalamanPemasok />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/pemasok/:id',
        element: (
          <ButuhIzin izin={[IZIN.productView]}>
            <Tunggu>
              <HalamanRincianPemasok />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/stok/utang',
        element: (
          <ButuhIzin izin={[IZIN.stockView]}>
            <Tunggu>
              <HalamanUtangPemasok />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/pelanggan',
        element: (
          <ButuhIzin izin={[IZIN.customerView]}>
            <Tunggu>
              <HalamanPelanggan />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/kasbon',
        element: (
          <ButuhIzin izin={[IZIN.receivableManage]}>
            <Tunggu>
              <HalamanKasbon />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/pengaturan',
        element: (
          <ButuhIzin izin={[IZIN.outletManage, IZIN.userManage, IZIN.roleManage]}>
            <Tunggu>
              <HalamanPengaturan />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/pengaturan/toko',
        element: (
          <ButuhIzin izin={[IZIN.outletManage]}>
            <Tunggu>
              <HalamanToko />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/pengaturan/pengguna',
        element: (
          <ButuhIzin izin={[IZIN.userManage]}>
            <Tunggu>
              <HalamanPengguna />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/pengaturan/peran',
        element: (
          <ButuhIzin izin={[IZIN.roleManage]}>
            <Tunggu>
              <HalamanPeran />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/sdm',
        element: (
          <ButuhIzin izin={[IZIN.hrEmployeeView, IZIN.hrEmployeeEdit]}>
            <Tunggu>
              <HalamanKaryawan />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/sdm/gaji',
        element: (
          <ButuhIzin izin={[IZIN.hrPayrollRun, IZIN.hrSalaryView]}>
            <Tunggu>
              <HalamanGaji />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/kanal',
        element: (
          <ButuhIzin izin={[IZIN.channelManage, IZIN.channelOrderAccept]}>
            <Tunggu>
              <HalamanKanal />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/langganan',
        element: (
          <ButuhIzin izin={[IZIN.billingManage]}>
            <Tunggu>
              <HalamanLangganan />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/crm',
        element: (
          <ButuhIzin izin={[IZIN.crmLeadViewOwn, IZIN.crmLeadViewAll]}>
            <Tunggu>
              <HalamanProspekCRM />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/crm/kunjungan',
        element: (
          <ButuhIzin izin={[IZIN.crmVisitCheckin]}>
            <Tunggu>
              <HalamanKunjungan />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        path: '/laporan',
        element: (
          <ButuhIzin izin={[IZIN.reportView]}>
            <Tunggu>
              <HalamanLaporan />
            </Tunggu>
          </ButuhIzin>
        ),
      },
      {
        // Butuh report.view DAN stock.view: ButuhIzin cukup salah satu, jadi
        // yang kedua diperiksa di dalam — lihat HalamanLaporanBelanja.
        path: '/laporan/belanja',
        element: (
          <ButuhIzin izin={[IZIN.reportView]}>
            <Tunggu>
              <HalamanLaporanBelanja />
            </Tunggu>
          </ButuhIzin>
        ),
      },
    ],
  },
  // ── Portal mitra: realm terpisah ──────────────────────────────────────────
  //
  // Bukan cabang dari pohon toko. Tokennya berbeda dan ditolak silang oleh
  // backend, jadi penjagaan, layout, dan penyimpanan sesinya juga dipisah.
  {
    path: '/mitra',
    element: (
      <Tunggu>
        <PenjagaMitra />
      </Tunggu>
    ),
    children: [
      { index: true, element: <Tunggu><HalamanDashboardMitra /></Tunggu> },
      { path: 'prospek', element: <Tunggu><HalamanProspekMitra /></Tunggu> },
      { path: 'komisi', element: <Tunggu><HalamanKomisiMitra /></Tunggu> },
    ],
  },
  // ── Panel internal: realm ketiga ─────────────────────────────────────────
  {
    path: '/panel',
    element: (
      <Tunggu>
        <HalamanPanel />
      </Tunggu>
    ),
  },
  { path: '*', element: <Navigate to="/" replace /> },
])

export function Rute() {
  return <RouterProvider router={router} />
}

/**
 * Penjaga realm mitra. Menyediakan sesinya sekaligus — sesi mitra tidak boleh
 * hidup di seluruh aplikasi, hanya di bawah /mitra.
 */
function PenjagaMitra() {
  return (
    <PenyediaSesiMitra>
      <IsiMitra />
    </PenyediaSesiMitra>
  )
}

function IsiMitra() {
  const { sudahMasuk, memuat } = useSesiMitra()

  if (memuat) {
    return (
      <div className="p-4">
        <KerangkaBaris jumlah={4} />
      </div>
    )
  }

  if (!sudahMasuk) {
    return (
      <Tunggu>
        <HalamanMasukMitra />
      </Tunggu>
    )
  }

  return (
    <Tunggu>
      <LayoutMitra />
    </Tunggu>
  )
}
