import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { PemilihToko } from './pemilih-toko'

/*
 * Sesi & toast DITIRU: yang diuji di sini keputusan tampilan pemilih toko
 * (kapan ada tombol, apa yang disebut, apa yang terjadi saat memilih), bukan
 * cara sesi dimuat — itu sudah diuji di use-sesi.test.tsx.
 *
 * Radix Popover juga ditiru, dengan alasan yang sama seperti di
 * pilihan.test.tsx: Popover sungguhan di jsdom membuat tiap tes menganggur
 * ±13 detik. Tiruannya menjaga kontrak buka/tutup lewat open/onOpenChange.
 */
const sesi = vi.hoisted(() => ({
  nilai: {} as Record<string, unknown>,
}))
const toast = vi.hoisted(() => ({ berhasil: vi.fn() }))

vi.mock('@/bersama/hooks/use-sesi', () => ({ useSesi: () => sesi.nilai }))
vi.mock('@/bersama/komponen/toast', () => ({ useToast: () => toast }))
vi.mock('@radix-ui/react-popover', async () => {
  const React = await import('react')
  type Konteks = { open: boolean; onOpenChange: (o: boolean) => void }
  const K = React.createContext<Konteks>({ open: false, onOpenChange: () => {} })
  return {
    Root: ({ open, onOpenChange, children }: Konteks & { children: React.ReactNode }) => (
      <K.Provider value={{ open: !!open, onOpenChange }}>{children}</K.Provider>
    ),
    Trigger: ({ children }: { children: React.ReactElement<{ onClick?: () => void }> }) => {
      const k = React.useContext(K)
      return React.cloneElement(children, { onClick: () => k.onOpenChange(!k.open) })
    },
    Portal: ({ children }: { children: React.ReactNode }) => <>{children}</>,
    Content: ({ children }: { children: React.ReactNode }) => {
      const k = React.useContext(K)
      return k.open ? <div>{children}</div> : null
    },
  }
})

const toko = (id: string, name: string) => ({ id, name })

function aturSesi(outlets: { id: string; name: string }[], aktif: string, usaha = 'Warung Bu Sari') {
  const gantiToko = vi.fn()
  sesi.nilai = {
    profil: { tenant: { business_name: usaha }, outlets },
    tokoAktif: aktif,
    rincianToko: outlets.find((t) => t.id === aktif),
    gantiToko,
  }
  return gantiToko
}

describe('PemilihToko', () => {
  beforeEach(() => toast.berhasil.mockClear())

  it('satu toko bernama sama dengan usahanya: tanpa tombol, nama tidak diulang', () => {
    aturSesi([toko('A', 'Warung Bu Sari')], 'A')
    render(<PemilihToko />)
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
    expect(screen.getAllByText('Warung Bu Sari')).toHaveLength(1)
    expect(screen.getByText('Kasir UMKM')).toBeInTheDocument()
  })

  it('satu toko bernama lain: namanya disebut, tetap tanpa tombol', () => {
    aturSesi([toko('A', 'Cabang Pasar')], 'A')
    render(<PemilihToko />)
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
    expect(screen.getByText('Cabang Pasar')).toBeInTheDocument()
  })

  it('beberapa toko: tombol menyebut toko aktif, memilih toko lain memanggil gantiToko', () => {
    const gantiToko = aturSesi([toko('A', 'Pusat'), toko('B', 'Cabang Pasar')], 'A')
    render(<PemilihToko />)

    fireEvent.click(screen.getByRole('button', { name: /Toko yang dipakai: Pusat/ }))
    expect(screen.getByRole('button', { name: 'Pusat' })).toHaveAttribute('aria-current', 'true')

    fireEvent.click(screen.getByRole('button', { name: 'Cabang Pasar' }))
    expect(gantiToko).toHaveBeenCalledWith('B')
    expect(toast.berhasil).toHaveBeenCalledWith('Sekarang memakai Cabang Pasar')
  })

  it('memilih toko yang sudah aktif tidak mengganti apa pun', () => {
    const gantiToko = aturSesi([toko('A', 'Pusat'), toko('B', 'Cabang Pasar')], 'A')
    render(<PemilihToko />)
    fireEvent.click(screen.getByRole('button', { name: /Toko yang dipakai/ }))
    fireEvent.click(screen.getByRole('button', { name: 'Pusat' }))
    expect(gantiToko).not.toHaveBeenCalled()
    expect(toast.berhasil).not.toHaveBeenCalled()
  })
})
