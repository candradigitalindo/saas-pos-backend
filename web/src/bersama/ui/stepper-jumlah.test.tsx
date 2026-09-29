import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { StepperJumlah } from './stepper-jumlah'

function Uji({ awal = '5', minimal, onNilai }: { awal?: string; minimal?: string; onNilai?: (q: string) => void }) {
  const [nilai, setNilai] = useState(awal)
  return (
    <StepperJumlah
      nilai={nilai}
      minimal={minimal}
      satuan="pcs"
      label="Hitungan Kopi"
      onNilai={(q) => {
        onNilai?.(q)
        setNilai(q)
      }}
    />
  )
}

const kolom = () => screen.getByRole('textbox', { name: 'Hitungan Kopi' })

describe('StepperJumlah — angka bisa diketik', () => {
  it('ketik lalu Enter menyimpan, tanpa mengetuk + berkali-kali', async () => {
    const onNilai = vi.fn()
    render(<Uji onNilai={onNilai} />)
    await userEvent.click(kolom())
    await userEvent.keyboard('146{Enter}')
    expect(onNilai).toHaveBeenLastCalledWith('146')
    expect(kolom()).toHaveValue('146')
  })

  it('koma desimal diterima', async () => {
    render(<Uji />)
    await userEvent.click(kolom())
    await userEvent.keyboard('2,5{Enter}')
    expect(kolom()).toHaveValue('2,5')
  })

  it('isian bukan angka atau di bawah minimum dikembalikan ke nilai lama', async () => {
    const onNilai = vi.fn()
    render(<Uji minimal="1" onNilai={onNilai} />)
    await userEvent.click(kolom())
    await userEvent.keyboard('abc{Enter}')
    expect(kolom()).toHaveValue('5')
    await userEvent.click(kolom())
    await userEvent.keyboard('0{Enter}')
    expect(kolom()).toHaveValue('5')
    expect(onNilai).not.toHaveBeenCalled()
  })

  it('Escape membatalkan ketikan', async () => {
    const onNilai = vi.fn()
    render(<Uji onNilai={onNilai} />)
    await userEvent.click(kolom())
    await userEvent.keyboard('99{Escape}')
    expect(kolom()).toHaveValue('5')
    expect(onNilai).not.toHaveBeenCalled()
  })

  it('tombol − dan + tetap bekerja', async () => {
    render(<Uji />)
    await userEvent.click(screen.getByRole('button', { name: 'Tambah hitungan kopi' }))
    expect(kolom()).toHaveValue('6')
    await userEvent.click(screen.getByRole('button', { name: 'Kurangi hitungan kopi' }))
    await userEvent.click(screen.getByRole('button', { name: 'Kurangi hitungan kopi' }))
    expect(kolom()).toHaveValue('4')
  })
})
