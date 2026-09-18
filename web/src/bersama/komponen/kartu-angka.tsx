import { ArrowDown, ArrowUp, Minus } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { formatRupiah, persenSelisih } from '@/bersama/util/uang'
import { cn } from '@/bersama/util/cn'

/**
 * Kartu angka pendamping.
 *
 * Urutan sengaja: label kecil dulu, baru angka besar. Dan SELALU ada
 * pembanding — angka tanpa pembanding tidak memberi tahu apa pun
 * (ui/01-PRINSIP-DESAIN.md §4). Perbandingan ditandai ikon panah, bukan hanya
 * warna.
 *
 * Perubahan ditampilkan sebagai LENCANA bertumpu latar redup, bukan teks
 * telanjang berwarna. Sebabnya bukan gaya: deretan teks merah di beberapa kartu
 * sekaligus membuat halaman terbaca seperti peringatan, padahal isinya laporan
 * biasa. Lencana memberi angka itu batas, sehingga terbaca sebagai keterangan —
 * dan pasangan warnanya (`bahaya-teks`/`hijau-700` di atas `permukaan-2`) justru
 * pasangan yang sudah diverifikasi di audit kontras.
 */
export function KartuAngka({
  label,
  nilai,
  pembanding,
  labelPembanding = 'dibanding kemarin',
  keterangan,
  uang = true,
  /** true bila naik itu BURUK (mis. jumlah retur) — arah panah tetap, warnanya dibalik. */
  naikItuBuruk = false,
  ringkas = false,
  ikon: Ikon,
  className,
}: {
  label: string
  nilai: number
  pembanding?: number
  labelPembanding?: string
  keterangan?: string
  uang?: boolean
  naikItuBuruk?: boolean
  /**
   * Angka 24px, bukan 40px — untuk kartu pendamping yang berjajar rapat.
   *
   * Dua alasan, keduanya ketahuan dari layar sungguhan. Di HP, beberapa kartu
   * 40px menumpuk ke bawah dan mendorong angka penting keluar layar pertama. Di
   * desktop empat kolom, "Rp 1.734.000" pada 40px tidak muat selebar kartunya
   * dan pecah jadi dua baris.
   *
   * Ukuran yang berbeda inilah yang membentuk susunannya: satu angka disorot
   * 40px (KartuSorotan), sisanya lebih kecil. Kalau semua 40px, tidak ada yang
   * tersorot.
   */
  ringkas?: boolean
  /** Ikon kecil di keping bertinta merek, penanda cepat saat kartu berjajar. */
  ikon?: LucideIcon
  className?: string
}) {
  const persen = pembanding === undefined ? null : persenSelisih(nilai, pembanding)
  const naik = persen !== null && persen > 0
  const turun = persen !== null && persen < 0
  const bagus = naik ? !naikItuBuruk : turun ? naikItuBuruk : true

  const Panah = naik ? ArrowUp : turun ? ArrowDown : Minus

  return (
    <Kartu className={cn('flex flex-col p-4', className)}>
      <div className="flex items-start gap-3">
        {Ikon && (
          // Tinta 10% dari warna merek: cukup untuk terbaca sebagai keping,
          // terlalu tipis untuk menggeser kontras teks di atasnya — dan ikut
          // benar sendiri di mode gelap karena bertumpu pada `utama`.
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-kontrol bg-utama/10 text-utama">
            <Ikon className="h-[18px] w-[18px]" aria-hidden />
          </span>
        )}
        <p className="min-w-0 pt-0.5 text-label font-medium text-teks-sekunder">{label}</p>
      </div>

      <p
        className={cn(
          'mt-2 font-extrabold tabular-nums text-teks-utama',
          ringkas ? 'text-judul-kartu sm:text-judul' : 'text-angka',
        )}
      >
        {uang ? formatRupiah(nilai) : nilai.toLocaleString('id-ID')}
      </p>

      {persen !== null ? (
        <p className="mt-2">
          <span
            className={cn(
              'inline-flex items-center gap-1 rounded-full bg-permukaan-2 px-2 py-1',
              'text-keterangan font-semibold',
              persen === 0 ? 'text-teks-redup' : bagus ? 'text-hijau-700' : 'text-bahaya-teks',
            )}
          >
            <Panah className="h-3.5 w-3.5" aria-hidden />
            {Math.abs(persen)}% {labelPembanding}
          </span>
        </p>
      ) : (
        /* Angka tanpa pembanding tidak memberi tahu apa pun (ui/01 §4). Kalau
           pembandingnya memang belum ada, KATAKAN — jangan tinggalkan angka
           telanjang yang tampak seperti baris yang lupa dimuat. */
        <p className="mt-2 text-keterangan text-teks-redup">
          {keterangan ??
            (pembanding === undefined
              ? ''
              : `Belum ada data pembanding ${labelPembanding.replace(/^dibanding /, '')}`)}
        </p>
      )}
    </Kartu>
  )
}
