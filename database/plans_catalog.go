package database

// planCatalog & termDiscountCatalog adalah data awal langganan platform (§5.13,
// blueprint "Diskon langganan dibayar di muka"). Seeder mengisinya idempoten;
// admin platform menyesuaikan harga & fitur lewat panel (Fase 8+).
//
// Harga bulanan dari blueprint: Basic Rp79.000, Pro Rp199.000. `free` untuk masa
// evaluasi, `multi` untuk usaha multi-outlet.

var planCatalog = []struct {
	Code, Name string
	Monthly    int64
	Features   string // JSON
}{
	{"free", "Gratis", 0, `{"qris":false,"crm_freelance":false,"online_channel":false}`},
	{"basic", "Basic", 79000, `{"qris":true,"crm_freelance":false,"online_channel":false}`},
	{"pro", "Pro", 199000, `{"qris":true,"crm_freelance":true,"online_channel":true}`},
	{"multi", "Multi-Outlet", 399000, `{"qris":true,"crm_freelance":true,"online_channel":true,"multi_outlet":true}`},
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
