package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"gorm.io/gorm"
)

// Role mendefinisikan skema tabel 'roles' (migrasi 000001 + 000003).
//
// Sejak Fase 1 setiap role dimiliki satu tenant (TenantID selalu terisi).
// Kolom tenant_id di DB nullable ("NULL = peran bawaan sistem") untuk keperluan
// masa depan, tapi aplikasi tidak membuat role tanpa tenant.
type Role struct {
	ID          string `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID    string `json:"tenant_id" gorm:"type:char(26);index"`
	Name        string `json:"name" gorm:"not null"` // Unik per tenant di antara baris hidup (uq_roles_tenant_name)
	Description string `json:"description"`
	IsSystem    bool   `json:"is_system" gorm:"not null;default:false"` // true = peran bawaan tenant, tidak boleh dihapus/ganti nama

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// BeforeCreate meng-generate ULID baru untuk field ID bila belum diisi.
func (r *Role) BeforeCreate(tx *gorm.DB) (err error) {
	if r.ID == "" {
		r.ID = ulid.New()
	}
	return
}
