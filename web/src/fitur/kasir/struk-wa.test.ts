import { describe, expect, it } from 'vitest'
import type { Transaksi } from '@/bersama/tipe/pos'
import { alamatStruk, nomorWA, tautanWA, teksStrukWA } from './struk-wa'

describe('nomorWA', () => {
  it('menormalkan penulisan nomor Indonesia ke format wa.me', () => {
    expect(nomorWA('0812-3456-7890')).toBe('6281234567890')
    expect(nomorWA('+62 812 3456 7890')).toBe('6281234567890')
    expect(nomorWA('812 3456 7890')).toBe('6281234567890')
    expect(nomorWA('6281234567890')).toBe('6281234567890')
  })

  it('kosong = pilih kontak di WhatsApp; nomor tak masuk akal = null', () => {
    expect(nomorWA('')).toBe('')
    expect(nomorWA('  ')).toBe('')
    expect(nomorWA('0812')).toBeNull()
    expect(nomorWA('1234567890123456')).toBeNull()
  })
})

describe('teksStrukWA', () => {
  const t = {
    id: 'S1',
    receipt_no: 'A-0001',
    occurred_at: '2026-09-28T09:05:00Z',
    subtotal: 38000,
    discount_amount: 2000,
    tax_amount: 0,
    service_amount: 0,
    total: 36000,
    change_amount: 14000,
    items: [
      { id: 'i1', product_id: 'P1', product_name: 'Kopi (Besar)', unit_name: 'pcs', qty: '2', unit_price: 15000,
        unit_cost: 0, discount_amount: 0, tax_amount: 0, line_total: 30000, note: 'tanpa gula' },
      { id: 'i2', product_id: 'P2', product_name: 'Roti', unit_name: 'pcs', qty: '1', unit_price: 8000,
        unit_cost: 0, discount_amount: 0, tax_amount: 0, line_total: 8000 },
    ],
    payments: [{ id: 'p1', method: 'cash', amount: 50000, paid_at: '' }],
  } as unknown as Transaksi

  it('memuat toko, nota, rincian, total, pembayaran, kembalian, dan tautan', () => {
    // formatRupiah memakai spasi tak-putus ("Rp 15.000" tidak terpisah di
    // ujung baris WhatsApp); dinormalkan supaya harapan mudah dibaca.
    const teks = teksStrukWA(t, 'Warung Sari', 'https://kasir.contoh.id/struk/abc').replace(/ /g, ' ')
    expect(teks).toContain('*Warung Sari*')
    expect(teks).toContain('Struk A-0001')
    expect(teks).toContain('Kopi (Besar)\n  2 × Rp 15.000 = Rp 30.000\n  (tanpa gula)')
    expect(teks).toContain('Diskon −Rp 2.000')
    expect(teks).toContain('*Total Rp 36.000*')
    expect(teks).toContain('Tunai Rp 50.000')
    expect(teks).toContain('Kembalian Rp 14.000')
    expect(teks).toContain('Lihat struk: https://kasir.contoh.id/struk/abc')
  })

  it('tanpa tautan (offline) tidak menyebut "Lihat struk"', () => {
    expect(teksStrukWA(t, 'Warung Sari')).not.toContain('Lihat struk')
  })

  it('tautan wa.me mengodekan isi pesan', () => {
    expect(tautanWA('6281234567890', 'Total *Rp 1.000*\nTerima kasih!')).toBe(
      'https://wa.me/6281234567890?text=Total%20*Rp%201.000*%0ATerima%20kasih!',
    )
    expect(alamatStruk('abc', 'https://kasir.contoh.id')).toBe('https://kasir.contoh.id/struk/abc')
  })
})
