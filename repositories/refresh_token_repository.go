package repositories

import (
	"context"
	"errors"
	"time"

	"candra/backend-api/database"
	"candra/backend-api/models"

	"gorm.io/gorm"
)

// ErrRefreshTokenNotFound dikembalikan bila hash token tidak cocok dengan baris
// mana pun.
var ErrRefreshTokenNotFound = errors.New("refresh token tidak ditemukan")

// ErrRefreshTokenReused dikembalikan bila token yang SUDAH dicabut/dirotasi
// dipakai lagi. Ini indikasi kuat token dicuri — pemanggil wajib mencabut
// SELURUH sesi user tersebut (§9).
var ErrRefreshTokenReused = errors.New("refresh token sudah dipakai")

// CreateRefreshToken menyimpan satu baris refresh token baru.
func CreateRefreshToken(ctx context.Context, token *models.RefreshToken) error {
	return database.DB.WithContext(ctx).Create(token).Error
}

// FindRefreshTokenByHash mengambil baris refresh token berdasarkan hash-nya
// TANPA menyaring status — pemanggil sendiri yang memutuskan arti "dicabut" atau
// "kedaluwarsa" (lihat models.RefreshToken.IsActive). Mengembalikan
// ErrRefreshTokenNotFound bila tidak ada.
func FindRefreshTokenByHash(ctx context.Context, tokenHash string, token *models.RefreshToken) error {
	err := database.DB.WithContext(ctx).
		Where("token_hash = ?", tokenHash).
		First(token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrRefreshTokenNotFound
	}
	return err
}

// RotateRefreshToken mencabut token lama dan menyimpan token baru dalam SATU
// transaksi, sehingga tidak pernah ada keadaan "lama sudah dicabut tapi baru
// belum ada" atau sebaliknya.
//
// Bila token lama ternyata sudah dicabut (dirotasi oleh permintaan lain, atau
// upaya pemakaian ulang), mengembalikan ErrRefreshTokenReused dan tidak menulis
// apa pun.
func RotateRefreshToken(ctx context.Context, oldID string, newToken *models.RefreshToken) error {
	return database.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&models.RefreshToken{}).
			Where("id = ? AND revoked_at IS NULL", oldID).
			Update("revoked_at", time.Now().UTC())
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrRefreshTokenReused
		}
		return tx.Create(newToken).Error
	})
}

// RevokeRefreshToken mencabut satu token (idempoten: mengembalikan nil walau
// token sudah dicabut atau tidak ada, supaya endpoint logout tidak menjadi
// oracle keberadaan token).
func RevokeRefreshToken(ctx context.Context, id string) error {
	return database.DB.WithContext(ctx).
		Model(&models.RefreshToken{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Update("revoked_at", time.Now().UTC()).Error
}

// RevokeAllUserRefreshTokens mencabut semua token aktif milik satu user.
// Dipakai saat mendeteksi pemakaian ulang token (kemungkinan pencurian) dan saat
// "logout dari semua perangkat".
func RevokeAllUserRefreshTokens(ctx context.Context, userID string) error {
	return database.DB.WithContext(ctx).
		Model(&models.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", time.Now().UTC()).Error
}
