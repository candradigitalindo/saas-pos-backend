import { describe, expect, it } from 'vitest'
import { inisialNama, inisialPetak, inisialProduk } from './inisial'

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

describe('inisialNama', () => {
  it('huruf pertama dua kata pertama, huruf besar', () => {
    expect(inisialNama('Sari Dewi')).toBe('SD')
    expect(inisialNama('ahmad bin umar')).toBe('AB')
    expect(inisialNama('  budi  ')).toBe('B')
  })

  it('tanda baca tidak menjadi inisial', () => {
    expect(inisialNama('Tokopedia & Shop')).toBe('TS')
    expect(inisialNama('Warung (Pusat)')).toBe('WP')
    expect(inisialNama('&')).toBe('?')
  })

  it('nama kosong tidak menghasilkan avatar kosong', () => {
    expect(inisialNama('')).toBe('?')
  })
})

describe('inisialPetak', () => {
  it('dua huruf pertama dari dua kata pembeda, tanpa satuan', () => {
    expect(inisialPetak('Air Mineral 600ml')).toBe('AM')
    expect(inisialPetak('Kopi Susu Gula Aren')).toBe('KS')
    expect(inisialPetak('Gula Pasir 1kg')).toBe('GP')
  })

  it('satu kata: dua huruf pertamanya', () => {
    expect(inisialPetak('teh')).toBe('Te')
  })

  it('nama kosong tetap memberi penanda', () => {
    expect(inisialPetak('')).toBe('?')
  })
})
