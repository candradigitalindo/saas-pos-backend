package repositories

import (
	"context"
	"time"

	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ReturnedQtyByItem: jumlah yang SUDAH diretur per baris nota (satuan beli
// baris itu), untuk membatasi retur berikutnya.
func ReturnedQtyByItem(ctx context.Context, tx *gorm.DB, purchaseID string) (map[string]decimal.Decimal, error) {
	var rows []struct {
		PurchaseItemID string
		Qty            decimal.Decimal
	}
	err := tenantDB(ctx, tx).Raw(`
		SELECT ri.purchase_item_id, SUM(ri.qty) AS qty
		FROM purchase_return_items ri
		JOIN purchase_returns r ON r.tenant_id = ri.tenant_id AND r.id = ri.return_id
		WHERE ri.tenant_id = ? AND r.purchase_id = ?
		GROUP BY ri.purchase_item_id`, reqctx.TenantID(ctx), purchaseID).Scan(&rows).Error
	out := make(map[string]decimal.Decimal, len(rows))
	for _, r := range rows {
		out[r.PurchaseItemID] = r.Qty
	}
	return out, err
}

// CreatePurchaseReturn menyimpan retur beserta barisnya (tenant_id distempel).
func CreatePurchaseReturn(ctx context.Context, tx *gorm.DB, r *models.PurchaseReturn) error {
	items := r.Items
	r.Items = nil
	if err := createTenant(ctx, tx, r); err != nil {
		return err
	}
	tid := currentTenantID(ctx)
	for i := range items {
		items[i].TenantID, items[i].ReturnID = tid, r.ID
	}
	if len(items) > 0 {
		if err := tenantDB(ctx, tx).Create(&items).Error; err != nil {
			return err
		}
	}
	r.Items = items
	return nil
}

// SetPurchaseAfterReturn menyetel total, returned_amount, dan paid_amount
// pembelian (sudah dikunci pemanggil) setelah retur.
func SetPurchaseAfterReturn(ctx context.Context, tx *gorm.DB, id string, total, returned, paid int64) error {
	return scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.Purchase{}).Where("id = ?", id).
		Updates(map[string]any{"total": total, "returned_amount": returned, "paid_amount": paid, "updated_at": gorm.Expr("now()")}).Error
}

// PurchaseReturnRow: satu retur untuk rincian nota.
type PurchaseReturnRow struct {
	ID            string
	Reason        string
	Total         int64
	RefundAmount  int64
	RefundSource  *string
	OccurredAt    time.Time
	CreatedByName string
}

// PurchaseReturnItemRow: satu baris retur dengan nama barang.
type PurchaseReturnItemRow struct {
	ReturnID       string
	PurchaseItemID string
	ProductName    string
	Qty            decimal.Decimal
	UnitName       string
	LineTotal      int64
}

// ListPurchaseReturns: retur satu nota (terlama dulu) beserta barisnya.
func ListPurchaseReturns(ctx context.Context, purchaseID string) ([]PurchaseReturnRow, []PurchaseReturnItemRow, error) {
	tid := reqctx.TenantID(ctx)
	var kepala []PurchaseReturnRow
	if err := tenantDB(ctx, nil).Raw(`
		SELECT r.id, r.reason, r.total, r.refund_amount, r.refund_source, r.occurred_at,
			COALESCE(u.name, '') AS created_by_name
		FROM purchase_returns r
		LEFT JOIN users u ON u.tenant_id = r.tenant_id AND u.id = r.created_by
		WHERE r.tenant_id = ? AND r.purchase_id = ?
		ORDER BY r.occurred_at, r.id`, tid, purchaseID).Scan(&kepala).Error; err != nil {
		return nil, nil, err
	}
	var baris []PurchaseReturnItemRow
	err := tenantDB(ctx, nil).Raw(`
		SELECT ri.return_id, ri.purchase_item_id, p.name AS product_name, ri.qty,
			COALESCE(NULLIF(pi.unit_name, ''), un.name, '') AS unit_name, ri.line_total
		FROM purchase_return_items ri
		JOIN purchase_returns r ON r.tenant_id = ri.tenant_id AND r.id = ri.return_id
		JOIN purchase_items pi ON pi.tenant_id = ri.tenant_id AND pi.id = ri.purchase_item_id
		JOIN products p ON p.tenant_id = ri.tenant_id AND p.id = ri.product_id
		LEFT JOIN units un ON un.tenant_id = p.tenant_id AND un.id = p.unit_id
		WHERE ri.tenant_id = ? AND r.purchase_id = ?
		ORDER BY ri.id`, tid, purchaseID).Scan(&baris).Error
	return kepala, baris, err
}
