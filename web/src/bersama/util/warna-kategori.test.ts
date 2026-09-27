import { describe, expect, it } from 'vitest'
import { KELAS_PETAK_NETRAL, kelasPetak, petaWarnaKategori } from './warna-kategori'

describe('petaWarnaKategori', () => {
  it('enam kategori pertama mendapat enam warna berbeda', () => {
    const peta = petaWarnaKategori(
      ['A', 'B', 'C', 'D', 'E', 'F'].map((id) => ({ id, name: `Kat ${id}` })),
    )
    expect(new Set(peta.values()).size).toBe(6)
  })

  it('urutan mengikuti sort_order lalu nama — bukan urutan larik masukan', () => {
    const dariApi = petaWarnaKategori([
      { id: 'min', name: 'Minuman', sort_order: 0 },
      { id: 'mak', name: 'Makanan', sort_order: 0 },
    ])
    // Katalog lokal kasir memakai `nama` dan urutan berbeda — hasilnya sama.
    const dariKasir = petaWarnaKategori([
      { id: 'mak', nama: 'Makanan', sort_order: 0 },
      { id: 'min', nama: 'Minuman', sort_order: 0 },
    ])
    expect(dariApi.get('mak')).toBe(dariKasir.get('mak'))
    expect(dariApi.get('mak')).toContain('petak-1')
    expect(dariApi.get('min')).toContain('petak-2')
  })

  it('barang tanpa kategori atau kategori tak dikenal memakai petak netral', () => {
    const peta = petaWarnaKategori([{ id: 'A', name: 'A' }])
    expect(kelasPetak(peta, undefined)).toBe(KELAS_PETAK_NETRAL)
    expect(kelasPetak(peta, 'hilang')).toBe(KELAS_PETAK_NETRAL)
  })
})
