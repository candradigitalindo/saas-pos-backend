package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"gorm.io/gorm"
)

// Role mendefinisikan skema untuk tabel 'roles' (migrasi 000001).
type Role struct {
	ID   string `json:"id" gorm:"primaryKey;type:char(26)"`
	Name string `json:"name" gorm:"not null"` // Unik di antara baris hidup (partial unique index uq_roles_name)

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"` // Soft delete (§3.5)
}

// BeforeCreate meng-generate ULID baru untuk field ID bila belum diisi.
func (r *Role) BeforeCreate(tx *gorm.DB) (err error) {
	if r.ID == "" {
		r.ID = ulid.New()
	}
	return
}
