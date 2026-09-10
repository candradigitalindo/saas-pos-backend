import { describe, expect, it } from 'vitest'
import { cn } from './cn'

/**
 * `text-*` di Tailwind menampung ukuran huruf DAN warna teks sekaligus. Untuk
 * nama kita sendiri, tailwind-merge tidak bisa membedakannya kecuali skalanya
 * didaftarkan — dan tanpa itu ukuran 40px pada angka uang HILANG tanpa suara.
 *
 * Bug ini benar-benar terjadi di kartu "Untung bersih" halaman Laporan.
 */
describe('cn: ukuran huruf vs warna teks', () => {
  it('ukuran dan warna bisa hidup berdampingan', () => {
    const k = cn('text-angka font-extrabold', 'text-teks-utama')
    expect(k).toContain('text-angka')
    expect(k).toContain('text-teks-utama')
  })

  it('berlaku untuk seluruh skala tipografi dokumen', () => {
    for (const ukuran of ['keterangan', 'label', 'isi', 'judul-kartu', 'judul', 'angka']) {
      const k = cn(`text-${ukuran}`, 'text-bahaya-teks')
      expect(k, `text-${ukuran} hilang`).toContain(`text-${ukuran}`)
      expect(k).toContain('text-bahaya-teks')
    }
  })

  it('dua ukuran tetap bertabrakan — yang terakhir menang', () => {
    expect(cn('text-angka', 'text-judul')).toBe('text-judul')
  })

  it('dua warna tetap bertabrakan — yang terakhir menang', () => {
    expect(cn('text-teks-utama', 'text-bahaya-teks')).toBe('text-bahaya-teks')
  })

  it('kelas lain tetap digabung seperti biasa', () => {
    expect(cn('h-12', 'h-16')).toBe('h-16')
  })
})
