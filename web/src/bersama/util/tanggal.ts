/**
 * Waktu di server dan database seluruhnya UTC, dan `business_date` DIHITUNG
 * SERVER berdasarkan zona waktu outlet — jam HP kasir sering salah, jadi
 * frontend tidak pernah menghitungnya sendiri (ui/03-ARSITEKTUR-FRONTEND.md).
 *
 * Yang boleh dilakukan di sini hanya MENAMPILKAN.
 */

const ZONA_BAWAAN = 'Asia/Jakarta'

/** "2026-09-26 00:22:10" — bentuk lama dari server, TANPA penanda zona. */
const TANPA_ZONA = /^(\d{4}-\d{2}-\d{2})[ T](\d{2}:\d{2}(?::\d{2}(?:\.\d+)?)?)$/

/**
 * Mengubah string waktu dari server menjadi Date.
 *
 * Server mengirim RFC 3339 UTC ("…T00:22:10Z"). Versi lama mengirim
 * "2026-09-26 00:22:10" tanpa zona, dan `new Date()` menafsirkan bentuk itu
 * sebagai jam LOKAL perangkat — jam UTC tampil mundur 7 jam di layar WIB.
 * Bentuk tanpa zona karena itu dibaca sebagai UTC, sesuai kontraknya.
 */
export function keDate(iso: string): Date {
  const m = TANPA_ZONA.exec(iso)
  return new Date(m ? `${m[1]}T${m[2]}Z` : iso)
}

/** "9 Sep 2026" */
export function formatTanggal(iso: string, zona = ZONA_BAWAAN): string {
  if (!iso) return '—'
  return new Intl.DateTimeFormat('id-ID', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    timeZone: zona,
  }).format(keDate(iso))
}

/** "14:30" */
export function formatJam(iso: string, zona = ZONA_BAWAAN): string {
  if (!iso) return '—'
  return new Intl.DateTimeFormat('id-ID', {
    hour: '2-digit',
    minute: '2-digit',
    timeZone: zona,
  }).format(keDate(iso))
}

/** "9 Sep 2026 · 14:30" */
export function formatTanggalJam(iso: string, zona = ZONA_BAWAAN): string {
  if (!iso) return '—'
  return `${formatTanggal(iso, zona)} · ${formatJam(iso, zona)}`
}

/** "hari ini" / "kemarin" / "9 Sep" untuk daftar riwayat. */
export function formatTanggalAkrab(iso: string, zona = ZONA_BAWAAN): string {
  if (!iso) return '—'
  const kunci = (d: Date) =>
    new Intl.DateTimeFormat('en-CA', { timeZone: zona }).format(d)
  const target = kunci(keDate(iso))
  const kini = new Date()
  if (target === kunci(kini)) return 'Hari ini'
  const kemarin = new Date(kini.getTime() - 86_400_000)
  if (target === kunci(kemarin)) return 'Kemarin'
  return formatTanggal(iso, zona)
}

/** YYYY-MM-DD untuk parameter query — dari objek Date lokal pengguna. */
export function tanggalISO(d = new Date(), zona = ZONA_BAWAAN): string {
  return new Intl.DateTimeFormat('en-CA', { timeZone: zona }).format(d)
}
