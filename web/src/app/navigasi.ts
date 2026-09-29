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
  HandCoins,
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
  /**
   * Kata lain yang dipakai orang untuk halaman ini — dicocokkan palet perintah
   * (Ctrl/⌘ K). Pemilik warung mengetik "piutang" atau "hutang", bukan
   * "Kasbon"; "produk", bukan "Barang".
   */
  kataKunci?: string[]
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
  { ke: '/stok/utang', label: 'Utang Pemasok', ikon: HandCoins, izin: [IZIN.stockView] },
  { ke: '/kanal', label: 'Kanal Online', ikon: Store, izin: [IZIN.channelManage, IZIN.channelOrderAccept], fitur: FITUR.kanalOnline },
  { ke: '/crm', label: 'Prospek', ikon: Building2, izin: [IZIN.crmLeadViewOwn, IZIN.crmLeadViewAll], fitur: FITUR.crmFreelance },
  { ke: '/crm/kunjungan', label: 'Kunjungan', ikon: MapPin, izin: [IZIN.crmVisitCheckin], fitur: FITUR.salesLapangan },
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

/**
 * Menu teratas navigasi samping, tanpa judul kelompok: halaman yang dibuka
 * setiap hari. "Kasir" tidak di sini — ia tombol utama berwarna penuh di atas
 * daftar ("Buka Kasir"), pola tombol aksi utama di aplikasi SaaS pada umumnya.
 */
export const MENU_ATAS: ItemMenu[] = [
  { ke: '/', label: 'Beranda', ikon: Home, kataKunci: ['dasbor', 'dashboard', 'ringkasan', 'hari ini'] },
  {
    ke: '/laporan', label: 'Laporan', ikon: BarChart3, izin: [IZIN.reportView], butuhInternet: true,
    kataKunci: ['omzet', 'untung', 'laba', 'penjualan', 'terlaris', 'grafik'],
  },
]

export const KELOMPOK_SAMPING: KelompokMenu[] = [
  {
    judul: 'Penjualan',
    item: [
      {
        ke: '/kasir/riwayat', label: 'Riwayat Penjualan', ikon: ReceiptText, izin: [IZIN.saleCreate],
        kataKunci: ['transaksi', 'nota', 'struk', 'batal', 'void', 'retur'],
      },
      // Serah terima butuh KEDUA izin — menutup shift lama dan membuka yang baru.
      {
        ke: '/kasir/ganti-shift', label: 'Ganti Shift', ikon: ArrowRightLeft, izin: [IZIN.shiftClose, IZIN.shiftOpen],
        kataKunci: ['serah terima', 'shift'],
      },
      {
        ke: '/kasir/kas', label: 'Uang Masuk & Keluar', ikon: Wallet, izin: [IZIN.cashMovement],
        kataKunci: ['kas', 'laci', 'pengeluaran', 'setoran'],
      },
      {
        ke: '/kanal', label: 'Kanal Online', ikon: Store, izin: [IZIN.channelManage, IZIN.channelOrderAccept],
        fitur: FITUR.kanalOnline, kataKunci: ['gofood', 'grabfood', 'shopeefood', 'marketplace', 'online'],
      },
    ],
  },
  {
    judul: 'Barang & Stok',
    item: [
      { ke: '/stok', label: 'Stok', ikon: Boxes, izin: [IZIN.stockView], kataKunci: ['inventori', 'persediaan', 'habis', 'sisa'] },
      { ke: '/barang', label: 'Barang', ikon: ShoppingBag, izin: [IZIN.productView], kataKunci: ['produk', 'item', 'harga', 'menu'] },
      { ke: '/barang/master', label: 'Kategori & Satuan', ikon: Tags, izin: [IZIN.productEdit], kataKunci: ['satuan', 'unit', 'pemasok', 'supplier'] },
      { ke: '/stok/opname', label: 'Hitung Fisik', ikon: ClipboardList, izin: [IZIN.stockOpname], kataKunci: ['opname', 'hitung stok'] },
      { ke: '/stok/transfer', label: 'Kirim Antar Toko', ikon: Truck, izin: [IZIN.stockTransfer], kataKunci: ['transfer', 'mutasi', 'cabang'] },
      {
        ke: '/stok/utang', label: 'Utang Pemasok', ikon: HandCoins, izin: [IZIN.stockView],
        kataKunci: ['hutang', 'utang', 'supplier', 'pemasok', 'bayar pemasok', 'jatuh tempo', 'belum lunas'],
      },
    ],
  },
  {
    judul: 'Pelanggan',
    item: [
      { ke: '/pelanggan', label: 'Pelanggan', ikon: Users, izin: [IZIN.customerView], kataKunci: ['customer', 'member', 'langganan pembeli'] },
      { ke: '/kasbon', label: 'Kasbon', ikon: NotebookPen, izin: [IZIN.receivableManage], kataKunci: ['piutang', 'utang', 'hutang', 'tagih'] },
    ],
  },
  {
    judul: 'CRM',
    item: [
      {
        ke: '/crm', label: 'Prospek', ikon: Building2, izin: [IZIN.crmLeadViewOwn, IZIN.crmLeadViewAll],
        fitur: FITUR.crmFreelance, kataKunci: ['deal', 'penawaran', 'calon pelanggan', 'proyek'],
      },
      {
        ke: '/crm/kunjungan', label: 'Kunjungan', ikon: MapPin, izin: [IZIN.crmVisitCheckin],
        fitur: FITUR.salesLapangan, kataKunci: ['sales', 'check-in', 'kanvas'],
      },
    ],
  },
  {
    judul: 'SDM',
    item: [
      { ke: '/sdm', label: 'Karyawan', ikon: UserCog, izin: [IZIN.hrEmployeeView, IZIN.hrEmployeeEdit], kataKunci: ['pegawai', 'staf', 'absensi'] },
      { ke: '/sdm/gaji', label: 'Gaji', ikon: Wallet, izin: [IZIN.hrPayrollRun, IZIN.hrSalaryView], kataKunci: ['payroll', 'slip', 'upah'] },
    ],
  },
]

/**
 * Menu di kaki navigasi samping — pengaturan akun usaha, jarang dibuka,
 * diletakkan di bawah seperti "Settings" di aplikasi SaaS pada umumnya.
 */
export const MENU_BAWAH: ItemMenu[] = [
  { ke: '/langganan', label: 'Langganan', ikon: CreditCard, izin: [IZIN.billingManage], kataKunci: ['paket', 'tagihan', 'bayar', 'upgrade'] },
  {
    ke: '/pengaturan', label: 'Pengaturan', ikon: Settings, izin: [IZIN.outletManage, IZIN.userManage, IZIN.roleManage],
    kataKunci: ['setting', 'toko', 'cabang', 'pajak', 'pengguna', 'peran'],
  },
]

/**
 * Tujuan TAMBAHAN palet perintah — aksi & halaman yang tidak punya baris di
 * navigasi samping (halaman turunan, formulir). Palet menampilkannya bersama
 * seluruh menu di atas.
 */
export const AKSI_PALET: ItemMenu[] = [
  { ke: '/kasir', label: 'Buka Kasir', ikon: Receipt, izin: [IZIN.saleCreate], kataKunci: ['jual', 'transaksi baru', 'pos', 'kasir'] },
  { ke: '/barang/baru', label: 'Tambah barang', ikon: ShoppingBag, izin: [IZIN.productEdit], kataKunci: ['produk baru', 'item baru'] },
  { ke: '/stok/masuk', label: 'Catat barang masuk', ikon: Boxes, izin: [IZIN.stockAdjust], kataKunci: ['pembelian', 'restok', 'belanja', 'kulakan'] },
  { ke: '/stok/koreksi', label: 'Koreksi stok', ikon: ClipboardList, izin: [IZIN.stockAdjust], kataKunci: ['sesuaikan stok', 'stok awal'] },
  { ke: '/barang/impor', label: 'Impor barang dari Excel', ikon: ShoppingBag, izin: [IZIN.productImport], kataKunci: ['csv', 'excel', 'unggah'] },
  { ke: '/kasir/tutup-shift', label: 'Tutup shift', ikon: ArrowRightLeft, izin: [IZIN.shiftClose], kataKunci: ['setor', 'akhir hari'] },
  { ke: '/pengaturan/toko', label: 'Toko & Cabang', ikon: Store, izin: [IZIN.outletManage], kataKunci: ['pajak', 'alamat', 'struk', 'jam tutup buku'] },
  { ke: '/pengaturan/pengguna', label: 'Pengguna', ikon: Users, izin: [IZIN.userManage], kataKunci: ['akun', 'staf', 'kasir baru'] },
  { ke: '/pengaturan/peran', label: 'Peran & Hak Akses', ikon: UserCog, izin: [IZIN.roleManage], kataKunci: ['izin', 'akses', 'role'] },
]

/** Menyaring kelompok; kelompok yang kosong setelah disaring dibuang. */
export function saringKelompok(
  boleh: (...kode: KodeIzin[]) => boolean,
): KelompokMenu[] {
  return KELOMPOK_SAMPING.map((k) => ({ ...k, item: saringMenu(k.item, boleh) })).filter(
    (k) => k.item.length > 0,
  )
}

/**
 * Semua tujuan yang punya baris di navigasi samping (atas, kelompok, bawah)
 * yang boleh dibuka pengguna — bahan menuAktif.
 */
export function tujuanSamping(boleh: (...kode: KodeIzin[]) => boolean): string[] {
  return [
    ...saringMenu(MENU_ATAS, boleh),
    ...saringKelompok(boleh).flatMap((k) => k.item),
    ...saringMenu(MENU_BAWAH, boleh),
  ].map((m) => m.ke)
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
