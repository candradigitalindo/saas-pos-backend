package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/internal/timez"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const idempotencyScopePurchase = "purchase.create"

// PurchaseItemInput satu baris penerimaan.
type PurchaseItemInput struct {
	ProductID     string
	VariantID     string
	ProductUnitID string // "" = satuan dasar; selain itu kemasan (qty & harga per kemasan)
	Qty           decimal.Decimal
	UnitCost      int64
}

// PurchaseInput masukan penerimaan barang.
type PurchaseInput struct {
	OutletID       string
	SupplierID     string
	InvoiceNo      string
	DiscountAmount int64
	TaxAmount      int64
	PaidAmount     int64
	DueDate        *time.Time
	Items          []PurchaseItemInput
	IdempotencyKey string
	RequestHash    string
}

// ReceivePurchase mencatat penerimaan barang (§5.6): satu transaksi menulis
// purchase + item + gerakan stok kind='purchase' + memperbarui harga modal
// produk (last-cost). Idempoten lewat Idempotency-Key.
func ReceivePurchase(ctx context.Context, in PurchaseInput) (int, []byte, error) {
	if in.IdempotencyKey == "" {
		return 0, nil, fmt.Errorf("%w: header Idempotency-Key wajib", helpers.ErrValidation)
	}
	if len(in.Items) == 0 {
		return 0, nil, fmt.Errorf("%w: tidak ada item", helpers.ErrValidation)
	}
	for _, it := range in.Items {
		if it.Qty.LessThanOrEqual(decimal.Zero) || it.UnitCost < 0 {
			return 0, nil, fmt.Errorf("%w: qty harus > 0 dan harga modal ≥ 0", helpers.ErrValidation)
		}
	}

	if err := ensureOutletAccess(ctx, in.OutletID); err != nil {
		return 0, nil, err
	}
	var outlet models.Outlet
	if err := repositories.FindOutletByID(ctx, nil, in.OutletID, &outlet); err != nil {
		return 0, nil, fmt.Errorf("%w: outlet tidak ditemukan", helpers.ErrValidation)
	}
	now := time.Now().UTC()
	bizDate, err := timez.BusinessDate(now, outlet.Timezone, outlet.DayStartOffset())
	if err != nil {
		return 0, nil, err
	}

	var (
		outStatus int
		outBody   []byte
	)
	txErr := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		m, err := repositories.LookupIdempotency(ctx, tx, idempotencyScopePurchase, in.IdempotencyKey, in.RequestHash)
		if err != nil {
			return err
		}
		if m.Found {
			if !m.SameRequest {
				return fmt.Errorf("%w: Idempotency-Key sudah dipakai untuk permintaan berbeda", helpers.ErrConflict)
			}
			outStatus, outBody = m.ResponseStatus, m.ResponseBody
			return nil
		}

		ids := uniqueProductIDs(in.Items)
		products, err := repositories.ProductsByIDs(ctx, tx, ids)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if _, ok := products[id]; !ok {
				return fmt.Errorf("%w: produk %s tidak ditemukan", helpers.ErrValidation, id)
			}
		}
		var idKemasan []string
		for _, it := range in.Items {
			if it.ProductUnitID != "" {
				idKemasan = append(idKemasan, it.ProductUnitID)
			}
		}
		kemasan, err := repositories.ProductUnitsByIDs(ctx, tx, idKemasan)
		if err != nil {
			return err
		}
		if in.SupplierID != "" {
			var sup models.Supplier
			if err := repositories.FindSupplierInTenant(ctx, tx, in.SupplierID, &sup); err != nil {
				return fmt.Errorf("%w: supplier tidak ditemukan", helpers.ErrValidation)
			}
		}

		p := models.Purchase{
			OutletID:       in.OutletID,
			InvoiceNo:      in.InvoiceNo,
			IdempotencyKey: in.IdempotencyKey,
			Status:         "received",
			DiscountAmount: in.DiscountAmount,
			TaxAmount:      in.TaxAmount,
			PaidAmount:     in.PaidAmount,
			DueDate:        in.DueDate,
			OccurredAt:     now,
			BusinessDate:   bizDate,
			CreatedBy:      reqctx.UserID(ctx),
		}
		if in.SupplierID != "" {
			p.SupplierID = &in.SupplierID
		}

		deltas := make([]repositories.StockDelta, 0, len(in.Items))
		modalDasar := map[string]int64{} // harga beli terakhir per SATUAN DASAR
		for _, it := range in.Items {
			lineTotal := helpers.LineAmount(it.Qty, it.UnitCost)
			p.Subtotal += lineTotal
			// Kemasan: "5 dus @ 120.000" → stok +5×isi satuan dasar, modal
			// per satuan dasar = harga dus ÷ isi (dibulatkan ke rupiah).
			konversi, namaSatuan := decimal.NewFromInt(1), ""
			if pr := products[it.ProductID]; pr.Unit != nil {
				namaSatuan = pr.Unit.Name
			}
			if it.ProductUnitID != "" {
				k, ok := kemasan[it.ProductUnitID]
				if !ok || k.ProductID != it.ProductID {
					return fmt.Errorf("%w: kemasan %s tidak cocok dengan produk", helpers.ErrValidation, it.ProductUnitID)
				}
				konversi = k.Conversion
				if k.Unit != nil {
					namaSatuan = k.Unit.Name
				}
			}
			biayaDasar := decimal.NewFromInt(it.UnitCost).Div(konversi).Round(0).IntPart()
			item := models.PurchaseItem{
				ProductID: it.ProductID, Qty: it.Qty, UnitCost: it.UnitCost, LineTotal: lineTotal,
				UnitConversion: konversi, UnitName: namaSatuan,
			}
			if it.VariantID != "" {
				vid := it.VariantID
				item.VariantID = &vid
			}
			if it.ProductUnitID != "" {
				kid := it.ProductUnitID
				item.ProductUnitID = &kid
			}
			p.Items = append(p.Items, item)
			deltas = append(deltas, repositories.StockDelta{
				ProductID: it.ProductID, VariantID: it.VariantID,
				Delta: it.Qty.Mul(konversi), UnitCost: biayaDasar,
			})
			modalDasar[it.ProductID] = biayaDasar
		}
		p.Total = p.Subtotal - p.DiscountAmount + p.TaxAmount

		if err := repositories.CreatePurchase(ctx, tx, &p); err != nil {
			return err
		}
		if _, err := repositories.ApplyStockDeltas(ctx, tx, in.OutletID, deltas, repositories.MovementMeta{
			Kind: "purchase", RefTable: "purchases", RefID: p.ID, OccurredAt: now, BusinessDate: bizDate,
		}); err != nil {
			return err
		}
		// Harga modal produk = harga beli terakhir (last-cost).
		for id, biaya := range modalDasar {
			if err := repositories.SetProductLastCost(ctx, tx, id, biaya); err != nil {
				return err
			}
		}

		body, err := json.Marshal(structs.SuccessResponse[structs.PurchaseResponse]{
			Success: true, Message: "Pembelian dicatat", Data: PurchaseToResponse(&p),
		})
		if err != nil {
			return err
		}
		if err := repositories.SaveIdempotency(ctx, tx, idempotencyScopePurchase, in.IdempotencyKey, in.RequestHash,
			http.StatusCreated, body, idempotencyTTL()); err != nil {
			return err
		}
		outStatus, outBody = http.StatusCreated, body
		return nil
	})
	if txErr != nil {
		return 0, nil, txErr
	}
	return outStatus, outBody, nil
}

func uniqueProductIDs(items []PurchaseItemInput) []string {
	set := map[string]struct{}{}
	for _, it := range items {
		set[it.ProductID] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return out
}
