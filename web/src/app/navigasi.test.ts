import { describe, expect, it } from 'vitest'
import { KELOMPOK_SAMPING, menuAktif } from './navigasi'

const semua = ['/', ...KELOMPOK_SAMPING.flatMap((k) => k.item.map((m) => m.ke))]

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
    expect(menuAktif('/kasir/tutup-shift', semua)).toBe('/kasir')
  })

  it('Beranda hanya untuk "/" persis, dan awalan dicocokkan per ruas', () => {
    expect(menuAktif('/', semua)).toBe('/')
    expect(menuAktif('/laporan', semua)).toBe('/laporan')
    // "/stokopname" bukan turunan "/stok".
    expect(menuAktif('/stokopname', semua)).toBeUndefined()
  })
})
