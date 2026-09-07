package repositories

import (
	"context"
	"sort"
	"time"

	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// StockDelta adalah satu perubahan saldo yang akan ditulis ke buku besar.
// Kind opsional — bila kosong memakai MovementMeta.Kind. Berguna saat satu
// operasi menulis beberapa jenis gerakan sekaligus (mis. checkout: 'sale' untuk
// produk jadi + 'recipe' untuk bahan baku) dalam satu penguncian.
type StockDelta struct {
	ProductID string
	VariantID string // "" = tanpa varian
	Delta     decimal.Decimal
	UnitCost  int64
	Kind      string
}

// MovementMeta melekat pada semua gerakan yang dihasilkan satu operasi.
type MovementMeta struct {
	Kind         string // 'sale','void','refund','purchase','opname','recipe','adjustment','initial','transfer_in','transfer_out'
	RefTable     string
	RefID        string
	Reason       string
	OccurredAt   time.Time
	BusinessDate time.Time
}

// ApplyStockDeltas menerapkan sekumpulan perubahan saldo untuk satu outlet dalam
// satu transaksi: kunci baris stok (URUT product_id MENAIK — anti-deadlock §7),
// hitung balance_after SECARA BERURUTAN (dua delta pada produk yang sama
// menumpuk dengan benar), tulis stock_movements (append-only), lalu samakan
// cache `stocks`.
//
// Saldo minus diizinkan (§13.1) — pemanggil yang memutuskan menolaknya.
// Mengembalikan gerakan yang tercatat (untuk response / audit).
func ApplyStockDeltas(ctx context.Context, tx *gorm.DB, outletID string, deltas []StockDelta, meta MovementMeta) ([]models.StockMovement, error) {
	if len(deltas) == 0 {
		return nil, nil
	}

	// Kumpulkan & urutkan product_id unik untuk penguncian.
	idSet := map[string]struct{}{}
	for _, d := range deltas {
		idSet[d.ProductID] = struct{}{}
	}
	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	locked, err := LockStocks(ctx, tx, outletID, ids)
	if err != nil {
		return nil, err
	}

	// Saldo berjalan, dibibit dari yang terkunci.
	running := map[StockKey]decimal.Decimal{}
	for k, s := range locked {
		running[k] = s.Qty
	}

	uid := reqctx.UserID(ctx)
	moves := make([]models.StockMovement, 0, len(deltas))
	touched := map[StockKey]decimal.Decimal{}

	for _, d := range deltas {
		key := StockKey{OutletID: outletID, ProductID: d.ProductID, VariantID: d.VariantID}
		before, ok := running[key]
		if !ok {
			before = decimal.Zero
		}
		after := before.Add(d.Delta)
		running[key] = after
		touched[key] = after

		kind := d.Kind
		if kind == "" {
			kind = meta.Kind
		}
		mv := models.StockMovement{
			OutletID: outletID, ProductID: d.ProductID,
			Kind: kind, QtyDelta: d.Delta, BalanceAfter: after,
			UnitCost: d.UnitCost, Reason: meta.Reason,
			RefTable: meta.RefTable, OccurredAt: meta.OccurredAt, BusinessDate: meta.BusinessDate,
		}
		if meta.RefID != "" {
			ref := meta.RefID
			mv.RefID = &ref
		}
		if d.VariantID != "" {
			v := d.VariantID
			mv.VariantID = &v
		}
		if uid != "" {
			mv.CreatedBy = &uid
		}
		moves = append(moves, mv)
	}

	if err := RecordMovements(ctx, tx, moves); err != nil {
		return nil, err
	}
	// Samakan cache sekali per (product,variant) yang tersentuh.
	for key, qty := range touched {
		if err := UpsertStockQty(ctx, tx, outletID, key.ProductID, key.VariantID, qty); err != nil {
			return nil, err
		}
	}
	return moves, nil
}

// ReconcileStocks membangun ulang cache `stocks` satu outlet dari SUM(qty_delta)
// seluruh stock_movements (§5.6). Dipakai perintah rekonsiliasi manual & job
// terjadwal. Mengembalikan jumlah baris (product,variant) yang disamakan.
func ReconcileStocks(ctx context.Context, outletID string) (int64, error) {
	tid := currentTenantID(ctx)
	var affected int64
	err := WithTenant(ctx, func(tx *gorm.DB) error {
		res := tx.WithContext(ctx).Exec(`
			INSERT INTO stocks (tenant_id, outlet_id, product_id, variant_id, qty, updated_at)
			SELECT tenant_id, outlet_id, product_id, COALESCE(variant_id, ''), SUM(qty_delta), now()
			FROM stock_movements
			WHERE tenant_id = ? AND outlet_id = ?
			GROUP BY tenant_id, outlet_id, product_id, COALESCE(variant_id, '')
			ON CONFLICT (tenant_id, outlet_id, product_id, variant_id)
			DO UPDATE SET qty = EXCLUDED.qty, updated_at = now()`,
			tid, outletID)
		affected = res.RowsAffected
		return res.Error
	})
	return affected, err
}
