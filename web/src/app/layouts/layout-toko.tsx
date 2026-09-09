import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { LogOut } from 'lucide-react'
import {
  IKON_LAINNYA,
  MENU_LAINNYA,
  MENU_UTAMA,
  saringMenu,
  type ItemMenu,
} from '@/app/navigasi'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { StatusKoneksi } from '@/bersama/komponen/status-koneksi'
import { cn } from '@/bersama/util/cn'

/**
 * Kerangka aplikasi toko.
 *
 * Mobile-first: HP mendapat navigasi bawah lima ikon+teks; layar ≥1024px
 * mendapat navigasi samping tetap dengan isi maksimum 1280px agar baris teks
 * tidak terlalu lebar untuk dibaca.
 */
export function LayoutToko() {
  const { boleh, profil } = useSesi()

  const utama = saringMenu(MENU_UTAMA, boleh)
  const lainnya = saringMenu(MENU_LAINNYA, boleh)

  // Navigasi bawah maksimal 5 kolom; slot terakhir jadi "Lainnya" bila perlu.
  const bawah = utama.slice(0, lainnya.length > 0 ? 4 : 5)

  return (
    <div className="min-h-dvh bg-latar">
      <SampingBesar utama={utama} lainnya={lainnya} />

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
          {/* Ikon selalu ditemani teks — ikon sendirian tidak universal. */}
          <span className="text-[11px] font-medium leading-none">{label}</span>
        </>
      )}
    </NavLink>
  )
}

function SampingBesar({ utama, lainnya }: { utama: ItemMenu[]; lainnya: ItemMenu[] }) {
  const { profil, keluar } = useSesi()
  const navigate = useNavigate()

  return (
    <aside className="fixed inset-y-0 left-0 z-30 hidden w-64 flex-col border-r border-garis bg-permukaan lg:flex">
      <div className="border-b border-garis px-4 py-4">
        <p className="truncate text-judul-kartu font-bold text-teks-utama">
          {profil?.tenant.business_name ?? 'Usaha Saya'}
        </p>
        <p className="truncate text-keterangan text-teks-redup">
          {profil?.user.name} · {profil?.user.role_name}
        </p>
      </div>

      <nav aria-label="Navigasi samping" className="flex-1 overflow-y-auto p-2">
        <ul className="flex flex-col gap-0.5">
          {[...utama, ...lainnya].map((m) => (
            <li key={m.ke}>
              <NavLink
                to={m.ke}
                end={m.ke === '/'}
                className={({ isActive }) =>
                  cn(
                    'flex h-12 items-center gap-3 rounded-kontrol px-3 text-label font-medium',
                    isActive
                      ? 'bg-sorot text-utama'
                      : 'text-teks-sekunder hover:bg-permukaan-2',
                  )
                }
              >
                <m.ikon className="h-5 w-5 shrink-0" aria-hidden />
                {m.label}
              </NavLink>
            </li>
          ))}
        </ul>
      </nav>

      <div className="border-t border-garis p-3">
        <StatusKoneksi className="mb-2 px-1" />
        <button
          type="button"
          onClick={async () => {
            await keluar()
            navigate('/masuk', { replace: true })
          }}
          className="flex h-12 w-full items-center gap-3 rounded-kontrol px-3 text-label font-medium text-teks-sekunder hover:bg-permukaan-2"
        >
          <LogOut className="h-5 w-5" aria-hidden />
          Keluar
        </button>
      </div>
    </aside>
  )
}
