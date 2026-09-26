package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

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

	IdempotencyKey string // dari header Idempotency-Key (wajib, §8)
	RequestHash    string // sha256 dari body mentah
}

// idempotencyScopeStockAdjust untuk penyesuaian stok.
const idempotencyScopeStockAdjust = "stock.adjust"

// AdjustStock menyetel/menggeser saldo stok satu (outlet, product, variant) dan
// mencatat gerakan kind='adjustment' (atau 'initial' bila saldo awal belum ada),
// dalam satu transaksi dengan penguncian baris. `reason` wajib (§5.6).
//
// Salah satu dari NewQty (set absolut) atau Delta (geser) wajib diisi.
//
// Idempoten lewat Idempotency-Key (§8). Mode `delta` paling membutuhkannya:
// "tambah 10" yang terkirim dua kali (klik ganda, kirim ulang setelah sinyal
// putus) dulu menambah 20. Mengembalikan (status HTTP, body JSON siap kirim).
func AdjustStock(ctx context.Context, in StockAdjustInput) (int, []byte, error) {
	if in.Reason == "" {
		return 0, nil, fmt.Errorf("%w: alasan penyesuaian wajib", helpers.ErrValidation)
	}
	if (in.NewQty == nil) == (in.Delta == nil) {
		return 0, nil, fmt.Errorf("%w: isi salah satu dari new_qty atau delta", helpers.ErrValidation)
	}

	if err := ensureOutletAccess(ctx, in.OutletID); err != nil {
		return 0, nil, err
	}
	var outlet models.Outlet
	if err := repositories.FindOutletByID(ctx, nil, in.OutletID, &outlet); err != nil {
		return 0, nil, fmt.Errorf("%w: outlet tidak ditemukan", helpers.ErrValidation)
	}

	status, body, _, err := jalankanIdempoten(ctx, idempotencyScopeStockAdjust, in.IdempotencyKey, in.RequestHash,
		func(tx *gorm.DB) (int, []byte, error) {
			mv, err := adjustStockInTx(ctx, tx, in)
			if err != nil {
				return 0, nil, err
			}
			body, err := json.Marshal(structs.SuccessResponse[structs.StockMovementResponse]{
				Success: true, Message: "Stok disesuaikan", Data: StockMovementToResponse(mv),
			})
			return http.StatusCreated, body, err
		})
	return status, body, err
}

// adjustStockInTx mengerjakan penyesuaian stok di dalam tx pemanggil.
func adjustStockInTx(ctx context.Context, tx *gorm.DB, in StockAdjustInput) (models.StockMovement, error) {
	var mv models.StockMovement
	err := func() error {
		var prod models.Product
		if err := repositories.FindProductInTenant(ctx, tx, in.ProductID, &prod); err != nil {
			return fmt.Errorf("%w: produk tidak ditemukan", helpers.ErrValidation)
		}

		// Baca saldo saat ini (dikunci) untuk menentukan delta bila new_qty
		// mode, dan kind ('initial' bila baris belum ada). Barisnya dipastikan
		// ada DULU supaya benar-benar terkunci — tanpa itu penyesuaian
		// "setel ke N" yang bersamaan dengan barang masuk pertama menghitung
		// delta dari saldo 0 yang sudah basi.
		key := repositories.StockKey{OutletID: in.OutletID, ProductID: in.ProductID, VariantID: in.VariantID}
		created, err := repositories.EnsureStockRows(ctx, tx, in.OutletID, []repositories.StockKey{key})
		if err != nil {
			return err
		}
		locked, err := repositories.LockStocks(ctx, tx, in.OutletID, []string{in.ProductID})
		if err != nil {
			return err
		}
		existed := !created[key]
		before := decimal.Zero
		if cur, ok := locked[key]; ok {
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
	}()
	return mv, err
}
