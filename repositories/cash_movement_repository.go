package repositories

import (
	"context"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

// CreateCashMovement menyimpan kas masuk/keluar non-penjualan. tx opsional.
func CreateCashMovement(ctx context.Context, tx *gorm.DB, row *models.CashMovement) error {
	return createTenant(ctx, tx, row)
}

// ListCashMovements mengembalikan gerakan kas satu shift, berpaginasi.
func ListCashMovements(ctx context.Context, shiftID string, limit, offset int) ([]models.CashMovement, int64, error) {
	where, args := "", []any(nil)
	if shiftID != "" {
		where, args = "shift_id = ?", []any{shiftID}
	}
	return paginateTenant[models.CashMovement](ctx, where, args, "occurred_at DESC, id DESC", limit, offset)
}
