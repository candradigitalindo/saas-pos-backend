import { useCallback, useState } from 'react'
import { NavLink, Outlet } from 'react-router-dom'
import {
  IKON_LAINNYA,
  MENU_LAINNYA,
  MENU_UTAMA,
  saringMenu,
  type ItemMenu,
} from '@/app/navigasi'
import { NavigasiSamping } from '@/app/layouts/navigasi-samping'
import { PaletPerintah, usePintasanPalet } from '@/app/layouts/palet-perintah'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { cn } from '@/bersama/util/cn'

/** Kunci localStorage: navigasi samping diciutkan jadi lajur ikon. */
const KUNCI_RINGKAS = 'navigasi.ringkas'

function bacaRingkas(): boolean {
  try {
    return localStorage.getItem(KUNCI_RINGKAS) === '1'
  } catch {
    return false
  }
}

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

  // Navigasi samping bisa diciutkan jadi lajur ikon — diingat per perangkat
  // (kenyamanan saja; gagal menyimpan tidak mengubah apa pun selain itu).
  const [ringkas, setRingkas] = useState(bacaRingkas)
  const ubahRingkas = () =>
    setRingkas((r) => {
      try {
        localStorage.setItem(KUNCI_RINGKAS, r ? '0' : '1')
      } catch {
        /* abaikan */
      }
      return !r
    })

  // Palet perintah: Ctrl/⌘ K dari mana pun, atau tombol "Cari…".
  const [paletBuka, setPaletBuka] = useState(false)
  const ubahPalet = useCallback((f: (b: boolean) => boolean) => setPaletBuka(f), [])
  usePintasanPalet(ubahPalet)

  return (
    <div className="min-h-dvh bg-latar">
      <NavigasiSamping
        ringkas={ringkas}
        onUbahRingkas={ubahRingkas}
        onBukaPalet={() => setPaletBuka(true)}
      />
      <PaletPerintah buka={paletBuka} onBukaBerubah={setPaletBuka} />

      <div
        className={cn(
          'transition-[padding] duration-200 ease-out motion-reduce:transition-none',
          ringkas ? 'lg:pl-[72px]' : 'lg:pl-64',
        )}
      >
        <main className="mx-auto w-full max-w-7xl px-4 pb-24 pt-4 lg:pb-8">
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
