/**
 * Struk lewat WhatsApp — dikirim dari WhatsApp TOKO di perangkat kasir
 * (keputusan pemilik produk: pengirimnya nomor toko, bukan nomor platform).
 * Aplikasi hanya menyiapkan pesannya; kasir yang menekan kirim di WhatsApp.
 *
 * Semua angka berasal dari transaksi balasan server (atau struk offline) —
 * tidak ada yang dihitung ulang di sini.
 */
import type { Transaksi } from '@/bersama/tipe/pos'
import { formatRupiah } from '@/bersama/util/uang'
import { formatQty } from '@/bersama/util/desimal'
import { formatTanggalJam } from '@/bersama/util/tanggal'
import { namaMetode } from './label-transaksi'

/**
 * Nomor WhatsApp ke format internasional tanpa "+" (yang dipakai wa.me):
 * "0812-3456-7890" / "+62 812…" / "812…" → "62812…". Kosong → "" (WhatsApp
 * lalu meminta kasir memilih kontak). Tidak masuk akal → null.
 */
export function nomorWA(teks: string): string | null {
  let d = teks.replace(/\D/g, '')
  if (d === '') return ''
  if (d.startsWith('0')) d = '62' + d.slice(1)
  else if (d.startsWith('8')) d = '62' + d
  return d.length >= 10 && d.length <= 15 ? d : null
}

/** Alamat halaman struk digital untuk token dari server. */
export function alamatStruk(token: string, asal = window.location.origin): string {
  return `${asal}/struk/${token}`
}

/**
 * Isi pesan. Tebal (*…*) hanya untuk nama toko & total — yang dicari mata
 * pembeli. Tautan diletakkan di akhir supaya WhatsApp menampilkan
 * pratinjaunya di bawah rincian.
 */
export function teksStrukWA(t: Transaksi, namaToko: string, tautan?: string): string {
  const baris: string[] = [`*${namaToko}*`, `Struk ${t.receipt_no}`, formatTanggalJam(t.occurred_at), '']
  for (const i of t.items ?? []) {
    baris.push(i.product_name)
    const diskon = i.discount_amount > 0 ? ` − ${formatRupiah(i.discount_amount)}` : ''
    baris.push(`  ${formatQty(i.qty)} × ${formatRupiah(i.unit_price)}${diskon} = ${formatRupiah(i.line_total)}`)
    if (i.note) baris.push(`  (${i.note})`)
  }
  baris.push('')
  if (t.discount_amount > 0 || t.tax_amount > 0 || t.service_amount > 0) {
    baris.push(`Subtotal ${formatRupiah(t.subtotal)}`)
    if (t.discount_amount > 0) baris.push(`Diskon −${formatRupiah(t.discount_amount)}`)
    if (t.tax_amount > 0) baris.push(`Pajak ${formatRupiah(t.tax_amount)}`)
    if (t.service_amount > 0) baris.push(`Biaya layanan ${formatRupiah(t.service_amount)}`)
  }
  baris.push(`*Total ${formatRupiah(t.total)}*`)
  for (const p of t.payments ?? []) baris.push(`${namaMetode(p.method)} ${formatRupiah(p.amount)}`)
  if (t.change_amount > 0) baris.push(`Kembalian ${formatRupiah(t.change_amount)}`)
  if (tautan) baris.push('', `Lihat struk: ${tautan}`)
  baris.push('', 'Terima kasih!')
  return baris.join('\n')
}

/** Tautan wa.me — membuka aplikasi WhatsApp (HP) atau WhatsApp Web (desktop). */
export function tautanWA(nomor: string, teks: string): string {
  return `https://wa.me/${nomor}?text=${encodeURIComponent(teks)}`
}
