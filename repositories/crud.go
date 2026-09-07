package repositories

import (
	"context"

	"gorm.io/gorm"
)

// Helper generik untuk pola CRUD tenant-scoped yang berulang di banyak resource
// master data. Resource dengan kebutuhan khusus (mis. products yang butuh JOIN
// nama unit/kategori) tetap menulis query-nya sendiri.

// paginateTenant mengambil satu halaman baris model T milik tenant konteks
// beserta total-nya. `where`/`args` filter tambahan opsional; `order` wajib
// deterministik (biasanya "id DESC"). Query halaman dilewati bila total 0.
func paginateTenant[T any](ctx context.Context, where string, args []any, order string, limit, offset int) ([]T, int64, error) {
	var zero T

	countQ := scopeTenant(ctx, tenantDB(ctx, nil).Model(&zero))
	if where != "" {
		countQ = countQ.Where(where, args...)
	}
	var total int64
	if err := countQ.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []T{}, 0, nil
	}

	rows := make([]T, 0, limit)
	q := scopeTenant(ctx, tenantDB(ctx, nil).Model(&zero)).Order(order).Limit(limit).Offset(offset)
	if where != "" {
		q = q.Where(where, args...)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// firstTenant memuat satu baris model T milik tenant konteks berdasarkan id ke
// `out`. Mengembalikan gorm.ErrRecordNotFound bila tidak ada (pemanggil
// menerjemahkannya ke sentinel spesifik resource). tx opsional.
func firstTenant[T any](ctx context.Context, tx *gorm.DB, id string, out *T) error {
	return scopeTenant(ctx, tenantDB(ctx, tx)).First(out, "id = ?", id).Error
}

// createTenant menyimpan baris baru. Pemanggil (service/controller) mengisi
// TenantID dari konteks. tx opsional.
func createTenant[T any](ctx context.Context, tx *gorm.DB, row *T) error {
	return tenantDB(ctx, tx).Create(row).Error
}

// updateTenantColumns memperbarui kolom terpilih dari `row` (identitasnya lewat
// primary key di dalam row), dibatasi tenant konteks. Mengembalikan
// gorm.ErrRecordNotFound bila tidak ada baris yang cocok. tx opsional.
func updateTenantColumns[T any](ctx context.Context, tx *gorm.DB, row *T, columns ...string) error {
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Model(row).Select(columns).Updates(row)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// softDeleteTenant menandai baris terhapus (deleted_at), dibatasi tenant konteks.
// Mengembalikan gorm.ErrRecordNotFound bila tidak ada. tx opsional.
func softDeleteTenant[T any](ctx context.Context, tx *gorm.DB, id string) error {
	var zero T
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Delete(&zero, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
