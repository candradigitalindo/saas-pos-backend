package repositories

import (
	"context"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

// Pendukung kuota paket (services/plan_entitlement_service.go).
//
// Pemeriksaan kuota berbentuk "hitung dulu, lalu buat". Tanpa penguncian, dua
// permintaan bersamaan sama-sama menghitung 0 dari batas 1 dan keduanya
// lolos. Karena itu penghitungan dilakukan DI DALAM transaksi pembuatnya,
// setelah LockTenantRow: permintaan kedua menunggu sampai yang pertama selesai
// dan melihat hasilnya.

// LockTenantRow mengunci baris tenant konteks sampai transaksi `tx` selesai.
// Dipakai sebagai kunci serialisasi per-tenant untuk pemeriksaan kuota.
func LockTenantRow(ctx context.Context, tx *gorm.DB) error {
	var id string
	return tx.WithContext(ctx).Raw(
		"SELECT id FROM tenants WHERE id = ? FOR UPDATE", currentTenantID(ctx),
	).Scan(&id).Error
}

// CountOutlets menghitung cabang tenant yang belum dihapus.
func CountOutlets(ctx context.Context, tx *gorm.DB) (int64, error) {
	var n int64
	err := scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.Outlet{}).Count(&n).Error
	return n, err
}

// CountUsers menghitung pengguna tenant yang belum dihapus.
func CountUsers(ctx context.Context, tx *gorm.DB) (int64, error) {
	var n int64
	err := scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.User{}).Count(&n).Error
	return n, err
}

// CountProducts menghitung barang tenant yang belum dihapus.
func CountProducts(ctx context.Context, tx *gorm.DB) (int64, error) {
	var n int64
	err := scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.Product{}).Count(&n).Error
	return n, err
}
