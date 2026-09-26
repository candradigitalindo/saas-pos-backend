package repositories

import (
	"context"
	"errors"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

var ErrPurchaseNotFound = errors.New("pembelian tidak ditemukan")

// CreatePurchase menyimpan pembelian + itemnya (tenant_id di-stempel).
func CreatePurchase(ctx context.Context, tx *gorm.DB, p *models.Purchase) error {
	tid := currentTenantID(ctx)
	p.TenantID = tid
	for i := range p.Items {
		p.Items[i].TenantID = tid
	}
	return tx.WithContext(ctx).Create(p).Error
}

// FindPurchaseInTenant memuat satu pembelian beserta itemnya. tx opsional.
func FindPurchaseInTenant(ctx context.Context, tx *gorm.DB, id string, out *models.Purchase) error {
	err := scopeTenant(ctx, tenantDB(ctx, tx)).Preload("Items").First(out, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrPurchaseNotFound
	}
	return err
}

// ListPurchases mengembalikan satu halaman pembelian (opsional per outlet).
func ListPurchases(ctx context.Context, outletID string, limit, offset int) ([]models.Purchase, int64, error) {
	where, args := "", []any(nil)
	if outletID != "" {
		where, args = "outlet_id = ?", []any{outletID}
	}
	where, args = whereOutlet(ctx, where, args, "outlet_id")
	return paginateTenant[models.Purchase](ctx, where, args, "occurred_at DESC, id DESC", limit, offset)
}

// SetProductLastCost menyetel harga modal produk ke unit_cost pembelian terakhir
// (metode last-cost). Dipakai setelah menerima pembelian.
func SetProductLastCost(ctx context.Context, tx *gorm.DB, productID string, cost int64) error {
	return scopeTenant(ctx, tenantDB(ctx, tx)).
		Model(&models.Product{}).
		Where("id = ?", productID).
		UpdateColumn("cost_price", cost).Error
}
