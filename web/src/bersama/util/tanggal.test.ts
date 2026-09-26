import { describe, expect, it } from 'vitest'
import { formatJam, keDate } from './tanggal'

describe('tanggal', () => {
  it('RFC 3339 UTC dari server tampil dalam jam WIB', () => {
    expect(formatJam('2026-09-26T00:22:10Z')).toBe('07.22')
  })

  it('bentuk lama tanpa zona dibaca sebagai UTC, bukan jam lokal perangkat', () => {
    expect(keDate('2026-09-26 00:22:10').toISOString()).toBe('2026-09-26T00:22:10.000Z')
    expect(formatJam('2026-09-26 00:22:10')).toBe('07.22')
  })

  it('string yang sudah membawa zona tidak diubah', () => {
    expect(keDate('2026-09-26T07:22:10+07:00').toISOString()).toBe('2026-09-26T00:22:10.000Z')
  })
})
