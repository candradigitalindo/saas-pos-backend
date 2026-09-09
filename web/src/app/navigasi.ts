import type { LucideIcon } from 'lucide-react'
import {
  BarChart3,
  Boxes,
  Building2,
  ClipboardList,
  Home,
  MoreHorizontal,
  NotebookPen,
  Receipt,
  ReceiptText,
  Settings,
  ShoppingBag,
  Store,
  Tags,
  Truck,
  Users,
  Wallet,
} from 'lucide-react'
import { IZIN, type KodeIzin } from '@/lib/izin'

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
  { ke: '/kasir/kas', label: 'Uang Masuk & Keluar', ikon: Wallet, izin: [IZIN.cashMovement] },
  { ke: '/barang', label: 'Barang', ikon: ShoppingBag, izin: [IZIN.productView] },
  { ke: '/barang/master', label: 'Kategori & Satuan', ikon: Tags, izin: [IZIN.productEdit] },
  { ke: '/pelanggan', label: 'Pelanggan', ikon: Users, izin: [IZIN.customerView] },
  { ke: '/kasbon', label: 'Kasbon', ikon: NotebookPen, izin: [IZIN.receivableManage] },
  { ke: '/stok/opname', label: 'Hitung Fisik', ikon: ClipboardList, izin: [IZIN.stockOpname] },
  { ke: '/stok/transfer', label: 'Kirim Antar Toko', ikon: Truck, izin: [IZIN.stockTransfer] },
  { ke: '/kanal', label: 'Kanal Online', ikon: Store, izin: [IZIN.channelManage, IZIN.channelOrderAccept] },
  { ke: '/crm', label: 'Prospek & Proyek', ikon: Building2, izin: [IZIN.crmLeadViewOwn, IZIN.crmLeadViewAll] },
  { ke: '/sdm', label: 'Karyawan & Gaji', ikon: Users, izin: [IZIN.hrEmployeeView, IZIN.hrEmployeeEdit] },
  { ke: '/pengaturan', label: 'Pengaturan', ikon: Settings, izin: [IZIN.outletManage, IZIN.userManage, IZIN.roleManage] },
]

export const IKON_LAINNYA = MoreHorizontal

/** Menyaring menu menurut izin yang benar-benar dimiliki pengguna. */
export function saringMenu(
  menu: ItemMenu[],
  boleh: (...kode: KodeIzin[]) => boolean,
): ItemMenu[] {
  return menu.filter((m) => !m.izin || boleh(...m.izin))
}
