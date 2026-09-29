import { describe, expect, it } from 'vitest'
import { formatJam, formatLaluHari, keDate, zonaIndonesiaPerangkat } from './tanggal'

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

  it('zona perangkat dipetakan ke tiga zona yang diterima server', () => {
    expect(zonaIndonesiaPerangkat('Asia/Jakarta')).toBe('Asia/Jakarta')
    expect(zonaIndonesiaPerangkat('Asia/Pontianak')).toBe('Asia/Jakarta')
    expect(zonaIndonesiaPerangkat('Asia/Makassar')).toBe('Asia/Makassar')
    expect(zonaIndonesiaPerangkat('Asia/Ujung_Pandang')).toBe('Asia/Makassar')
    expect(zonaIndonesiaPerangkat('Asia/Jayapura')).toBe('Asia/Jayapura')
    expect(zonaIndonesiaPerangkat('Asia/Singapore')).toBeUndefined()
  })
})

describe('formatLaluHari', () => {
  // 27 Sep 2026 10.00 WIB
  const kini = new Date('2026-09-27T03:00:00Z')

  it('menghitung menurut tanggal di zona toko, bukan selisih 24 jam', () => {
    // 26 Sep 23.00 WIB — baru 11 jam lalu, tapi tanggalnya kemarin.
    expect(formatLaluHari('2026-09-26T16:00:00Z', kini)).toBe('Kemarin')
    expect(formatLaluHari('2026-09-27T01:00:00Z', kini)).toBe('Hari ini')
  })

  it('hari lalu sampai sebulan, lalu tanggal lengkap', () => {
    expect(formatLaluHari('2026-09-18T05:00:00Z', kini)).toBe('9 hari lalu')
    expect(formatLaluHari('2026-08-01T05:00:00Z', kini)).toBe('1 Agu 2026')
  })
})
