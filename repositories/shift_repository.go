package repositories

import (
	"context"
	"errors"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

var (
	ErrShiftNotFound    = errors.New("shift tidak ditemukan")
	ErrNoOpenShift      = errors.New("belum ada shift terbuka untuk outlet ini")
	ErrShiftAlreadyOpen = errors.New("sudah ada shift terbuka untuk outlet ini")
)

// FindOpenShift memuat shift 'open' untuk sebuah outlet. tx opsional.
// Mengembalikan ErrNoOpenShift bila tidak ada.
func FindOpenShift(ctx context.Context, tx *gorm.DB, outletID string, out *models.Shift) error {
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("outlet_id = ? AND status = 'open'", outletID).
		First(out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNoOpenShift
	}
	return err
}

// FindShiftInTenant memuat satu shift milik tenant konteks. tx opsional.
func FindShiftInTenant(ctx context.Context, tx *gorm.DB, id string, out *models.Shift) error {
	err := firstTenant(ctx, tx, id, out)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrShiftNotFound
	}
	return err
}

// CreateShift menyimpan shift baru. Unique partial index uq_shifts_open_per_outlet
// menjadi pengaman terakhir bila dua permintaan buka-shift bersamaan → error
// duplikat diterjemahkan pemanggil ke ErrShiftAlreadyOpen.
func CreateShift(ctx context.Context, tx *gorm.DB, row *models.Shift) error {
	return createTenant(ctx, tx, row)
}

// UpdateShift menyimpan perubahan shift (dipakai saat menutup). tx opsional.
func UpdateShift(ctx context.Context, tx *gorm.DB, row *models.Shift) error {
	err := updateTenantColumns(ctx, tx, row,
		"closed_by", "closed_at", "expected_cash", "counted_cash", "difference", "note", "status")
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrShiftNotFound
	}
	return err
}

// ListShifts mengambil satu halaman shift milik tenant konteks (opsional per outlet).
func ListShifts(ctx context.Context, outletID string, limit, offset int) ([]models.Shift, int64, error) {
	where, args := "", []any(nil)
	if outletID != "" {
		where, args = "outlet_id = ?", []any{outletID}
	}
	return paginateTenant[models.Shift](ctx, where, args, "opened_at DESC, id DESC", limit, offset)
}

// ShiftCashTotals menghitung komponen kas sebuah shift dari transaksi & gerakan
// kas, untuk menentukan expected_cash saat menutup:
//
//	expected = opening_cash + cash_sales + cash_in - cash_out
//
// cash_sales = Σ sale_payments.amount (method='cash') pada sales berstatus
// 'completed' milik shift ini.
func ShiftCashTotals(ctx context.Context, tx *gorm.DB, shiftID string) (cashSales, cashIn, cashOut int64, err error) {
	tid := currentTenantID(ctx)

	err = tenantDB(ctx, tx).
		Table("sale_payments sp").
		Joins("JOIN sales s ON s.tenant_id = sp.tenant_id AND s.id = sp.sale_id").
		Where("sp.tenant_id = ? AND s.shift_id = ? AND s.status = 'completed' AND sp.method = 'cash'", tid, shiftID).
		Select("COALESCE(SUM(sp.amount), 0)").Scan(&cashSales).Error
	if err != nil {
		return
	}

	err = scopeTenant(ctx, tenantDB(ctx, tx).Model(&models.CashMovement{})).
		Where("shift_id = ? AND direction = 'in'", shiftID).
		Select("COALESCE(SUM(amount), 0)").Scan(&cashIn).Error
	if err != nil {
		return
	}

	err = scopeTenant(ctx, tenantDB(ctx, tx).Model(&models.CashMovement{})).
		Where("shift_id = ? AND direction = 'out'", shiftID).
		Select("COALESCE(SUM(amount), 0)").Scan(&cashOut).Error
	return
}
