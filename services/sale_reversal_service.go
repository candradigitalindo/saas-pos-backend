package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/internal/timez"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"gorm.io/gorm"
)

// VoidSale membatalkan penjualan (§13.2): status → 'canceled', gerakan stok
// dibalik dengan baris baru kind='void' (baris asli TIDAK diubah), piutang
// kasbon yang belum dibayar dihapusbukukan.
//
// Menolak bila penjualan bukan 'completed', atau bila kasbonnya sudah sebagian
// dibayar (butuh penanganan manual).
func VoidSale(ctx context.Context, saleID, reason string) (*structs.SaleResponse, error) {
	var result models.Sale
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		if err := voidSaleInTx(ctx, tx, saleID, reason); err != nil {
			return err
		}
		return repositories.FindSaleInTenant(ctx, tx, saleID, &result)
	})
	if err != nil {
		return nil, err
	}
	r := SaleToResponse(&result)
	return &r, nil
}

// voidSaleInTx menjalankan pembatalan penjualan (§13.2) DI DALAM transaksi yang
// sudah ada: status → 'canceled', gerakan stok dibalik dengan baris kind='void',
// kasbon belum-terbayar dihapusbukukan, agregat laporan dihitung ulang. Dipakai
// VoidSale (endpoint kasir) dan pembatalan pesanan kanal (Fase 11a).
func voidSaleInTx(ctx context.Context, tx *gorm.DB, saleID, reason string) error {
	var sale models.Sale
	if err := repositories.FindSaleInTenant(ctx, tx, saleID, &sale); err != nil {
		return err
	}
	if sale.Status != "completed" {
		return fmt.Errorf("%w: hanya transaksi berstatus selesai yang bisa dibatalkan", helpers.ErrConflict)
	}
	if err := writeOffKasbonIfAny(ctx, tx, saleID); err != nil {
		return err
	}
	bizDate, err := saleBusinessDate(ctx, tx, sale.OutletID)
	if err != nil {
		return err
	}
	if err := reverseSaleStock(ctx, tx, saleID, saleID, "void", bizDate); err != nil {
		return err
	}
	if err := repositories.MarkSaleCanceled(ctx, tx, saleID, reason); err != nil {
		return err
	}
	// Setelah statusnya 'canceled', rekalkulasi hari transaksi asal otomatis
	// membuang kontribusinya (§13.2).
	return repositories.RefreshDailySummary(ctx, tx, sale.OutletID, sale.BusinessDate)
}

// RefundSale membuat transaksi RETUR PENUH: sale baru berstatus 'returned'
// dengan nilai negatif yang menunjuk sale asal, plus gerakan stok kind='refund'.
// Penjualan asli tidak diubah.
//
// Idempoten lewat kolom return_of_sale_id: bila retur untuk sale ini sudah ada,
// yang lama dikembalikan.
func RefundSale(ctx context.Context, saleID, reason string) (*structs.SaleResponse, error) {
	var result models.Sale
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		var orig models.Sale
		if err := repositories.FindSaleInTenant(ctx, tx, saleID, &orig); err != nil {
			return err
		}
		if orig.Status != "completed" {
			return fmt.Errorf("%w: hanya transaksi selesai yang bisa diretur", helpers.ErrConflict)
		}

		// Retur yang sudah ada → kembalikan (idempoten).
		var existing models.Sale
		exErr := repositories.FindReturnSale(ctx, tx, saleID, &existing)
		if exErr == nil {
			return repositories.FindSaleInTenant(ctx, tx, existing.ID, &result)
		}
		if !errors.Is(exErr, repositories.ErrSaleNotFound) {
			return exErr
		}

		if err := writeOffKasbonIfAny(ctx, tx, saleID); err != nil {
			return err
		}

		now := time.Now().UTC()
		var outlet models.Outlet
		if err := repositories.FindOutletByID(ctx, tx, orig.OutletID, &outlet); err != nil {
			return err
		}
		bizDate, err := timez.BusinessDate(now, outlet.Timezone, outlet.DayStartOffset())
		if err != nil {
			return err
		}
		seq, err := repositories.NextReceiptSeq(ctx, tx, orig.OutletID, bizDate)
		if err != nil {
			return err
		}

		refOf := orig.ID
		ret := models.Sale{
			OutletID:       orig.OutletID,
			ShiftID:        orig.ShiftID,
			CustomerID:     orig.CustomerID,
			ReceiptNo:      formatReceiptNo(outlet, bizDate, seq),
			IdempotencyKey: "refund:" + orig.ID,
			OrderType:      orig.OrderType,
			Status:         "returned",
			Subtotal:       -orig.Subtotal,
			DiscountAmount: -orig.DiscountAmount,
			TaxAmount:      -orig.TaxAmount,
			ServiceAmount:  -orig.ServiceAmount,
			RoundingAmount: -orig.RoundingAmount,
			Total:          -orig.Total,
			PaidAmount:     -orig.PaidAmount,
			CostTotal:      -orig.CostTotal,
			ReturnOfSaleID: &refOf,
			Note:           strings.TrimSpace("Retur " + orig.ReceiptNo + " " + reason),
			OccurredAt:     now,
			BusinessDate:   bizDate,
			CreatedBy:      reqctx.UserID(ctx),
		}
		for _, it := range orig.Items {
			ret.Items = append(ret.Items, models.SaleItem{
				ProductID:      it.ProductID,
				VariantID:      it.VariantID,
				ProductName:    it.ProductName,
				UnitName:       it.UnitName,
				Qty:            it.Qty, // tetap positif (CHECK qty > 0)
				UnitPrice:      it.UnitPrice,
				UnitCost:       it.UnitCost,
				DiscountAmount: -it.DiscountAmount,
				TaxAmount:      -it.TaxAmount,
				LineTotal:      -it.LineTotal,
				Note:           it.Note,
			})
		}
		if err := repositories.CreateSale(ctx, tx, &ret); err != nil {
			return err
		}
		if err := reverseSaleStock(ctx, tx, orig.ID, ret.ID, "refund", bizDate); err != nil {
			return err
		}
		// Retur bernilai negatif dicatat pada hari retur — hitung ulang agregat
		// hari itu agar omzet & laba turun sesuai (§13.2).
		if err := repositories.RefreshDailySummary(ctx, tx, orig.OutletID, bizDate); err != nil {
			return err
		}
		return repositories.FindSaleInTenant(ctx, tx, ret.ID, &result)
	})
	if err != nil {
		return nil, err
	}
	r := SaleToResponse(&result)
	return &r, nil
}

// writeOffKasbonIfAny menghapusbukukan piutang kasbon dari sebuah penjualan bila
// belum ada pembayaran. Menolak (ErrConflict) bila sudah sebagian dibayar.
func writeOffKasbonIfAny(ctx context.Context, tx *gorm.DB, saleID string) error {
	var rec models.Receivable
	err := repositories.FindReceivableBySource(ctx, tx, "sales", saleID, &rec)
	if errors.Is(err, repositories.ErrReceivableNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if rec.PaidAmount > 0 {
		return fmt.Errorf("%w: kasbon transaksi ini sudah sebagian dibayar", helpers.ErrConflict)
	}
	return tx.WithContext(ctx).Model(&models.Receivable{}).
		Where("tenant_id = ? AND id = ?", reqctx.TenantID(ctx), rec.ID).
		Updates(map[string]any{"status": "written_off", "updated_at": gorm.Expr("now()")}).Error
}

// saleBusinessDate menghitung tanggal usaha outlet sekarang.
func saleBusinessDate(ctx context.Context, tx *gorm.DB, outletID string) (time.Time, error) {
	var outlet models.Outlet
	if err := repositories.FindOutletByID(ctx, tx, outletID, &outlet); err != nil {
		return time.Time{}, err
	}
	return timez.BusinessDate(time.Now().UTC(), outlet.Timezone, outlet.DayStartOffset())
}

// reverseSaleStock menulis gerakan pembalik untuk SETIAP gerakan (kind 'sale'
// maupun 'recipe') milik `origSaleID`, dengan qty_delta berlawanan tanda dan
// kind 'void'/'refund'; RefID gerakan baru = `reversalSaleID`.
func reverseSaleStock(ctx context.Context, tx *gorm.DB, origSaleID, reversalSaleID, newKind string, bizDate time.Time) error {
	orig, err := repositories.MovementsByRef(ctx, tx, "sales", origSaleID, "") // semua kind
	if err != nil {
		return err
	}
	if len(orig) == 0 {
		return nil
	}

	deltas := make([]repositories.StockDelta, 0, len(orig))
	for _, m := range orig {
		variantID := ""
		if m.VariantID != nil {
			variantID = *m.VariantID
		}
		deltas = append(deltas, repositories.StockDelta{
			ProductID: m.ProductID, VariantID: variantID,
			Delta: m.QtyDelta.Neg(), UnitCost: m.UnitCost, Kind: newKind,
		})
	}
	_, err = repositories.ApplyStockDeltas(ctx, tx, orig[0].OutletID, deltas, repositories.MovementMeta{
		Kind: newKind, RefTable: "sales", RefID: reversalSaleID,
		OccurredAt: time.Now().UTC(), BusinessDate: bizDate,
	})
	return err
}
