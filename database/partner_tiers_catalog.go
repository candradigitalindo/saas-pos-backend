package database

// Katalog tingkat mitra bawaan (Fase 12, §5.12, blueprint G.1/G.2). Di-seed
// idempoten (ON CONFLICT name DO NOTHING) — nilai yang sudah diubah admin TIDAK
// ditimpa. Satu tingkat per mitra; TIDAK ADA jaringan berjenjang (blueprint G.3).

type partnerTierSeed struct {
	Name, Kind, Rate      string
	RecurringMonths       *int // nil = selama merchant aktif
	ActivationBonus       int64
	MinActiveMerchants    int
	ActMinTxn, ActMinDays int
	AttributionDays       int
	ClawbackDays          int
}

// partnerTierCatalog — angka awal yang WAJIB diverifikasi ke perjanjian mitra
// sebelum pencairan pertama (blueprint G.2 #6). Nama mengikuti §5.12.
var partnerTierCatalog = []partnerTierSeed{
	{
		Name: "Afiliasi", Kind: "affiliate", Rate: "0.10",
		ActivationBonus: 0, MinActiveMerchants: 0,
		ActMinTxn: 30, ActMinDays: 30, AttributionDays: 60, ClawbackDays: 90,
	},
	{
		Name: "Agen", Kind: "agent", Rate: "0.20",
		ActivationBonus: 100000, MinActiveMerchants: 3,
		ActMinTxn: 30, ActMinDays: 30, AttributionDays: 60, ClawbackDays: 90,
	},
	{
		Name: "Agen Utama", Kind: "agent", Rate: "0.25",
		ActivationBonus: 150000, MinActiveMerchants: 10,
		ActMinTxn: 30, ActMinDays: 30, AttributionDays: 90, ClawbackDays: 90,
	},
}
