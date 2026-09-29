package repositories

import (
	"context"
	"errors"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

// ErrSupplierNotFound dikembalikan bila supplier tidak ada di tenant konteks.
var ErrSupplierNotFound = errors.New("supplier tidak ditemukan")

func supplierSearch(search string) (string, []any) {
	if search == "" {
		return "", nil
	}
	p := "%" + escapeLike(search) + "%"
	return "name ILIKE ? OR phone ILIKE ?", []any{p, p}
}

// ListSuppliers mengambil satu halaman supplier milik tenant konteks.
func ListSuppliers(ctx context.Context, search string, limit, offset int) ([]models.Supplier, int64, error) {
	where, args := supplierSearch(search)
	return paginateTenant[models.Supplier](ctx, where, args, "name ASC, id ASC", limit, offset)
}

// FindSupplierInTenant memuat satu supplier milik tenant konteks. tx opsional.
func FindSupplierInTenant(ctx context.Context, tx *gorm.DB, id string, out *models.Supplier) error {
	err := firstTenant(ctx, tx, id, out)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrSupplierNotFound
	}
	return err
}

// CreateSupplier menyimpan supplier baru (TenantID diisi pemanggil). tx opsional.
func CreateSupplier(ctx context.Context, tx *gorm.DB, row *models.Supplier) error {
	return createTenant(ctx, tx, row)
}

// UpdateSupplier menyimpan perubahan atribut supplier. tx opsional.
func UpdateSupplier(ctx context.Context, tx *gorm.DB, row *models.Supplier) error {
	err := updateTenantColumns(ctx, tx, row, "name", "phone", "address", "note")
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrSupplierNotFound
	}
	return err
}

// DeleteSupplier menandai supplier terhapus. Relasi ke purchases (Fase 4) pakai
// ON DELETE RESTRICT untuk hard delete; soft delete di sini tidak memutus data
// pembelian historis.
func DeleteSupplier(ctx context.Context, tx *gorm.DB, id string) error {
	err := softDeleteTenant[models.Supplier](ctx, tx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrSupplierNotFound
	}
	return err
}
