package repositories

import (
	"context"
	"errors"
	"strings"
	"time"

	"candra/backend-api/database"
	"candra/backend-api/models"

	"gorm.io/gorm"
)

// ErrUserNotFound dikembalikan bila user tidak ada (di tenant konteks, untuk
// fungsi yang tenant-scoped).
var ErrUserNotFound = errors.New("user tidak ditemukan")

// userSearchCondition membangun klausa WHERE pencarian user.
// Kolom di-prefix "users." agar tidak ambigu saat JOIN ke roles.
func userSearchCondition(search string) (string, []interface{}) {
	if search == "" {
		return "", nil
	}
	pattern := "%" + escapeLike(search) + "%"
	return "users.name ILIKE ? OR users.username ILIKE ? OR users.email ILIKE ?",
		[]interface{}{pattern, pattern, pattern}
}

// ListUsers mengambil satu halaman user milik TENANT KONTEKS + total-nya.
// Jalur baca: lapisan 1 (scopeTenant) yang menjamin isolasi.
func ListUsers(ctx context.Context, search string, limit, offset int) ([]models.User, int64, error) {
	cond, args := userSearchCondition(search)

	countQ := scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.User{}))
	if cond != "" {
		countQ = countQ.Where(cond, args...)
	}
	var total int64
	if err := countQ.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []models.User{}, 0, nil
	}

	users := make([]models.User, 0, limit)
	q := scopeTenantOn(ctx, tenantDB(ctx, nil), "users").
		Joins("Role").
		Order("users.id DESC").
		Limit(limit).
		Offset(offset)
	if cond != "" {
		q = q.Where(cond, args...)
	}
	if err := q.Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// FindUserInTenant mengambil satu user milik tenant konteks (dengan Role).
// tx opsional. Mengembalikan ErrUserNotFound bila tidak ada di tenant ini —
// termasuk bila id valid tapi milik tenant lain.
func FindUserInTenant(ctx context.Context, tx *gorm.DB, id string, user *models.User) error {
	err := scopeTenantOn(ctx, tenantDB(ctx, tx), "users").
		Joins("Role").
		Where("users.id = ?", id).
		First(user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrUserNotFound
	}
	return err
}

// FindUserByID mengambil user beserta role-nya TANPA scope tenant. Dipakai jalur
// auth yang belum tahu tenant: bootstrap sesi di middleware, dan pengecekan
// keberadaan user saat refresh token. RLS `users` permisif saat GUC belum
// disetel, jadi query ini berjalan normal.
func FindUserByID(ctx context.Context, id string, user *models.User) error {
	return database.DB.WithContext(ctx).
		Joins("Role").
		Where("users.id = ?", id).
		First(user).Error
}

// FindUserByUsername mengambil user beserta role-nya (untuk Login). Tanpa scope
// tenant — login lintas tenant lewat satu endpoint.
func FindUserByUsername(ctx context.Context, username string, user *models.User) error {
	return database.DB.WithContext(ctx).
		Joins("Role").
		Where("users.username = ?", username).
		First(user).Error
}

// FindUserForLogin mencari user untuk halaman masuk dari NAMA PENGGUNA atau
// EMAIL (spasi di tepi diabaikan). Urutannya:
//
//  1. nama pengguna persis — perilaku lama; nama pengguna TIDAK dilarang
//     memuat "@", jadi akun lama yang nama penggunanya berbentuk email tetap
//     bisa masuk seperti biasa;
//  2. email persis (memakai uq_users_email);
//  3. email tanpa peduli huruf besar-kecil — orang mengetik
//     "Sari@Warung.id" untuk akun "sari@warung.id" — HANYA bila tepat satu
//     akun cocok. uq_users_email peka huruf besar, jadi dua akun bisa berbeda
//     hanya di kapitalisasi; dalam keadaan itu tidak ada yang ditebak.
//
// Dulu hanya langkah 1, dan halaman masuk tidak menjelaskannya: yang mengetik
// email — biasanya lebih diingat daripada nama pengguna — selalu ditolak.
// Tanpa scope tenant — login lintas tenant lewat satu endpoint.
func FindUserForLogin(ctx context.Context, login string, user *models.User) error {
	login = strings.TrimSpace(login)
	err := FindUserByUsername(ctx, login, user)
	if err == nil || !errors.Is(err, gorm.ErrRecordNotFound) || !strings.Contains(login, "@") {
		return err
	}
	err = database.DB.WithContext(ctx).Joins("Role").Where("users.email = ?", login).First(user).Error
	if err == nil || !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	var cocok []models.User
	if err := database.DB.WithContext(ctx).Joins("Role").
		Where("LOWER(users.email) = LOWER(?)", login).
		Limit(2).Find(&cocok).Error; err != nil {
		return err
	}
	if len(cocok) != 1 {
		return gorm.ErrRecordNotFound
	}
	*user = cocok[0]
	return nil
}

// CreateUser menyimpan user baru. Pemanggil (service) mengisi TenantID & RoleID
// dari konteks — repo tidak menebak. tx opsional.
func CreateUser(ctx context.Context, tx *gorm.DB, user *models.User) error {
	return tenantDB(ctx, tx).Create(user).Error
}

// UpdateUser menyimpan perubahan user milik tenant konteks.
// Omit("Role") mencegah GORM ikut meng-upsert baris roles. scopeTenant memastikan
// user tenant lain tidak bisa disentuh walau id-nya ditebak.
func UpdateUser(ctx context.Context, tx *gorm.DB, user *models.User) error {
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Omit("Role").Save(user)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

// SoftDeleteUser menandai user terhapus (deleted_at) di tenant konteks.
func SoftDeleteUser(ctx context.Context, tx *gorm.DB, id string) error {
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Delete(&models.User{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

// UpdateLastLogin mencatat waktu login terakhir. Best-effort: dipanggil setelah
// login sukses, kegagalannya tidak membatalkan login. Tanpa scope tenant
// (konteks login belum ber-tenant).
func UpdateLastLogin(ctx context.Context, userID string, at time.Time) error {
	return database.DB.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", userID).
		UpdateColumn("last_login_at", at).Error
}
