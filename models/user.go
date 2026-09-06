package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"gorm.io/gorm"
)

// User mendefinisikan skema untuk tabel 'users' (migrasi 000001).
type User struct {
	ID       string `json:"id" gorm:"primaryKey;type:char(26)"` // Primary key ULID
	Name     string `json:"name" gorm:"not null"`               // Nama lengkap user
	Username string `json:"username" gorm:"not null"`           // Username login. Unik di antara baris hidup (partial unique index uq_users_username)
	Email    string `json:"email" gorm:"not null"`              // Email. Unik di antara baris hidup (partial unique index uq_users_email)
	Password string `json:"-" gorm:"not null"`                  // Hash bcrypt; diabaikan saat serialisasi JSON
	RoleID   string `json:"role_id" gorm:"type:char(26);index"` // FK ke roles.id (di-index untuk JOIN & pengecekan FK)
	Role     Role   `json:"role" gorm:"foreignKey:RoleID"`      // Relasi ke Role

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"` // Soft delete: GORM otomatis menyaring baris terhapus (§3.5)
}

// BeforeCreate meng-generate ULID baru untuk field ID bila belum diisi.
// ID user dibuat SERVER (bukan klien) — user bukan entitas yang lahir offline.
func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == "" {
		u.ID = ulid.New()
	}
	return
}
