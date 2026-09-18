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

export function inisialProduk(nama: string, hurufPerKata = 3): string {
  const kata = (nama ?? '')
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

  if (kata.length === 0) return '?'

  return kata
    .slice(0, MAKS_POTONGAN)
    .map((k) => {
      const potong = k.slice(0, hurufPerKata)
      return potong.charAt(0).toLocaleUpperCase('id-ID') + potong.slice(1)
    })
    .join('')
}
