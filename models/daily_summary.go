package models

import "time"

// DailySalesSummary adalah AGREGAT laporan harian — satu baris per
// (tenant, outlet, hari usaha, kanal). Dashboard & laporan rentang tanggal
// membaca tabel ini, bukan menjumlahkan `sales` penuh (§5.14, §16 Fase 5 DoD:
// "dashboard < 1 detik pada 100.000 transaksi").
//
// Boleh dihitung ulang kapan saja dari `sales` + `sale_payments` lewat
// repositories.RefreshDailySummary — `sales` tetap sumber kebenaran, ini cache
// yang dipelihara inkremental saat checkout dan direkalkulasi saat void/retur.
//
// Semua nilai uang BIGINT rupiah bulat (§3.3). ChannelID memakai "" untuk
// "tanpa kanal" (transaksi kasir langsung), bukan NULL — agar PK & ON CONFLICT
// sederhana; kolomnya TEXT supaya "" tidak di-padding seperti pada CHAR.
type DailySalesSummary struct {
	TenantID     string    `json:"tenant_id" gorm:"primaryKey;type:char(26)"`
	OutletID     string    `json:"outlet_id" gorm:"primaryKey;type:char(26)"`
	BusinessDate time.Time `json:"business_date" gorm:"primaryKey;type:date"`
	ChannelID    string    `json:"channel_id" gorm:"primaryKey;type:text;default:''"`

	SalesCount     int64 `json:"sales_count"`     // hanya transaksi berstatus 'completed'
	GrossAmount    int64 `json:"gross_amount"`    // Σ subtotal (sebelum diskon)
	DiscountAmount int64 `json:"discount_amount"` // Σ discount_amount
	TaxAmount      int64 `json:"tax_amount"`      // Σ tax_amount
	NetAmount      int64 `json:"net_amount"`      // Σ total (termasuk service & pembulatan)
	CostAmount     int64 `json:"cost_amount"`     // Σ cost_total
	FeeAmount      int64 `json:"fee_amount"`      // Σ sale_payments.fee_amount (MDR / komisi kanal)
	GrossProfit    int64 `json:"gross_profit"`    // net_amount - cost_amount - fee_amount

	UpdatedAt time.Time `json:"updated_at"`
}

// TableName memaksa nama tabel jamak — GORM tidak diberi kesempatan menebak.
func (DailySalesSummary) TableName() string { return "daily_sales_summaries" }
