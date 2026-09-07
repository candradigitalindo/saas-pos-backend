package repositories

import (
	"context"
	"errors"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

var ErrCustomerNotFound = errors.New("pelanggan tidak ditemukan")

func customerSearch(search string) (string, []any) {
	if search == "" {
		return "", nil
	}
	p := "%" + escapeLike(search) + "%"
	return "name ILIKE ? OR phone ILIKE ? OR code ILIKE ?", []any{p, p, p}
}

// ListCustomers mengambil satu halaman pelanggan milik tenant konteks.
func ListCustomers(ctx context.Context, search string, limit, offset int) ([]models.Customer, int64, error) {
	where, args := customerSearch(search)
	return paginateTenant[models.Customer](ctx, where, args, "name ASC, id ASC", limit, offset)
}

// FindCustomerInTenant memuat satu pelanggan milik tenant konteks. tx opsional.
func FindCustomerInTenant(ctx context.Context, tx *gorm.DB, id string, out *models.Customer) error {
	err := firstTenant(ctx, tx, id, out)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrCustomerNotFound
	}
	return err
}

// CreateCustomer menyimpan pelanggan baru (TenantID diisi pemanggil). tx opsional.
func CreateCustomer(ctx context.Context, tx *gorm.DB, row *models.Customer) error {
	return createTenant(ctx, tx, row)
}

// UpdateCustomer menyimpan perubahan atribut pelanggan. tx opsional.
func UpdateCustomer(ctx context.Context, tx *gorm.DB, row *models.Customer) error {
	err := updateTenantColumns(ctx, tx, row,
		"code", "name", "phone", "email", "address", "type",
		"price_list_id", "owner_id", "credit_limit", "latitude", "longitude", "note")
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrCustomerNotFound
	}
	return err
}

// DeleteCustomer menandai pelanggan terhapus (soft delete). tx opsional.
func DeleteCustomer(ctx context.Context, tx *gorm.DB, id string) error {
	err := softDeleteTenant[models.Customer](ctx, tx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrCustomerNotFound
	}
	return err
}

// OutstandingReceivableTotal mengembalikan total sisa piutang seorang pelanggan
// (untuk cek batas kredit sebelum menerima kasbon baru). tx opsional.
func OutstandingReceivableTotal(ctx context.Context, tx *gorm.DB, customerID string) (int64, error) {
	var total int64
	err := scopeTenant(ctx, tenantDB(ctx, tx).Model(&models.Receivable{})).
		Where("customer_id = ? AND status IN ?", customerID, []string{"open", "partial"}).
		Select("COALESCE(SUM(amount - paid_amount), 0)").
		Scan(&total).Error
	return total, err
}
