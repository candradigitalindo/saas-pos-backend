/**
 * Pembacaan katalog yang dipakai lintas modul — pemilih barang di kasir,
 * barang masuk, koreksi stok, dan opname semuanya butuh daftar yang sama.
 *
 * Ditaruh di `bersama/` supaya modul di `fitur/` tidak perlu saling mengimpor.
 * Hanya BACA: perubahan barang tetap milik modul produk.
 */
import { useQuery } from '@tanstack/react-query'
import { api, type Halaman } from '@/lib/api-client'
import type { Kategori, Pemasok, Produk, Satuan } from '@/bersama/tipe/katalog'

export function useDaftarProduk(cari?: string, kategoriId?: string, aktifSaja = true) {
  return useQuery({
    queryKey: ['katalog-produk', cari ?? '', kategoriId ?? '', aktifSaja],
    queryFn: () =>
      api.get<Halaman<Produk>>('/products', {
        query: {
          q: cari || undefined,
          category_id: kategoriId || undefined,
          is_active: aktifSaja ? 'true' : undefined,
          limit: 100,
        },
      }),
    staleTime: 60_000,
  })
}

export function useKategori() {
  return useQuery({
    queryKey: ['katalog-kategori'],
    queryFn: () => api.get<Halaman<Kategori>>('/categories', { query: { limit: 100 } }),
    staleTime: 5 * 60_000,
  })
}

export function useSatuan() {
  return useQuery({
    queryKey: ['katalog-satuan'],
    queryFn: () => api.get<Halaman<Satuan>>('/units', { query: { limit: 100 } }),
    staleTime: 5 * 60_000,
  })
}

export function usePemasok() {
  return useQuery({
    queryKey: ['katalog-pemasok'],
    queryFn: () => api.get<Halaman<Pemasok>>('/suppliers', { query: { limit: 100 } }),
    staleTime: 5 * 60_000,
  })
}
