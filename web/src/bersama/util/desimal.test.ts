import { describe, expect, it } from 'vitest'
import { bandingQty, formatQty, kurangQty, qtyKosong, tambahQty } from './desimal'

describe('aritmetika qty', () => {
  it('menjumlah tanpa galat pecahan biner', () => {
    // 0.1 + 0.2 dengan Number menghasilkan 0.30000000000000004
    expect(tambahQty('0.1', '0.2')).toBe('0.3')
    expect(tambahQty('44', 1)).toBe('45')
  })

  it('mengurangi tanpa menembus batas bawah', () => {
    expect(kurangQty('2', 1)).toBe('1')
    expect(kurangQty('1', 5)).toBe('0')
    expect(kurangQty('1', 5, '-99')).toBe('-4')
  })

  it('membandingkan sebagai angka, bukan sebagai teks', () => {
    expect(bandingQty('10', '9')).toBe(1) // "10" < "9" bila dibanding teks
    expect(bandingQty('0.5', '0.50')).toBe(0)
  })

  it('mengenali qty habis', () => {
    expect(qtyKosong('0')).toBe(true)
    expect(qtyKosong('0.000')).toBe(true)
    expect(qtyKosong('-3')).toBe(true)
    expect(qtyKosong('0.5')).toBe(false)
  })
})

describe('formatQty', () => {
  it('membuang nol berlebih dan memakai koma desimal', () => {
    expect(formatQty('44.000')).toBe('44')
    expect(formatQty('1.5')).toBe('1,5')
  })
})
