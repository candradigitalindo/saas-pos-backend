package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"

	"candra/backend-api/helpers"
	"candra/backend-api/structs"
)

// Ekspor laporan (Fase 5, §8). CSV standar sudah cukup untuk kebutuhan
// akuntansi fase ini (BLUEPRINT-SAAS-POS "Ekspor untuk akuntansi | Fase 5 |
// CSV standar sudah cukup"); XLSX & PDF ditunda ke fase berikutnya agar tidak
// menambah dependensi berat sekarang.

// i64 memformat int64 sebagai desimal untuk sel CSV.
func i64(v int64) string { return strconv.FormatInt(v, 10) }

// ExportReportCSV menjalankan laporan yang diminta lalu men-serialisasi hasilnya
// ke CSV. Mengembalikan nama berkas yang disarankan + isi berkas.
//
// reportType: "sales" | "profit" | "dashboard".
func ExportReportCSV(ctx context.Context, reportType, outletID, from, to, groupBy, date string) (string, []byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	var filename string
	switch reportType {
	case "sales":
		rep, err := SalesReport(ctx, outletID, from, to, groupBy)
		if err != nil {
			return "", nil, err
		}
		filename = fmt.Sprintf("laporan-penjualan-%s_%s.csv", rep.From, rep.To)
		_ = w.Write([]string{
			"key", "sales_count", "gross_amount", "discount_amount", "tax_amount",
			"net_amount", "cost_amount", "fee_amount", "gross_profit",
		})
		for _, r := range rep.Rows {
			_ = w.Write([]string{
				r.Key, i64(r.SalesCount), i64(r.GrossAmount), i64(r.DiscountAmount), i64(r.TaxAmount),
				i64(r.NetAmount), i64(r.CostAmount), i64(r.FeeAmount), i64(r.GrossProfit),
			})
		}
		tt := rep.Totals
		_ = w.Write([]string{
			"TOTAL", i64(tt.SalesCount), i64(tt.GrossAmount), i64(tt.DiscountAmount), i64(tt.TaxAmount),
			i64(tt.NetAmount), i64(tt.CostAmount), i64(tt.FeeAmount), i64(tt.GrossProfit),
		})

	case "profit":
		rep, err := ProfitReport(ctx, outletID, from, to)
		if err != nil {
			return "", nil, err
		}
		filename = fmt.Sprintf("laporan-laba-%s_%s.csv", rep.From, rep.To)
		_ = w.Write([]string{"channel_id", "omzet", "modal", "biaya_kanal", "laba_bersih"})
		for _, r := range rep.ByChannel {
			_ = w.Write([]string{r.ChannelID, i64(r.Omzet), i64(r.Modal), i64(r.BiayaKanal), i64(r.LabaBersih)})
		}
		tt := rep.Totals
		_ = w.Write([]string{"TOTAL", i64(tt.Omzet), i64(tt.Modal), i64(tt.BiayaKanal), i64(tt.LabaBersih)})

	case "dashboard":
		rep, err := DashboardReport(ctx, outletID, date)
		if err != nil {
			return "", nil, err
		}
		filename = fmt.Sprintf("laporan-dashboard-%s.csv", rep.Date)
		_ = w.Write([]string{
			"scope", "sales_count", "gross_amount", "discount_amount", "tax_amount",
			"net_amount", "cost_amount", "fee_amount", "gross_profit",
		})
		row := func(scope string, t structs.SummaryTotals) {
			_ = w.Write([]string{
				scope, i64(t.SalesCount), i64(t.GrossAmount), i64(t.DiscountAmount), i64(t.TaxAmount),
				i64(t.NetAmount), i64(t.CostAmount), i64(t.FeeAmount), i64(t.GrossProfit),
			})
		}
		row("today", rep.Today)
		row("month_to_date", rep.MonthToDate)
		for _, c := range rep.ByChannel {
			row("channel:"+c.ChannelID, c.SummaryTotals)
		}

	default:
		return "", nil, fmt.Errorf("%w: type harus salah satu dari sales, profit, dashboard", helpers.ErrValidation)
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return "", nil, err
	}
	return filename, buf.Bytes(), nil
}
