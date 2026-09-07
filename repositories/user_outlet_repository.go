package repositories

import (
	"context"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

// LinkUserOutlet memberi seorang user akses ke sebuah outlet. Idempoten (ON
// CONFLICT DO NOTHING pada PK (user_id, outlet_id)). tx opsional. tenant_id pada
// row wajib diisi pemanggil agar FK komposit & RLS lolos.
func LinkUserOutlet(ctx context.Context, tx *gorm.DB, link models.UserOutlet) error {
	return tenantDB(ctx, tx).
		Clauses(onConflictDoNothing("user_id", "outlet_id")).
		Create(&link).Error
}

// UnlinkUserOutlet mencabut akses user ke sebuah outlet.
func UnlinkUserOutlet(ctx context.Context, userID, outletID string) error {
	return scopeTenant(ctx, tenantDB(ctx, nil)).
		Where("user_id = ? AND outlet_id = ?", userID, outletID).
		Delete(&models.UserOutlet{}).Error
}

// OutletIDsForUser mengembalikan id outlet yang boleh diakses user, dalam tenant
// konteks. Dipakai middleware RequireOutletAccess dan endpoint /me.
func OutletIDsForUser(ctx context.Context, userID string) ([]string, error) {
	var ids []string
	err := scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.UserOutlet{})).
		Where("user_id = ?", userID).
		Pluck("outlet_id", &ids).Error
	return ids, err
}

// UserCanAccessOutlet melaporkan apakah user punya baris user_outlets untuk
// outletID di tenant konteks.
func UserCanAccessOutlet(ctx context.Context, userID, outletID string) (bool, error) {
	var count int64
	err := scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.UserOutlet{})).
		Where("user_id = ? AND outlet_id = ?", userID, outletID).
		Count(&count).Error
	return count > 0, err
}
