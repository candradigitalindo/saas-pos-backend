package services

import (
	"context"
	"fmt"
	"time"

	"candra/backend-api/helpers"
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

		// Baca saldo saat ini (dikunci) untuk menentukan delta bila new_qty
		// mode, dan kind ('initial' bila baris belum ada).
		locked, err := repositories.LockStocks(ctx, tx, in.OutletID, []string{in.ProductID})
		if err != nil {
			return err
		}
		key := repositories.StockKey{OutletID: in.OutletID, ProductID: in.ProductID, VariantID: in.VariantID}
		cur, existed := locked[key]
		before := decimal.Zero
		if existed {
			before = cur.Qty
		}

		var delta decimal.Decimal
		if in.NewQty != nil {
			delta = in.NewQty.Sub(before)
		} else {
			delta = *in.Delta
		}
		kind := "adjustment"
		if !existed {
			kind = "initial"
		}
		bizDate, err := saleBusinessDate(ctx, tx, in.OutletID)
		if err != nil {
			return err
		}

		moves, err := repositories.ApplyStockDeltas(ctx, tx, in.OutletID,
			[]repositories.StockDelta{{
				ProductID: in.ProductID, VariantID: in.VariantID,
				Delta: delta, UnitCost: prod.CostPrice, Kind: kind,
			}},
			repositories.MovementMeta{
				Kind: kind, Reason: in.Reason,
				OccurredAt: time.Now().UTC(), BusinessDate: bizDate,
			})
		if err != nil {
			return err
		}
		mv = moves[0]
		return nil
	})
	return mv, err
}
