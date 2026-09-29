import { formatQty } from '@/bersama/util/desimal'

/** Satu baris pesanan ke pemasok, dalam satuan beli (dus bila biasa per dus). */
export interface BarisPesanan {
  nama: string
  jumlah: number
  satuan: string
  /** Isi satuan beli (satuan dasar); > 1 = kemasan, disebut isinya. */
  isi: number
  satuanDasar: string
}

/**
 * Jumlah beli dalam satuan beli dari saran dalam satuan dasar: dibulatkan KE
 * ATAS ke kemasan utuh (saran 18 pcs, dus isi 12 → 2 dus) — pemasok tidak
 * menjual setengah dus. Paling sedikit 1.
 */
export function jumlahSatuanBeli(saranDasar: number, isi: number): number {
  const per = isi > 0 ? isi : 1
  return Math.max(1, Math.ceil(saranDasar / per))
}

/**
 * Teks pesanan WhatsApp ke pemasok — bahasa sehari-hari, bernomor, dengan
 * isi kemasan disebut supaya tidak salah kirim ("2 dus (isi 12 pcs)").
 */
export function teksPesanan(namaPemasok: string, namaToko: string, baris: BarisPesanan[]): string {
  const daftar = baris
    .map((b, i) => {
      const isi = b.isi > 1 ? ` (isi ${formatQty(String(b.isi))} ${b.satuanDasar})` : ''
      return `${i + 1}. ${b.nama} — ${formatQty(String(b.jumlah))} ${b.satuan}${isi}`
    })
    .join('\n')
  return `Halo ${namaPemasok}, saya dari ${namaToko}. Mau pesan:\n${daftar}\n\nTerima kasih 🙏`
}
