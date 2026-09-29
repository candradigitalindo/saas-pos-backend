package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"gorm.io/gorm"
)

// Supplier adalah pemasok barang milik satu tenant.
type Supplier struct {
	ID       string `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID string `json:"tenant_id" gorm:"type:char(26);not null;index"`
	Name     string `json:"name" gorm:"not null"`
	Phone    string `json:"phone"`
	Address  string `json:"address"`
	Note     string `json:"note"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (s *Supplier) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == "" {
		s.ID = ulid.New()
	}
	return
}
