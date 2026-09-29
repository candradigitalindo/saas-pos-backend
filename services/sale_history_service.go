package services

import (
	"context"
	"fmt"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"
)

// Riwayat penjualan (layar Kasir › Riwayat).
//
// Dulu daftar riwayat hanya berisi nomor nota, jam, dan total: isi belanja,
// cara bayar, pelanggan, dan kasirnya baru terlihat bila transaksi dibuka satu
// per satu — padahal itu yang dicari orang saat menelusuri ("yang bayar QRIS
// tadi siang", "kasbon Bu Rina"). Kini satu halaman memuat semuanya dengan
// kueri tetap (bukan satu kueri per baris), plus ringkasan harinya.

// ListSalesDetailed: satu halaman transaksi lengkap dengan item, pembayaran,
// nama pelanggan, dan nama kasir.
func ListSalesDetailed(ctx context.Context, f repositories.SaleFilter, limit, offset int) ([]structs.SaleResponse, int64, error) {
	rows, total, err := repositories.ListSales(ctx, f, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	if err := repositories.AttachSaleLines(ctx, rows); err != nil {
		return nil, 0, err
	}
	pelanggan, kasir, err := repositories.SaleNames(ctx, rows)
	if err != nil {
		return nil, 0, err
	}
	asal, diretur, err := repositories.SaleReturnLinks(ctx, rows)
	if err != nil {
		return nil, 0, err
	}
	out := make([]structs.SaleResponse, len(rows))
	for i := range rows {
		out[i] = SaleToResponse(&rows[i])
		if rows[i].CustomerID != nil {
			out[i].CustomerName = pelanggan[*rows[i].CustomerID]
		}
		out[i].CashierName = kasir[rows[i].CreatedBy]
		out[i].ReturnOfReceiptNo = asal[rows[i].ID]
		out[i].ReturnedByReceiptNo = diretur[rows[i].ID]
	}
	return out, total, nil
}

// SalesDaySummary merangkum satu business_date: penjualan selesai, retur,
// batal, dan uang masuk per cara bayar (rumus yang sama dengan laporan: hanya
// transaksi selesai, tunai dikurangi kembalian).
func SalesDaySummary(ctx context.Context, outletID, date string) (structs.SaleSummaryResponse, error) {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return structs.SaleSummaryResponse{}, fmt.Errorf("%w: business_date harus berformat YYYY-MM-DD", helpers.ErrValidation)
	}
	out, err := salesSummary(ctx, repositories.SaleScope{OutletID: outletID, BusinessDate: date})
	out.BusinessDate = date
	return out, err
}

// ShiftSalesSummary: ringkasan penjualan satu shift — yang dilihat dua kasir
// saat serah terima ("berapa yang sudah kamu jual, lewat apa saja").
func ShiftSalesSummary(ctx context.Context, shiftID string) (structs.SaleSummaryResponse, error) {
	return salesSummary(ctx, repositories.SaleScope{ShiftID: shiftID})
}

func salesSummary(ctx context.Context, sc repositories.SaleScope) (structs.SaleSummaryResponse, error) {
	var out structs.SaleSummaryResponse
	c, err := repositories.CountSales(ctx, sc)
	if err != nil {
		return out, err
	}
	out = structs.SaleSummaryResponse{
		SalesCount:    c.SalesCount,
		SalesTotal:    c.SalesTotal,
		ReturnsCount:  c.ReturnsCount,
		ReturnsTotal:  -c.ReturnsTotal,
		CanceledCount: c.CanceledCount,
		CanceledTotal: c.CanceledTotal,
		NetTotal:      c.SalesTotal + c.ReturnsTotal,
		ByMethod:      []structs.SaleMethodTotal{},
	}
	if c.SalesCount > 0 {
		out.AverageSale = c.SalesTotal / c.SalesCount
	}
	cara, err := repositories.SalesByMethod(ctx, sc)
	if err != nil {
		return out, err
	}
	for _, r := range cara {
		out.ByMethod = append(out.ByMethod, structs.SaleMethodTotal{Method: r.Method, Count: r.SalesCount, Amount: r.Amount})
	}
	return out, nil
}
