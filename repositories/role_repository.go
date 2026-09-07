package repositories

import (
	"context"
	"errors"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

// ErrRoleInUse dikembalikan saat mencoba menghapus role yang masih dipakai user.
var ErrRoleInUse = errors.New("role sedang digunakan oleh satu atau lebih user dan tidak dapat dihapus")

// ErrRoleNotFound dikembalikan bila role tidak ada di tenant konteks.
var ErrRoleNotFound = errors.New("role tidak ditemukan")

// roleSearchCondition membangun klausa WHERE pencarian role.
func roleSearchCondition(search string) (string, []interface{}) {
	if search == "" {
		return "", nil
	}
	return "name ILIKE ?", []interface{}{"%" + escapeLike(search) + "%"}
}

// ListRoles mengambil satu halaman role milik TENANT KONTEKS + total-nya.
func ListRoles(ctx context.Context, search string, limit, offset int) ([]models.Role, int64, error) {
	cond, args := roleSearchCondition(search)

	countQ := scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.Role{}))
	if cond != "" {
		countQ = countQ.Where(cond, args...)
	}
	var total int64
	if err := countQ.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []models.Role{}, 0, nil
	}

	roles := make([]models.Role, 0, limit)
	q := scopeTenant(ctx, tenantDB(ctx, nil)).
		Order("id DESC").
		Limit(limit).
		Offset(offset)
	if cond != "" {
		q = q.Where(cond, args...)
	}
	if err := q.Find(&roles).Error; err != nil {
		return nil, 0, err
	}
	return roles, total, nil
}

// FindRoleInTenant mengambil satu role milik tenant konteks. tx opsional.
// ErrRoleNotFound juga muncul bila id valid tapi milik tenant lain.
func FindRoleInTenant(ctx context.Context, tx *gorm.DB, id string, role *models.Role) error {
	err := scopeTenant(ctx, tenantDB(ctx, tx)).First(role, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrRoleNotFound
	}
	return err
}

// CreateRole menyimpan role baru. Pemanggil mengisi TenantID dari konteks. tx
// opsional (registration service memakainya di dalam transaksinya).
func CreateRole(ctx context.Context, tx *gorm.DB, role *models.Role) error {
	return tenantDB(ctx, tx).Create(role).Error
}

// UpdateRole menyimpan perubahan role milik tenant konteks (nama & deskripsi).
func UpdateRole(ctx context.Context, tx *gorm.DB, role *models.Role) error {
	res := scopeTenant(ctx, tenantDB(ctx, tx)).
		Model(role).
		Select("name", "description").
		Updates(role)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrRoleNotFound
	}
	return nil
}

// DeleteRole menandai role terhapus (soft delete) di tenant konteks.
//
// Karena soft delete = UPDATE, FK ON DELETE RESTRICT tidak ikut menjaga. Jadi di
// dalam satu transaksi: tolak bila masih ada user aktif yang memakainya
// (ErrRoleInUse) atau bila role bawaan (is_system). Cek + hapus dalam satu
// transaksi menutup celah race "user baru menyelip di antara cek dan hapus".
func DeleteRole(ctx context.Context, id string) error {
	return WithTenant(ctx, func(tx *gorm.DB) error {
		var role models.Role
		if err := scopeTenant(ctx, tx).First(&role, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRoleNotFound
			}
			return err
		}
		if role.IsSystem {
			return ErrRoleInUse
		}

		var inUse int64
		if err := scopeTenant(ctx, tx.Model(&models.User{})).
			Where("role_id = ?", id).
			Count(&inUse).Error; err != nil {
			return err
		}
		if inUse > 0 {
			return ErrRoleInUse
		}

		res := scopeTenant(ctx, tx).Delete(&models.Role{}, "id = ?", id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrRoleNotFound
		}
		return nil
	})
}
