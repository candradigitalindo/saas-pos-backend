package repositories

import (
	"context"
	"errors"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

var ErrReceivableNotFound = errors.New("piutang tidak ditemukan")

// CreateReceivable menyimpan piutang baru (dari kasbon / invoice). tx opsional.
func CreateReceivable(ctx context.Context, tx *gorm.DB, row *models.Receivable) error {
	return createTenant(ctx, tx, row)
}

// FindReceivableInTenant memuat satu piutang. tx opsional.
func FindReceivableInTenant(ctx context.Context, tx *gorm.DB, id string, out *models.Receivable) error {
	err := firstTenant(ctx, tx, id, out)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrReceivableNotFound
	}
	return err
}

// FindReceivableBySource memuat piutang berdasarkan sumbernya (mis. sale id).
// Dipakai saat void/refund penjualan kasbon.
func FindReceivableBySource(ctx context.Context, tx *gorm.DB, sourceTable, sourceID string, out *models.Receivable) error {
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("source_table = ? AND source_id = ?", sourceTable, sourceID).
		First(out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrReceivableNotFound
	}
	return err
}

// ListReceivables mengembalikan satu halaman piutang (opsional per pelanggan /
// status), berpaginasi.
func ListReceivables(ctx context.Context, customerID, status string, limit, offset int) ([]models.Receivable, int64, error) {
	conds, args := []string{}, []any{}
	if customerID != "" {
		conds, args = append(conds, "customer_id = ?"), append(args, customerID)
	}
	if status != "" {
		conds, args = append(conds, "status = ?"), append(args, status)
	}
	where := ""
	for i, c := range conds {
		if i > 0 {
			where += " AND "
		}
		where += c
	}
	return paginateTenant[models.Receivable](ctx, where, args, "created_at DESC, id DESC", limit, offset)
}

// AddReceivablePayment mencatat pembayaran cicilan dan memperbarui
// paid_amount/status piutang, dalam satu transaksi. Menolak bila amount melebihi
// sisa (ErrOverpay).
func AddReceivablePayment(ctx context.Context, pay *models.ReceivablePayment) (models.Receivable, error) {
	var rec models.Receivable
	err := WithTenant(ctx, func(tx *gorm.DB) error {
		if err := scopeTenant(ctx, tx).Clauses(lockForUpdate()).
			First(&rec, "id = ?", pay.ReceivableID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrReceivableNotFound
			}
			return err
		}
		if rec.Status == "paid" || rec.Status == "written_off" {
			return ErrReceivableSettled
		}
		if pay.Amount > rec.Outstanding() {
			return ErrOverpay
		}

		pay.TenantID = currentTenantID(ctx)
		if err := tx.WithContext(ctx).Create(pay).Error; err != nil {
			return err
		}

		newPaid := rec.PaidAmount + pay.Amount
		newStatus := "partial"
		if newPaid >= rec.Amount {
			newStatus = "paid"
		}
		rec.PaidAmount, rec.Status = newPaid, newStatus
		return scopeTenant(ctx, tx).Model(&models.Receivable{}).
			Where("id = ?", rec.ID).
			Updates(map[string]any{"paid_amount": newPaid, "status": newStatus, "updated_at": gorm.Expr("now()")}).Error
	})
	return rec, err
}

var (
	ErrReceivableSettled = errors.New("piutang sudah lunas atau dihapusbukukan")
	ErrOverpay           = errors.New("nominal pembayaran melebihi sisa piutang")
)
