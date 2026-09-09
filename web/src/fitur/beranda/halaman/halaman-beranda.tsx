import { Link } from 'react-router-dom'
import { Boxes, PackagePlus, Receipt } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { StatusKoneksi } from '@/bersama/komponen/status-koneksi'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { IZIN } from '@/lib/izin'
import { cn } from '@/bersama/util/cn'

/**
 * Beranda pemilik.
 *
 * Pada tahap U0 ini baru berisi sapaan dan pintasan; kartu angka diisi dari
 * GET /reports/dashboard pada tahap U4. Pintasannya sudah dipasang sekarang
 * karena aturan "maksimal tiga ketukan ke pekerjaan harian" berlaku sejak awal.
 */
export function HalamanBeranda() {
  const { profil, boleh } = useSesi()
  const nama = profil?.user.name?.split(' ')[0] ?? ''

  const pintasan = [
    { ke: '/kasir', label: 'Jual barang', ikon: Receipt, izin: [IZIN.saleCreate] },
    { ke: '/stok/masuk', label: 'Barang masuk', ikon: PackagePlus, izin: [IZIN.stockAdjust] },
    { ke: '/stok', label: 'Lihat stok', ikon: Boxes, izin: [IZIN.stockView] },
  ].filter((p) => boleh(...p.izin))

  return (
    <div className="flex flex-col gap-4">
      <header className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-judul font-bold text-teks-utama">
            {sapaan()}
            {nama && `, ${nama}`}
          </h1>
          <p className="text-label text-teks-sekunder">
            {profil?.tenant.business_name}
          </p>
        </div>
      </header>

      <StatusKoneksi />

      {pintasan.length > 0 && (
        <section aria-label="Pintasan" className="grid grid-cols-2 gap-3 sm:grid-cols-3">
          {pintasan.map((p) => (
            <Link
              key={p.ke}
              to={p.ke}
              className={cn(
                'flex min-h-24 flex-col items-center justify-center gap-2 rounded-kartu',
                'border border-garis bg-permukaan p-4 text-center shadow-kartu',
                'hover:bg-permukaan-2',
              )}
            >
              <p.ikon className="h-7 w-7 text-utama" aria-hidden />
              <span className="text-label font-semibold text-teks-utama">{p.label}</span>
            </Link>
          ))}
        </section>
      )}

      <Kartu className="p-4">
        <p className="text-label text-teks-sekunder">
          Angka penjualan dan untung akan muncul di sini begitu ada transaksi.
        </p>
      </Kartu>
    </div>
  )
}

function sapaan(): string {
  const jam = new Date().getHours()
  if (jam < 11) return 'Selamat pagi'
  if (jam < 15) return 'Selamat siang'
  if (jam < 18) return 'Selamat sore'
  return 'Selamat malam'
}
