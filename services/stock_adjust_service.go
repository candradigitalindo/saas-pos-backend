package services

import (
	"context"
	"fmt"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// StockAdjustInput adalah masukan penyesuaian stok.
type StockAdjustInput struct {
	OutletID  string
	ProductID string
	VariantID string // "" = tanpa varian
	NewQty    *decimal.Decimal
	Delta     *decimal.Decimal
	Reason    string
}

// AdjustStock menyetel/menggeser saldo stok satu (outlet, product, variant) dan
// mencatat gerakan kind='adjustment' (atau 'initial' bila saldo awal belum ada),
// dalam satu transaksi dengan penguncian baris. `reason` wajib (§5.6).
//
// Salah satu dari NewQty (set absolut) atau Delta (geser) wajib diisi.
func AdjustStock(ctx context.Context, in StockAdjustInput) (models.StockMovement, error) {
	if in.Reason == "" {
		return models.StockMovement{}, fmt.Errorf("%w: alasan penyesuaian wajib", helpers.ErrValidation)
	}
	if (in.NewQty == nil) == (in.Delta == nil) {
		return models.StockMovement{}, fmt.Errorf("%w: isi salah satu dari new_qty atau delta", helpers.ErrValidation)
	}

	var outlet models.Outlet
	if err := repositories.FindOutletByID(ctx, nil, in.OutletID, &outlet); err != nil {
		return models.StockMovement{}, fmt.Errorf("%w: outlet tidak ditemukan", helpers.ErrValidation)
	}

	var mv models.StockMovement
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		var prod models.Product
		if err := repositories.FindProductInTenant(ctx, tx, in.ProductID, &prod); err != nil {
			return fmt.Errorf("%w: produk tidak ditemukan", helpers.ErrValidation)
		}

		locked, err := repositories.LockStocks(ctx, tx, in.OutletID, []string{in.ProductID})
		if err != nil {
			return err
		}
		key := repositories.StockKey{OutletID: in.OutletID, ProductID: in.ProductID, VariantID: in.VariantID}
		before := decimal.Zero
		_, existed := locked[key]
		if existed {
			before = locked[key].Qty
		}

		var after, delta decimal.Decimal
		if in.NewQty != nil {
			after = *in.NewQty
			delta = after.Sub(before)
		} else {
			delta = *in.Delta
			after = before.Add(delta)
		}

		kind := "adjustment"
		if !existed {
			kind = "initial"
		}
		now := time.Now().UTC()
		bizDate, err := saleBusinessDate(ctx, tx, in.OutletID)
		if err != nil {
			return err
		}
		uid := reqctx.UserID(ctx)

		mv = models.StockMovement{
			OutletID: in.OutletID, ProductID: in.ProductID,
			Kind: kind, QtyDelta: delta, BalanceAfter: after,
			UnitCost: prod.CostPrice, Reason: in.Reason,
			OccurredAt: now, BusinessDate: bizDate,
		}
		if in.VariantID != "" {
			vid := in.VariantID
			mv.VariantID = &vid
		}
		if uid != "" {
			mv.CreatedBy = &uid
		}
		if err := repositories.RecordMovements(ctx, tx, []models.StockMovement{mv}); err != nil {
			return err
		}
		return repositories.UpsertStockQty(ctx, tx, in.OutletID, in.ProductID, in.VariantID, after)
	})
	return mv, err
}
