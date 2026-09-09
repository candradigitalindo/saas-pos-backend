/**
 * Waktu di server dan database seluruhnya UTC, dan `business_date` DIHITUNG
 * SERVER berdasarkan zona waktu outlet — jam HP kasir sering salah, jadi
 * frontend tidak pernah menghitungnya sendiri (ui/03-ARSITEKTUR-FRONTEND.md).
 *
 * Yang boleh dilakukan di sini hanya MENAMPILKAN.
 */

const ZONA_BAWAAN = 'Asia/Jakarta'

/** "9 Sep 2026" */
export function formatTanggal(iso: string, zona = ZONA_BAWAAN): string {
  if (!iso) return '—'
  return new Intl.DateTimeFormat('id-ID', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    timeZone: zona,
  }).format(new Date(iso))
}

/** "14:30" */
export function formatJam(iso: string, zona = ZONA_BAWAAN): string {
  if (!iso) return '—'
  return new Intl.DateTimeFormat('id-ID', {
    hour: '2-digit',
    minute: '2-digit',
    timeZone: zona,
  }).format(new Date(iso))
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
  const target = kunci(new Date(iso))
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
