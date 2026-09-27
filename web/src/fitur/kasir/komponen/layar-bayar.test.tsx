import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { LayarBayar } from './layar-bayar'

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

    expect(onSelesai).toHaveBeenCalledWith('cash', 44000)
  })

  it('QRIS tidak menanyakan uang diterima dan dikirim pas', async () => {
    const p = userEvent.setup()
    const { onSelesai } = tampilkan({ total: 44000 })

    await p.click(screen.getByRole('button', { name: /qris/i }))

    expect(screen.queryByLabelText(/uang diterima/i)).not.toBeInTheDocument()
    await p.click(screen.getByRole('button', { name: /selesai/i }))
    expect(onSelesai).toHaveBeenCalledWith('qris', 44000)
  })

  it('kasbon memperingatkan bahwa belanja dicatat sebagai utang', async () => {
    const p = userEvent.setup()
    tampilkan()
    await p.click(screen.getByRole('button', { name: /kasbon/i }))
    expect(screen.getByText(/dicatat sebagai utang pelanggan/i)).toBeInTheDocument()
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
