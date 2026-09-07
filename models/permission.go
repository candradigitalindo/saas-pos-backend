package models

import (
	"candra/backend-api/internal/ulid"

	"gorm.io/gorm"
)

// Permission adalah satu kode hak akses di katalog global (§5.3).
// Bukan tabel bertenant: sama untuk semua tenant, diisi seeder.
type Permission struct {
	ID          string `json:"id" gorm:"primaryKey;type:char(26)"`
	Code        string `json:"code" gorm:"not null;uniqueIndex"` // 'sale.void', 'report.view', ...
	GroupName   string `json:"group_name" gorm:"not null"`
	Description string `json:"description" gorm:"not null"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (p *Permission) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = ulid.New()
	}
	return
}

// RolePermission adalah baris penghubung role ↔ permission. Tabel penghubung
// murni: primary key pasangan kunci, tanpa kolom id, tanpa timestamp.
type RolePermission struct {
	RoleID       string `json:"role_id" gorm:"primaryKey;type:char(26)"`
	PermissionID string `json:"permission_id" gorm:"primaryKey;type:char(26)"`
}

// TableName memastikan GORM memakai nama tabel yang benar (bukan "role_permission").
func (RolePermission) TableName() string { return "role_permissions" }

// UserOutlet memberi seorang user akses ke sebuah outlet. Tabel penghubung
// murni; tenant_id ikut disimpan agar FK-nya komposit (menutup relasi lintas
// tenant di level database).
type UserOutlet struct {
	TenantID string `json:"tenant_id" gorm:"type:char(26);not null"`
	UserID   string `json:"user_id" gorm:"primaryKey;type:char(26)"`
	OutletID string `json:"outlet_id" gorm:"primaryKey;type:char(26)"`
}

// TableName memastikan GORM memakai "user_outlets".
func (UserOutlet) TableName() string { return "user_outlets" }
