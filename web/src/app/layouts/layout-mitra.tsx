import { NavLink, Outlet } from 'react-router-dom'
import { Coins, Home, LogOut, Users } from 'lucide-react'
import { useSesiMitra } from '@/fitur/mitra/sesi-mitra'
import { cn } from '@/bersama/util/cn'

/**
 * Kerangka portal mitra.
 *
 * Tata letak, aksen, dan menunya SENGAJA berbeda dari aplikasi toko supaya
 * tidak ada mitra yang mengira sedang melihat data operasional merchant
 * (ui/04-PETA-LAYAR.md). Aksennya biru — bukan Hijau Tumbuh — dan kepala
 * halamannya menyebut "Portal Mitra" secara permanen.
 */
const MENU = [
  { ke: '/mitra', label: 'Beranda', ikon: Home, ujung: true },
  { ke: '/mitra/prospek', label: 'Prospek', ikon: Users, ujung: false },
  { ke: '/mitra/komisi', label: 'Komisi', ikon: Coins, ujung: false },
]

export function LayoutMitra() {
  const { mitra, pengguna, keluar } = useSesiMitra()

  return (
    <div className="min-h-dvh bg-latar">
      {/* Kepala + menu layar lebar dalam SATU wadah menempel. Dulu menunya
          `fixed` di bawah kepala dengan pengganjal di dasar halaman — isinya
          tertutup: sapaan "Halo, …" tidak pernah terlihat di tablet/desktop. */}
      <div className="sticky top-0 z-30">
      <header className="border-b-2 border-info bg-permukaan">
        <div className="mx-auto flex w-full max-w-3xl items-center justify-between gap-3 px-4 py-3">
          <div className="min-w-0">
            <p className="text-keterangan font-semibold uppercase tracking-wide text-info-teks">
              Portal Mitra
            </p>
            <p className="truncate text-label text-teks-sekunder">
              {mitra?.name}
              {pengguna?.name && ` · ${pengguna.name}`}
            </p>
          </div>
          <button
            type="button"
            onClick={keluar}
            className="flex h-11 shrink-0 items-center gap-2 rounded-kontrol px-3 text-label font-medium text-teks-sekunder hover:bg-permukaan-2"
          >
            <LogOut className="h-5 w-5" aria-hidden />
            Keluar
          </button>
        </div>
      </header>


      <nav
        aria-label="Navigasi portal mitra"
        className="fixed inset-x-0 bottom-0 z-30 border-t border-garis bg-permukaan pb-[env(safe-area-inset-bottom)] sm:hidden"
      >
        <ul className="mx-auto flex max-w-3xl">
          {MENU.map((m) => (
            <li key={m.ke} className="flex-1">
              <NavLink
                to={m.ke}
                end={m.ujung}
                className={({ isActive }) =>
                  cn(
                    'flex h-14 flex-col items-center justify-center gap-0.5',
                    isActive ? 'text-info-teks' : 'text-teks-redup',
                  )
                }
              >
                <m.ikon className="h-6 w-6" aria-hidden />
                <span className="text-keterangan font-medium leading-none">{m.label}</span>
              </NavLink>
            </li>
          ))}
        </ul>
      </nav>

      {/* Layar lebar: menu mendatar di bawah kepala halaman. */}
      <nav
        aria-label="Navigasi portal mitra"
        className="hidden border-b border-garis bg-permukaan sm:block"
      >
        <ul className="mx-auto flex w-full max-w-3xl gap-1 px-4">
          {MENU.map((m) => (
            <li key={m.ke}>
              <NavLink
                to={m.ke}
                end={m.ujung}
                className={({ isActive }) =>
                  cn(
                    'flex h-12 items-center gap-2 border-b-2 px-3 text-label font-medium',
                    isActive
                      ? 'border-info text-info-teks'
                      : 'border-transparent text-teks-sekunder hover:bg-permukaan-2',
                  )
                }
              >
                <m.ikon className="h-5 w-5" aria-hidden />
                {m.label}
              </NavLink>
            </li>
          ))}
        </ul>
      </nav>
      </div>

      <main className="mx-auto w-full max-w-3xl px-4 pb-24 pt-4 sm:pb-8">
        <Outlet />
      </main>
    </div>
  )
}
