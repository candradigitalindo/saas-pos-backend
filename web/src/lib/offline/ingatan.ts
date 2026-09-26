/**
 * Ingatan terakhir untuk data yang MENENTUKAN apakah kasir bisa dibuka.
 *
 * Kasir tidak boleh pernah diblokir status koneksi (ui/01). Dua data berasal
 * dari server dan dulu hanya hidup di memori React Query:
 *
 *   - profil (GET /me): tanpanya penjaga rute menganggap pengguna belum masuk
 *     dan melempar ke layar masuk — yang juga butuh internet;
 *   - shift aktif: tanpanya layar kasir menyodorkan "Buka shift".
 *
 * Maka kasir yang memuat ulang aplikasi tanpa sinyal (HP mati, PWA dibuka ulang
 * pagi hari di warung tanpa jaringan) tidak bisa berjualan sama sekali, padahal
 * katalog dan antreannya sudah ada di perangkat.
 *
 * Nilai terakhir yang berhasil diambil disimpan di sini dan dipakai sebagai
 * data awal; begitu server terjangkau, data segar menggantikannya. Sengaja
 * localStorage (bukan Dexie): data awal React Query harus tersedia SEKETIKA,
 * bukan lewat janji.
 *
 * Yang disimpan bukan rahasia — izin di profil hanya untuk menyusun menu,
 * server tetap memeriksa izin di setiap permintaan.
 */

const AWALAN = 'pos.ingat.'

export function ingat<T>(kunci: string, nilai: T): void {
  try {
    localStorage.setItem(AWALAN + kunci, JSON.stringify(nilai))
  } catch {
    // Penyimpanan penuh / diblokir: kehilangan ingatan bukan kegagalan fatal.
  }
}

/** undefined = belum pernah diingat (berbeda dari null yang memang diingat). */
export function ingatan<T>(kunci: string): T | undefined {
  try {
    const mentah = localStorage.getItem(AWALAN + kunci)
    return mentah === null ? undefined : (JSON.parse(mentah) as T)
  } catch {
    return undefined
  }
}

/**
 * Membuang seluruh ingatan — WAJIB saat keluar dan sebelum sesi baru dipakai,
 * supaya profil pengguna sebelumnya tidak pernah tampil untuk pengguna berikutnya
 * di tablet yang dipakai bergantian.
 */
export function lupakanSemua(): void {
  try {
    const kunci: string[] = []
    for (let i = 0; i < localStorage.length; i++) {
      const k = localStorage.key(i)
      if (k?.startsWith(AWALAN)) kunci.push(k)
    }
    for (const k of kunci) localStorage.removeItem(k)
  } catch {
    // Abaikan — lihat ingat().
  }
}
