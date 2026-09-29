import { describe, expect, it } from 'vitest'
import { statusJatuhTempo, tambahHari } from './utang'

describe('statusJatuhTempo', () => {
  const hariIni = '2026-09-29'
  it.each([
    [undefined, 'tanpa', 'tanpa jatuh tempo'],
    ['2026-09-26', 'lewat', 'lewat 3 hari'],
    ['2026-09-29', 'hariIni', 'jatuh tempo hari ini'],
    ['2026-09-30', 'dekat', 'jatuh tempo besok'],
    ['2026-10-04', 'dekat', '5 hari lagi'],
  ] as const)('%s → %s', (jt, nada, teks) => {
    expect(statusJatuhTempo(jt, hariIni)).toEqual({ nada, teks })
  })
  it('lebih dari seminggu: tanggalnya', () => {
    const s = statusJatuhTempo('2026-10-20', hariIni)
    expect(s.nada).toBe('nanti')
    expect(s.teks).toMatch(/^jatuh tempo 20 Okt/)
  })
})

describe('tambahHari', () => {
  it('melewati akhir bulan', () => {
    expect(tambahHari('2026-09-29', 7)).toBe('2026-10-06')
    expect(tambahHari('2026-12-30', 2)).toBe('2027-01-01')
  })
})
