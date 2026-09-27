/**
 * Penanda huruf untuk barang yang belum punya foto: TIGA HURUF PERTAMA setiap
 * kata pada namanya.
 *
 * Kartu barang tanpa gambar sama sekali membuat grid kasir jadi dinding teks
 * yang seragam, dan mata kasir kehilangan pegangan untuk melompat. Sepotong
 * huruf yang tetap sama untuk barang yang sama memberi pegangan itu tanpa
 * menunggu pemiliknya sempat memotret seratus barang.
 *
 * Kata yang tidak memberi pembeda apa pun dibuang lebih dulu — "Kopi Susu Gula
 * Aren" lebih berguna sebagai "KopSusGulAre" daripada ikut memuat "dan" atau
 * "1kg" yang dimiliki banyak barang lain.
 *
 *   Gula Pasir 1kg       → GulPas
 *   Indomie Goreng       → IndGor
 *   Kopi Susu Gula Aren  → KopSusGul
 *   Teh                  → Teh
 */

/** Kata yang tidak membedakan satu barang dari yang lain. */
const KATA_ABAI = new Set(['dan', 'atau', 'the', 'of'])

/** Potongan huruf maksimum, supaya tetap terbaca di kartu sempit. */
const MAKS_POTONGAN = 3

/** Kata-kata nama barang yang membedakan: tanpa kata sambung & tanpa satuan. */
function kataPembeda(nama: string): string[] {
  return (nama ?? '')
    .split(/[\s/\-_,.]+/)
    .map((k) => k.trim())
    .filter(Boolean)
    // Angka & satuan ("1kg", "600ml") dibuang bila masih ada kata lain: ia
    // dimiliki banyak barang sekaligus, jadi justru mengaburkan pembedanya.
    .filter((k, _, semua) => {
      if (KATA_ABAI.has(k.toLowerCase())) return false
      const angka = /^\d/.test(k)
      return !angka || semua.every((x) => /^\d/.test(x))
    })
}

export function inisialProduk(nama: string, hurufPerKata = 3): string {
  const kata = kataPembeda(nama)
  if (kata.length === 0) return '?'

  return kata
    .slice(0, MAKS_POTONGAN)
    .map((k) => {
      const potong = k.slice(0, hurufPerKata)
      return potong.charAt(0).toLocaleUpperCase('id-ID') + potong.slice(1)
    })
    .join('')
}

/**
 * Inisial ORANG untuk avatar: huruf pertama dua kata pertama.
 *
 *   Sari Dewi      → SD
 *   budi           → B
 *   Ahmad bin Umar → AB
 */
export function inisialNama(nama: string): string {
  const kata = (nama ?? '').trim().split(/\s+/).filter(Boolean)
  if (kata.length === 0) return '?'
  return kata
    .slice(0, 2)
    .map((k) => k.charAt(0).toLocaleUpperCase('id-ID'))
    .join('')
}

/**
 * Inisial PETAK barang tanpa foto: dua huruf, seperti petak aplikasi kasir
 * pada umumnya.
 *
 *   Air Mineral 600ml   → AM
 *   Kopi Susu Gula Aren → KS
 *   Teh                 → Te
 *
 * Kenapa bukan potongan tiga huruf ("AirMin") lagi: di petak kasir itu
 * terbaca sebagai kata yang salah eja, bukan sebagai penanda. Pembeda antar
 * barang kini dipikul WARNA kategori (warna-kategori.ts) dan nama lengkap yang
 * selalu tertulis tepat di bawah petaknya; inisial cukup menjadi pegangan
 * mata. Kata satuan ("600ml", "1kg") tetap dibuang — dimiliki banyak barang.
 */
export function inisialPetak(nama: string): string {
  const kata = kataPembeda(nama)
  if (kata.length === 0) return '?'
  const besar = (h: string) => h.toLocaleUpperCase('id-ID')
  if (kata.length === 1) {
    const k = kata[0] ?? ''
    return besar(k.charAt(0)) + k.charAt(1).toLocaleLowerCase('id-ID')
  }
  return kata
    .slice(0, 2)
    .map((k) => besar(k.charAt(0)))
    .join('')
}
