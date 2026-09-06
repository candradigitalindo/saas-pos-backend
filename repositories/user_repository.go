package repositories

import (
	"context"

	"candra/backend-api/database"
	"candra/backend-api/models"
)

// userSearchCondition membangun klausa WHERE untuk pencarian user.
// Kolom di-prefix "users." agar tidak ambigu saat query melakukan JOIN ke roles
// (tabel roles juga punya kolom "name").
func userSearchCondition(search string) (string, []interface{}) {
	if search == "" {
		return "", nil
	}
	pattern := "%" + escapeLike(search) + "%"
	return "users.name ILIKE ? OR users.username ILIKE ? OR users.email ILIKE ?",
		[]interface{}{pattern, pattern, pattern}
}

// ListUsers mengambil satu halaman user beserta total keseluruhannya.
//
// Optimasi:
//   - Joins (bukan Preload) -> data user + role diambil dalam SATU query,
//     bukan dua round-trip terpisah.
//   - Count dijalankan TANPA join karena filter hanya menyentuh kolom users.
//   - Order by users.id (ULID terurut kronologis) -> hasil paginasi deterministik
//     dan memanfaatkan index primary key. Tanpa ORDER BY, PostgreSQL tidak
//     menjamin urutan baris sehingga data bisa terlewat/terduplikasi antar halaman.
//   - Query halaman dilewati bila total = 0.
func ListUsers(ctx context.Context, search string, limit, offset int) ([]models.User, int64, error) {
	cond, args := userSearchCondition(search)

	var total int64
	countQuery := database.DB.WithContext(ctx).Model(&models.User{})
	if cond != "" {
		countQuery = countQuery.Where(cond, args...)
	}
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []models.User{}, 0, nil
	}

	users := make([]models.User, 0, limit)
	query := database.DB.WithContext(ctx).
		Joins("Role").
		Order("users.id DESC").
		Limit(limit).
		Offset(offset)
	if cond != "" {
		query = query.Where(cond, args...)
	}
	if err := query.Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func CreateUser(ctx context.Context, user *models.User) error {
	return database.DB.WithContext(ctx).Create(user).Error
}

// FindUserByUsername mengambil user beserta role-nya dalam satu query (JOIN).
func FindUserByUsername(ctx context.Context, username string, user *models.User) error {
	return database.DB.WithContext(ctx).
		Joins("Role").
		Where("users.username = ?", username).
		First(user).Error
}

// FindUserByID mengambil user beserta role-nya dalam satu query (JOIN).
func FindUserByID(ctx context.Context, id string, user *models.User) error {
	return database.DB.WithContext(ctx).
		Joins("Role").
		Where("users.id = ?", id).
		First(user).Error
}

// UpdateUser menyimpan perubahan user.
// Omit("Role") mencegah GORM ikut meng-upsert baris di tabel roles hanya karena
// association Role ter-load di struct — menghemat satu write dan menghindari
// perubahan data role yang tidak disengaja.
func UpdateUser(ctx context.Context, user *models.User) error {
	return database.DB.WithContext(ctx).Omit("Role").Save(user).Error
}

func DeleteUser(ctx context.Context, user *models.User) error {
	return database.DB.WithContext(ctx).Delete(user).Error
}
