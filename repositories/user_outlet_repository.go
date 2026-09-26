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

// OutletIDsForUser mengembalikan id outlet AKTIF yang boleh diakses user, dalam
// tenant konteks, urut waktu dibuat. Dipakai endpoint /me dan detail user.
// Cabang yang sudah dihapus/dinonaktifkan tidak ikut, walau baris aksesnya
// masih tersisa — klien tidak boleh ditawari toko yang sudah tutup.
func OutletIDsForUser(ctx context.Context, userID string) ([]string, error) {
	var ids []string
	err := tenantDB(ctx, nil).Table("user_outlets uo").
		Joins("JOIN outlets o ON o.tenant_id = uo.tenant_id AND o.id = uo.outlet_id").
		Where("uo.tenant_id = ? AND uo.user_id = ? AND o.deleted_at IS NULL AND o.is_active",
			currentTenantID(ctx), userID).
		Order("o.created_at, o.id").
		Pluck("uo.outlet_id", &ids).Error
	return ids, err
}

// AccessOutletIDs mengembalikan SELURUH id outlet pada user_outlets milik user
// (termasuk cabang yang sudah ditutup — riwayatnya tetap boleh dibaca orang
// yang dulu bekerja di sana). Dipakai middleware TenantScope untuk batas
// cabang per permintaan.
func AccessOutletIDs(ctx context.Context, userID string) ([]string, error) {
	ids := []string{}
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

// ActiveOutletIDs mengembalikan id seluruh outlet AKTIF milik tenant konteks,
// urut waktu dibuat (outlet pertama lebih dulu). tx opsional.
//
// Dipakai untuk dua hal: daftar outlet pemegang outlet.manage di /me (mereka
// mengelola SEMUA cabang, termasuk yang baru dibuat), dan bawaan akses staf
// baru yang dibuat tanpa outlet_ids.
func ActiveOutletIDs(ctx context.Context, tx *gorm.DB) ([]string, error) {
	var ids []string
	err := scopeTenant(ctx, tenantDB(ctx, tx).Model(&models.Outlet{})).
		Where("is_active").
		Order("created_at, id").
		Pluck("id", &ids).Error
	return ids, err
}

// SetUserOutlets MENGGANTI seluruh akses outlet seorang user dengan
// `outletIDs` (hapus lalu sisipkan), dalam tx pemanggil. Pemanggil wajib sudah
// memastikan setiap outlet milik tenant yang sama — FK komposit
// (tenant_id, outlet_id) adalah jaring pengaman terakhirnya.
func SetUserOutlets(ctx context.Context, tx *gorm.DB, tenantID, userID string, outletIDs []string) error {
	if err := tenantDB(ctx, tx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Delete(&models.UserOutlet{}).Error; err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(outletIDs))
	for _, oid := range outletIDs {
		if _, dup := seen[oid]; dup || oid == "" {
			continue
		}
		seen[oid] = struct{}{}
		if err := LinkUserOutlet(ctx, tx, models.UserOutlet{
			TenantID: tenantID, UserID: userID, OutletID: oid,
		}); err != nil {
			return err
		}
	}
	return nil
}

// OutletIDsForUsers mengembalikan peta user_id → id outlet yang boleh
// diakses, untuk banyak user sekaligus (satu query, bukan N+1 di daftar user).
func OutletIDsForUsers(ctx context.Context, userIDs []string) (map[string][]string, error) {
	out := make(map[string][]string, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	var rows []models.UserOutlet
	if err := scopeTenant(ctx, tenantDB(ctx, nil)).
		Where("user_id IN ?", userIDs).
		Order("user_id, outlet_id").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.UserID] = append(out[r.UserID], r.OutletID)
	}
	return out, nil
}

// OutletsByIDs memuat outlet milik tenant konteks untuk id-id yang diberikan,
// MENGIKUTI urutan `ids` (id yang tidak ditemukan dilewati). tx opsional.
func OutletsByIDs(ctx context.Context, tx *gorm.DB, ids []string) ([]models.Outlet, error) {
	if len(ids) == 0 {
		return []models.Outlet{}, nil
	}
	var rows []models.Outlet
	if err := scopeTenant(ctx, tenantDB(ctx, tx)).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	perID := make(map[string]models.Outlet, len(rows))
	for _, o := range rows {
		perID[o.ID] = o
	}
	out := make([]models.Outlet, 0, len(ids))
	for _, id := range ids {
		if o, ok := perID[id]; ok {
			out = append(out, o)
		}
	}
	return out, nil
}
