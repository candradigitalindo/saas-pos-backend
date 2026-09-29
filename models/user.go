package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"gorm.io/gorm"
)

// User mendefinisikan skema tabel 'users' (migrasi 000001 + 000003).
type User struct {
	ID       string `json:"id" gorm:"primaryKey;type:char(26)"`   // Primary key ULID
	TenantID string `json:"tenant_id" gorm:"type:char(26);index"` // Tenant pemilik user; kosong hanya untuk user platform
	Name     string `json:"name" gorm:"not null"`
	Username string `json:"username" gorm:"not null"` // Unik di antara baris hidup (partial unique index)
	Email    string `json:"email" gorm:"not null"`    // Unik di antara baris hidup (partial unique index)
	Password string `json:"-" gorm:"not null"`        // Hash bcrypt
	PinHash  string `json:"-" gorm:"column:pin_hash"` // Hash bcrypt PIN ganti kasir cepat; opsional

	RoleID string `json:"role_id" gorm:"type:char(26);index"` // FK ke roles.id
	Role   Role   `json:"role" gorm:"foreignKey:RoleID"`      // Relasi ke Role

	IsActive    bool       `json:"is_active" gorm:"not null"` // false = akun dinonaktifkan, tidak bisa login
	LastLoginAt *time.Time `json:"last_login_at"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"` // Soft delete (§3.5)
}

// BeforeCreate meng-generate ULID baru untuk field ID bila belum diisi.
// ID user dibuat SERVER — user bukan entitas yang lahir offline.
func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == "" {
		u.ID = ulid.New()
	}
	return
}
