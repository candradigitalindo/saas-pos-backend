package database

// Katalog tingkat mitra bawaan (Fase 12, blueprint G.1/G.2). Di-seed idempoten
// (ON CONFLICT code DO NOTHING) — nilai yang sudah diubah admin TIDAK ditimpa.
// Satu tingkat saja per mitra; TIDAK ADA jaringan berjenjang (blueprint G.3).

type partnerTierSeed struct {
	Code, Name, Kind, Rate string
	Recurring              bool
	OneTimeMonths          int
	ActMinTxn, ActMinDays  int
	AttributionDays        int
	ClawbackDays           int
}

// partnerTierCatalog — angka awal yang WAJIB diverifikasi ke perjanjian mitra
// sebelum pencairan pertama (blueprint G.2 #6).
var partnerTierCatalog = []partnerTierSeed{
	{
		Code: "afiliasi", Name: "Afiliasi", Kind: "afiliasi", Rate: "0.10",
		Recurring: true, ActMinTxn: 30, ActMinDays: 30, AttributionDays: 60, ClawbackDays: 90,
	},
	{
		Code: "agen", Name: "Agen Daerah", Kind: "agen", Rate: "0.20",
		Recurring: true, ActMinTxn: 30, ActMinDays: 30, AttributionDays: 60, ClawbackDays: 90,
	},
	{
		Code: "agen_senior", Name: "Agen Daerah Senior", Kind: "agen", Rate: "0.25",
		Recurring: true, ActMinTxn: 30, ActMinDays: 30, AttributionDays: 90, ClawbackDays: 90,
	},
}
