package repositories

import (
	"context"
	"errors"
	"time"

	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"

	"gorm.io/gorm"
)

// IdempotencyMatch adalah hasil pencarian kunci idempotensi.
type IdempotencyMatch struct {
	Found          bool   // ada baris untuk (scope, key) ini
	SameRequest    bool   // request_hash sama → boleh replay
	ResponseStatus int    // status HTTP tersimpan
	ResponseBody   []byte // body JSON tersimpan
}

// LookupIdempotency mencari hasil tersimpan untuk (scope, key) pada tenant
// konteks. Dipanggil DI DALAM transaksi bisnis (tx wajib) — lihat §8/§13.1.
//
//   - Found=false                 → kerjakan operasinya.
//   - Found=true, SameRequest=true → kembalikan ResponseStatus/ResponseBody, jangan
//     kerjakan ulang.
//   - Found=true, SameRequest=false → kunci dipakai untuk request berbeda → 409.
func LookupIdempotency(ctx context.Context, tx *gorm.DB, scope, key, requestHash string) (IdempotencyMatch, error) {
	tid := reqctx.TenantID(ctx)

	var row models.IdempotencyKey
	err := tx.WithContext(ctx).
		Where("tenant_id = ? AND scope = ? AND key = ?", tid, scope, key).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return IdempotencyMatch{Found: false}, nil
	}
	if err != nil {
		return IdempotencyMatch{}, err
	}
	return IdempotencyMatch{
		Found:          true,
		SameRequest:    row.RequestHash == requestHash,
		ResponseStatus: row.ResponseStatus,
		ResponseBody:   row.ResponseBody,
	}, nil
}

// SaveIdempotency menyimpan hasil sebuah operasi idempoten. Dipanggil di DALAM
// transaksi yang sama dengan operasinya, sebagai langkah terakhir sebelum commit.
// Unique index (COALESCE(tenant_id,”), scope, key) menjadi pengaman terakhir
// bila dua permintaan lolos LookupIdempotency bersamaan.
func SaveIdempotency(ctx context.Context, tx *gorm.DB, scope, key, requestHash string, status int, body []byte, ttl time.Duration) error {
	row := models.IdempotencyKey{
		TenantID:       reqctx.TenantID(ctx),
		Scope:          scope,
		Key:            key,
		RequestHash:    requestHash,
		ResponseStatus: status,
		ResponseBody:   body,
		ExpiresAt:      time.Now().UTC().Add(ttl),
	}
	return tx.WithContext(ctx).Create(&row).Error
}

// DeleteExpiredIdempotency membersihkan kunci yang kedaluwarsa (dipanggil job
// harian di fase berikutnya). Aman dipanggil tanpa tenant.
func DeleteExpiredIdempotency(ctx context.Context) (int64, error) {
	res := tenantDB(ctx, nil).
		Where("expires_at < ?", time.Now().UTC()).
		Delete(&models.IdempotencyKey{})
	return res.RowsAffected, res.Error
}
