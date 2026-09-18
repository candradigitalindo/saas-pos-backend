import { cn } from '@/bersama/util/cn'

/**
 * Grafik batang mungil untuk menaruh sebuah angka dalam konteks beberapa hari
 * terakhir.
 *
 * Kenapa bukan Recharts seperti halaman Laporan: Recharts dimuat malas dan
 * berat karena di sana ia memang perlu sumbu, tooltip, dan legenda. Di sini
 * yang dibutuhkan hanya bentuk — naik atau turun. Beranda adalah layar pertama
 * yang dilihat pemilik warung setiap pagi; ia tidak boleh menunggu bundel
 * grafik.
 *
 * Digambar dengan elemen biasa, bukan SVG. Versi SVG-nya memakai
 * `preserveAspectRatio="none"` supaya bisa melar selebar kartu, dan itu ikut
 * melarkan sudut membulat batangnya jadi elips picak. Batang berbasis flex
 * melar tanpa merusak apa pun.
 *
 * Warnanya `currentColor` supaya ikut benar di atas kartu berwarna penuh maupun
 * kartu putih, dan di mode gelap.
 */
export function GrafikMini({
  nilai,
  label,
  className,
}: {
  /** Nilai per hari, paling lama di depan. Hari ini paling belakang. */
  nilai: number[]
  /** Dibacakan pembaca layar sebagai pengganti grafik. */
  label: string
  className?: string
}) {
  // Grafik butuh MINIMAL DUA hari berisi untuk menunjukkan bentuk. Dengan satu
  // hari berisi dan sisanya nol, yang tergambar hanyalah satu batang tunggal di
  // antara garis-garis tipis — terlihat seperti komponen yang rusak, bukan
  // seperti data. Warung yang baru mulai memakai aplikasi ini justru persis ada
  // di keadaan itu di minggu pertamanya.
  if (nilai.length < 2 || nilai.filter((v) => v > 0).length < 2) return null

  const maks = Math.max(...nilai, 1)

  return (
    <div
      role="img"
      aria-label={label}
      className={cn('flex h-9 w-full items-end gap-1.5', className)}
    >
      {nilai.map((v, i) => (
        <div
          key={i}
          className={cn(
            'flex-1 rounded-t-kontrol bg-current',
            // Hari ini penuh, hari lalu meredup — matanya langsung tahu mana
            // "sekarang" tanpa perlu sumbu atau label tanggal.
            i === nilai.length - 1 ? 'opacity-100' : 'opacity-35',
          )}
          // Batang terpendek tetap terlihat: hari tanpa jualan menyisakan garis
          // tipis, supaya terbaca sebagai data — bukan sebagai celah.
          style={{ height: `${Math.max(8, (v / maks) * 100)}%` }}
        />
      ))}
    </div>
  )
}
