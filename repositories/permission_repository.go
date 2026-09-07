package repositories

import (
	"context"

	"candra/backend-api/database"
	"candra/backend-api/models"

	"gorm.io/gorm"
)

// ListPermissions mengembalikan seluruh katalog permission, diurutkan agar
// tampilan pengaturan peran stabil. Bukan tabel bertenant → tanpa scopeTenant.
func ListPermissions(ctx context.Context) ([]models.Permission, error) {
	var perms []models.Permission
	err := database.DB.WithContext(ctx).
		Order("group_name, code").
		Find(&perms).Error
	return perms, err
}

// AllPermissionIDs mengembalikan id seluruh permission. Dipakai registration
// service untuk memberi peran Pemilik SEMUA hak akses. tx wajib (dipanggil di
// dalam transaksi registrasi).
func AllPermissionIDs(ctx context.Context, tx *gorm.DB) ([]string, error) {
	var ids []string
	err := tx.WithContext(ctx).Model(&models.Permission{}).Pluck("id", &ids).Error
	return ids, err
}

// PermissionIDsByCode mengembalikan id permission untuk daftar kode tertentu.
// Kode yang tidak ada di katalog diabaikan (tidak error) — pemanggil bisa
// membandingkan len(hasil) dengan len(codes) bila perlu ketat. tx opsional.
func PermissionIDsByCode(ctx context.Context, tx *gorm.DB, codes []string) ([]string, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	db := database.DB
	if tx != nil {
		db = tx
	}
	var ids []string
	err := db.WithContext(ctx).
		Model(&models.Permission{}).
		Where("code IN ?", codes).
		Pluck("id", &ids).Error
	return ids, err
}

// EffectivePermissionCodes mengembalikan kode permission efektif untuk sebuah
// role, lewat join role_permissions → permissions.
//
// Tidak ada RLS di role_permissions / permissions; keamanannya dijaga karena
// roleID datang dari user yang sudah diverifikasi middleware (user.role_id).
// Dibaca setiap permintaan (§9) — cukup satu query.
func EffectivePermissionCodes(ctx context.Context, roleID string) ([]string, error) {
	if roleID == "" {
		return nil, nil
	}
	var codes []string
	err := database.DB.WithContext(ctx).
		Table("role_permissions AS rp").
		Joins("JOIN permissions p ON p.id = rp.permission_id").
		Where("rp.role_id = ?", roleID).
		Pluck("p.code", &codes).Error
	return codes, err
}

// ReplaceRolePermissions mengganti seluruh pemetaan permission sebuah role
// dengan permissionIDs yang diberikan, dalam satu transaksi (hapus lalu sisip).
// Dipakai endpoint pengaturan peran.
func ReplaceRolePermissions(ctx context.Context, roleID string, permissionIDs []string) error {
	return database.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&models.RolePermission{}).Error; err != nil {
			return err
		}
		if len(permissionIDs) == 0 {
			return nil
		}
		rows := make([]models.RolePermission, 0, len(permissionIDs))
		for _, pid := range permissionIDs {
			rows = append(rows, models.RolePermission{RoleID: roleID, PermissionID: pid})
		}
		return tx.Create(&rows).Error
	})
}

// AssignRolePermissions menyisipkan pemetaan permission untuk sebuah role di
// dalam transaksi tx (dipakai registration service). ON CONFLICT DO NOTHING agar
// aman dipanggil ulang.
func AssignRolePermissions(ctx context.Context, tx *gorm.DB, roleID string, permissionIDs []string) error {
	if len(permissionIDs) == 0 {
		return nil
	}
	rows := make([]models.RolePermission, 0, len(permissionIDs))
	for _, pid := range permissionIDs {
		rows = append(rows, models.RolePermission{RoleID: roleID, PermissionID: pid})
	}
	return tx.WithContext(ctx).
		Clauses(onConflictDoNothing("role_id", "permission_id")).
		Create(&rows).Error
}
