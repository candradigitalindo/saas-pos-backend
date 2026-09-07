package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Unit adalah satuan jual/beli produk (pcs, dus, kg). Satuan turunan menunjuk
// BaseUnitID dengan Conversion sebagai faktor (1 dus = 24 pcs → conversion 24).
//
// BaseUnitID *string: NULL bila ini satuan dasar (FK komposit ke units).
type Unit struct {
	ID           string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID     string          `json:"tenant_id" gorm:"type:char(26);not null;index"`
	Name         string          `json:"name" gorm:"not null"`
	BaseUnitID   *string         `json:"base_unit_id" gorm:"type:char(26)"`
	Conversion   decimal.Decimal `json:"conversion" gorm:"type:numeric(14,6);not null;default:1"`
	AllowDecimal bool            `json:"allow_decimal" gorm:"not null;default:false"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (u *Unit) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == "" {
		u.ID = ulid.New()
	}
	return
}
