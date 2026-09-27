import { describe, expect, it } from 'vitest'
import { tujuanSetelahMasuk } from './tujuan-masuk'

describe('tujuanSetelahMasuk', () => {
  it('kembali ke halaman yang tadi dibuka', () => {
    expect(tujuanSetelahMasuk({ dari: '/kasir/ganti-shift' })).toBe('/kasir/ganti-shift')
  })
  it('tanpa asal → Beranda', () => {
    expect(tujuanSetelahMasuk(null)).toBe('/')
    expect(tujuanSetelahMasuk(undefined)).toBe('/')
    expect(tujuanSetelahMasuk({ dari: 42 })).toBe('/')
  })
  it('menolak alamat di luar aplikasi dan halaman masuk sendiri', () => {
    expect(tujuanSetelahMasuk({ dari: '//situs-lain.com' })).toBe('/')
    expect(tujuanSetelahMasuk({ dari: 'https://situs-lain.com' })).toBe('/')
    expect(tujuanSetelahMasuk({ dari: '/masuk' })).toBe('/')
  })
})
