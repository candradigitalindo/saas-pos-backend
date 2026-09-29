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

// UserPermissionCodes mengembalikan izin efektif seorang USER: GABUNGAN izin
// seluruh peran yang dipegangnya (migrasi 000034). Satu query, DISTINCT.
//
// Inilah yang dipakai middleware TenantScope setiap permintaan — bukan
// EffectivePermissionCodes(user.role_id), yang hanya melihat peran utama dan
// akan mengabaikan peran tambahan.
func UserPermissionCodes(ctx context.Context, userID string) ([]string, error) {
	if userID == "" {
		return nil, nil
	}
	var codes []string
	err := database.DB.WithContext(ctx).
		Table("user_roles AS ur").
		Joins("JOIN role_permissions rp ON rp.role_id = ur.role_id").
		Joins("JOIN permissions p ON p.id = rp.permission_id").
		Where("ur.user_id = ?", userID).
		Distinct().
		Pluck("p.code", &codes).Error
	return codes, err
}

// RoleIDsForUsers memuat peran BANYAK user sekaligus — satu query untuk seluruh
// halaman daftar user, bukan satu query per baris.
func RoleIDsForUsers(ctx context.Context, userIDs []string) (map[string][]string, error) {
	out := map[string][]string{}
	if len(userIDs) == 0 {
		return out, nil
	}
	var rows []models.UserRole
	if err := scopeTenant(ctx, tenantDB(ctx, nil)).
		Where("user_id IN ?", userIDs).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.UserID] = append(out[r.UserID], r.RoleID)
	}
	return out, nil
}

// RoleIDsForUser mengembalikan seluruh peran yang dipegang user.
func RoleIDsForUser(ctx context.Context, tx *gorm.DB, userID string) ([]string, error) {
	var ids []string
	err := scopeTenant(ctx, tenantDB(ctx, tx).Model(&models.UserRole{})).
		Where("user_id = ?", userID).Pluck("role_id", &ids).Error
	return ids, err
}

// SetUserRoles mengganti SELURUH peran seorang user sekaligus menyetel peran
// utamanya (`users.role_id`), dalam satu operasi.
//
// SATU-SATUNYA tempat kedua sumber itu ditulis, supaya tidak pernah bisa
// berbeda: peran utama dipastikan selalu ikut masuk `user_roles`. roleIDs boleh
// kosong — artinya user hanya memegang peran utama.
// tenantID diminta EKSPLISIT, tidak diambil dari context: pendaftaran usaha
// baru memanggil ini saat tenant-nya baru saja lahir dan belum ada di context.
func SetUserRoles(ctx context.Context, tx *gorm.DB, tenantID, userID, primaryRoleID string, roleIDs []string) error {
	tid := tenantID

	// Peran utama WAJIB termasuk; duplikat dibuang.
	want := map[string]struct{}{}
	if primaryRoleID != "" {
		want[primaryRoleID] = struct{}{}
	}
	for _, id := range roleIDs {
		if id != "" {
			want[id] = struct{}{}
		}
	}

	if err := tx.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tid, userID).
		Delete(&models.UserRole{}).Error; err != nil {
		return err
	}
	if len(want) > 0 {
		rows := make([]models.UserRole, 0, len(want))
		for id := range want {
			rows = append(rows, models.UserRole{TenantID: tid, UserID: userID, RoleID: id})
		}
		if err := tx.WithContext(ctx).Create(&rows).Error; err != nil {
			return err
		}
	}
	return tx.WithContext(ctx).Model(&models.User{}).
		Where("tenant_id = ? AND id = ?", tid, userID).
		UpdateColumn("role_id", nilIfEmptyStr(primaryRoleID)).Error
}

// UsersHoldingRole menghitung berapa user yang masih memegang sebuah peran —
// lewat peran utama MAUPUN peran tambahan. Dipakai sebelum menghapus peran.
func UsersHoldingRole(ctx context.Context, tx *gorm.DB, roleID string) (int64, error) {
	var n int64
	err := scopeTenant(ctx, tenantDB(ctx, tx).Model(&models.UserRole{})).
		Where("role_id = ?", roleID).Count(&n).Error
	return n, err
}

func nilIfEmptyStr(s string) any {
	if s == "" {
		return nil
	}
	return s
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
