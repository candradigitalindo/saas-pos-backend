package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"gorm.io/gorm"
)

// PurchasePayment adalah satu pembayaran ke pemasok untuk satu pembelian —
// saat barang datang maupun pelunasan utang kemudian (000047). Sumbernya
// "drawer" (laci kasir; CashMovementID menunjuk uang keluarnya) atau "other"
// (dompet/rekening, laci tidak berubah).
type PurchasePayment struct {
	ID             string    `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID       string    `json:"tenant_id" gorm:"type:char(26);not null;index"`
	PurchaseID     string    `json:"purchase_id" gorm:"type:char(26);not null"`
	OutletID       string    `json:"outlet_id" gorm:"type:char(26);not null"`
	Amount         int64     `json:"amount" gorm:"not null"`
	Source         string    `json:"source" gorm:"not null"` // drawer | other
	CashMovementID *string   `json:"cash_movement_id" gorm:"type:char(26)"`
	Note           string    `json:"note"`
	PaidAt         time.Time `json:"paid_at"`
	BusinessDate   time.Time `json:"business_date" gorm:"type:date"`
	CreatedBy      string    `json:"created_by" gorm:"type:char(26);not null"`
	CreatedAt      time.Time `json:"created_at"`
}

func (p *PurchasePayment) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = ulid.New()
	}
	return
}
