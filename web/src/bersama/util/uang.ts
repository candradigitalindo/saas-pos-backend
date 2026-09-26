/**
 * Uang selalu bilangan bulat rupiah (int64 di server). Tidak pernah pecahan
 * JavaScript — 0.1 + 0.2 = 0.30000000000000004, dan selisih beberapa rupiah
 * antara layar dan struk menghancurkan kepercayaan pemilik usaha.
 *
 * Aturan yang dipegang berkas ini (ui/03-ARSITEKTUR-FRONTEND.md):
 *   1. Frontend tidak mengarang aturan pembulatannya sendiri. Satu-satunya
 *      hitungan uang yang harus SAMA PERSIS dengan server — total kasir — ada
 *      di bersama/util/total.ts dan dijaga kontrak dua sisi.
 *   2. Perhitungan di sini hanya untuk PRATINJAU; angka final selalu dari server.
 */

/** "Rp 1.250.000" — pemisah ribuan titik, tanpa desimal, spasi setelah "Rp". */
export function formatRupiah(nilai: number): string {
  const bulat = Math.trunc(nilai)
  const negatif = bulat < 0
  const angka = Math.abs(bulat).toLocaleString('id-ID')
  return negatif ? `−Rp ${angka}` : `Rp ${angka}`
}

/** Angka saja tanpa awalan "Rp" — untuk kolom isian uang. */
export function formatAngka(nilai: number): string {
  return Math.abs(Math.trunc(nilai)).toLocaleString('id-ID')
}

/**
 * Membaca kembali angka dari teks yang diketik pengguna ("1.250.000" atau
 * "Rp 1.250.000" atau "1250000"). Titik dan spasi dibuang; nilai tak terbaca
 * menjadi 0 supaya kolom tidak pernah berisi NaN.
 */
export function parseRupiah(teks: string): number {
  const bersih = teks.replace(/[^0-9-]/g, '')
  if (bersih === '' || bersih === '-') return 0
  const n = Number.parseInt(bersih, 10)
  return Number.isFinite(n) ? n : 0
}

/** Selisih dalam persen, dibulatkan ke bilangan bulat. null bila tak bermakna. */
export function persenSelisih(sekarang: number, sebelumnya: number): number | null {
  if (sebelumnya === 0) return null
  return Math.round(((sekarang - sebelumnya) / Math.abs(sebelumnya)) * 100)
}

/**
 * Pratinjau subtotal satu baris keranjang: harga satuan (rupiah bulat) × qty
 * (string desimal). Hasil dipotong ke rupiah bulat mengikuti perilaku server
 * yang membulatkan PER BARIS lalu menjumlahkan.
 *
 * Sekali lagi: ini hanya pratinjau. Total yang dicetak di struk selalu diambil
 * dari balasan server.
 */
export function pratinjauBaris(hargaSatuan: number, qty: string, diskon = 0): number {
  const jumlah = Number.parseFloat(qty)
  if (!Number.isFinite(jumlah)) return 0
  const kotor = Math.round(hargaSatuan * jumlah)
  return Math.max(0, kotor - diskon)
}
