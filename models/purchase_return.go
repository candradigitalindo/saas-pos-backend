package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// PurchaseReturn adalah retur barang ke pemasok atas satu nota (000049).
// Total nota berkurang sebesar Total; bila yang sudah dibayar melebihi total
// baru, pemasok mengembalikan RefundAmount — ke laci (CashMovementID) atau
// uang lain.
type PurchaseReturn struct {
	ID             string    `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID       string    `json:"tenant_id" gorm:"type:char(26);not null;index"`
	OutletID       string    `json:"outlet_id" gorm:"type:char(26);not null"`
	PurchaseID     string    `json:"purchase_id" gorm:"type:char(26);not null"`
	Reason         string    `json:"reason" gorm:"not null"`
	Total          int64     `json:"total" gorm:"not null"`
	RefundAmount   int64     `json:"refund_amount" gorm:"not null;default:0"`
	RefundSource   *string   `json:"refund_source"`
	CashMovementID *string   `json:"cash_movement_id" gorm:"type:char(26)"`
	OccurredAt     time.Time `json:"occurred_at"`
	BusinessDate   time.Time `json:"business_date" gorm:"type:date"`
	CreatedBy      string    `json:"created_by" gorm:"type:char(26);not null"`
	CreatedAt      time.Time `json:"created_at"`

	Items []PurchaseReturnItem `json:"items,omitempty" gorm:"foreignKey:ReturnID;references:ID"`
}

func (r *PurchaseReturn) BeforeCreate(tx *gorm.DB) (err error) {
	if r.ID == "" {
		r.ID = ulid.New()
	}
	return
}

// PurchaseReturnItem: satu baris nota yang diretur, dalam satuan beli baris itu.
type PurchaseReturnItem struct {
	ID             string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID       string          `json:"tenant_id" gorm:"type:char(26);not null;index"`
	ReturnID       string          `json:"return_id" gorm:"type:char(26);not null"`
	PurchaseItemID string          `json:"purchase_item_id" gorm:"type:char(26);not null"`
	ProductID      string          `json:"product_id" gorm:"type:char(26);not null"`
	Qty            decimal.Decimal `json:"qty" gorm:"type:numeric(14,3);not null"`
	UnitConversion decimal.Decimal `json:"unit_conversion" gorm:"type:numeric(14,6);not null;default:1"`
	UnitCost       int64           `json:"unit_cost" gorm:"not null"`
	LineTotal      int64           `json:"line_total" gorm:"not null"`
}

func (i *PurchaseReturnItem) BeforeCreate(tx *gorm.DB) (err error) {
	if i.ID == "" {
		i.ID = ulid.New()
	}
	return
}
