package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// StockOpname adalah sesi hitung fisik stok satu outlet.
type StockOpname struct {
	ID           string     `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID     string     `json:"tenant_id" gorm:"type:char(26);not null;index"`
	OutletID     string     `json:"outlet_id" gorm:"type:char(26);not null"`
	Status       string     `json:"status" gorm:"not null;default:draft"` // draft|posted|canceled
	Note         string     `json:"note"`
	CountedAt    *time.Time `json:"counted_at"`
	BusinessDate time.Time  `json:"business_date" gorm:"type:date"`
	CreatedBy    string     `json:"created_by" gorm:"type:char(26);not null"`

	Items []StockOpnameItem `json:"items,omitempty" gorm:"foreignKey:OpnameID;references:ID"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (o *StockOpname) BeforeCreate(tx *gorm.DB) (err error) {
	if o.ID == "" {
		o.ID = ulid.New()
	}
	return
}

// StockOpnameItem: system_qty di-snapshot saat item dimasukkan, diff = counted -
// system. Saat opname di-post, diff ditulis sebagai gerakan kind='opname'.
type StockOpnameItem struct {
	ID         string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID   string          `json:"tenant_id" gorm:"type:char(26);not null;index"`
	OpnameID   string          `json:"opname_id" gorm:"type:char(26);not null;index"`
	ProductID  string          `json:"product_id" gorm:"type:char(26);not null"`
	VariantID  *string         `json:"variant_id" gorm:"type:char(26)"`
	SystemQty  decimal.Decimal `json:"system_qty" gorm:"type:numeric(14,3);not null"`
	CountedQty decimal.Decimal `json:"counted_qty" gorm:"type:numeric(14,3);not null"`
	DiffQty    decimal.Decimal `json:"diff_qty" gorm:"type:numeric(14,3);not null"`
}

func (i *StockOpnameItem) BeforeCreate(tx *gorm.DB) (err error) {
	if i.ID == "" {
		i.ID = ulid.New()
	}
	return
}
