import type { LucideIcon } from 'lucide-react'
import {
  ArrowRightLeft,
  BarChart3,
  Boxes,
  Building2,
  ClipboardList,
  CreditCard,
  Home,
  MapPin,
  MoreHorizontal,
  NotebookPen,
  Receipt,
  ReceiptText,
  Settings,
  ShoppingBag,
  Store,
  Tags,
  Truck,
  UserCog,
  Users,
  Wallet,
} from 'lucide-react'
import { IZIN, type KodeIzin } from '@/lib/izin'
import { FITUR, type KodeFitur } from '@/lib/fitur'

/**
 * Menu DIBANGUN DARI IZIN, bukan dari daftar tetap.
 *
 * Ini cara utama menjaga aplikasi terasa sederhana bagi pengguna gaptek: kasir
 * tidak pernah melihat menu Gaji, dan tidak perlu bertanya-tanya kenapa ada
 * tombol yang tidak bisa ditekan.
 *
 * Ikon SELALU ditemani teks — ikon sendirian tidak universal.
 */
export interface ItemMenu {
  ke: string
  label: string
  ikon: LucideIcon
  /** Cukup salah satu izin dimiliki (sama seperti Require di backend). */
  izin?: KodeIzin[]
  /** Butuh internet — ditandai, bukan disembunyikan, saat offline. */
  butuhInternet?: boolean
  /**
   * Fitur paket yang membuka menu ini. Menu TETAP tampil bila fiturnya
   * terkunci — diberi ikon gembok, dan halamannya menjelaskan paket mana yang
   * membukanya. Disembunyikan, pemilik tidak pernah tahu fitur itu ada.
   */
  fitur?: KodeFitur
}

/** Lima menu bawah di HP: yang paling sering dipakai, tidak lebih. */
export const MENU_UTAMA: ItemMenu[] = [
  { ke: '/', label: 'Beranda', ikon: Home },
  { ke: '/kasir', label: 'Kasir', ikon: Receipt, izin: [IZIN.saleCreate] },
  { ke: '/stok', label: 'Stok', ikon: Boxes, izin: [IZIN.stockView, IZIN.productView] },
  {
    ke: '/laporan',
    label: 'Laporan',
    ikon: BarChart3,
    izin: [IZIN.reportView],
    butuhInternet: true,
  },
]

/** Sisanya, muncul di "Lainnya" (HP) atau langsung di navigasi samping. */
export const MENU_LAINNYA: ItemMenu[] = [
  { ke: '/kasir/riwayat', label: 'Riwayat Penjualan', ikon: ReceiptText, izin: [IZIN.saleCreate] },
  { ke: '/kasir/ganti-shift', label: 'Ganti Shift', ikon: ArrowRightLeft, izin: [IZIN.shiftClose, IZIN.shiftOpen] },
  { ke: '/kasir/kas', label: 'Uang Masuk & Keluar', ikon: Wallet, izin: [IZIN.cashMovement] },
  { ke: '/barang', label: 'Barang', ikon: ShoppingBag, izin: [IZIN.productView] },
  { ke: '/barang/master', label: 'Kategori & Satuan', ikon: Tags, izin: [IZIN.productEdit] },
  { ke: '/pelanggan', label: 'Pelanggan', ikon: Users, izin: [IZIN.customerView] },
  { ke: '/kasbon', label: 'Kasbon', ikon: NotebookPen, izin: [IZIN.receivableManage] },
  { ke: '/stok/opname', label: 'Hitung Fisik', ikon: ClipboardList, izin: [IZIN.stockOpname] },
  { ke: '/stok/transfer', label: 'Kirim Antar Toko', ikon: Truck, izin: [IZIN.stockTransfer] },
  { ke: '/kanal', label: 'Kanal Online', ikon: Store, izin: [IZIN.channelManage, IZIN.channelOrderAccept], fitur: FITUR.kanalOnline },
  { ke: '/crm', label: 'Prospek', ikon: Building2, izin: [IZIN.crmLeadViewOwn, IZIN.crmLeadViewAll], fitur: FITUR.crmFreelance },
  { ke: '/crm/kunjungan', label: 'Kunjungan', ikon: MapPin, izin: [IZIN.crmVisitCheckin] },
  { ke: '/sdm', label: 'Karyawan', ikon: UserCog, izin: [IZIN.hrEmployeeView, IZIN.hrEmployeeEdit] },
  { ke: '/sdm/gaji', label: 'Gaji', ikon: Wallet, izin: [IZIN.hrPayrollRun, IZIN.hrSalaryView] },
  { ke: '/langganan', label: 'Langganan', ikon: CreditCard, izin: [IZIN.billingManage] },
  { ke: '/pengaturan', label: 'Pengaturan', ikon: Settings, izin: [IZIN.outletManage, IZIN.userManage, IZIN.roleManage] },
]

export const IKON_LAINNYA = MoreHorizontal

/**
 * Pengelompokan untuk navigasi samping (ui/04-PETA-LAYAR.md).
 *
 * Daftar rata 19 baris membuat orang memindai satu per satu setiap kali.
 * Dokumen menyebut kelompoknya secara eksplisit: Penjualan · Barang & Stok ·
 * Pelanggan · CRM · Kanal · SDM · Pengaturan. Kelompok yang seluruh isinya
 * tidak diizinkan tidak muncul sama sekali — judul kosong lebih membingungkan
 * daripada tidak ada judul.
 */
export interface KelompokMenu {
  judul: string
  item: ItemMenu[]
}

export const KELOMPOK_SAMPING: KelompokMenu[] = [
  {
    judul: 'Penjualan',
    item: [
      { ke: '/kasir', label: 'Kasir', ikon: Receipt, izin: [IZIN.saleCreate] },
      { ke: '/kasir/riwayat', label: 'Riwayat Penjualan', ikon: ReceiptText, izin: [IZIN.saleCreate] },
      // Serah terima butuh KEDUA izin — menutup shift lama dan membuka yang baru.
      { ke: '/kasir/ganti-shift', label: 'Ganti Shift', ikon: ArrowRightLeft, izin: [IZIN.shiftClose, IZIN.shiftOpen] },
      { ke: '/kasir/kas', label: 'Uang Masuk & Keluar', ikon: Wallet, izin: [IZIN.cashMovement] },
      { ke: '/kanal', label: 'Kanal Online', ikon: Store, izin: [IZIN.channelManage, IZIN.channelOrderAccept], fitur: FITUR.kanalOnline },
    ],
  },
  {
    judul: 'Barang & Stok',
    item: [
      { ke: '/stok', label: 'Stok', ikon: Boxes, izin: [IZIN.stockView] },
      { ke: '/barang', label: 'Barang', ikon: ShoppingBag, izin: [IZIN.productView] },
      { ke: '/barang/master', label: 'Kategori & Satuan', ikon: Tags, izin: [IZIN.productEdit] },
      { ke: '/stok/opname', label: 'Hitung Fisik', ikon: ClipboardList, izin: [IZIN.stockOpname] },
      { ke: '/stok/transfer', label: 'Kirim Antar Toko', ikon: Truck, izin: [IZIN.stockTransfer] },
    ],
  },
  {
    judul: 'Pelanggan',
    item: [
      { ke: '/pelanggan', label: 'Pelanggan', ikon: Users, izin: [IZIN.customerView] },
      { ke: '/kasbon', label: 'Kasbon', ikon: NotebookPen, izin: [IZIN.receivableManage] },
    ],
  },
  {
    judul: 'Laporan',
    item: [
      { ke: '/laporan', label: 'Laporan', ikon: BarChart3, izin: [IZIN.reportView], butuhInternet: true },
    ],
  },
  {
    judul: 'CRM',
    item: [
      { ke: '/crm', label: 'Prospek', ikon: Building2, izin: [IZIN.crmLeadViewOwn, IZIN.crmLeadViewAll], fitur: FITUR.crmFreelance },
      { ke: '/crm/kunjungan', label: 'Kunjungan', ikon: MapPin, izin: [IZIN.crmVisitCheckin] },
    ],
  },
  {
    judul: 'SDM',
    item: [
      { ke: '/sdm', label: 'Karyawan', ikon: UserCog, izin: [IZIN.hrEmployeeView, IZIN.hrEmployeeEdit] },
      { ke: '/sdm/gaji', label: 'Gaji', ikon: Wallet, izin: [IZIN.hrPayrollRun, IZIN.hrSalaryView] },
    ],
  },
  {
    judul: 'Pengaturan',
    item: [
      { ke: '/langganan', label: 'Langganan', ikon: CreditCard, izin: [IZIN.billingManage] },
      { ke: '/pengaturan', label: 'Pengaturan', ikon: Settings, izin: [IZIN.outletManage, IZIN.userManage, IZIN.roleManage] },
    ],
  },
]

/** Menyaring kelompok; kelompok yang kosong setelah disaring dibuang. */
export function saringKelompok(
  boleh: (...kode: KodeIzin[]) => boolean,
): KelompokMenu[] {
  return KELOMPOK_SAMPING.map((k) => ({ ...k, item: saringMenu(k.item, boleh) })).filter(
    (k) => k.item.length > 0,
  )
}

/** Menyaring menu menurut izin yang benar-benar dimiliki pengguna. */
export function saringMenu(
  menu: ItemMenu[],
  boleh: (...kode: KodeIzin[]) => boolean,
): ItemMenu[] {
  return menu.filter((m) => !m.izin || boleh(...m.izin))
}

/**
 * Tujuan menu yang sedang aktif untuk `pathname`: yang jalurnya PALING PANJANG
 * dan menjadi awalan pathname (per ruas, bukan per huruf).
 *
 * Kenapa bukan `isActive` bawaan NavLink: itu mencocokkan AWALAN, sehingga di
 * /kasir/riwayat menu "Kasir" DAN "Riwayat Penjualan" menyala bersamaan — dua
 * penanda "Anda di sini" yang saling membantah. Memberi `end` pada semua menu
 * juga salah: halaman turunan yang tidak punya menu sendiri (/stok/masuk,
 * /pengaturan/peran) jadi tidak menyalakan apa pun. Awalan terpanjang memenuhi
 * keduanya: tepat satu menu menyala, yaitu yang paling spesifik.
 */
export function menuAktif(pathname: string, tujuan: string[]): string | undefined {
  let terbaik: string | undefined
  for (const ke of tujuan) {
    const cocok =
      ke === '/' ? pathname === '/' : pathname === ke || pathname.startsWith(`${ke}/`)
    if (cocok && (!terbaik || ke.length > terbaik.length)) terbaik = ke
  }
  return terbaik
}
