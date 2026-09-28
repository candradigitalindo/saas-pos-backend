import { describe, expect, it } from 'vitest'
import { cukupHari, isiMeter, keadaanSaldo, lakuPerHari, teksCukup } from './keadaan-stok'

describe('keadaanSaldo — batasnya sama dengan server', () => {
  it.each([
    ['-2', '5', 'negative'],
    ['0', '0', 'out'],
    ['0', '5', 'out'],
    ['5', '5', 'low'],
    ['0.5', '0', 'safe'],
    ['6', '5', 'safe'],
  ] as const)('sisa %s, batas %s → %s', (qty, min_stock, mau) => {
    expect(keadaanSaldo({ qty, min_stock })).toBe(mau)
  })
})

describe('laku & perkiraan cukup', () => {
  it('laku per hari = keluar 30 hari ÷ 30', () => {
    expect(lakuPerHari({ sold_30d: '60' })).toBe(2)
    expect(lakuPerHari({ sold_30d: '0' })).toBe(0)
    expect(lakuPerHari({})).toBe(0)
  })

  it('cukup berapa hari: sisa ÷ laju', () => {
    expect(cukupHari({ qty: '10', sold_30d: '60' })).toBe(5)
  })

  it('tidak diperkirakan untuk barang habis atau yang tidak laku', () => {
    expect(cukupHari({ qty: '0', sold_30d: '60' })).toBeNull()
    expect(cukupHari({ qty: '-3', sold_30d: '60' })).toBeNull()
    expect(cukupHari({ qty: '10', sold_30d: '0' })).toBeNull()
  })

  it('pembulatannya kasar, sesuai ketelitian perkiraan', () => {
    expect(teksCukup(0.4)).toBe('habis hari ini')
    expect(teksCukup(2.4)).toBe('cukup ±2 hari')
    expect(teksCukup(20)).toBe('cukup ±3 minggu')
    expect(teksCukup(75)).toBe('cukup ±3 bulan')
    expect(teksCukup(400)).toBe('cukup > 3 bulan')
  })
})

describe('isiMeter', () => {
  it('garis batas tepat di tengah meteran', () => {
    expect(isiMeter({ qty: '5', min_stock: '5' })).toBe(0.5)
    expect(isiMeter({ qty: '2', min_stock: '4' })).toBe(0.25)
  })
  it('dijepit 0…1', () => {
    expect(isiMeter({ qty: '50', min_stock: '5' })).toBe(1)
    expect(isiMeter({ qty: '-3', min_stock: '5' })).toBe(0)
  })
  it('tanpa batas minimum tidak ada meteran', () => {
    expect(isiMeter({ qty: '5', min_stock: '0' })).toBeNull()
  })
})
