package repositories

import (
	"context"
	"sort"
	"strings"

	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StockKey mengidentifikasi satu saldo stok.
type StockKey struct {
	OutletID, ProductID, VariantID string
}

// LockStocks mengunci (SELECT ... FOR UPDATE) baris stok untuk pasangan
// (outlet, product) yang diberikan, DIURUT product_id MENAIK agar urutan
// penguncian global konsisten dan tidak terjadi deadlock (§7, §13.1 langkah 2).
//
// Baris yang belum ada TIDAK dibuat di sini — pemanggil menganggap saldo 0.
// Mengembalikan peta dari (outlet|product|variant) ke Stock.
func LockStocks(ctx context.Context, tx *gorm.DB, outletID string, productIDsSortedAsc []string) (map[StockKey]models.Stock, error) {
	out := map[StockKey]models.Stock{}
	if len(productIDsSortedAsc) == 0 {
		return out, nil
	}

	var rows []models.Stock
	err := tx.WithContext(ctx).
		Clauses(lockForUpdate()).
		Where("tenant_id = ? AND outlet_id = ? AND product_id IN ?",
			reqctx.TenantID(ctx), outletID, productIDsSortedAsc).
		Order("product_id ASC, variant_id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[StockKey{r.OutletID, r.ProductID, r.VariantID}] = r
	}
	return out, nil
}

// EnsureStockRows memastikan baris saldo `stocks` ADA untuk setiap key (dibuat
// dengan qty 0 bila belum), SEBELUM LockStocks dipanggil. Mengembalikan key
// yang baru dibuat oleh panggilan ini.
//
// Kenapa perlu: SELECT ... FOR UPDATE hanya bisa mengunci baris yang sudah
// ada. Untuk barang yang belum pernah bergerak, dua transaksi bersamaan
// sama-sama "mengunci" nol baris, sama-sama menganggap saldo awal 0, lalu
// UpsertStockQty yang kedua menimpa hasil yang pertama — 12 pembelian
// bersamaan pernah berakhir dengan stok 1. INSERT ... ON CONFLICT DO NOTHING
// menunggu transaksi lain yang sedang menyisipkan key yang sama selesai,
// sehingga LockStocks sesudahnya selalu menemukan baris untuk dikunci dan
// membaca saldo yang sudah di-commit.
//
// Disisipkan URUT (product_id, variant_id) — urutan yang sama dengan
// LockStocks — supaya dua transaksi tidak saling menunggu dalam lingkaran.
func EnsureStockRows(ctx context.Context, tx *gorm.DB, outletID string, keys []StockKey) (map[StockKey]bool, error) {
	created := map[StockKey]bool{}
	if len(keys) == 0 {
		return created, nil
	}
	uniq := make(map[StockKey]struct{}, len(keys))
	sorted := make([]StockKey, 0, len(keys))
	for _, k := range keys {
		k.OutletID = outletID
		if _, ok := uniq[k]; ok {
			continue
		}
		uniq[k] = struct{}{}
		sorted = append(sorted, k)
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].ProductID != sorted[j].ProductID {
			return sorted[i].ProductID < sorted[j].ProductID
		}
		return sorted[i].VariantID < sorted[j].VariantID
	})

	tid := reqctx.TenantID(ctx)
	var sb strings.Builder
	args := make([]any, 0, len(sorted)*4)
	sb.WriteString("INSERT INTO stocks (tenant_id, outlet_id, product_id, variant_id) VALUES ")
	for i, k := range sorted {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("(?, ?, ?, ?)")
		args = append(args, tid, outletID, k.ProductID, k.VariantID)
	}
	sb.WriteString(" ON CONFLICT (tenant_id, outlet_id, product_id, variant_id) DO NOTHING" +
		" RETURNING product_id, variant_id")

	var rows []struct {
		ProductID string
		VariantID string
	}
	if err := tx.WithContext(ctx).Raw(sb.String(), args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		created[StockKey{OutletID: outletID, ProductID: r.ProductID, VariantID: r.VariantID}] = true
	}
	return created, nil
}

// UpsertStockQty menyetel saldo cache stok ke qty (hasil hitung dari
// balance_after gerakan terakhir). ON CONFLICT pada PK komposit → qty ditimpa,
// updated_at diperbarui ke waktu server.
func UpsertStockQty(ctx context.Context, tx *gorm.DB, outletID, productID, variantID string, qty decimal.Decimal) error {
	row := models.Stock{
		TenantID:  reqctx.TenantID(ctx),
		OutletID:  outletID,
		ProductID: productID,
		VariantID: variantID,
		Qty:       qty,
	}
	return tx.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "tenant_id"}, {Name: "outlet_id"}, {Name: "product_id"}, {Name: "variant_id"},
			},
			DoUpdates: clause.Assignments(map[string]any{
				"qty":        gorm.Expr("EXCLUDED.qty"),
				"updated_at": gorm.Expr("now()"),
			}),
		}).
		Create(&row).Error
}

// RecordMovements menyisipkan gerakan stok (append-only). tenant_id di-stempel
// dari context — pemanggil (service) tidak perlu mengisinya di tiap baris.
func RecordMovements(ctx context.Context, tx *gorm.DB, moves []models.StockMovement) error {
	if len(moves) == 0 {
		return nil
	}
	tid := currentTenantID(ctx)
	for i := range moves {
		moves[i].TenantID = tid
	}
	return tx.WithContext(ctx).Create(&moves).Error
}

// StockQtyMap mengembalikan saldo cache banyak barang sekaligus untuk satu
// outlet, dalam SATU query (bukan CurrentStockQty per barang). Key yang tidak
// punya baris tidak ada di peta — pemanggil menganggapnya 0. tx opsional.
func StockQtyMap(ctx context.Context, tx *gorm.DB, outletID string, productIDs []string) (map[StockKey]decimal.Decimal, error) {
	out := map[StockKey]decimal.Decimal{}
	if len(productIDs) == 0 {
		return out, nil
	}
	var rows []models.Stock
	if err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("outlet_id = ? AND product_id IN ?", outletID, productIDs).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[StockKey{OutletID: r.OutletID, ProductID: r.ProductID, VariantID: r.VariantID}] = r.Qty
	}
	return out, nil
}

// CurrentStockQty mengembalikan saldo cache untuk satu (outlet, product, variant),
// 0 bila belum ada baris. tx opsional.
func CurrentStockQty(ctx context.Context, tx *gorm.DB, outletID, productID, variantID string) (decimal.Decimal, error) {
	var row models.Stock
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("outlet_id = ? AND product_id = ? AND variant_id = ?", outletID, productID, variantID).
		First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return decimal.Zero, nil
	}
	if err != nil {
		return decimal.Zero, err
	}
	return row.Qty, nil
}

// ListStocks mengembalikan saldo stok satu outlet (opsional hanya yang di bawah
// min_stock), berpaginasi, dengan nama produk & satuan.
func ListStocks(ctx context.Context, outletID string, lowOnly bool, limit, offset int) ([]StockRow, int64, error) {
	// build menyusun query dasar (JOIN produk & satuan) yang sama untuk count
	// maupun ambil-halaman.
	build := func() *gorm.DB {
		q := tenantDB(ctx, nil).
			Table("stocks").
			Joins("JOIN products p ON p.tenant_id = stocks.tenant_id AND p.id = stocks.product_id AND p.deleted_at IS NULL").
			Joins("JOIN units u ON u.tenant_id = p.tenant_id AND u.id = p.unit_id").
			Where("stocks.tenant_id = ?", reqctx.TenantID(ctx))
		if outletID != "" {
			q = q.Where("stocks.outlet_id = ?", outletID)
		}
		q = scopeOutlet(ctx, q, "stocks.outlet_id")
		if lowOnly {
			q = q.Where("stocks.qty <= p.min_stock")
		}
		return q
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []StockRow{}, 0, nil
	}

	var rows []StockRow
	err := build().
		Select("stocks.outlet_id, stocks.product_id, stocks.variant_id, stocks.qty, stocks.reserved_qty, p.name AS product_name, p.min_stock, u.name AS unit_name").
		Order("p.name ASC, stocks.product_id ASC").
		Limit(limit).Offset(offset).
		Scan(&rows).Error
	return rows, total, err
}

// StockRow adalah baris saldo stok yang diperkaya untuk ditampilkan.
type StockRow struct {
	OutletID    string          `json:"outlet_id"`
	ProductID   string          `json:"product_id"`
	VariantID   string          `json:"variant_id"`
	Qty         decimal.Decimal `json:"qty"`
	ReservedQty decimal.Decimal `json:"reserved_qty"`
	ProductName string          `json:"product_name"`
	MinStock    decimal.Decimal `json:"min_stock"`
	UnitName    string          `json:"unit_name"`
}

// MovementsByRef mengembalikan gerakan stok yang dihasilkan sebuah dokumen
// (mis. semua kind='sale' dari satu penjualan), untuk dibalik saat void/refund.
// tx opsional.
func MovementsByRef(ctx context.Context, tx *gorm.DB, refTable, refID, kind string) ([]models.StockMovement, error) {
	var rows []models.StockMovement
	q := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("ref_table = ? AND ref_id = ?", refTable, refID)
	if kind != "" {
		q = q.Where("kind = ?", kind)
	}
	err := q.Order("product_id ASC").Find(&rows).Error
	return rows, err
}

// ListStockMovements mengembalikan kartu stok satu produk (opsional per outlet),
// terbaru dulu, berpaginasi.
func ListStockMovements(ctx context.Context, productID, outletID string, limit, offset int) ([]models.StockMovement, int64, error) {
	where := "product_id = ?"
	args := []any{productID}
	if outletID != "" {
		where += " AND outlet_id = ?"
		args = append(args, outletID)
	}
	where, args = whereOutlet(ctx, where, args, "outlet_id")
	return paginateTenant[models.StockMovement](ctx, where, args, "occurred_at DESC, id DESC", limit, offset)
}
