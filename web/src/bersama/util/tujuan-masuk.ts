/**
 * Halaman tujuan setelah masuk: yang tadi hendak dibuka (`state.dari`, dikirim
 * penjaga rute atau tombol "Masuk sebagai kasir lain"), atau Beranda.
 *
 * Hanya jalur di dalam aplikasi yang diterima — `//situs-lain` dan alamat
 * lengkap ditolak supaya tautan masuk tidak bisa dipakai melempar orang keluar.
 */
export function tujuanSetelahMasuk(state: unknown): string {
  const dari = (state as { dari?: unknown } | null)?.dari
  if (typeof dari !== 'string' || !dari.startsWith('/') || dari.startsWith('//')) return '/'
  if (dari === '/masuk' || dari.startsWith('/masuk?')) return '/'
  return dari
}
