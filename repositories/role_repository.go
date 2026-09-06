package repositories

import (
	"context"
	"errors"

	"candra/backend-api/database"
	"candra/backend-api/models"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// ErrRoleInUse dikembalikan saat mencoba menghapus role yang masih digunakan oleh user.
var ErrRoleInUse = errors.New("role sedang digunakan oleh satu atau lebih user dan tidak dapat dihapus")

// roleSearchCondition membangun klausa WHERE untuk pencarian role.
func roleSearchCondition(search string) (string, []interface{}) {
	if search == "" {
		return "", nil
	}
	return "name ILIKE ?", []interface{}{"%" + escapeLike(search) + "%"}
}

// ListRoles mengambil satu halaman role beserta total keseluruhannya.
// Order by id (ULID terurut kronologis) menjamin paginasi deterministik.
func ListRoles(ctx context.Context, search string, limit, offset int) ([]models.Role, int64, error) {
	cond, args := roleSearchCondition(search)

	var total int64
	countQuery := database.DB.WithContext(ctx).Model(&models.Role{})
	if cond != "" {
		countQuery = countQuery.Where(cond, args...)
	}
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []models.Role{}, 0, nil
	}

	roles := make([]models.Role, 0, limit)
	query := database.DB.WithContext(ctx).
		Model(&models.Role{}).
		Order("id DESC").
		Limit(limit).
		Offset(offset)
	if cond != "" {
		query = query.Where(cond, args...)
	}
	if err := query.Find(&roles).Error; err != nil {
		return nil, 0, err
	}

	return roles, total, nil
}

func FindRoleByID(ctx context.Context, id string, role *models.Role) error {
	return database.DB.WithContext(ctx).First(role, "id = ?", id).Error
}

// FindRoleByName mencari role berdasarkan namanya. Dipakai untuk menetapkan
// role default ('user') saat registrasi mandiri.
func FindRoleByName(ctx context.Context, name string, role *models.Role) error {
	return database.DB.WithContext(ctx).First(role, "name = ?", name).Error
}

func CreateRole(ctx context.Context, role *models.Role) error {
	return database.DB.WithContext(ctx).Create(role).Error
}

// UpdateRoleModel menyimpan perubahan pada model role.
func UpdateRoleModel(ctx context.Context, role *models.Role) error {
	return database.DB.WithContext(ctx).Save(role).Error
}

// DeleteRole menghapus role dan mengembalikan ErrRoleInUse bila masih dipakai user.
//
// Optimasi: cukup SATU query DELETE, tanpa transaksi COUNT-lalu-DELETE.
// Perlindungan diserahkan ke foreign key constraint di database, sehingga:
//   - bebas race condition (pendekatan COUNT lalu DELETE masih bisa kebobolan
//     bila ada user baru memakai role tersebut di antara kedua query), dan
//   - lebih cepat karena menghemat satu round-trip + overhead transaksi.
//
// PostgreSQL mengembalikan SQLSTATE 23503 (foreign_key_violation) bila role
// masih direferensikan oleh baris di tabel users.
func DeleteRole(ctx context.Context, id string) error {
	result := database.DB.WithContext(ctx).Delete(&models.Role{}, "id = ?", id)
	if err := result.Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return ErrRoleInUse
		}
		return err
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
