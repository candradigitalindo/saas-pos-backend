package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"gorm.io/gorm"
)

// Category adalah kategori produk milik satu tenant. Maksimal dua tingkat
// (induk → anak); kedalaman dijaga di service, bukan skema.
//
// ParentID *string: harus NULL (bukan "") saat kategori tingkat atas — kolomnya
// FK komposit ke categories, dan "" bukan id yang sah.
type Category struct {
	ID        string  `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID  string  `json:"tenant_id" gorm:"type:char(26);not null;index"`
	ParentID  *string `json:"parent_id" gorm:"type:char(26)"`
	Name      string  `json:"name" gorm:"not null"`
	SortOrder int     `json:"sort_order" gorm:"not null;default:0"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (c *Category) BeforeCreate(tx *gorm.DB) (err error) {
	if c.ID == "" {
		c.ID = ulid.New()
	}
	return
}
