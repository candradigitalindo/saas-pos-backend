package database

// permissionCatalog adalah daftar lengkap kode permission yang dikenal sistem
// (docs/TECHNICAL-BACKEND.md §5.3). Sumber tunggal: seeder mengisi tabel
// `permissions` dari sini, dan registrasi tenant memberikan SELURUH baris
// `permissions` ke peran Pemilik.
//
// Menambah permission baru = menambah satu baris di sini lalu jalankan seeder
// ulang (idempoten). JANGAN mengganti `Code` yang sudah dipakai — itu memutus
// pemetaan role_permissions yang ada.
//
// `Group` hanya untuk pengelompokan tampilan di UI pengaturan peran.
var permissionCatalog = []struct {
	Code, Group, Description string
}{
	// Penjualan
	{"sale.create", "Penjualan", "Membuat transaksi penjualan"},
	{"sale.void", "Penjualan", "Membatalkan transaksi (void)"},
	{"sale.refund", "Penjualan", "Melakukan retur/refund"},
	{"sale.discount", "Penjualan", "Memberi diskon pada transaksi"},
	{"sale.price_override", "Penjualan", "Mengubah harga jual saat transaksi"},

	// Produk
	{"product.view", "Produk", "Melihat daftar & detail produk"},
	{"product.edit", "Produk", "Menambah & mengubah produk"},
	{"product.delete", "Produk", "Menghapus produk"},
	{"product.import", "Produk", "Impor produk massal dari CSV/Excel"},

	// Stok
	{"stock.view", "Stok", "Melihat saldo & kartu stok"},
	{"stock.adjust", "Stok", "Penyesuaian stok manual"},
	{"stock.opname", "Stok", "Melakukan stok opname"},
	{"stock.transfer", "Stok", "Transfer stok antar outlet"},

	// Laporan
	{"report.view", "Laporan", "Melihat laporan penjualan"},
	{"report.profit", "Laporan", "Melihat laporan laba (harga modal)"},
	{"report.export", "Laporan", "Mengekspor laporan"},

	// Shift & kas
	{"shift.open", "Shift & Kas", "Membuka shift kasir"},
	{"shift.close", "Shift & Kas", "Menutup shift kasir"},
	{"shift.reconcile", "Shift & Kas", "Merekonsiliasi selisih kas shift"},
	{"cash.movement", "Shift & Kas", "Mencatat kas masuk/keluar non-penjualan"},

	// Pelanggan & piutang
	{"customer.view", "Pelanggan", "Melihat data pelanggan"},
	{"customer.edit", "Pelanggan", "Menambah & mengubah pelanggan"},
	{"receivable.manage", "Pelanggan", "Mengelola piutang & pembayaran"},

	// Administrasi
	{"user.manage", "Administrasi", "Mengelola user dalam tenant"},
	{"role.manage", "Administrasi", "Mengelola peran & hak akses"},
	{"outlet.manage", "Administrasi", "Mengelola outlet"},
	{"setting.manage", "Administrasi", "Mengubah pengaturan usaha"},
	{"billing.manage", "Administrasi", "Mengelola langganan & tagihan platform"},

	// CRM
	{"crm.lead.view.own", "CRM", "Melihat prospek milik sendiri"},
	{"crm.lead.view.all", "CRM", "Melihat semua prospek tenant"},
	{"crm.deal.edit", "CRM", "Mengubah deal/peluang"},
	{"crm.visit.checkin", "CRM", "Check-in kunjungan lapangan"},
	{"crm.commission.view", "CRM", "Melihat komisi sales"},

	// Dokumen penjualan
	{"quotation.approve", "Dokumen", "Menyetujui penawaran"},
	{"invoice.issue", "Dokumen", "Menerbitkan invoice pelanggan"},
	{"invoice.void", "Dokumen", "Membatalkan invoice pelanggan"},

	// Kanal online
	{"channel.manage", "Kanal", "Mengelola kanal & katalog kanal"},
	{"channel.order.accept", "Kanal", "Menerima pesanan kanal"},
	{"channel.settlement.view", "Kanal", "Melihat rekonsiliasi settlement kanal"},
}
