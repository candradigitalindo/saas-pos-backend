import { describe, expect, it } from 'vitest'
import {
  daftarTanggal,
  geserTanggal,
  labelHariPendek,
  labelRentang,
  lengkapiHari,
  periodeBerpembanding,
  ringkasPeriode,
} from './deret'
import type { BarisLaporan } from './api'

const baris = (key: string, net: number, n = 1): BarisLaporan => ({
  key,
  sales_count: n,
  gross_amount: net,
  discount_amount: 0,
  tax_amount: 0,
  net_amount: net,
  cost_amount: 0,
  fee_amount: 0,
  gross_profit: net,
})

describe('geserTanggal', () => {
  it('melintasi akhir bulan dan akhir tahun', () => {
    expect(geserTanggal('2026-09-30', 1)).toBe('2026-10-01')
    expect(geserTanggal('2026-01-01', -1)).toBe('2025-12-31')
    expect(geserTanggal('2028-02-28', 1)).toBe('2028-02-29') // tahun kabisat
  })
})

describe('daftarTanggal', () => {
  it('inklusif di kedua ujung', () => {
    expect(daftarTanggal('2026-09-29', '2026-10-02')).toEqual([
      '2026-09-29',
      '2026-09-30',
      '2026-10-01',
      '2026-10-02',
    ])
  })

  it('rentang terbalik menghasilkan daftar kosong, bukan perulangan tanpa ujung', () => {
    expect(daftarTanggal('2026-09-10', '2026-09-01')).toEqual([])
  })
})

describe('lengkapiHari', () => {
  it('hari tanpa penjualan diisi nol, bukan dilewati', () => {
    const hasil = lengkapiHari([baris('2026-09-10', 5000), baris('2026-09-12', 7000)], '2026-09-09', '2026-09-12')
    expect(hasil.map((r) => [r.key, r.net_amount])).toEqual([
      ['2026-09-09', 0],
      ['2026-09-10', 5000],
      ['2026-09-11', 0],
      ['2026-09-12', 7000],
    ])
  })

  it('data belum datang tetap menghasilkan deret nol sepanjang periode', () => {
    expect(lengkapiHari(undefined, '2026-09-01', '2026-09-07')).toHaveLength(7)
  })
})

describe('ringkasPeriode', () => {
  it('rata-rata dibagi SEMUA hari, termasuk yang sepi', () => {
    const r = ringkasPeriode(lengkapiHari([baris('2026-09-02', 70000, 7)], '2026-09-01', '2026-09-07'))
    expect(r.uangMasuk).toBe(70000)
    expect(r.transaksi).toBe(7)
    expect(r.rataPerHari).toBe(10000)
    expect(r.hariTeramai).toEqual({ tanggal: '2026-09-02', uangMasuk: 70000 })
  })

  it('periode tanpa penjualan tidak punya hari teramai', () => {
    expect(ringkasPeriode(lengkapiHari([], '2026-09-01', '2026-09-03')).hariTeramai).toBeUndefined()
  })
})

describe('periodeBerpembanding', () => {
  it('periode pembanding sama panjang dan tepat bersambung', () => {
    expect(periodeBerpembanding('2026-09-27', 7)).toEqual({
      dari: '2026-09-21',
      sampai: '2026-09-27',
      dariLalu: '2026-09-14',
      sampaiLalu: '2026-09-20',
    })
  })
})

describe('label tanggal', () => {
  it('rentang sebulan menyebut bulannya sekali, lintas bulan dua kali', () => {
    expect(labelRentang('2026-09-21', '2026-09-27')).toBe('21–27 Sep')
    expect(labelRentang('2026-08-29', '2026-09-27')).toBe('29 Agu – 27 Sep')
  })

  it('hari pendek memuat nama hari', () => {
    expect(labelHariPendek('2026-09-18')).toBe('Jum, 18 Sep')
  })
})
