import { forwardRef } from 'react'
import { Slot } from '@radix-ui/react-slot'
import { cva, type VariantProps } from 'class-variance-authority'
import { Loader2 } from 'lucide-react'
import { cn } from '@/bersama/util/cn'

/**
 * Aturan tombol (ui/01-PRINSIP-DESAIN.md §3):
 *   - Satu tombol UTAMA per layar, berwarna penuh.
 *   - Tombol berbahaya tidak pernah bersebelahan dengan tombol utama.
 *   - Labelnya kata kerja: "Simpan Barang", bukan "OK".
 *   - Saat memuat, LEBARNYA TIDAK BERUBAH supaya tata letak tidak melompat.
 */
const gaya = cva(
  'inline-flex items-center justify-center gap-2 rounded-kontrol font-semibold ' +
    'transition-colors duration-150 ease-out disabled:pointer-events-none ' +
    'disabled:opacity-50 select-none',
  {
    variants: {
      jenis: {
        utama: 'bg-utama text-utama-teks hover:bg-hijau-800 active:bg-hijau-800',
        kedua:
          'border border-garis bg-permukaan text-teks-utama hover:bg-permukaan-2',
        bahaya: 'bg-bahaya text-white hover:brightness-95',
        teks: 'text-utama hover:bg-sorot',
        /* Jingga Semangat: hanya untuk momen pencapaian, dan SELALU teks gelap. */
        pencapaian: 'bg-jingga-400 text-teks-utama hover:brightness-95',
      },
      ukuran: {
        // "Padat" 40px hanya berlaku untuk penunjuk halus (tetikus). Di
        // perangkat sentuh ia naik ke 48px.
        //
        // Dua dokumen sempat bertabrakan di sini: ui/02 mencantumkan 40px
        // sebagai tinggi tombol yang sah, sedangkan ui/01 §3 mewajibkan target
        // sentuh minimal 48×48 px. Prinsip di ui/01 yang mengikat, jadi 40px
        // dipertahankan HANYA di tempat yang pasti dipakai dengan tetikus.
        padat: 'h-12 px-4 text-label pointer-fine:h-10',
        normal: 'h-12 px-5 text-isi',
        kasir: 'h-16 px-6 text-judul-kartu',
        ikon: 'h-12 w-12',
      },
      lebarPenuh: { true: 'w-full', false: '' },
    },
    defaultVariants: { jenis: 'utama', ukuran: 'normal', lebarPenuh: false },
  },
)

export interface PropTombol
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof gaya> {
  asChild?: boolean
  /** Menampilkan pemuat dan mengunci tombol; label diganti `labelMemuat`. */
  memuat?: boolean
  labelMemuat?: string
}

export const Tombol = forwardRef<HTMLButtonElement, PropTombol>(function Tombol(
  {
    className,
    jenis,
    ukuran,
    lebarPenuh,
    asChild,
    memuat,
    labelMemuat = 'Menyimpan…',
    children,
    disabled,
    ...sisa
  },
  ref,
) {
  const Komp = asChild ? Slot : 'button'
  return (
    <Komp
      ref={ref}
      className={cn(gaya({ jenis, ukuran, lebarPenuh }), className)}
      disabled={disabled || memuat}
      aria-busy={memuat || undefined}
      {...sisa}
    >
      {memuat ? (
        <>
          <Loader2 className="h-5 w-5 animate-spin" aria-hidden />
          {labelMemuat}
        </>
      ) : (
        children
      )}
    </Komp>
  )
})
