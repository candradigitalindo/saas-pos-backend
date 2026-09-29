import { useState } from 'react'
import { inisialPetak } from '@/bersama/util/inisial'
import { KELAS_PETAK_NETRAL } from '@/bersama/util/warna-kategori'
import { cn } from '@/bersama/util/cn'

/**
 * Foto barang, dengan penanda huruf sebagai gantinya bila belum ada.
 *
 * Kasir mencari barang dengan MATA, bukan dengan membaca. Grid tanpa gambar
 * adalah dinding teks seragam yang memaksa tiap kartu dibaca satu per satu —
 * dan itu terjadi di layar yang dipakai sambil ada antrean.
 *
 * Penanda hurufnya bukan penambal sementara: warung dengan seratus barang
 * tidak akan pernah selesai memotret semuanya, jadi kartu tanpa foto harus
 * tetap enak dipindai mata selamanya. Bentuk dan warnanya dibuat SAMA dengan
 * kartu berfoto supaya grid tidak belang.
 *
 * Gambar yang gagal dimuat jatuh ke penanda huruf, tidak meninggalkan ikon
 * rusak: alamat foto bisa basi setelah pemulihan cadangan, dan kasir tidak
 * seharusnya melihat bekas kerusakan itu.
 *
 * Penandanya DUA HURUF di atas warna kategori barang (lihat inisialPetak &
 * warna-kategori.ts) — pola petak aplikasi kasir pada umumnya. Versi
 * sebelumnya memakai potongan tiga huruf ("AirMin", "KopSusGul") di atas
 * satu warna untuk semua barang: terbaca seperti kata salah eja, dan grid
 * tanpa foto tetap berupa dinding seragam.
 */
export function FotoBarang({
  nama,
  url,
  kelasWarna = KELAS_PETAK_NETRAL,
  kecil = false,
  className,
}: {
  nama: string
  url?: string | null
  /** Kelas latar+teks petak dari kelasPetak(); bawaannya netral. */
  kelasWarna?: string
  /** Gambar mini persegi (daftar barang): huruf lebih kecil. */
  kecil?: boolean
  className?: string
}) {
  const [gagal, setGagal] = useState(false)
  const tampilkanFoto = !!url && !gagal

  return (
    <div
      className={cn(
        // 3:2, bukan persegi. Persegi memakan 160px tinggi di kartu HP dua
        // kolom dan memangkas grid kasir dari empat baris terlihat jadi dua
        // setengah — di layar yang dipakai sambil ada antrean, baris yang
        // hilang itu lebih mahal daripada gambar yang lebih besar.
        'relative w-full shrink-0 overflow-hidden rounded-kontrol',
        kecil ? 'aspect-square' : 'aspect-[3/2]',
        tampilkanFoto ? 'bg-permukaan-2' : kelasWarna,
        className,
      )}
    >
      {tampilkanFoto ? (
        <img
          src={url ?? ''}
          alt=""
          // alt kosong & aria-hidden: nama barangnya sudah tertulis tepat di
          // bawah kartu ini, dan pembaca layar tidak perlu mendengarnya dua kali.
          aria-hidden
          loading="lazy"
          decoding="async"
          onError={() => setGagal(true)}
          className="h-full w-full object-cover"
        />
      ) : (
        <span
          aria-hidden
          className={cn(
            'flex h-full w-full items-center justify-center font-extrabold tracking-tight',
            // Warna teksnya ikut kelasWarna (pasangan terukur per tema), jadi
            // di sini hanya ukuran yang diatur.
            kecil ? 'text-label' : 'text-[1.75rem] leading-none',
          )}
        >
          {inisialPetak(nama)}
        </span>
      )}
    </div>
  )
}
