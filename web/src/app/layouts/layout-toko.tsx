import { Link, NavLink, Outlet, useLocation } from 'react-router-dom'
import { Home } from 'lucide-react'
import {
  IKON_LAINNYA,
  MENU_LAINNYA,
  MENU_UTAMA,
  menuAktif,
  saringKelompok,
  saringMenu,
  type ItemMenu,
} from '@/app/navigasi'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { PemilihToko } from '@/bersama/komponen/pemilih-toko'
import { StatusKoneksi } from '@/bersama/komponen/status-koneksi'
import { TombolKeluar } from '@/bersama/komponen/tombol-keluar'
import { useSinkron } from '@/lib/offline/mesin'
import { inisialNama } from '@/bersama/util/inisial'
import { cn } from '@/bersama/util/cn'

/**
 * Kerangka aplikasi toko.
 *
 * Mobile-first: HP mendapat navigasi bawah lima ikon+teks; layar ≥1024px
 * mendapat navigasi samping tetap dengan isi maksimum 1280px agar baris teks
 * tidak terlalu lebar untuk dibaca.
 *
 * Aturan lebar halaman di dalamnya: halaman data (dasbor, tabel, daftar
 * panjang) memakai lebar penuh; halaman formulir & pengaturan boleh sempit
 * (max-w-lg / max-w-2xl) tetapi RATA KIRI — TANPA mx-auto. Dulu yang sempit
 * diletakkan di tengah, sehingga judul halaman melompat dari x≈378 (Barang)
 * ke x≈711 (Karyawan) setiap kali berpindah menu. Rata kiri membuat setiap
 * judul mulai di tempat yang sama.
 */
export function LayoutToko() {
  const { boleh, profil } = useSesi()

  const utama = saringMenu(MENU_UTAMA, boleh)
  const lainnya = saringMenu(MENU_LAINNYA, boleh)

  // Navigasi bawah maksimal 5 kolom; slot terakhir jadi "Lainnya" bila perlu.
  const bawah = utama.slice(0, lainnya.length > 0 ? 4 : 5)

  return (
    <div className="min-h-dvh bg-latar">
      <SampingBesar />

      <div className="lg:pl-64">
        <main className="mx-auto w-full max-w-[1280px] px-4 pb-24 pt-4 lg:pb-8">
          <Outlet />
        </main>
      </div>

      <nav
        aria-label="Navigasi utama"
        className={cn(
          'fixed inset-x-0 bottom-0 z-30 border-t border-garis bg-permukaan lg:hidden',
          'pb-[env(safe-area-inset-bottom)]',
        )}
      >
        <ul className="flex">
          {bawah.map((m) => (
            <li key={m.ke} className="flex-1">
              <TautanBawah item={m} />
            </li>
          ))}
          {lainnya.length > 0 && (
            <li className="flex-1">
              <TautanBawah
                item={{ ke: '/lainnya', label: 'Lainnya', ikon: IKON_LAINNYA }}
              />
            </li>
          )}
        </ul>
      </nav>

      <p className="sr-only">Masuk sebagai {profil?.user.name}</p>
    </div>
  )
}

function TautanBawah({ item }: { item: ItemMenu }) {
  const { ikon: Ikon, label, ke } = item
  return (
    <NavLink
      to={ke}
      end={ke === '/'}
      className={({ isActive }) =>
        cn(
          // Target sentuh minimal 48px; di sini 56px karena dipakai sambil buru-buru.
          'flex h-14 flex-col items-center justify-center gap-0.5',
          isActive ? 'text-utama' : 'text-teks-redup',
        )
      }
    >
      {({ isActive }) => (
        <>
          <Ikon className="h-6 w-6" aria-hidden strokeWidth={isActive ? 2.4 : 2} />
          {/* Ikon selalu ditemani teks — ikon sendirian tidak universal.
              Ukurannya 13px: itu batas terkecil yang masih terbaca menurut
              ui/02, dan navigasi utama bukan tempat untuk melanggarnya. */}
          <span className="text-keterangan font-medium leading-none">{label}</span>
        </>
      )}
    </NavLink>
  )
}

function SampingBesar() {
  const { profil, boleh } = useSesi()
  const sinkron = useSinkron()
  const { pathname } = useLocation()
  const kelompok = saringKelompok(boleh)

  // Tepat satu menu menyala: yang jalurnya paling spesifik (lihat menuAktif).
  const aktif = menuAktif(pathname, ['/', ...kelompok.flatMap((k) => k.item.map((m) => m.ke))])

  return (
    <aside className="fixed inset-y-0 left-0 z-30 hidden w-64 flex-col border-r border-garis bg-permukaan lg:flex">
      <div className="border-b border-garis p-2">
        <PemilihToko />
      </div>

      <nav aria-label="Navigasi samping" className="flex-1 overflow-y-auto p-2">
        <ul className="mb-1 flex flex-col gap-0.5">
          <li>
            <TautanSamping item={{ ke: '/', label: 'Beranda', ikon: Home }} aktif={aktif === '/'} />
          </li>
        </ul>

        {/* Dikelompokkan sesuai ui/04-PETA-LAYAR.md. Daftar rata 19 baris
            memaksa orang memindai satu per satu setiap kali. */}
        {kelompok.map((k) => (
          <div key={k.judul} className="mb-1">
            <h2 className="px-3 pb-1 pt-3 text-keterangan font-semibold uppercase tracking-wide text-teks-redup">
              {k.judul}
            </h2>
            <ul className="flex flex-col gap-0.5">
              {k.item.map((m) => (
                <li key={m.ke}>
                  <TautanSamping item={m} aktif={aktif === m.ke} />
                </li>
              ))}
            </ul>
          </div>
        ))}
      </nav>

      <div className="border-t border-garis p-2">
        <StatusKoneksi menunggu={sinkron.menunggu} className="px-2 pb-2 pt-1" />
        {/* Antrean yang macet hanya terlihat di sini — beri jumlahnya, jangan
            biarkan penjualan menggantung tanpa ada yang tahu. */}
        {sinkron.perluDiperiksa > 0 && (
          <NavLink
            to="/kasir/belum-terkirim"
            className="mb-2 flex items-center gap-2 rounded-kontrol bg-permukaan-2 px-3 py-2 text-keterangan font-medium text-jingga-700"
          >
            {sinkron.perluDiperiksa} transaksi perlu diperiksa
          </NavLink>
        )}

        {/* Siapa yang sedang masuk — avatar + nama + peran, pola kaki
            navigasi yang lazim di aplikasi SaaS. Di toko yang HP/tabletnya
            dipakai bergantian, ini jawaban cepat untuk "akun siapa ini?". */}
        <div className="flex items-center gap-2 rounded-kontrol pl-2">
          <span
            className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-sorot text-keterangan font-bold text-hijau-800"
            aria-hidden
          >
            {inisialNama(profil?.user.name ?? '')}
          </span>
          <div className="min-w-0 flex-1">
            <p className="truncate text-label font-semibold text-teks-utama">{profil?.user.name}</p>
            <p className="truncate text-keterangan text-teks-redup">{profil?.user.role_name}</p>
          </div>
          <TombolKeluar ringkas />
        </div>
      </div>
    </aside>
  )
}

function TautanSamping({ item, aktif }: { item: ItemMenu; aktif: boolean }) {
  return (
    <Link
      to={item.ke}
      aria-current={aktif ? 'page' : undefined}
      className={cn(
        'flex h-12 items-center gap-3 rounded-kontrol px-3 text-label font-medium',
        aktif ? 'bg-sorot text-utama' : 'text-teks-sekunder hover:bg-permukaan-2',
      )}
    >
      <item.ikon className="h-5 w-5 shrink-0" aria-hidden />
      {item.label}
    </Link>
  )
}
