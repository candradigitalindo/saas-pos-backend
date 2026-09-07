package repositories

import (
	"context"
	"errors"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

// ErrCategoryNotFound dikembalikan bila kategori tidak ada di tenant konteks.
var ErrCategoryNotFound = errors.New("kategori tidak ditemukan")

func categorySearch(search string) (string, []any) {
	if search == "" {
		return "", nil
	}
	return "name ILIKE ?", []any{"%" + escapeLike(search) + "%"}
}

// ListCategories mengambil satu halaman kategori milik tenant konteks. Diurutkan
// sort_order lalu name agar tampilan pohon kategori stabil.
func ListCategories(ctx context.Context, search string, limit, offset int) ([]models.Category, int64, error) {
	where, args := categorySearch(search)
	return paginateTenant[models.Category](ctx, where, args, "sort_order ASC, name ASC, id ASC", limit, offset)
}

// FindCategoryInTenant memuat satu kategori milik tenant konteks. tx opsional.
func FindCategoryInTenant(ctx context.Context, tx *gorm.DB, id string, out *models.Category) error {
	err := firstTenant(ctx, tx, id, out)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrCategoryNotFound
	}
	return err
}

// CreateCategory menyimpan kategori baru (TenantID diisi pemanggil). tx opsional.
func CreateCategory(ctx context.Context, tx *gorm.DB, row *models.Category) error {
	return createTenant(ctx, tx, row)
}

// UpdateCategory menyimpan perubahan name/parent_id/sort_order. tx opsional.
func UpdateCategory(ctx context.Context, tx *gorm.DB, row *models.Category) error {
	err := updateTenantColumns(ctx, tx, row, "name", "parent_id", "sort_order")
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrCategoryNotFound
	}
	return err
}

// DeleteCategory menandai kategori terhapus. Menolak (ErrCategoryInUse) bila
// masih dipakai produk atau punya sub-kategori — cek + hapus dalam satu
// transaksi.
func DeleteCategory(ctx context.Context, id string) error {
	return WithTenant(ctx, func(tx *gorm.DB) error {
		var children int64
		if err := scopeTenant(ctx, tx.Model(&models.Category{})).
			Where("parent_id = ?", id).Count(&children).Error; err != nil {
			return err
		}
		if children > 0 {
			return ErrCategoryInUse
		}

		var used int64
		if err := scopeTenant(ctx, tx.Model(&models.Product{})).
			Where("category_id = ?", id).Count(&used).Error; err != nil {
			return err
		}
		if used > 0 {
			return ErrCategoryInUse
		}

		res := scopeTenant(ctx, tx).Delete(&models.Category{}, "id = ?", id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrCategoryNotFound
		}
		return nil
	})
}

// ErrCategoryInUse dikembalikan saat kategori masih dipakai produk / sub-kategori.
var ErrCategoryInUse = errors.New("kategori masih dipakai produk atau punya sub-kategori")
