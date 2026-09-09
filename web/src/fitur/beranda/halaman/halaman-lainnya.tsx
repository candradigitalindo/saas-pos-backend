import { Link } from 'react-router-dom'
import { ChevronRight } from 'lucide-react'
import { MENU_LAINNYA, saringMenu } from '@/app/navigasi'
import { Kartu } from '@/bersama/ui/kartu'
import { TombolKeluar } from '@/bersama/komponen/tombol-keluar'
import { useSesi } from '@/bersama/hooks/use-sesi'

/** Menu "Lainnya" di HP: isinya persis yang boleh dikerjakan pengguna ini. */
export function HalamanLainnya() {
  const { boleh, profil } = useSesi()
  const menu = saringMenu(MENU_LAINNYA, boleh)

  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-judul font-bold text-teks-utama">Lainnya</h1>

      <Kartu className="p-4">
        <p className="font-semibold text-teks-utama">{profil?.user.name}</p>
        <p className="text-keterangan text-teks-redup">
          {profil?.user.role_name} · {profil?.tenant.business_name}
        </p>
      </Kartu>

      {menu.length > 0 && (
        <Kartu className="divide-y divide-garis">
          {menu.map((m) => (
            <Link
              key={m.ke}
              to={m.ke}
              className="flex h-14 items-center gap-3 px-4 hover:bg-permukaan-2"
            >
              <m.ikon className="h-5 w-5 shrink-0 text-teks-sekunder" aria-hidden />
              <span className="flex-1 text-isi text-teks-utama">{m.label}</span>
              <ChevronRight className="h-5 w-5 text-teks-redup" aria-hidden />
            </Link>
          ))}
        </Kartu>
      )}

      <Kartu className="p-1">
        <TombolKeluar />
      </Kartu>
    </div>
  )
}
