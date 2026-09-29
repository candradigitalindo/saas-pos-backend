import { NavLink } from 'react-router-dom'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { IZIN } from '@/lib/izin'
import { cn } from '@/bersama/util/cn'

/**
 * Pindah antar laporan: Penjualan (uang masuk & untung) · Belanja (uang
 * keluar ke pemasok & utang). Tautan, bukan tab di satu halaman: tiap laporan
 * punya alamatnya sendiri dan bisa dibuka langsung. Tanpa izin stok, laporan
 * belanja tidak ada — dan satu tab sendirian tidak perlu ditampilkan.
 */
export function TabLaporan() {
  const { boleh } = useSesi()
  if (!boleh(IZIN.stockView)) return null
  const tab = [
    { ke: '/laporan', label: 'Penjualan' },
    { ke: '/laporan/belanja', label: 'Belanja' },
  ]
  return (
    <nav aria-label="Jenis laporan" className="-mb-1 flex gap-1 border-b border-garis">
      {tab.map((t) => (
        <NavLink
          key={t.ke}
          to={t.ke}
          end
          className={({ isActive }) =>
            cn(
              '-mb-px flex min-h-11 items-center border-b-2 px-3 text-label font-semibold transition-colors',
              isActive
                ? 'border-utama text-utama'
                : 'border-transparent text-teks-sekunder hover:text-teks-utama',
            )
          }
        >
          {t.label}
        </NavLink>
      ))}
    </nav>
  )
}
