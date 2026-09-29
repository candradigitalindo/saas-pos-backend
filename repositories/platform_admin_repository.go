package repositories

import (
	"context"
	"errors"
	"time"

	"candra/backend-api/database"
	"candra/backend-api/models"

	"gorm.io/gorm"
)

// Repositori akun admin platform (migrasi 000035, blueprint G.5).
// Tabel PLATFORM: tanpa RLS, tanpa tenant_id — tidak ada scopeTenant di sini.

var ErrPlatformAdminNotFound = errors.New("akun admin platform tidak ditemukan")

func CreatePlatformAdmin(ctx context.Context, a *models.PlatformAdmin) error {
	return database.DB.WithContext(ctx).Create(a).Error
}

func FindPlatformAdminByID(ctx context.Context, id string) (models.PlatformAdmin, error) {
	var a models.PlatformAdmin
	err := database.DB.WithContext(ctx).First(&a, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return a, ErrPlatformAdminNotFound
	}
	return a, err
}

// FindPlatformAdminByEmail dipakai saat masuk panel internal.
func FindPlatformAdminByEmail(ctx context.Context, email string) (models.PlatformAdmin, error) {
	var a models.PlatformAdmin
	err := database.DB.WithContext(ctx).First(&a, "email = ?", email).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return a, ErrPlatformAdminNotFound
	}
	return a, err
}

func ListPlatformAdmins(ctx context.Context) ([]models.PlatformAdmin, error) {
	var rows []models.PlatformAdmin
	err := database.DB.WithContext(ctx).Order("created_at").Find(&rows).Error
	return rows, err
}

func TouchPlatformAdminLogin(ctx context.Context, id string) error {
	return database.DB.WithContext(ctx).Model(&models.PlatformAdmin{}).
		Where("id = ?", id).UpdateColumn("last_login_at", time.Now().UTC()).Error
}

// SetPlatformAdminActive menyalakan/mematikan sebuah akun admin.
func SetPlatformAdminActive(ctx context.Context, id string, active bool) error {
	res := database.DB.WithContext(ctx).Model(&models.PlatformAdmin{}).
		Where("id = ?", id).
		Updates(map[string]any{"is_active": active, "updated_at": gorm.Expr("now()")})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrPlatformAdminNotFound
	}
	return nil
}

// CountActiveSuperadmins dipakai untuk mencegah superadmin terakhir dimatikan —
// tanpa itu panel internal terkunci total, sama seperti peran pemilik di tenant.
func CountActiveSuperadmins(ctx context.Context, exceptID string) (int64, error) {
	var n int64
	err := database.DB.WithContext(ctx).Model(&models.PlatformAdmin{}).
		Where("role = 'superadmin' AND is_active = true AND id <> ?", exceptID).
		Count(&n).Error
	return n, err
}
