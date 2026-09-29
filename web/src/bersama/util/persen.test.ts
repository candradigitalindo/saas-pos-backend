import { describe, expect, it } from 'vitest'
import { pecahanKePersen, persenKePecahan } from './persen'

describe('persen ⇄ pecahan tarif', () => {
  it('menerima persen bulat dan berkoma', () => {
    expect(persenKePecahan('10')).toBe('0.1')
    expect(persenKePecahan('11')).toBe('0.11')
    expect(persenKePecahan('7,5')).toBe('0.075')
    expect(persenKePecahan('7.5')).toBe('0.075')
    expect(persenKePecahan('0,01')).toBe('0.0001')
    expect(persenKePecahan(' ')).toBe('0')
  })

  it('menolak yang bukan persen sah', () => {
    for (const salah of ['100', '-5', 'sepuluh', '7,555', '1e1', '10%']) {
      expect(persenKePecahan(salah)).toBeNull()
    }
  })

  it('pecahan dari server tampil sebagai persen berkoma', () => {
    expect(pecahanKePersen('0.1')).toBe('10')
    expect(pecahanKePersen('0.075')).toBe('7,5')
    expect(pecahanKePersen('0')).toBe('')
    expect(pecahanKePersen(undefined)).toBe('')
  })

  it('bolak-balik tidak mengubah nilai', () => {
    for (const p of ['10', '11', '7,5', '12,25']) {
      expect(pecahanKePersen(persenKePecahan(p)!)).toBe(p)
    }
  })
})
