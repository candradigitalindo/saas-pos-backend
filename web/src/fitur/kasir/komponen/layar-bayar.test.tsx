import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { LayarBayar } from './layar-bayar'

// Popover Radix ditiru (lihat pilihan.test.tsx): yang sungguhan membuat tiap
// tes di jsdom menganggur belasan detik. Kontrak buka/tutupnya tetap dijaga.
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

const PELANGGAN = [
  { id: 'P1', name: 'Bu Rina', phone: '0811', credit_limit: 500000 },
  { id: 'P2', name: 'Pak Andi', phone: null, credit_limit: 0 },
]

function tampilkan(props: Partial<Parameters<typeof LayarBayar>[0]> = {}) {
  const onSelesai = vi.fn()
  render(
    <LayarBayar
      terbuka
      onTutup={() => {}}
      total={44000}
      mengirim={false}
      onSelesai={onSelesai}
      {...props}
    />,
  )
  return { onSelesai }
}

describe('layar bayar', () => {
  it('menampilkan total belanja', () => {
    tampilkan()
    expect(screen.getByText('Rp 44.000')).toBeInTheDocument()
  })

  it('menghitung kembalian dari uang yang diterima', async () => {
    const p = userEvent.setup()
    tampilkan({ total: 44000 })

    await p.type(screen.getByLabelText(/uang diterima/i), '50000')

    expect(screen.getByText('KEMBALIAN')).toBeInTheDocument()
    // 50.000 − 44.000 = 6.000, persis contoh di ui/05-ALUR-UTAMA.md
    expect(screen.getByText('Rp 6.000')).toBeInTheDocument()
  })

  it('menahan tombol Selesai selama uang belum cukup', async () => {
    const p = userEvent.setup()
    const { onSelesai } = tampilkan({ total: 44000 })

    await p.type(screen.getByLabelText(/uang diterima/i), '40000')

    expect(screen.getByText('Uang belum cukup')).toBeInTheDocument()
    // Kekurangannya disebut angkanya, bukan sekadar "tidak valid".
    expect(screen.getByText('Rp 4.000')).toBeInTheDocument()

    const selesai = screen.getByRole('button', { name: /selesai/i })
    expect(selesai).toBeDisabled()
    await p.click(selesai)
    expect(onSelesai).not.toHaveBeenCalled()
  })

  it('pintasan "Pas" mengisi tepat sebesar total', async () => {
    const p = userEvent.setup()
    const { onSelesai } = tampilkan({ total: 44000 })

    await p.click(screen.getByRole('button', { name: 'Pas' }))
    await p.click(screen.getByRole('button', { name: /selesai/i }))

    expect(onSelesai).toHaveBeenCalledWith('cash', 44000, undefined)
  })

  it('QRIS tidak menanyakan uang diterima dan dikirim pas', async () => {
    const p = userEvent.setup()
    const { onSelesai } = tampilkan({ total: 44000 })

    await p.click(screen.getByRole('button', { name: /qris/i }))

    expect(screen.queryByLabelText(/uang diterima/i)).not.toBeInTheDocument()
    await p.click(screen.getByRole('button', { name: /selesai/i }))
    expect(onSelesai).toHaveBeenCalledWith('qris', 44000, undefined)
  })

  it('kasbon memperingatkan bahwa belanja dicatat sebagai utang', async () => {
    const p = userEvent.setup()
    tampilkan()
    await p.click(screen.getByRole('button', { name: /kasbon/i }))
    expect(screen.getByText(/dicatat sebagai utang pelanggan/i)).toBeInTheDocument()
  })

  it('kasbon wajib memilih pelanggan — tanpa itu Selesai terkunci', async () => {
    const p = userEvent.setup()
    const { onSelesai } = tampilkan({ total: 44000, pelanggan: PELANGGAN })
    await p.click(screen.getByRole('button', { name: /kasbon/i }))

    const selesai = screen.getByRole('button', { name: /selesai/i })
    expect(selesai).toBeDisabled()

    // Pilihnya lewat daftar yang bisa dicari.
    fireEvent.click(screen.getByRole('combobox', { name: /^Pelanggan/ }))
    fireEvent.change(screen.getByRole('combobox', { name: 'Cari pelanggan' }), { target: { value: 'rina' } })
    fireEvent.keyDown(screen.getByRole('combobox', { name: 'Cari pelanggan' }), { key: 'Enter' })

    expect(screen.getByText(/Batas kasbon Bu Rina: Rp.500\.000/)).toBeInTheDocument()
    expect(screen.getByText(/dicatat sebagai utang Bu Rina sebesar/)).toBeInTheDocument()
    await p.click(screen.getByRole('button', { name: /selesai/i }))
    expect(onSelesai).toHaveBeenCalledWith('credit', 44000, 'P1')
  })

  it('kasbon tanpa pelanggan terdaftar: menyuruh menambah pelanggan dulu', async () => {
    const p = userEvent.setup()
    tampilkan({ pelanggan: [] })
    await p.click(screen.getByRole('button', { name: /kasbon/i }))
    expect(screen.getByText(/Belum ada pelanggan/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /selesai/i })).toBeDisabled()
  })

  it('menampilkan pesan galat apa adanya, tanpa kode status', () => {
    tampilkan({ galat: 'Belum ada internet. Data disimpan di HP dan dikirim otomatis nanti.' })
    expect(screen.getByText(/dikirim otomatis nanti/i)).toBeInTheDocument()
  })

  it('mengunci tombol saat transaksi sedang dikirim (cegah tekan dua kali)', () => {
    tampilkan({ mengirim: true })
    expect(screen.getByRole('button', { name: /menyimpan transaksi/i })).toBeDisabled()
  })

  it('QRIS terkunci paket: petaknya tetap ada, tidak bisa dipilih, dan menyebut paketnya', async () => {
    const p = userEvent.setup()
    const { onSelesai } = tampilkan({ total: 15000, kunciQris: 'Paket Basic' })

    const qris = screen.getByRole('button', { name: /QRIS, terkunci — tersedia di Paket Basic/ })
    expect(qris).toBeDisabled()
    expect(screen.getByText('Paket Basic')).toBeInTheDocument()

    // Ketukan pada petak terkunci tidak mengganti cara bayar: tetap tunai.
    await p.click(qris)
    expect(screen.getByLabelText(/uang diterima/i)).toBeInTheDocument()
    expect(onSelesai).not.toHaveBeenCalled()
  })

  it('tanpa kunci, QRIS bisa dipilih seperti biasa', () => {
    tampilkan()
    expect(screen.getByRole('button', { name: 'QRIS' })).toBeEnabled()
  })
})
