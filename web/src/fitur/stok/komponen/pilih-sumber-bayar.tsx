import { Landmark, Wallet } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { IZIN } from '@/lib/izin'
import { formatJam } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import { useShiftAktif } from '@/fitur/kasir/hooks'
import type { SumberBayar } from '../api'

/**
 * Sumber uang pembayaran pemasok — dipilih SETIAP pembayaran (keputusan
 * pemilik produk 2026-09-29):
 *
 * - "Uang lain" (dompet, rekening): laci kasir tidak berubah.
 * - "Dari laci kasir": dicatat sebagai uang keluar shift yang sedang buka,
 *   supaya hitungan laci saat tutup shift tetap cocok. Tidak bisa dipilih bila
 *   kasir di toko ini belum dibuka, atau pengguna tidak punya izin uang masuk
 *   & keluar — alasannya ditulis, bukan pilihan yang hilang diam-diam.
 */
export function PilihSumberBayar({
  nilai,
  onPilih,
  arah = 'keluar',
  legenda,
  butuhIzinKas = true,
}: {
  nilai: SumberBayar
  onPilih: (s: SumberBayar) => void
  /** keluar = membayar pemasok; masuk = uang kembali dari pemasok (retur). */
  arah?: 'keluar' | 'masuk'
  legenda?: string
  /**
   * Laci hanya bisa dipilih pemegang izin uang masuk & keluar. Setoran kasbon
   * tidak memerlukannya (uang bertambah — server tidak memintanya).
   */
  butuhIzinKas?: boolean
}) {
  const { boleh } = useSesi()
  const { shift, memuat } = useShiftAktif()
  const bolehLaci = !butuhIzinKas || boleh(IZIN.cashMovement)
  const alasanTidak = !bolehLaci
    ? 'Butuh izin uang masuk & keluar'
    : memuat
      ? 'Memeriksa kasir…'
      : !shift
        ? 'Kasir di toko ini belum dibuka'
        : null

  return (
    <fieldset className="flex min-w-0 flex-col gap-1.5">
      <legend className="mb-1.5 text-label font-medium text-teks-sekunder">
        {legenda ?? (arah === 'masuk' ? 'Uang dari pemasok masuk ke' : 'Sumber uang')}
      </legend>
      <div className="grid grid-cols-2 gap-2">
        <Pilihan
          ikon={Landmark}
          judul="Uang lain"
          keterangan="Dompet / rekening"
          aktif={nilai === 'other'}
          onPilih={() => onPilih('other')}
        />
        <Pilihan
          ikon={Wallet}
          judul={arah === 'masuk' ? 'Laci kasir' : 'Dari laci kasir'}
          keterangan={
            alasanTidak ??
            `Uang ${arah === 'masuk' ? 'masuk' : 'keluar'} shift ${shift?.opened_by_name ?? ''} (sejak ${formatJam(shift!.opened_at)})`.replace(
              '  ',
              ' ',
            )
          }
          aktif={nilai === 'drawer'}
          nonaktif={!!alasanTidak}
          onPilih={() => onPilih('drawer')}
        />
      </div>
    </fieldset>
  )
}

function Pilihan({
  ikon: Ikon,
  judul,
  keterangan,
  aktif,
  nonaktif = false,
  onPilih,
}: {
  ikon: LucideIcon
  judul: string
  keterangan: string
  aktif: boolean
  nonaktif?: boolean
  onPilih: () => void
}) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={aktif}
      disabled={nonaktif}
      onClick={onPilih}
      className={cn(
        'flex min-h-14 flex-col items-start gap-0.5 rounded-kontrol border px-3 py-2 text-left transition-colors',
        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-utama',
        aktif ? 'border-utama bg-sorot' : 'border-garis bg-permukaan enabled:hover:bg-permukaan-2/60',
        'disabled:cursor-not-allowed disabled:opacity-60',
      )}
    >
      <span className="flex items-center gap-1.5 text-label font-semibold text-teks-utama">
        <Ikon className={cn('h-4 w-4', aktif ? 'text-utama' : 'text-teks-sekunder')} aria-hidden />
        {judul}
      </span>
      <span className="text-keterangan leading-tight text-teks-redup">{keterangan}</span>
    </button>
  )
}
