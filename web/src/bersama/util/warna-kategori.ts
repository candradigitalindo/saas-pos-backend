/**
 * Warna petak per KATEGORI barang.
 *
 * Petak barang tanpa foto dulu semuanya berwarna sama, sehingga satu-satunya
 * pembeda adalah potongan huruf. Aplikasi kasir pada umumnya mewarnai petak
 * menurut kategori: mata kasir belajar "minuman itu biru" dan melompat ke
 * kelompoknya tanpa membaca.
 *
 * Warnanya ditentukan URUTAN kategori (sort_order, lalu nama) — bukan hash
 * nama. Hash membuat dua dari empat kategori berwarna sama dengan peluang
 * ±70%; urutan menjamin enam kategori pertama berbeda semua. Urutannya sama
 * dengan urutan tab kategori di layar kasir, dan fungsi ini dipakai baik di
 * Kasir maupun di daftar Barang, jadi satu kategori berwarna sama di mana pun.
 *
 * Barang tanpa kategori memakai petak netral, bukan salah satu warna — kalau
 * ikut hijau, ia akan tampak sekelompok dengan kategori pertama.
 */

/** Kelas petak; ditulis utuh supaya Tailwind menemukannya saat memindai. */
const KELAS_PETAK = [
  'bg-petak-1-latar text-petak-1-teks',
  'bg-petak-2-latar text-petak-2-teks',
  'bg-petak-3-latar text-petak-3-teks',
  'bg-petak-4-latar text-petak-4-teks',
  'bg-petak-5-latar text-petak-5-teks',
  'bg-petak-6-latar text-petak-6-teks',
] as const

/** Petak barang tanpa kategori. teks-sekunder di atas permukaan-2 ≥ 6,8:1. */
export const KELAS_PETAK_NETRAL = 'bg-permukaan-2 text-teks-sekunder'

export interface KategoriBerurut {
  id: string
  /** Nama dari API (`name`) atau dari katalog lokal kasir (`nama`). */
  name?: string
  nama?: string
  sort_order?: number
}

/** id kategori → kelas petaknya, menurut urutan kategori. */
export function petaWarnaKategori(kategori: KategoriBerurut[]): Map<string, string> {
  const nama = (k: KategoriBerurut) => k.name ?? k.nama ?? ''
  const urut = [...kategori].sort(
    (a, b) =>
      (a.sort_order ?? 0) - (b.sort_order ?? 0) ||
      nama(a).localeCompare(nama(b), 'id') ||
      a.id.localeCompare(b.id),
  )
  return new Map(urut.map((k, i) => [k.id, KELAS_PETAK[i % KELAS_PETAK.length] ?? KELAS_PETAK_NETRAL]))
}

/** Kelas petak untuk satu barang; tanpa kategori (atau tak dikenal) → netral. */
export function kelasPetak(peta: Map<string, string>, kategoriId?: string | null): string {
  return (kategoriId && peta.get(kategoriId)) || KELAS_PETAK_NETRAL
}

/**
 * Warna avatar orang dari id-nya — tetap sama untuk orang yang sama. Di sini
 * tabrakan warna tidak apa-apa: warnanya hiasan pembeda baris, bukan
 * pengelompokan seperti pada kategori barang.
 */
export function kelasAvatar(id: string): string {
  let h = 0
  for (let i = 0; i < id.length; i++) h = (h * 31 + id.charCodeAt(i)) >>> 0
  return KELAS_PETAK[h % KELAS_PETAK.length] ?? KELAS_PETAK_NETRAL
}
