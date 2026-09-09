/**
 * Mencari pesan galat untuk sebuah kolom.
 *
 * Backend memakai nama kolom DAUN dari tag json (helpers/validator.go +
 * RegisterTagNameFunc), jadi galat untuk `owner.name` datang sebagai `name`.
 * Fungsi ini mencoba beberapa nama supaya pesan tetap mendarat di bawah kolom
 * yang benar — kalau meleset, pengguna melihat form yang "gagal tanpa sebab".
 */
export function galatKolom(
  kolom: Record<string, string>,
  ...kunci: string[]
): string | undefined {
  for (const k of kunci) {
    const langsung = kolom[k]
    if (langsung) return langsung
    // "owner.name" → coba juga "name"
    const daun = k.includes('.') ? k.slice(k.lastIndexOf('.') + 1) : undefined
    if (daun && kolom[daun]) return kolom[daun]
  }
  return undefined
}
