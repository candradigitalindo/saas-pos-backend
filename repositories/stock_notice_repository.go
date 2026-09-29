package repositories

import (
	"context"

	"candra/backend-api/internal/reqctx"
	"candra/backend-api/internal/ulid"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// StockAlert: satu barang yang sedang habis / di bawah batas minimum di satu
// toko — kandidat pengingat stok (cmd/stock-reminders).
type StockAlert struct {
	TenantID     string
	OutletID     string
	OutletName   string
	ProductID    string
	ProductName  string
	UnitName     string
	Qty          decimal.Decimal
	MinStock     decimal.Decimal
	SupplierName string
}

// StockAlertCandidates: barang aktif berlacak stok yang sisanya ≤ batas
// minimum (termasuk habis & minus) LINTAS tenant (atau satu tenant).
func StockAlertCandidates(ctx context.Context, tenantID string) ([]StockAlert, error) {
	q := tenantDB(ctx, nil).Table("stocks s").
		Joins("JOIN products p ON p.tenant_id = s.tenant_id AND p.id = s.product_id AND p.deleted_at IS NULL").
		Joins("JOIN outlets o ON o.tenant_id = s.tenant_id AND o.id = s.outlet_id AND o.deleted_at IS NULL").
		Joins("LEFT JOIN units un ON un.tenant_id = p.tenant_id AND un.id = p.unit_id").
		Joins("LEFT JOIN suppliers sp ON sp.tenant_id = p.tenant_id AND sp.id = p.supplier_id AND sp.deleted_at IS NULL").
		Where("s.variant_id = '' AND p.track_stock AND p.is_active AND s.qty <= p.min_stock")
	if tenantID != "" {
		q = q.Where("s.tenant_id = ?", tenantID)
	}
	var rows []StockAlert
	err := q.Select(`s.tenant_id, s.outlet_id, o.name AS outlet_name, s.product_id, p.name AS product_name,
			COALESCE(un.name, '') AS unit_name, s.qty, p.min_stock, COALESCE(sp.name, '') AS supplier_name`).
		Order("s.tenant_id, o.name, s.qty, p.name").
		Scan(&rows).Error
	return rows, err
}

// ClearRecoveredStockNotices menghapus catatan pengingat tenant konteks untuk
// barang yang sudah pulih — "habis" yang kini ada lagi, "hampir habis" yang
// kini di atas batas — supaya penurunan berikutnya diingatkan lagi.
func ClearRecoveredStockNotices(ctx context.Context, tx *gorm.DB) error {
	return tenantDB(ctx, tx).Exec(`
		DELETE FROM stock_notices n
		USING stocks s, products p
		WHERE n.tenant_id = ? AND s.tenant_id = n.tenant_id AND s.outlet_id = n.outlet_id
			AND s.product_id = n.product_id AND s.variant_id = ''
			AND p.tenant_id = n.tenant_id AND p.id = n.product_id
			AND ((n.kind = 'out' AND s.qty > 0) OR (n.kind = 'low' AND s.qty > p.min_stock))`,
		reqctx.TenantID(ctx)).Error
}

// InsertStockNoticeOnce mencatat pengingat (toko, barang, jenis) sekali saja;
// false bila sudah tercatat.
func InsertStockNoticeOnce(ctx context.Context, tx *gorm.DB, outletID, productID, kind string) (bool, error) {
	var id string
	err := tenantDB(ctx, tx).Raw(`
		INSERT INTO stock_notices (id, tenant_id, outlet_id, product_id, kind) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (tenant_id, outlet_id, product_id, kind) DO NOTHING RETURNING id`,
		ulid.New(), reqctx.TenantID(ctx), outletID, productID, kind).Scan(&id).Error
	return id != "", err
}
