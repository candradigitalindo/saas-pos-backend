import { clsx, type ClassValue } from 'clsx'
import { extendTailwindMerge } from 'tailwind-merge'

/**
 * Skala tipografi kita (ui/02-SISTEM-DESAIN.md). Harus didaftarkan ke
 * tailwind-merge, dan ini BUKAN kerapian belaka:
 *
 * `text-*` di Tailwind menampung dua hal sekaligus — ukuran huruf DAN warna
 * teks. Untuk nama bawaan (`text-lg`, `text-red-500`) tailwind-merge bisa
 * membedakannya. Untuk nama kita sendiri ia tidak bisa, sehingga
 * `cn('text-angka', 'text-teks-utama')` menghasilkan `text-teks-utama` saja —
 * ukuran 40px-nya HILANG tanpa suara, dan angka uang di layar mengecil jadi
 * ukuran biasa tanpa ada yang menyadarinya.
 *
 * Itu bukan dugaan: bug ini benar-benar terjadi di kartu "Untung bersih"
 * halaman Laporan sebelum daftar ini dibuat.
 */
const UKURAN_TEKS = [
  'keterangan',
  'label',
  'isi',
  'judul-kartu',
  'judul',
  'angka',
] as const

const twMerge = extendTailwindMerge({
  extend: {
    classGroups: {
      'font-size': [{ text: [...UKURAN_TEKS] }],
    },
  },
})

/** Gabung kelas Tailwind; kelas yang bentrok dimenangkan yang terakhir. */
export function cn(...kelas: ClassValue[]): string {
  return twMerge(clsx(kelas))
}
