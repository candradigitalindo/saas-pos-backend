package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Purchase adalah penerimaan barang dari supplier — stok masuk. Menghasilkan
// gerakan kind='purchase' dan memperbarui harga modal produk (last-cost).
type Purchase struct {
	ID             string     `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID       string     `json:"tenant_id" gorm:"type:char(26);not null;index"`
	OutletID       string     `json:"outlet_id" gorm:"type:char(26);not null"`
	SupplierID     *string    `json:"supplier_id" gorm:"type:char(26)"`
	InvoiceNo      string     `json:"invoice_no"`
	IdempotencyKey string     `json:"-" gorm:"not null"`
	Status         string     `json:"status" gorm:"not null;default:received"` // draft|received|canceled
	Subtotal       int64      `json:"subtotal" gorm:"not null;default:0"`
	DiscountAmount int64      `json:"discount_amount" gorm:"not null;default:0"`
	TaxAmount      int64      `json:"tax_amount" gorm:"not null;default:0"`
	Total          int64      `json:"total" gorm:"not null;default:0"`
	PaidAmount     int64      `json:"paid_amount" gorm:"not null;default:0"`
	DueDate        *time.Time `json:"due_date" gorm:"type:date"`
	OccurredAt     time.Time  `json:"occurred_at"`
	BusinessDate   time.Time  `json:"business_date" gorm:"type:date"`
	CreatedBy      string     `json:"created_by" gorm:"type:char(26);not null"`

	Items []PurchaseItem `json:"items,omitempty" gorm:"foreignKey:PurchaseID;references:ID"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (p *Purchase) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = ulid.New()
	}
	return
}

// PurchaseItem adalah satu baris penerimaan. unit_cost = harga modal saat beli
// (di-snapshot).
type PurchaseItem struct {
	ID         string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID   string          `json:"tenant_id" gorm:"type:char(26);not null;index"`
	PurchaseID string          `json:"purchase_id" gorm:"type:char(26);not null;index"`
	ProductID  string          `json:"product_id" gorm:"type:char(26);not null"`
	VariantID  *string         `json:"variant_id" gorm:"type:char(26)"`
	Qty        decimal.Decimal `json:"qty" gorm:"type:numeric(14,3);not null"`
	UnitCost   int64           `json:"unit_cost" gorm:"not null"`
	LineTotal  int64           `json:"line_total" gorm:"not null"`
}

func (i *PurchaseItem) BeforeCreate(tx *gorm.DB) (err error) {
	if i.ID == "" {
		i.ID = ulid.New()
	}
	return
}
