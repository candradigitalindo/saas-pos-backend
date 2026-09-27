import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { BannerKunciFitur } from './kunci-fitur'

/*
 * Sesi DITIRU: yang diuji keputusan tampilan banner (kapan muncul, paket mana
 * yang disebut, siapa yang diberi tombol ke Langganan), bukan cara profil
 * dimuat.
 */
const sesi = vi.hoisted(() => ({
  fitur: {} as Record<string, boolean>,
  izin: new Set<string>(),
  naik: {} as Record<string, string>,
}))
vi.mock('@/bersama/hooks/use-sesi', () => ({
  useSesi: () => ({
    punyaFitur: (k: string) => !!sesi.fitur[k],
    paketUntuk: (k: string) => sesi.naik[k],
    boleh: (...k: string[]) => k.some((x) => sesi.izin.has(x)),
  }),
}))

function tampilkan() {
  render(
    <MemoryRouter>
      <BannerKunciFitur
        fitur="online_channel"
        nama="Kanal online"
        penjelasan="Pesanan lama tetap bisa dilihat."
      />
    </MemoryRouter>,
  )
}

describe('BannerKunciFitur', () => {
  it('tidak muncul bila fiturnya termasuk paket', () => {
    sesi.fitur = { online_channel: true }
    tampilkan()
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('menyebut paket termurah yang membukanya, dan pemilik mendapat tombol ke Langganan', () => {
    sesi.fitur = {}
    sesi.naik = { online_channel: 'Pro' }
    sesi.izin = new Set(['billing.manage'])
    tampilkan()
    expect(screen.getByText('Kanal online tersedia mulai paket Pro')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Lihat paket' })).toHaveAttribute('href', '/langganan')
  })

  it('pengguna tanpa izin langganan tidak diberi tombol, tapi diberi tahu siapa yang memutuskan', () => {
    sesi.fitur = {}
    sesi.naik = { online_channel: 'Pro' }
    sesi.izin = new Set()
    tampilkan()
    expect(screen.queryByRole('link', { name: 'Lihat paket' })).not.toBeInTheDocument()
    expect(screen.getByText(/Minta pemilik usaha menaikkan paket/)).toBeInTheDocument()
  })
})
