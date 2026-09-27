import { describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { PaletPerintah, saringPalet } from './palet-perintah'

/*
 * Sesi DITIRU: yang diuji isi & perilaku palet (penyaringan izin, kata kunci,
 * Enter pindah halaman, gembok fitur), bukan cara profil dimuat.
 */
const sesi = vi.hoisted(() => ({ izin: new Set<string>(), fitur: {} as Record<string, boolean> }))
vi.mock('@/bersama/hooks/use-sesi', () => ({
  useSesi: () => ({
    boleh: (...k: string[]) => k.some((x) => sesi.izin.has(x)),
    punyaFitur: (k: string) => !!sesi.fitur[k],
  }),
}))

function tampilkan() {
  const onBukaBerubah = vi.fn()
  render(
    <MemoryRouter initialEntries={['/']}>
      <Routes>
        <Route path="*" element={<PaletPerintah buka onBukaBerubah={onBukaBerubah} />} />
      </Routes>
      <Routes>
        <Route path="/kasbon" element={<p>HALAMAN KASBON</p>} />
        <Route path="/stok/masuk" element={<p>HALAMAN BARANG MASUK</p>} />
        <Route path="*" element={null} />
      </Routes>
    </MemoryRouter>,
  )
  return { onBukaBerubah, kotak: screen.getByPlaceholderText('Cari halaman atau aksi…') }
}

describe('PaletPerintah', () => {
  it('kata lain menemukan halamannya: "piutang" → Kasbon, dan Enter membukanya', () => {
    sesi.izin = new Set(['receivable.manage', 'customer.view'])
    const { onBukaBerubah, kotak } = tampilkan()

    fireEvent.change(kotak, { target: { value: 'piutang' } })
    const opsi = screen.getAllByRole('option')
    expect(opsi[0]).toHaveTextContent('Kasbon')

    fireEvent.keyDown(kotak, { key: 'Enter' })
    expect(screen.getByText('HALAMAN KASBON')).toBeInTheDocument()
    expect(onBukaBerubah).toHaveBeenCalledWith(false)
  })

  it('aksi cepat ikut dicari: "kulakan" → Catat barang masuk', () => {
    sesi.izin = new Set(['stock.adjust', 'stock.view'])
    const { kotak } = tampilkan()
    fireEvent.change(kotak, { target: { value: 'kulakan' } })
    fireEvent.keyDown(kotak, { key: 'Enter' })
    expect(screen.getByText('HALAMAN BARANG MASUK')).toBeInTheDocument()
  })

  it('tanpa izin, halamannya tidak ditawarkan sama sekali', () => {
    sesi.izin = new Set(['sale.create'])
    tampilkan()
    expect(screen.queryByRole('option', { name: /Gaji/ })).not.toBeInTheDocument()
    expect(screen.getByRole('option', { name: /Buka Kasir/ })).toBeInTheDocument()
  })

  it('fitur yang terkunci paket tetap muncul, bertanda terkunci', () => {
    sesi.izin = new Set(['channel.manage'])
    sesi.fitur = {}
    tampilkan()
    expect(screen.getByRole('option', { name: /Kanal Online.*terkunci paket/ })).toBeInTheDocument()
  })
})

describe('saringPalet', () => {
  it('mencocokkan kata utuh atau potongannya, bukan huruf berserakan', () => {
    // Dulu (pencocokan kabur bawaan) "hutang" ikut menemukan "Hitung Fisik".
    expect(saringPalet('Hitung Fisik /stok/opname', 'hutang', ['opname'])).toBe(0)
    expect(saringPalet('Kasbon /kasbon', 'hutang', ['piutang', 'hutang'])).toBeGreaterThan(0)
  })

  it('label yang diawali kata yang dicari diutamakan, tanpa peduli huruf besar', () => {
    const label = saringPalet('Barang /barang', 'BAR', ['produk'])
    const kunci = saringPalet('Kategori & Satuan /barang/master', 'satuan', ['unit'])
    expect(label).toBe(1)
    expect(kunci).toBeLessThanOrEqual(1)
    expect(saringPalet('Barang /barang', 'produk', ['produk'])).toBe(0.6)
  })

  it('beberapa kata harus semuanya ada', () => {
    expect(saringPalet('Catat barang masuk /stok/masuk', 'barang masuk')).toBe(0.8)
    expect(saringPalet('Catat barang masuk /stok/masuk', 'barang keluar')).toBe(0)
  })
})
