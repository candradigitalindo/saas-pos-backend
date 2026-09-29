package database

// planCatalog & termDiscountCatalog adalah data awal langganan platform (§5.13,
// blueprint "Diskon langganan dibayar di muka"). Seeder mengisinya idempoten;
// admin platform menyesuaikan harga & fitur lewat panel (Fase 8+).
//
// Harga bulanan dari blueprint: Basic Rp79.000, Pro Rp199.000. `multi` untuk
// usaha multi-outlet.
//
// `free` = GRATIS SELAMANYA: yang dibatasi hanya FITUR (QRIS, kanal online,
// CRM, sales lapangan, banyak cabang), TANPA kuota pengguna, barang, maupun
// transaksi —
// keputusan pemilik produk 2026-09-27 (menggantikan asumsi "1 pengguna, 50
// produk, 300 transaksi" di docs/BUSINESS-MODEL-CANVAS.md). Pendaftar baru
// langsung memakai paket ini, tanpa masa coba otomatis; masa coba baru dimulai
// saat memilih paket berbayar. Dijaga TestPaketGratisSelamanyaTanpaKuota.
//
// `crm_sales` (sales lapangan) termasuk Pro & Multi-Outlet (keputusan yang
// sama). Seeder hanya menyisipkan paket BARU; basis data yang sudah berjalan
// mendapat kuncinya dari migrasi 000038.

var planCatalog = []struct {
	Code, Name string
	Monthly    int64
	Features   string // JSON
}{
	{"free", "Gratis", 0, `{"qris":false,"crm_freelance":false,"crm_sales":false,"online_channel":false}`},
	{"basic", "Basic", 79000, `{"qris":true,"crm_freelance":false,"crm_sales":false,"online_channel":false}`},
	{"pro", "Pro", 199000, `{"qris":true,"crm_freelance":true,"crm_sales":true,"online_channel":true}`},
	{"multi", "Multi-Outlet", 399000, `{"qris":true,"crm_freelance":true,"crm_sales":true,"online_channel":true,"multi_outlet":true}`},
}

// termDiscountCatalog: tangga diskon prabayar. Rate pecahan (0.1670 = 16,7%).
var termDiscountCatalog = []struct {
	Term int
	Rate string
}{
	{1, "0"},
	{3, "0.05"},
	{6, "0.10"},
	{9, "0.125"},
	{12, "0.167"},
}
