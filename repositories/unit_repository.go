package repositories

import (
	"context"
	"errors"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

// ErrUnitNotFound dikembalikan bila satuan tidak ada di tenant konteks.
var ErrUnitNotFound = errors.New("satuan tidak ditemukan")

// ErrUnitInUse dikembalikan saat satuan masih dipakai produk atau satuan turunan.
var ErrUnitInUse = errors.New("satuan masih dipakai produk atau satuan turunan")

func unitSearch(search string) (string, []any) {
	if search == "" {
		return "", nil
	}
	return "name ILIKE ?", []any{"%" + escapeLike(search) + "%"}
}

// ListUnits mengambil satu halaman satuan milik tenant konteks.
func ListUnits(ctx context.Context, search string, limit, offset int) ([]models.Unit, int64, error) {
	where, args := unitSearch(search)
	return paginateTenant[models.Unit](ctx, where, args, "name ASC, id ASC", limit, offset)
}

// FindUnitInTenant memuat satu satuan milik tenant konteks. tx opsional.
func FindUnitInTenant(ctx context.Context, tx *gorm.DB, id string, out *models.Unit) error {
	err := firstTenant(ctx, tx, id, out)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrUnitNotFound
	}
	return err
}

// FindUnitByName memuat satuan berdasarkan nama persis (dipakai impor CSV yang
// menyebut satuan dengan nama). Mengembalikan ErrUnitNotFound bila tidak ada.
func FindUnitByName(ctx context.Context, tx *gorm.DB, name string, out *models.Unit) error {
	err := scopeTenant(ctx, tenantDB(ctx, tx)).First(out, "name = ?", name).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrUnitNotFound
	}
	return err
}

// CreateUnit menyimpan satuan baru (TenantID diisi pemanggil). tx opsional.
func CreateUnit(ctx context.Context, tx *gorm.DB, row *models.Unit) error {
	return createTenant(ctx, tx, row)
}

// UpdateUnit menyimpan perubahan atribut satuan. tx opsional.
func UpdateUnit(ctx context.Context, tx *gorm.DB, row *models.Unit) error {
	err := updateTenantColumns(ctx, tx, row, "name", "base_unit_id", "conversion", "allow_decimal")
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrUnitNotFound
	}
	return err
}

// DeleteUnit menandai satuan terhapus. Menolak bila masih dipakai produk atau
// satuan turunan.
func DeleteUnit(ctx context.Context, id string) error {
	return WithTenant(ctx, func(tx *gorm.DB) error {
		var derived int64
		if err := scopeTenant(ctx, tx.Model(&models.Unit{})).
			Where("base_unit_id = ?", id).Count(&derived).Error; err != nil {
			return err
		}
		var used int64
		if err := scopeTenant(ctx, tx.Model(&models.Product{})).
			Where("unit_id = ?", id).Count(&used).Error; err != nil {
			return err
		}
		if derived > 0 || used > 0 {
			return ErrUnitInUse
		}

		res := scopeTenant(ctx, tx).Delete(&models.Unit{}, "id = ?", id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrUnitNotFound
		}
		return nil
	})
}
