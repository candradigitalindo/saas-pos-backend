import { api, type Halaman } from '@/lib/api-client'
import type { Kategori, Pemasok, Produk, Satuan, VarianProduk } from '@/bersama/tipe/katalog'

export interface InputProduk {
  name: string
  unit_id: string
  category_id?: string
  sku?: string
  barcode?: string
  sell_price?: number
  cost_price?: number
  track_stock?: boolean
  min_stock?: string
  is_active?: boolean
  /** Untuk menu aplikasi antar; string kosong = dihapus. */
  description?: string
}

/** Isi varian. PUT mengganti utuh, jadi semua kolom selalu dikirim. */
export interface InputVarian {
  name: string
  /** Selisih terhadap harga jual barang (boleh negatif). */
  price_delta: number
  sku?: string
  barcode?: string
  is_active?: boolean
}

/** Satu baris CSV yang ditolak, lengkap dengan nomor barisnya. */
export interface GalatBarisImpor {
  row: number
  field: string
  message: string
}

export interface HasilImpor {
  dry_run: boolean
  total: number
  imported: number
  failed: number
  errors: GalatBarisImpor[]
}

export const produkApi = {
  daftar: (cari?: string, kategoriId?: string, page = 1, limit = 25) =>
    api.get<Halaman<Produk>>('/products', {
      query: { q: cari || undefined, category_id: kategoriId || undefined, page, limit },
    }),

  satu: (id: string) => api.get<Produk>(`/products/${id}`),

  buat: (input: InputProduk) => api.post<Produk>('/products', input),

  ubah: (id: string, input: Partial<InputProduk>) => api.put<Produk>(`/products/${id}`, input),

  unggahFoto: (id: string, berkas: Blob) => {
    const f = new FormData()
    f.append('file', berkas, 'foto.jpg')
    return api.postBerkas<{ image_url: string }>(`/products/${id}/image`, f)
  },
  hapusFoto: (id: string) => api.hapus<{ image_url: string }>(`/products/${id}/image`),

  hapus: (id: string) => api.hapus<null>(`/products/${id}`),

  // ── Varian ───────────────────────────────────────────────────────────────
  varian: (id: string) => api.get<VarianProduk[]>(`/products/${id}/variants`),
  buatVarian: (id: string, input: InputVarian) => api.post<VarianProduk>(`/products/${id}/variants`, input),
  ubahVarian: (id: string, vid: string, input: InputVarian) =>
    api.put<VarianProduk>(`/products/${id}/variants/${vid}`, input),
  hapusVarian: (id: string, vid: string) => api.hapus<null>(`/products/${id}/variants/${vid}`),

  /**
   * Impor CSV. `dryRun` WAJIB dijalankan lebih dulu di UI: pemilik harus
   * melihat berapa baris yang akan masuk dan mana yang bermasalah SEBELUM
   * datanya benar-benar tertulis.
   *
   * Kolom wajib: name, unit. Opsional: category, sku, barcode, sell_price,
   * cost_price, min_stock, track_stock, is_active.
   */
  impor: (csv: string, dryRun: boolean) =>
    api.postMentah<HasilImpor>('/products/import', csv, 'text/csv', {
      query: { dry_run: dryRun },
    }),

  // ── Master data ──────────────────────────────────────────────────────────
  buatKategori: (name: string) => api.post<Kategori>('/categories', { name }),
  hapusKategori: (id: string) => api.hapus<null>(`/categories/${id}`),

  buatSatuan: (name: string) => api.post<Satuan>('/units', { name }),
  hapusSatuan: (id: string) => api.hapus<null>(`/units/${id}`),

  buatPemasok: (input: { name: string; phone?: string; address?: string }) =>
    api.post<Pemasok>('/suppliers', input),
  hapusPemasok: (id: string) => api.hapus<null>(`/suppliers/${id}`),
}
