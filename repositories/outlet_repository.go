package repositories

import (
	"context"
	"errors"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

// ErrOutletNotFound dikembalikan bila outlet tidak ada di tenant konteks.
var ErrOutletNotFound = errors.New("outlet tidak ditemukan")

// outletSearchCondition membangun klausa WHERE pencarian outlet.
func outletSearchCondition(search string) (string, []interface{}) {
	if search == "" {
		return "", nil
	}
	pattern := "%" + escapeLike(search) + "%"
	return "outlets.name ILIKE ? OR outlets.address ILIKE ?", []interface{}{pattern, pattern}
}

// ListOutlets mengambil satu halaman outlet milik tenant konteks + total-nya.
//
// Semua query lewat scopeTenant (lapisan 1). Order by id (ULID kronologis) →
// paginasi deterministik. Query halaman dilewati bila total 0.
func ListOutlets(ctx context.Context, search string, limit, offset int) ([]models.Outlet, int64, error) {
	cond, args := outletSearchCondition(search)

	base := scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.Outlet{}))
	if cond != "" {
		base = base.Where(cond, args...)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []models.Outlet{}, 0, nil
	}

	outlets := make([]models.Outlet, 0, limit)
	q := scopeTenant(ctx, tenantDB(ctx, nil)).Order("id DESC").Limit(limit).Offset(offset)
	if cond != "" {
		q = q.Where(cond, args...)
	}
	if err := q.Find(&outlets).Error; err != nil {
		return nil, 0, err
	}
	return outlets, total, nil
}

// FindOutletByID mengambil satu outlet milik tenant konteks.
// tx opsional (dipakai service yang butuh outlet di dalam transaksinya).
func FindOutletByID(ctx context.Context, tx *gorm.DB, id string, outlet *models.Outlet) error {
	err := scopeTenant(ctx, tenantDB(ctx, tx)).First(outlet, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrOutletNotFound
	}
	return err
}

// CreateOutlet menyimpan outlet baru. tenant_id WAJIB sudah diisi pemanggil
// (service) dari tenant konteks — repo tidak menebaknya.
func CreateOutlet(ctx context.Context, tx *gorm.DB, outlet *models.Outlet) error {
	return tenantDB(ctx, tx).Create(outlet).Error
}

// UpdateOutlet menyimpan perubahan outlet. Pemanggil sudah memuat baris lewat
// FindOutletByID (jadi kepemilikannya sudah terverifikasi). Select membatasi
// kolom yang boleh berubah — tenant_id & id tidak termasuk.
func UpdateOutlet(ctx context.Context, tx *gorm.DB, outlet *models.Outlet) error {
	return scopeTenant(ctx, tenantDB(ctx, tx)).
		Model(outlet).
		Select(
			"name", "type", "address", "phone", "timezone", "business_day_start",
			"currency", "tax_enabled", "tax_rate", "tax_inclusive",
			"service_charge_rate", "receipt_header", "receipt_footer", "is_active",
		).
		Updates(outlet).Error
}

// SoftDeleteOutlet menandai outlet terhapus (deleted_at). Data transaksional
// yang menunjuknya tetap utuh. tx opsional.
func SoftDeleteOutlet(ctx context.Context, tx *gorm.DB, id string) error {
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Delete(&models.Outlet{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrOutletNotFound
	}
	return nil
}
