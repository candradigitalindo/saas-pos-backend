import { describe, expect, it } from 'vitest'
import { teksTagihan } from './tagihan'

const norm = (s: string) => s.replace(/ /g, ' ')

describe('teksTagihan', () => {
  it('merinci nota, jatuh tempo, dan total', () => {
    const t = teksTagihan({
      nama: 'Bu Sari',
      toko: 'Warung Mainyuk',
      total: 38000,
      hariIni: '2026-09-30',
      rincian: [
        { tanggal: '2026-09-12', nota: 'MY-260912-0003', sisa: 30000, jatuhTempo: '2026-09-26' },
        { tanggal: '2026-09-15', sisa: 8000, jatuhTempo: '2026-10-15' },
      ],
    })
    expect(norm(t)).toBe(
      'Halo Bu Sari, ini dari Warung Mainyuk. Mengingatkan kasbon yang belum dibayar:\n' +
        '1. 12 Sep 2026, nota MY-260912-0003 — Rp 30.000, sudah lewat jatuh tempo 4 hari\n' +
        '2. 15 Sep 2026 — Rp 8.000, jatuh tempo 15 Okt 2026\n\n' +
        'Total: Rp 38.000\n\n' +
        'Bisa dibayar langsung di toko atau lewat transfer. Terima kasih 🙏',
    )
  })

  it('ringkas tanpa rincian', () => {
    const t = teksTagihan({
      nama: 'Pak Budi',
      toko: 'Warung Mainyuk',
      total: 15000,
      hariIni: '2026-09-30',
      jumlahNota: 2,
      jatuhTempo: '2026-09-30',
    })
    expect(norm(t)).toBe(
      'Halo Pak Budi, ini dari Warung Mainyuk. Mengingatkan kasbon yang belum dibayar: Rp 15.000 (2 nota), jatuh tempo hari ini.\n\n' +
        'Bisa dibayar langsung di toko atau lewat transfer. Terima kasih 🙏',
    )
  })

  it('lebih dari lima nota: sisanya disebut jumlahnya', () => {
    const rincian = Array.from({ length: 7 }, (_, i) => ({ sisa: 1000 * (i + 1) }))
    const t = teksTagihan({ nama: 'A', toko: 'B', total: 28000, hariIni: '2026-09-30', rincian })
    expect(t).toContain('5. Rp')
    expect(t).not.toContain('6. Rp')
    expect(t).toContain('… dan 2 nota lainnya')
  })
})
