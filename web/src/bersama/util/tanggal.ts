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

/**
 * Tanggal "YYYY-MM-DD" (hari usaha dari server) → Date tengah malam UTC hari
 * itu, digeser `hari` hari. Aritmetikanya di UTC: dengan Date lokal, perangkat
 * yang zonanya punya jam musim panas bisa melompati atau menggandakan satu
 * tanggal. Hanya untuk berhitung & memformat dengan timeZone 'UTC'.
 */
export function tanggalKeUTC(iso: string, hari = 0): Date {
  const [th = 1970, bl = 1, tg = 1] = iso.split('-').map(Number)
  return new Date(Date.UTC(th, bl - 1, tg + hari))
}

/** "2026-09-27" + n hari → "2026-09-28" (n boleh negatif). */
export function geserTanggal(iso: string, hari: number): string {
  return tanggalKeUTC(iso, hari).toISOString().slice(0, 10)
}

/** "Jumat, 18 September 2026" untuk tanggal "YYYY-MM-DD" (tanpa geser zona). */
export function formatTanggalPanjang(iso: string): string {
  return new Intl.DateTimeFormat('id-ID', {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
    year: 'numeric',
    timeZone: 'UTC',
  }).format(tanggalKeUTC(iso))
}

/** "Minggu, 27 September 2026" — tanggal lengkap untuk kepala halaman. */
export function formatHariPanjang(d = new Date(), zona = ZONA_BAWAAN): string {
  return new Intl.DateTimeFormat('id-ID', {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
    year: 'numeric',
    timeZone: zona,
  }).format(d)
}

/**
 * "Hari ini" / "Kemarin" / "9 hari lalu" / "18 Agu 2026" — untuk "kapan
 * terakhir". Hari dihitung menurut TANGGAL di zona toko, bukan selisih 24
 * jam: kunjungan pukul 23.00 kemarin tetap "Kemarin" walau baru 2 jam lalu.
 * Lewat 30 hari, tanggal lengkap lebih berguna daripada "47 hari lalu".
 */
export function formatLaluHari(iso: string, kini = new Date(), zona = ZONA_BAWAAN): string {
  if (!iso) return '—'
  const hari = (d: Date) => tanggalKeUTC(tanggalISO(d, zona)).getTime() / 86_400_000
  const selisih = Math.round(hari(kini) - hari(keDate(iso)))
  if (selisih <= 0) return 'Hari ini'
  if (selisih === 1) return 'Kemarin'
  if (selisih < 30) return `${selisih} hari lalu`
  return formatTanggal(iso, zona)
}

/** YYYY-MM-DD untuk parameter query — dari objek Date lokal pengguna. */
export function tanggalISO(d = new Date(), zona = ZONA_BAWAAN): string {
  return new Intl.DateTimeFormat('en-CA', { timeZone: zona }).format(d)
}

/** Zona waktu outlet yang diterima server. */
export type ZonaIndonesia = 'Asia/Jakarta' | 'Asia/Makassar' | 'Asia/Jayapura'

const ZONA_PERANGKAT: Record<string, ZonaIndonesia> = {
  'Asia/Jakarta': 'Asia/Jakarta',
  'Asia/Pontianak': 'Asia/Jakarta', // WIB juga, tapi server hanya kenal satu nama per zona
  'Asia/Makassar': 'Asia/Makassar',
  'Asia/Ujung_Pandang': 'Asia/Makassar', // nama lama Makassar di basis data IANA
  'Asia/Jayapura': 'Asia/Jayapura',
}

/**
 * Zona waktu perangkat, bila termasuk WIB/WITA/WIT; selain itu undefined
 * (server lalu memakai WIB).
 *
 * Dipakai saat mendaftar. Dulu zona tidak dikirim sama sekali, jadi usaha di
 * Makassar atau Jayapura terdaftar sebagai WIB dan penjualan pukul 00.30
 * waktu setempat masuk ke hari kemarin. Jam HP tidak dipercaya untuk
 * menghitung hari penjualan, tapi ZONA-nya adalah tebakan awal terbaik —
 * dan tetap bisa diubah di Pengaturan → Toko.
 */
export function zonaIndonesiaPerangkat(
  zona: string = Intl.DateTimeFormat().resolvedOptions().timeZone,
): ZonaIndonesia | undefined {
  return ZONA_PERANGKAT[zona]
}
