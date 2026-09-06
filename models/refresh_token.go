package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"gorm.io/gorm"
)

// RefreshToken mendefinisikan skema tabel 'refresh_tokens' (migrasi 000002).
//
// Model penukaran & pencabutan sesi:
//   - Yang disimpan adalah HASH token (SHA-256 hex), bukan token mentahnya.
//   - Rotasi: setiap penukaran mencabut baris lama (mengisi RevokedAt) dan
//     membuat baris baru. Bila token yang SUDAH dicabut dipakai lagi, itu tanda
//     token dicuri → seluruh sesi user dicabut (§9).
//
// Tidak memakai gorm.DeletedAt: baris refresh token tidak pernah dihapus, hanya
// ditandai dicabut lewat RevokedAt. Tidak ada kolom updated_at di tabel ini,
// jadi tidak ada field UpdatedAt di struct.
type RefreshToken struct {
	ID         string     `json:"id" gorm:"primaryKey;type:char(26)"`
	UserID     string     `json:"user_id" gorm:"type:char(26);not null;index"` // FK ke users.id (ON DELETE CASCADE di DB)
	TokenHash  string     `json:"-" gorm:"not null;uniqueIndex"`               // SHA-256 hex; jangan pernah diserialisasi
	DeviceName string     `json:"device_name"`                                 // label perangkat untuk daftar "sesi aktif"
	ExpiresAt  time.Time  `json:"expires_at"`                                  // kedaluwarsa absolut
	RevokedAt  *time.Time `json:"revoked_at"`                                  // non-nil = dicabut / sudah dirotasi
	CreatedAt  time.Time  `json:"created_at"`
}

// BeforeCreate meng-generate ULID untuk primary key bila belum diisi.
func (t *RefreshToken) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == "" {
		t.ID = ulid.New()
	}
	return
}

// IsActive melaporkan apakah token masih bisa dipakai pada waktu `now`:
// belum dicabut dan belum kedaluwarsa.
func (t *RefreshToken) IsActive(now time.Time) bool {
	return t.RevokedAt == nil && now.Before(t.ExpiresAt)
}
