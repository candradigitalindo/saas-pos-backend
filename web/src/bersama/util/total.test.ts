import { describe, expect, it } from 'vitest'
import kasus from './kasus-total.json'
import { hitungTotal } from './total'

/**
 * Sisi web dari kontrak total kasir. Sisi server-nya:
 * services/total_kasir_test.go, dengan berkas kasus yang SAMA.
 */
describe('hitungTotal — sama persis dengan server', () => {
  for (const k of kasus) {
    it(k.nama, () => {
      expect(hitungTotal(k.baris, k.aturan, k.diskon_transaksi)).toEqual(k.harapan)
    })
  }

  it('tanpa aturan cabang (belum termuat) = tanpa pajak dan layanan', () => {
    expect(hitungTotal([{ harga: 1000, qty: '2', diskon: 0 }], undefined).total).toBe(2000)
  })
})
