import { describe, expect, it } from 'vitest'
import { menuAktif, tujuanSamping } from './navigasi'

/** Pemilik: semua izin. */
const semua = tujuanSamping(() => true)

describe('menuAktif — tepat satu menu menyala', () => {
  it('halaman turunan yang punya menu sendiri menyalakan menunya saja', () => {
    // Dulu "Kasir" ikut menyala di sini, bersama "Riwayat Penjualan".
    expect(menuAktif('/kasir/riwayat', semua)).toBe('/kasir/riwayat')
    expect(menuAktif('/barang/master', semua)).toBe('/barang/master')
    expect(menuAktif('/sdm/gaji', semua)).toBe('/sdm/gaji')
  })

  it('halaman turunan tanpa menu sendiri menyalakan induknya', () => {
    expect(menuAktif('/stok/masuk', semua)).toBe('/stok')
    expect(menuAktif('/pengaturan/peran', semua)).toBe('/pengaturan')
  })

  it('Kasir bukan baris menu (ia tombol utama), jadi halamannya tidak menyalakan apa pun', () => {
    expect(menuAktif('/kasir/tutup-shift', semua)).toBeUndefined()
  })

  it('Beranda hanya untuk "/" persis, dan awalan dicocokkan per ruas', () => {
    expect(menuAktif('/', semua)).toBe('/')
    expect(menuAktif('/laporan', semua)).toBe('/laporan')
    // "/stokopname" bukan turunan "/stok".
    expect(menuAktif('/stokopname', semua)).toBeUndefined()
  })
})

describe('tujuanSamping', () => {
  it('memuat menu atas, kelompok, dan kaki — dan tunduk pada izin', () => {
    expect(semua).toEqual(expect.arrayContaining(['/', '/laporan', '/stok', '/langganan', '/pengaturan']))
    // Tanpa izin apa pun: hanya Beranda (tanpa syarat izin).
    expect(tujuanSamping(() => false)).toEqual(['/'])
  })
})
