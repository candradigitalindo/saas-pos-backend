package services

import (
	"context"
	"fmt"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"gorm.io/gorm"
)

// Layanan laporan & dashboard (Fase 5, §8, §13.5).
//
// Prinsip: SEMUA angka rentang tanggal dibaca dari agregat
// `daily_sales_summaries`, tidak pernah menjumlahkan tabel `sales` penuh — itu
// yang menjaga "dashboard < 1 detik pada 100.000 transaksi" (§16 Fase 5 DoD).
// Pengecualian sengaja: group_by=cashier|payment membaca `sales`/`sale_payments`
// langsung karena dimensi itu bukan kolom tabel ringkasan.

const dateLayout = "2006-01-02"

// maxRebuildDays membatasi rentang rebuild manual agar tidak ada loop lari liar.
const maxRebuildDays = 366

// aggToTotals menyalin agregat repositori ke DTO (bentuk identik, paket berbeda).
func aggToTotals(a repositories.SummaryAggregate) structs.SummaryTotals {
	return structs.SummaryTotals{
		SalesCount:     a.SalesCount,
		GrossAmount:    a.GrossAmount,
		DiscountAmount: a.DiscountAmount,
		TaxAmount:      a.TaxAmount,
		NetAmount:      a.NetAmount,
		CostAmount:     a.CostAmount,
		FeeAmount:      a.FeeAmount,
		GrossProfit:    a.GrossProfit,
	}
}

// parseDate memvalidasi tanggal YYYY-MM-DD; kosong tidak diizinkan di sini.
func parseDate(field, s string) (time.Time, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %s harus berformat YYYY-MM-DD", helpers.ErrValidation, field)
	}
	return t, nil
}

// parseRange memvalidasi sepasang tanggal dan memastikan from <= to.
func parseRange(from, to string) (time.Time, time.Time, error) {
	f, err := parseDate("from", from)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	t, err := parseDate("to", to)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if t.Before(f) {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: to lebih awal dari from", helpers.ErrValidation)
	}
	return f, t, nil
}

// DashboardReport mengembalikan ringkasan satu hari + akumulasi bulan berjalan
// + rincian per kanal, seluruhnya dari daily_sales_summaries. date kosong =
// hari ini (UTC).
func DashboardReport(ctx context.Context, outletID, date string) (structs.DashboardResponse, error) {
	var out structs.DashboardResponse

	day := time.Now().UTC()
	if date != "" {
		d, err := parseDate("date", date)
		if err != nil {
			return out, err
		}
		day = d
	}
	dayStr := day.Format(dateLayout)
	monthStart := time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, time.UTC).Format(dateLayout)

	today, err := repositories.SummaryTotalsBetween(ctx, outletID, dayStr, dayStr)
	if err != nil {
		return out, err
	}
	mtd, err := repositories.SummaryTotalsBetween(ctx, outletID, monthStart, dayStr)
	if err != nil {
		return out, err
	}
	channels, err := repositories.SummaryByChannel(ctx, outletID, dayStr, dayStr)
	if err != nil {
		return out, err
	}

	out.Date = dayStr
	out.OutletID = outletID
	out.Today = aggToTotals(today)
	out.MonthToDate = aggToTotals(mtd)
	out.ByChannel = make([]structs.ChannelBucket, 0, len(channels))
	for _, c := range channels {
		out.ByChannel = append(out.ByChannel, structs.ChannelBucket{
			ChannelID: c.Key, SummaryTotals: aggToTotals(c.SummaryAggregate),
		})
	}
	return out, nil
}

// SalesReport mengembalikan laporan penjualan terkelompok pada rentang tanggal.
// groupBy: "day" (default) & "channel" dari tabel ringkasan; "cashier",
// "payment" & "hour" dari tabel sales/sale_payments; "product" dari
// sale_items (baris membawa label, satuan, dan qty — lihat SalesByProduct).
func SalesReport(ctx context.Context, outletID, from, to, groupBy string) (structs.SalesReportResponse, error) {
	var out structs.SalesReportResponse

	f, t, err := parseRange(from, to)
	if err != nil {
		return out, err
	}
	fromStr, toStr := f.Format(dateLayout), t.Format(dateLayout)

	if groupBy == "" {
		groupBy = "day"
	}
	var rows []repositories.SummaryGroupRow
	// Hanya terisi untuk group_by=product: nama, satuan & qty per baris,
	// sejajar indeksnya dengan `rows`.
	var perBarang []repositories.ProductSalesRow
	switch groupBy {
	case "day":
		rows, err = repositories.SummaryByDate(ctx, outletID, fromStr, toStr)
	case "channel":
		rows, err = repositories.SummaryByChannel(ctx, outletID, fromStr, toStr)
	case "cashier":
		rows, err = repositories.SalesByCashier(ctx, outletID, fromStr, toStr)
	case "payment":
		rows, err = repositories.SalesByPaymentMethod(ctx, outletID, fromStr, toStr)
	case "hour":
		// Jam dinding di zona outlet — lihat catatan panjang di SalesByHour
		// soal kenapa UTC akan menjawab pertanyaan yang salah.
		rows, err = repositories.SalesByHour(ctx, outletID, fromStr, toStr)
	case "product":
		perBarang, err = repositories.SalesByProduct(ctx, outletID, fromStr, toStr)
		for _, b := range perBarang {
			rows = append(rows, b.SummaryGroupRow)
		}
	default:
		return out, fmt.Errorf("%w: group_by harus salah satu dari day, hour, channel, cashier, payment, product", helpers.ErrValidation)
	}
	if err != nil {
		return out, err
	}

	totals, err := repositories.SummaryTotalsBetween(ctx, outletID, fromStr, toStr)
	if err != nil {
		return out, err
	}

	out.From, out.To, out.GroupBy = fromStr, toStr, groupBy
	out.Totals = aggToTotals(totals)
	out.Rows = make([]structs.SalesReportRow, 0, len(rows))
	for i, r := range rows {
		baris := structs.SalesReportRow{
			Key:            r.Key,
			SalesCount:     r.SalesCount,
			GrossAmount:    r.GrossAmount,
			DiscountAmount: r.DiscountAmount,
			TaxAmount:      r.TaxAmount,
			NetAmount:      r.NetAmount,
			CostAmount:     r.CostAmount,
			FeeAmount:      r.FeeAmount,
			GrossProfit:    r.GrossProfit,
		}
		if perBarang != nil {
			baris.Label = perBarang[i].Label
			baris.Unit = perBarang[i].Unit
			baris.Qty = perBarang[i].Qty.String()
		}
		out.Rows = append(out.Rows, baris)
	}
	return out, nil
}

// ProfitReport mengembalikan laba bersih per kanal pada rentang tanggal
// (bentuk §13.5), dari daily_sales_summaries.
func ProfitReport(ctx context.Context, outletID, from, to string) (structs.ProfitReportResponse, error) {
	var out structs.ProfitReportResponse

	f, t, err := parseRange(from, to)
	if err != nil {
		return out, err
	}
	fromStr, toStr := f.Format(dateLayout), t.Format(dateLayout)

	channels, err := repositories.SummaryByChannel(ctx, outletID, fromStr, toStr)
	if err != nil {
		return out, err
	}
	totals, err := repositories.SummaryTotalsBetween(ctx, outletID, fromStr, toStr)
	if err != nil {
		return out, err
	}

	out.From, out.To = fromStr, toStr
	out.ByChannel = make([]structs.ProfitReportRow, 0, len(channels))
	for _, c := range channels {
		out.ByChannel = append(out.ByChannel, profitRow(c.Key, c.SummaryAggregate))
	}
	out.Totals = profitRow("", totals)
	return out, nil
}

// profitRow memetakan agregat ke baris laba §13.5.
func profitRow(channelID string, a repositories.SummaryAggregate) structs.ProfitReportRow {
	return structs.ProfitReportRow{
		ChannelID:  channelID,
		Omzet:      a.NetAmount,
		Modal:      a.CostAmount,
		BiayaKanal: a.FeeAmount,
		LabaBersih: a.GrossProfit,
	}
}

// RebuildDailySummaries menghitung ulang daily_sales_summaries untuk tiap
// (outlet, hari) pada rentang — alat koreksi drift, sepadan dengan
// /stock-reconcile. outletID kosong = seluruh outlet yang punya transaksi pada
// rentang itu.
func RebuildDailySummaries(ctx context.Context, outletID, from, to string) (structs.RebuildSummariesResponse, error) {
	var out structs.RebuildSummariesResponse

	f, t, err := parseRange(from, to)
	if err != nil {
		return out, err
	}
	days := int(t.Sub(f).Hours()/24) + 1
	if days > maxRebuildDays {
		return out, fmt.Errorf("%w: rentang rebuild maksimal %d hari", helpers.ErrValidation, maxRebuildDays)
	}
	fromStr, toStr := f.Format(dateLayout), t.Format(dateLayout)

	outlets := []string{outletID}
	if outletID == "" {
		outlets, err = repositories.OutletIDsWithSales(ctx, fromStr, toStr)
		if err != nil {
			return out, err
		}
	}

	err = repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		for _, oid := range outlets {
			for d := f; !d.After(t); d = d.AddDate(0, 0, 1) {
				if err := repositories.RefreshDailySummary(ctx, tx, oid, d); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return out, err
	}

	out.From, out.To = fromStr, toStr
	out.Outlets = outlets
	out.DaysPerOutlet = days
	out.RowsRebuilt = len(outlets) * days
	return out, nil
}
