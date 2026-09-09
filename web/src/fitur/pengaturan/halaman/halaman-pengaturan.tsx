import { Link } from 'react-router-dom'
import { ChevronRight, Building2, ShieldCheck, Store, Users } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { IZIN, type KodeIzin } from '@/lib/izin'

/** Pintu masuk pengaturan. Isinya hanya yang boleh dikerjakan pengguna ini. */
export function HalamanPengaturan() {
  const { boleh, profil } = useSesi()

  const menu: { ke: string; label: string; keterangan: string; ikon: LucideIcon; izin: KodeIzin[] }[] = [
    {
      ke: '/pengaturan/toko',
      label: 'Toko & Cabang',
      keterangan: 'Nama, alamat, zona waktu, jam tutup buku',
      ikon: Store,
      izin: [IZIN.outletManage],
    },
    {
      ke: '/pengaturan/pengguna',
      label: 'Pengguna',
      keterangan: 'Siapa saja yang bisa masuk ke aplikasi ini',
      ikon: Users,
      izin: [IZIN.userManage],
    },
    {
      ke: '/pengaturan/peran',
      label: 'Peran & Hak Akses',
      keterangan: 'Menentukan siapa boleh mengerjakan apa',
      ikon: ShieldCheck,
      izin: [IZIN.roleManage],
    },
  ].filter((m) => boleh(...m.izin))

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-col gap-4">
      <h1 className="text-judul font-bold text-teks-utama">Pengaturan</h1>

      <Kartu className="flex items-center gap-3 p-4">
        <Building2 className="h-6 w-6 shrink-0 text-teks-redup" aria-hidden />
        <div className="min-w-0">
          <p className="truncate font-semibold text-teks-utama">
            {profil?.tenant.business_name}
          </p>
          <p className="text-keterangan text-teks-redup">
            Pemilik: {profil?.tenant.owner_name} · {profil?.tenant.phone}
          </p>
        </div>
      </Kartu>

      <Kartu className="divide-y divide-garis">
        {menu.map((m) => (
          <Link
            key={m.ke}
            to={m.ke}
            className="flex min-h-16 items-center gap-3 px-4 hover:bg-permukaan-2"
          >
            <m.ikon className="h-5 w-5 shrink-0 text-teks-sekunder" aria-hidden />
            <span className="min-w-0 flex-1">
              <span className="block text-isi text-teks-utama">{m.label}</span>
              <span className="block text-keterangan text-teks-redup">
                {m.keterangan}
              </span>
            </span>
            <ChevronRight className="h-5 w-5 shrink-0 text-teks-redup" aria-hidden />
          </Link>
        ))}
      </Kartu>
    </div>
  )
}
