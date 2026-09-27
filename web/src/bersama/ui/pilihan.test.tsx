import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { Pilihan } from './pilihan'

/*
 * Radix Popover DITIRU di tes ini. Di jsdom, Popover sungguhan (penempatannya
 * lewat floating-ui) membuat tiap tes yang membukanya menunggu ±13 detik dalam
 * keadaan menganggur — diukur dengan profil CPU: prosesnya idle, bukan sibuk,
 * dan jedanya menumpuk antar tes. Di peramban sungguhan pilihan ini terbuka
 * dalam ±110 ms. Tiruannya menjaga kontrak yang dipakai komponen — buka/tutup
 * lewat `open`/`onOpenChange`, Esc menutup, fokus pindah ke kotak cari — jadi
 * yang diuji di sini adalah logika KITA (penyaringan, pilihan, papan ketik
 * cmdk). Penempatan & interaksi Radix sungguhan diuji di peramban lewat
 * Playwright.
 */
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
      const ref = React.useRef<HTMLDivElement>(null)
      React.useEffect(() => {
        if (k.open) ref.current?.querySelector('input')?.focus()
      }, [k.open])
      if (!k.open) return null
      return (
        <div ref={ref} onKeyDown={(e) => e.key === 'Escape' && k.onOpenChange(false)}>
          {children}
        </div>
      )
    },
  }
})

function Contoh({ onUbah = () => {} }: { onUbah?: (v: string) => void }) {
  const [nilai, setNilai] = useState('')
  return (
    <Pilihan
      label="Pemasok"
      value={nilai}
      onChange={(e) => {
        setNilai(e.target.value)
        onUbah(e.target.value)
      }}
    >
      <option value="">Tanpa pemasok</option>
      {['Toko Sumber Rejeki', 'CV Maju Jaya', 'Café Kopi Nusantara'].map((n, i) => (
        <option key={n} value={`P${i}`}>
          {n}
        </option>
      ))}
    </Pilihan>
  )
}

const pemicu = () => screen.getByRole('combobox', { name: 'Pemasok' })
const kotakCari = () => screen.getByRole('combobox', { name: 'Cari pemasok' })
const buka = () => fireEvent.click(pemicu())
const ketik = (teks: string) => fireEvent.change(kotakCari(), { target: { value: teks } })

describe('Pilihan — kolom pilihan dengan pencarian', () => {
  it('menampilkan label opsi terpilih, termasuk opsi bernilai kosong', () => {
    render(<Contoh />)
    expect(pemicu()).toHaveTextContent('Tanpa pemasok')
  })

  it('kotak cari menyaring daftar tanpa peduli huruf besar & aksen', () => {
    render(<Contoh />)
    buka()
    expect(kotakCari()).toHaveFocus()
    expect(screen.getAllByRole('option')).toHaveLength(4)

    ketik('CAFE')
    const opsi = screen.getAllByRole('option')
    expect(opsi).toHaveLength(1)
    expect(opsi[0]).toHaveTextContent('Café Kopi Nusantara')
  })

  it('Enter memilih hasil pertama dan memanggil onChange dengan nilainya', () => {
    const onUbah = vi.fn()
    render(<Contoh onUbah={onUbah} />)
    buka()
    ketik('maju')
    fireEvent.keyDown(kotakCari(), { key: 'Enter' })

    expect(onUbah).toHaveBeenCalledWith('P1')
    expect(pemicu()).toHaveTextContent('CV Maju Jaya')
    expect(screen.queryByRole('option')).not.toBeInTheDocument()
  })

  it('panah bawah + Enter memilih opsi berikutnya', () => {
    const onUbah = vi.fn()
    render(<Contoh onUbah={onUbah} />)
    buka()
    fireEvent.keyDown(kotakCari(), { key: 'ArrowDown' })
    fireEvent.keyDown(kotakCari(), { key: 'Enter' })
    expect(onUbah).toHaveBeenCalledWith('P0')
  })

  it('pencarian tanpa hasil mengatakannya, bukan daftar kosong yang diam', () => {
    render(<Contoh />)
    buka()
    ketik('zzz')
    expect(screen.getByText(/Tidak ada yang cocok dengan/)).toBeInTheDocument()
  })

  it('kotak cari kosong lagi saat dibuka ulang', () => {
    render(<Contoh />)
    buka()
    ketik('toko')
    fireEvent.keyDown(kotakCari(), { key: 'Escape' })
    buka()
    expect(kotakCari()).toHaveValue('')
    expect(screen.getAllByRole('option')).toHaveLength(4)
  })
})
