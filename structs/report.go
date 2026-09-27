package structs

// DTO laporan & dashboard (Fase 5, §8). Semua nilai uang int64 rupiah bulat.

// SummaryTotals adalah delapan besaran agregat yang dibagikan seluruh laporan.
type SummaryTotals struct {
	SalesCount     int64 `json:"sales_count"`
	GrossAmount    int64 `json:"gross_amount"`
	DiscountAmount int64 `json:"discount_amount"`
	TaxAmount      int64 `json:"tax_amount"`
	NetAmount      int64 `json:"net_amount"`
	CostAmount     int64 `json:"cost_amount"`
	FeeAmount      int64 `json:"fee_amount"`
	GrossProfit    int64 `json:"gross_profit"`
}

// ChannelBucket adalah SummaryTotals untuk satu kanal ("" = kasir langsung).
type ChannelBucket struct {
	ChannelID string `json:"channel_id"`
	SummaryTotals
}

// DashboardResponse menjawab GET /api/v1/reports/dashboard. Semua angka berasal
// dari daily_sales_summaries — tidak ada SUM(sales) penuh.
type DashboardResponse struct {
	Date        string          `json:"date"`                // hari yang diminta (YYYY-MM-DD)
	OutletID    string          `json:"outlet_id,omitempty"` // kosong = seluruh outlet tenant
	Today       SummaryTotals   `json:"today"`
	MonthToDate SummaryTotals   `json:"month_to_date"` // 1 s.d. Date pada bulan yang sama
	ByChannel   []ChannelBucket `json:"by_channel"`
}

// SalesReportRow adalah satu baris laporan penjualan terkelompok. Kolom yang
// tidak berlaku untuk sebuah dimensi bernilai 0 (mis. modal/laba pada group_by
// payment).
type SalesReportRow struct {
	Key            string `json:"key"` // tanggal | channel_id | nama kasir | metode bayar | product_id
	SalesCount     int64  `json:"sales_count"`
	GrossAmount    int64  `json:"gross_amount"`
	DiscountAmount int64  `json:"discount_amount"`
	TaxAmount      int64  `json:"tax_amount"`
	NetAmount      int64  `json:"net_amount"`
	CostAmount     int64  `json:"cost_amount"`
	FeeAmount      int64  `json:"fee_amount"`
	GrossProfit    int64  `json:"gross_profit"`

	// Hanya untuk group_by=product (kosong & tidak dikirim pada dimensi lain).
	Label string `json:"label,omitempty"` // nama barang
	Unit  string `json:"unit,omitempty"`  // satuan barang
	Qty   string `json:"qty,omitempty"`   // jumlah terjual bersih, desimal string ("3", "0.25")
}

// SalesReportResponse menjawab GET /api/v1/reports/sales.
type SalesReportResponse struct {
	From    string           `json:"from"`
	To      string           `json:"to"`
	GroupBy string           `json:"group_by"`
	Rows    []SalesReportRow `json:"rows"`
	Totals  SummaryTotals    `json:"totals"`
}

// ProfitReportRow mengikuti bentuk §13.5 (laba bersih per kanal).
type ProfitReportRow struct {
	ChannelID  string `json:"channel_id"`
	Omzet      int64  `json:"omzet"`       // Σ net_amount
	Modal      int64  `json:"modal"`       // Σ cost_amount
	BiayaKanal int64  `json:"biaya_kanal"` // Σ fee_amount (MDR / komisi)
	LabaBersih int64  `json:"laba_bersih"` // omzet - modal - biaya_kanal
}

// ProfitReportResponse menjawab GET /api/v1/reports/profit.
type ProfitReportResponse struct {
	From      string            `json:"from"`
	To        string            `json:"to"`
	ByChannel []ProfitReportRow `json:"by_channel"`
	Totals    ProfitReportRow   `json:"totals"` // ChannelID kosong
}

// RebuildSummariesResponse menjawab POST /api/v1/reports/rebuild-summaries.
type RebuildSummariesResponse struct {
	From          string   `json:"from"`
	To            string   `json:"to"`
	Outlets       []string `json:"outlets"`
	DaysPerOutlet int      `json:"days_per_outlet"`
	RowsRebuilt   int      `json:"rows_rebuilt"` // (jumlah outlet) × (jumlah hari)
}
