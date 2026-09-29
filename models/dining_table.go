package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"gorm.io/gorm"
)

// DiningTableStatuses adalah nilai `status` yang sah (cermin CHECK migrasi 000005).
var DiningTableStatuses = []string{"free", "occupied", "reserved"}

// DiningTable adalah meja di sebuah outlet F&B (nomor meja & open bill).
// Nama unik per outlet.
type DiningTable struct {
	ID       string `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID string `json:"tenant_id" gorm:"type:char(26);not null;index"`
	OutletID string `json:"outlet_id" gorm:"type:char(26);not null;index"`
	Name     string `json:"name" gorm:"not null"`
	Capacity *int   `json:"capacity"`
	Status   string `json:"status" gorm:"not null;default:free"` // salah satu DiningTableStatuses

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (d *DiningTable) BeforeCreate(tx *gorm.DB) (err error) {
	if d.ID == "" {
		d.ID = ulid.New()
	}
	return
}
