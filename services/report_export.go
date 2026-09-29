package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"
)

// Ekspor laporan (Fase 5, §8) — CSV untuk program, XLSX untuk manusia.
//
// Setiap laporan disusun SEKALI sebagai tabel (judul + baris), lalu ditulis
// ke format yang diminta. Sel uang ditulis sebagai int64 dan jumlah desimal
// sebagai Angka, supaya di XLSX keduanya tersimpan sebagai angka (bisa
// dijumlahkan), sementara di CSV tetap teks biasa. PDF masih ditunda.

// i64 memformat int64 sebagai desimal untuk sel CSV.
func i64(v int64) string { return strconv.FormatInt(v, 10) }

// Berkas hasil ekspor.
type BerkasEkspor struct {
	Nama      string
	JenisMIME string
	Isi       []byte
}

const (
	mimeCSV  = "text/csv; charset=utf-8"
	mimeXLSX = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
)

// ExportReport menjalankan laporan yang diminta lalu menuliskannya sebagai
// berkas `format` ("csv" | "xlsx").
//
// reportType: "sales" | "profit" | "dashboard" | "purchases" (baris barang
// per nota belanja) | "purchase_payments" (pembayaran ke pemasok).
func ExportReport(ctx context.Context, reportType, format, outletID, from, to, groupBy, date string) (BerkasEkspor, error) {
	if format != "csv" && format != "xlsx" {
		return BerkasEkspor{}, fmt.Errorf("%w: format harus csv atau xlsx", helpers.ErrValidation)
	}
	nama, lembar, tabel, err := susunTabel(ctx, reportType, outletID, from, to, groupBy, date)
	if err != nil {
		return BerkasEkspor{}, err
	}
	if format == "xlsx" {
		isi, err := tulisXLSX(lembar, tabel)
		if err != nil {
			return BerkasEkspor{}, err
		}
		return BerkasEkspor{Nama: nama + ".xlsx", JenisMIME: mimeXLSX, Isi: isi}, nil
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	for _, r := range tabel {
		sel := make([]string, len(r))
		for i, v := range r {
			switch x := v.(type) {
			case int64:
				sel[i] = i64(x)
			case Angka:
				sel[i] = string(x)
			default:
				sel[i] = fmt.Sprint(x)
			}
		}
		_ = w.Write(sel)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return BerkasEkspor{}, err
	}
	return BerkasEkspor{Nama: nama + ".csv", JenisMIME: mimeCSV, Isi: buf.Bytes()}, nil
}

// susunTabel menjalankan laporan dan mengembalikan nama berkas (tanpa
// ekstensi), nama lembar, dan tabelnya (baris pertama = judul kolom).
func susunTabel(ctx context.Context, reportType, outletID, from, to, groupBy, date string) (string, string, [][]any, error) {
	var t [][]any
	tambah := func(sel ...any) { t = append(t, sel) }

	switch reportType {
	case "sales":
		rep, err := SalesReport(ctx, outletID, from, to, groupBy)
		if err != nil {
			return "", "", nil, err
		}
		// Per barang: key-nya ULID yang tak berarti di spreadsheet, jadi nama,
		// satuan, dan jumlah terjual ikut sebagai kolom setelah key.
		perBarang := rep.GroupBy == "product"
		judul := []any{"key"}
		if perBarang {
			judul = append(judul, "product_name", "unit", "qty")
		}
		tambah(append(judul, "sales_count", "gross_amount", "discount_amount", "tax_amount",
			"net_amount", "cost_amount", "fee_amount", "gross_profit")...)
		for _, r := range rep.Rows {
			sel := []any{r.Key}
			if perBarang {
				sel = append(sel, r.Label, r.Unit, Angka(r.Qty))
			}
			tambah(append(sel, r.SalesCount, r.GrossAmount, r.DiscountAmount, r.TaxAmount,
				r.NetAmount, r.CostAmount, r.FeeAmount, r.GrossProfit)...)
		}
		tt := rep.Totals
		total := []any{"TOTAL"}
		if perBarang {
			total = append(total, "", "", "")
		}
		tambah(append(total, tt.SalesCount, tt.GrossAmount, tt.DiscountAmount, tt.TaxAmount,
			tt.NetAmount, tt.CostAmount, tt.FeeAmount, tt.GrossProfit)...)
		return fmt.Sprintf("laporan-penjualan-%s_%s", rep.From, rep.To), "Penjualan", t, nil

	case "profit":
		rep, err := ProfitReport(ctx, outletID, from, to)
		if err != nil {
			return "", "", nil, err
		}
		tambah("channel_id", "omzet", "modal", "biaya_kanal", "laba_bersih")
		for _, r := range rep.ByChannel {
			tambah(r.ChannelID, r.Omzet, r.Modal, r.BiayaKanal, r.LabaBersih)
		}
		tt := rep.Totals
		tambah("TOTAL", tt.Omzet, tt.Modal, tt.BiayaKanal, tt.LabaBersih)
		return fmt.Sprintf("laporan-laba-%s_%s", rep.From, rep.To), "Laba", t, nil

	case "dashboard":
		rep, err := DashboardReport(ctx, outletID, date)
		if err != nil {
			return "", "", nil, err
		}
		tambah("scope", "sales_count", "gross_amount", "discount_amount", "tax_amount",
			"net_amount", "cost_amount", "fee_amount", "gross_profit")
		baris := func(scope string, s structs.SummaryTotals) {
			tambah(scope, s.SalesCount, s.GrossAmount, s.DiscountAmount, s.TaxAmount,
				s.NetAmount, s.CostAmount, s.FeeAmount, s.GrossProfit)
		}
		baris("today", rep.Today)
		baris("month_to_date", rep.MonthToDate)
		for _, c := range rep.ByChannel {
			baris("channel:"+c.ChannelID, c.SummaryTotals)
		}
		return fmt.Sprintf("laporan-dashboard-%s", rep.Date), "Dashboard", t, nil

	case "purchases", "purchase_payments":
		// Nilai pembelian: selain report.export (rute), butuh juga stock.view —
		// sama dengan GET /reports/purchases.
		if !reqctx.HasPermission(ctx, "stock.view") {
			return "", "", nil, fmt.Errorf("%w: ekspor belanja butuh izin melihat stok", helpers.ErrForbidden)
		}
		f, tt, err := parseRange(from, to)
		if err != nil {
			return "", "", nil, err
		}
		dari, sampai := f.Format(dateLayout), tt.Format(dateLayout)
		// Judul kolom berbahasa Indonesia: berkas ini dibuka pemilik & akuntan
		// di spreadsheet, bukan dibaca program.
		if reportType == "purchases" {
			rows, err := repositories.PurchaseLines(ctx, outletID, dari, sampai)
			if err != nil {
				return "", "", nil, err
			}
			tambah("Tanggal", "No. Nota", "Pemasok", "Barang", "Jumlah", "Satuan", "Harga Satuan",
				"Subtotal", "Total Nota", "Dibayar Nota", "Sisa Nota", "Jatuh Tempo")
			for _, r := range rows {
				nama := r.ProductName
				if r.VariantName != "" {
					nama += " (" + r.VariantName + ")"
				}
				tambah(r.BusinessDate, r.InvoiceNo, r.SupplierName, nama, Angka(r.Qty.String()), r.UnitName,
					r.UnitCost, r.LineTotal, r.NotaTotal, r.NotaPaid, r.NotaTotal-r.NotaPaid, r.DueDate)
			}
			return fmt.Sprintf("laporan-belanja-%s_%s", dari, sampai), "Belanja", t, nil
		}
		rows, err := repositories.PurchasePaymentsInRange(ctx, outletID, dari, sampai)
		if err != nil {
			return "", "", nil, err
		}
		tambah("Tanggal Bayar", "Pemasok", "No. Nota", "Tanggal Nota", "Jumlah", "Sumber Uang", "Catatan", "Dicatat Oleh")
		var total int64
		for _, r := range rows {
			sumber := "Uang lain"
			if r.Source == SumberLaci {
				sumber = "Laci kasir"
			}
			tambah(r.BusinessDate, r.SupplierName, r.InvoiceNo, r.NotaDate, r.Amount, sumber, r.Note, r.CreatedByName)
			total += r.Amount
		}
		tambah("TOTAL", "", "", "", total, "", "", "")
		return fmt.Sprintf("pembayaran-pemasok-%s_%s", dari, sampai), "Pembayaran", t, nil
	}
	return "", "", nil, fmt.Errorf("%w: type harus salah satu dari sales, profit, dashboard, purchases, purchase_payments",
		helpers.ErrValidation)
}
