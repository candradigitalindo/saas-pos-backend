package repositories

import (
	"context"
	"errors"

	"candra/backend-api/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrOpnameNotFound   = errors.New("stok opname tidak ditemukan")
	ErrTransferNotFound = errors.New("transfer stok tidak ditemukan")
)

// ── stock_opnames ──────────────────────────────────────────────────────────

func CreateOpname(ctx context.Context, tx *gorm.DB, o *models.StockOpname) error {
	return createTenant(ctx, tx, o)
}

func FindOpnameInTenant(ctx context.Context, tx *gorm.DB, id string, out *models.StockOpname) error {
	err := scopeTenant(ctx, tenantDB(ctx, tx)).Preload("Items").First(out, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrOpnameNotFound
	}
	return err
}

func ListOpnames(ctx context.Context, outletID string, limit, offset int) ([]models.StockOpname, int64, error) {
	where, args := "", []any(nil)
	if outletID != "" {
		where, args = "outlet_id = ?", []any{outletID}
	}
	return paginateTenant[models.StockOpname](ctx, where, args, "created_at DESC, id DESC", limit, offset)
}

// UpsertOpnameItem menyisipkan/memperbarui satu baris hitungan opname
// (ON CONFLICT pada (tenant_id, opname_id, product_id, variant_id)).
func UpsertOpnameItem(ctx context.Context, tx *gorm.DB, item *models.StockOpnameItem) error {
	item.TenantID = currentTenantID(ctx)
	return tx.WithContext(ctx). // BeforeCreate meng-generate ID bila kosong

					Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "tenant_id"}, {Name: "opname_id"}, {Name: "product_id"}, {Name: "variant_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"system_qty", "counted_qty", "diff_qty"}),
		}).
		Create(item).Error
}

func SetOpnameStatus(ctx context.Context, tx *gorm.DB, id, status string, countedAt any) error {
	upd := map[string]any{"status": status, "updated_at": gorm.Expr("now()")}
	if countedAt != nil {
		upd["counted_at"] = countedAt
	}
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.StockOpname{}).Where("id = ?", id).Updates(upd)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrOpnameNotFound
	}
	return nil
}

// ── stock_transfers ────────────────────────────────────────────────────────

func CreateTransfer(ctx context.Context, tx *gorm.DB, tr *models.StockTransfer) error {
	tid := currentTenantID(ctx)
	tr.TenantID = tid
	for i := range tr.Items {
		tr.Items[i].TenantID = tid
	}
	return tx.WithContext(ctx).Create(tr).Error
}

func FindTransferInTenant(ctx context.Context, tx *gorm.DB, id string, out *models.StockTransfer) error {
	err := scopeTenant(ctx, tenantDB(ctx, tx)).Preload("Items").First(out, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrTransferNotFound
	}
	return err
}

func ListTransfers(ctx context.Context, outletID string, limit, offset int) ([]models.StockTransfer, int64, error) {
	where, args := "", []any(nil)
	if outletID != "" {
		where = "from_outlet_id = ? OR to_outlet_id = ?"
		args = []any{outletID, outletID}
	}
	return paginateTenant[models.StockTransfer](ctx, where, args, "created_at DESC, id DESC", limit, offset)
}

func SetTransferStatus(ctx context.Context, tx *gorm.DB, id, status string, extra map[string]any) error {
	upd := map[string]any{"status": status, "updated_at": gorm.Expr("now()")}
	for k, v := range extra {
		upd[k] = v
	}
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.StockTransfer{}).Where("id = ?", id).Updates(upd)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrTransferNotFound
	}
	return nil
}
