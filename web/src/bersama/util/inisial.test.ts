import { describe, expect, it } from 'vitest'
import { inisialProduk } from './inisial'

describe('inisialProduk', () => {
  it('mengambil tiga huruf pertama tiap kata', () => {
    expect(inisialProduk('Indomie Goreng')).toBe('IndGor')
    expect(inisialProduk('Nasi Goreng Spesial')).toBe('NasGorSpe')
  })

  it('membuang angka & satuan bila masih ada kata pembeda', () => {
    // "1kg" dan "600ml" dimiliki banyak barang; menyertakannya justru
    // membuat dua barang berbeda terlihat mirip.
    expect(inisialProduk('Gula Pasir 1kg')).toBe('GulPas')
    expect(inisialProduk('Air Mineral 600ml')).toBe('AirMin')
  })

  it('mempertahankan angka bila memang hanya itu namanya', () => {
    expect(inisialProduk('1kg')).toBe('1kg')
  })

  it('memotong pada tiga kata supaya tetap muat di kartu sempit', () => {
    expect(inisialProduk('Kopi Susu Gula Aren')).toBe('KopSusGul')
  })

  it('menangani nama satu kata dan kata pendek', () => {
    expect(inisialProduk('Teh')).toBe('Teh')
    expect(inisialProduk('Es')).toBe('Es')
  })

  it('tahan terhadap nama kosong atau aneh', () => {
    expect(inisialProduk('')).toBe('?')
    expect(inisialProduk('   ')).toBe('?')
    expect(inisialProduk('---')).toBe('?')
  })

  it('memperlakukan pemisah selain spasi sebagai batas kata', () => {
    expect(inisialProduk('Kopi/Susu')).toBe('KopSus')
    expect(inisialProduk('Mie-Ayam')).toBe('MieAya')
  })
})
