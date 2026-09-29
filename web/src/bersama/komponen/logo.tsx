import { cn } from '@/bersama/util/cn'

/**
 * Tanda merek Kasir UMKM — bentuk yang sama dengan favicon & ikon PWA
 * (public/favicon.svg), digambar dari token supaya ikut mode gelap.
 *
 *   - `penuh`   : kotak Hijau Tumbuh, garis `utama-teks` — di atas latar biasa.
 *   - `terbalik`: kotak `utama-teks`, garis hijau — di atas permukaan sorotan
 *                 (gradien hijau), tempat kotak hijau akan lenyap.
 *
 * Kedua pasangan warnanya sudah diverifikasi di ui/02 (utama ⇄ utama-teks).
 */
export function LogoKasir({
  varian = 'penuh',
  className,
}: {
  varian?: 'penuh' | 'terbalik'
  className?: string
}) {
  const kotak = varian === 'penuh' ? 'var(--warna-utama)' : 'var(--warna-utama-teks)'
  const garis = varian === 'penuh' ? 'var(--warna-utama-teks)' : 'var(--warna-utama)'
  return (
    <svg viewBox="0 0 64 64" className={cn('h-10 w-10 shrink-0', className)} aria-hidden>
      <rect width="64" height="64" rx="14" fill={kotak} />
      <path
        d="M20 14v36M20 32l16-18M20 32l16 18"
        stroke={garis}
        strokeWidth="7"
        strokeLinecap="round"
        strokeLinejoin="round"
        fill="none"
      />
    </svg>
  )
}

/** Logo + nama produk, dipakai di halaman masuk & daftar. */
export function MerekKasir({
  varian = 'penuh',
  className,
}: {
  varian?: 'penuh' | 'terbalik'
  className?: string
}) {
  return (
    <div className={cn('flex items-center gap-3', className)}>
      <LogoKasir varian={varian} />
      <span className="text-judul-kartu font-extrabold tracking-tight">Kasir UMKM</span>
    </div>
  )
}
