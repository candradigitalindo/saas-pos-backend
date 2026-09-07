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
//
// TANPA lapis visibilitas — dipakai jalur transaksi internal (checkout, dokumen
// CRM) yang boleh menunjuk pelanggan mana pun di tenant. Untuk endpoint yang
// menghadap pengguna, pakai FindCustomerVisible.
func FindCustomerInTenant(ctx context.Context, tx *gorm.DB, id string, out *models.Customer) error {
	err := firstTenant(ctx, tx, id, out)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrCustomerNotFound
	}
	return err
}

// ListCustomersVisible seperti ListCustomers tetapi juga menerapkan lapis 3
// (visibilitas kepemilikan, §6): sales hanya melihat pelanggan miliknya + yang
// belum ber-owner. Dipakai GET /customers.
func ListCustomersVisible(ctx context.Context, search string, limit, offset int) ([]models.Customer, int64, error) {
	where, args := customerSearch(search)
	build := func() *gorm.DB {
		q := scopeVisibility(ctx, scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.Customer{})), "customers")
		if where != "" {
			q = q.Where(where, args...)
		}
		return q
	}
	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []models.Customer{}, 0, nil
	}
	rows := make([]models.Customer, 0, limit)
	err := build().Order("name ASC, id ASC").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

// FindCustomerVisible memuat satu pelanggan yang TERLIHAT oleh user konteks
// (lapis 1 + lapis 3). Dipakai GET /customers/:id.
func FindCustomerVisible(ctx context.Context, id string, out *models.Customer) error {
	err := scopeVisibility(ctx, scopeTenant(ctx, tenantDB(ctx, nil)), "customers").
		First(out, "customers.id = ?", id).Error
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
