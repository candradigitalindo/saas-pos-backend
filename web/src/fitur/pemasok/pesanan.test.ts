import { describe, expect, it } from 'vitest'
import { jumlahSatuanBeli, teksPesanan } from './pesanan'

describe('jumlahSatuanBeli', () => {
  it('dibulatkan ke atas ke kemasan utuh', () => {
    expect(jumlahSatuanBeli(18, 12)).toBe(2)
    expect(jumlahSatuanBeli(24, 12)).toBe(2)
    expect(jumlahSatuanBeli(7, 1)).toBe(7)
  })
  it('paling sedikit satu', () => {
    expect(jumlahSatuanBeli(0, 12)).toBe(1)
  })
})

describe('teksPesanan', () => {
  it('bernomor, menyebut isi kemasan', () => {
    const t = teksPesanan('CV Maju', 'Warung Bu Sari', [
      { nama: 'Kopi Hitam', jumlah: 10, satuan: 'pcs', isi: 1, satuanDasar: 'pcs' },
      { nama: 'Gula Pasir', jumlah: 2, satuan: 'dus', isi: 12, satuanDasar: 'pcs' },
    ])
    expect(t).toBe(
      'Halo CV Maju, saya dari Warung Bu Sari. Mau pesan:\n1. Kopi Hitam — 10 pcs\n2. Gula Pasir — 2 dus (isi 12 pcs)\n\nTerima kasih 🙏',
    )
  })
})
