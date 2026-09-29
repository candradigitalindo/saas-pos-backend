package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// StockTransfer memindahkan stok antar outlet. Alur: draft → sent (keluar dari
// asal) → received (masuk ke tujuan).
type StockTransfer struct {
	ID           string     `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID     string     `json:"tenant_id" gorm:"type:char(26);not null;index"`
	FromOutletID string     `json:"from_outlet_id" gorm:"type:char(26);not null"`
	ToOutletID   string     `json:"to_outlet_id" gorm:"type:char(26);not null"`
	Status       string     `json:"status" gorm:"not null;default:draft"` // draft|sent|received|canceled
	Note         string     `json:"note"`
	SentAt       *time.Time `json:"sent_at"`
	ReceivedAt   *time.Time `json:"received_at"`
	BusinessDate time.Time  `json:"business_date" gorm:"type:date"`
	CreatedBy    string     `json:"created_by" gorm:"type:char(26);not null"`

	Items []StockTransferItem `json:"items,omitempty" gorm:"foreignKey:TransferID;references:ID"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (tr *StockTransfer) BeforeCreate(tx *gorm.DB) (err error) {
	if tr.ID == "" {
		tr.ID = ulid.New()
	}
	return
}

type StockTransferItem struct {
	ID         string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID   string          `json:"tenant_id" gorm:"type:char(26);not null;index"`
	TransferID string          `json:"transfer_id" gorm:"type:char(26);not null;index"`
	ProductID  string          `json:"product_id" gorm:"type:char(26);not null"`
	VariantID  *string         `json:"variant_id" gorm:"type:char(26)"`
	Qty        decimal.Decimal `json:"qty" gorm:"type:numeric(14,3);not null"`
}

func (i *StockTransferItem) BeforeCreate(tx *gorm.DB) (err error) {
	if i.ID == "" {
		i.ID = ulid.New()
	}
	return
}
