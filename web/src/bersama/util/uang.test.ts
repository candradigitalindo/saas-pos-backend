import { describe, expect, it } from 'vitest'
import {
  formatAngka,
  formatRupiah,
  parseRupiah,
  persenSelisih,
  pratinjauBaris,
} from './uang'

describe('formatRupiah', () => {
  it('memberi pemisah ribuan dan tanpa desimal', () => {
    expect(formatRupiah(1250000)).toBe('Rp 1.250.000')
    expect(formatRupiah(0)).toBe('Rp 0')
    expect(formatRupiah(999)).toBe('Rp 999')
  })

  it('memakai tanda minus panjang untuk nilai negatif', () => {
    expect(formatRupiah(-75000)).toBe('−Rp 75.000')
  })

  it('memotong pecahan, tidak membulatkan ke atas', () => {
    expect(formatRupiah(1000.9)).toBe('Rp 1.000')
  })
})

describe('parseRupiah', () => {
  it('membaca kembali teks berformat', () => {
    expect(parseRupiah('Rp 1.250.000')).toBe(1250000)
    expect(parseRupiah('1.250.000')).toBe(1250000)
    expect(parseRupiah('50000')).toBe(50000)
  })

  it('tidak pernah menghasilkan NaN', () => {
    expect(parseRupiah('')).toBe(0)
    expect(parseRupiah('abc')).toBe(0)
    expect(parseRupiah('Rp ')).toBe(0)
  })

  it('bolak-balik dengan formatAngka tetap sama nilainya', () => {
    for (const n of [0, 7, 1500, 1250000, 987654321]) {
      expect(parseRupiah(formatAngka(n))).toBe(n)
    }
  })
})

describe('persenSelisih', () => {
  it('menghitung kenaikan dan penurunan', () => {
    expect(persenSelisih(112, 100)).toBe(12)
    expect(persenSelisih(88, 100)).toBe(-12)
  })

  it('mengembalikan null bila pembanding nol (tidak ada arti "naik tak hingga")', () => {
    expect(persenSelisih(1000, 0)).toBeNull()
  })
})

describe('pratinjauBaris', () => {
  it('mengalikan harga bulat dengan qty desimal', () => {
    expect(pratinjauBaris(18000, '2')).toBe(36000)
    expect(pratinjauBaris(20000, '0.5')).toBe(10000)
  })

  it('membulatkan per baris, bukan menyimpan pecahan', () => {
    // 3333 × 3 = 9999 tepat; 3333.33 tidak pernah muncul karena harga bulat.
    expect(pratinjauBaris(3333, '3')).toBe(9999)
    // qty 0.333 → 18000 × 0.333 = 5994
    expect(pratinjauBaris(18000, '0.333')).toBe(5994)
  })

  it('mengurangi diskon dan tidak pernah negatif', () => {
    expect(pratinjauBaris(18000, '2', 6000)).toBe(30000)
    expect(pratinjauBaris(18000, '1', 99000)).toBe(0)
  })

  it('qty tak terbaca menghasilkan 0, bukan NaN', () => {
    expect(pratinjauBaris(18000, '')).toBe(0)
  })
})
