import type { ReactNode } from 'react'
import type { LucideIcon } from 'lucide-react'
import { MerekKasir } from '@/bersama/komponen/logo'
import { cn } from '@/bersama/util/cn'

/**
 * Kerangka halaman masuk REALM LAIN — portal mitra dan panel internal.
 *
 * Dulu keduanya formulir polos di tengah layar kosong: tanpa merek, tanpa
 * kartu, dengan tombol abu-abu yang terbaca rusak. Di layar lebar tinggal
 * kotak kecil di tengah halaman putih. Kini polanya sama dengan halaman masuk
 * toko (LayoutKosong): panel merek di kiri, kartu formulir di kanan; di HP
 * pita merek di atas dan kartu menumpang di bawahnya.
 *
 * Warnanya SENGAJA berbeda dari toko (hijau) supaya tidak ada yang tertukar
 * pintu: mitra biru (`.permukaan-mitra`), panel internal gelap netral
 * (`bg-bingkai`). Keduanya tetap di kedua tema, jadi teksnya putih — rasio
 * kontrasnya dicatat di tokens.css.
 */
export interface Keunggulan {
  ikon: LucideIcon
  judul: string
  isi: string
}

export function KerangkaMasuk({
  nada,
  lencana,
  judul,
  subjudul,
  keunggulan,
  children,
}: {
  nada: 'mitra' | 'panel'
  /** Nama realm di samping merek: "Portal Mitra", "Panel Internal". */
  lencana: string
  judul: string
  subjudul: string
  keunggulan: Keunggulan[]
  children: ReactNode
}) {
  const latar = nada === 'mitra' ? 'permukaan-mitra' : 'bg-bingkai'
  const aksen = nada === 'mitra' ? 'bg-info/10 text-info-teks' : 'bg-permukaan-2 text-teks-utama'

  return (
    <div className="min-h-dvh bg-latar lg:grid lg:grid-cols-2 xl:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
      {/* Panel merek — layar besar. */}
      <aside
        className={cn(
          latar,
          'relative hidden overflow-hidden text-white lg:sticky lg:top-0 lg:flex lg:h-dvh lg:flex-col lg:justify-center lg:px-12 xl:px-16',
        )}
      >
        <div aria-hidden className="pointer-events-none absolute -right-24 -top-24 h-80 w-80 rounded-full border border-current opacity-15" />
        <div aria-hidden className="pointer-events-none absolute -bottom-32 -left-20 h-96 w-96 rounded-full border border-current opacity-10" />

        <div className="relative max-w-xl">
          <div className="flex flex-wrap items-center gap-3">
            <MerekKasir varian="terbalik" />
            <span className="rounded-full border border-current/40 px-3 py-1 text-label font-semibold">
              {lencana}
            </span>
          </div>
          <h2 className="mt-10 text-balance text-angka font-extrabold">{judul}</h2>
          <p className="mt-3 text-balance text-judul-kartu opacity-90">{subjudul}</p>
          <ul className="mt-10 flex flex-col gap-5">
            {keunggulan.map(({ ikon: Ikon, judul: j, isi }) => (
              <li key={j} className="flex gap-4">
                <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-kontrol border border-current/30">
                  <Ikon className="h-5 w-5" aria-hidden />
                </span>
                <div>
                  <p className="text-isi font-semibold">{j}</p>
                  <p className="text-label opacity-85">{isi}</p>
                </div>
              </li>
            ))}
          </ul>
        </div>
      </aside>

      {/* Pita merek — HP & tablet potret. */}
      <header className={cn(latar, 'px-6 pb-14 pt-8 text-white lg:hidden')}>
        <div className="flex flex-wrap items-center gap-3">
          <MerekKasir varian="terbalik" />
          <span className="rounded-full border border-current/40 px-2.5 py-0.5 text-keterangan font-semibold">
            {lencana}
          </span>
        </div>
        <p className="mt-3 max-w-sm text-isi opacity-90 [@media(max-height:700px)]:hidden">{subjudul}</p>
      </header>

      <main className="flex flex-col items-center px-4 pb-10 lg:justify-center lg:px-10 lg:py-12">
        <div className="gerak-lapis -mt-10 w-full max-w-md rounded-dialog border border-garis bg-permukaan p-6 shadow-melayang sm:p-8 lg:mt-0">
          {children}
        </div>

        <ul className="mt-8 flex w-full max-w-md flex-col gap-4 px-2 lg:hidden">
          {keunggulan.map(({ ikon: Ikon, judul: j, isi }) => (
            <li key={j} className="flex gap-3">
              <span className={cn('flex h-10 w-10 shrink-0 items-center justify-center rounded-kontrol', aksen)}>
                <Ikon className="h-5 w-5" aria-hidden />
              </span>
              <div>
                <p className="text-label font-semibold text-teks-utama">{j}</p>
                <p className="text-keterangan text-teks-sekunder">{isi}</p>
              </div>
            </li>
          ))}
        </ul>
      </main>
    </div>
  )
}
