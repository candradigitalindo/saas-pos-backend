import { Banknote, CreditCard, NotebookPen, QrCode, Smartphone, Wallet, type LucideIcon } from 'lucide-react'

/** Nama cara bayar untuk layar (nilai enum server → bahasa sehari-hari). */
export const NAMA_METODE: Record<string, string> = {
  cash: 'Tunai',
  qris: 'QRIS',
  transfer: 'Transfer',
  card: 'Kartu',
  ewallet: 'Dompet digital',
  credit: 'Kasbon',
}

export const IKON_METODE: Record<string, LucideIcon> = {
  cash: Banknote,
  qris: QrCode,
  transfer: Smartphone,
  card: CreditCard,
  ewallet: Wallet,
  credit: NotebookPen,
}

export const NAMA_JENIS_PESANAN: Record<string, string> = {
  dine_in: 'Makan di tempat',
  takeaway: 'Bawa pulang',
  delivery: 'Diantar',
  pickup: 'Ambil sendiri',
}

export const namaMetode = (m: string) => NAMA_METODE[m] ?? m
export const ikonMetode = (m: string): LucideIcon => IKON_METODE[m] ?? Wallet
