package repositories

import (
	"context"
	"errors"

	"candra/backend-api/database"
	"candra/backend-api/models"

	"gorm.io/gorm"
)

// ErrTenantNotFound dikembalikan bila tenant tidak ada.
var ErrTenantNotFound = errors.New("tenant tidak ditemukan")

// CreateTenant menyimpan baris tenant baru. Dipakai HANYA oleh registration
// service, di dalam transaksinya sendiri (tx wajib).
func CreateTenant(ctx context.Context, tx *gorm.DB, tenant *models.Tenant) error {
	return tx.WithContext(ctx).Create(tenant).Error
}

// FindTenantByID mengambil tenant berdasarkan id.
//
// tx opsional: bila nil, memakai DB global. Karena RLS `tenants` mengunci
// berdasarkan id, pemanggil di dalam WithTenant hanya bisa mengambil tenant-nya
// sendiri — persis yang diinginkan untuk endpoint semacam GET /me.
func FindTenantByID(ctx context.Context, tx *gorm.DB, id string, tenant *models.Tenant) error {
	db := database.DB
	if tx != nil {
		db = tx
	}
	err := db.WithContext(ctx).First(tenant, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrTenantNotFound
	}
	return err
}
