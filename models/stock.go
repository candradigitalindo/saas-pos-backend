package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// StockMovementKinds adalah nilai `kind` yang sah (cermin CHECK migrasi 000008).
var StockMovementKinds = []string{
	"sale", "void", "refund", "purchase", "adjustment",
	"transfer_in", "transfer_out", "opname", "recipe", "initial",
}

// StockMovement adalah satu baris buku besar stok — SUMBER KEBENARAN (aturan #9).
// Tidak pernah diubah/dihapus; pembalikan selalu berupa baris baru.
type StockMovement struct {
	ID           string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID     string          `json:"tenant_id" gorm:"type:char(26);not null;index"`
	OutletID     string          `json:"outlet_id" gorm:"type:char(26);not null"`
	ProductID    string          `json:"product_id" gorm:"type:char(26);not null"`
	VariantID    *string         `json:"variant_id" gorm:"type:char(26)"`
	Kind         string          `json:"kind" gorm:"not null"`
	QtyDelta     decimal.Decimal `json:"qty_delta" gorm:"type:numeric(14,3);not null"`     // negatif = keluar
	BalanceAfter decimal.Decimal `json:"balance_after" gorm:"type:numeric(14,3);not null"` // saldo setelah gerakan ini
	UnitCost     int64           `json:"unit_cost" gorm:"not null;default:0"`
	RefTable     string          `json:"ref_table"`
	RefID        *string         `json:"ref_id" gorm:"type:char(26)"`
	Reason       string          `json:"reason"`
	OccurredAt   time.Time       `json:"occurred_at"`
	BusinessDate time.Time       `json:"business_date" gorm:"type:date"`
	CreatedBy    *string         `json:"created_by" gorm:"type:char(26)"`
	CreatedAt    time.Time       `json:"created_at"`
}

func (m *StockMovement) BeforeCreate(tx *gorm.DB) (err error) {
	if m.ID == "" {
		m.ID = ulid.New()
	}
	return
}

// Stock adalah CACHE saldo per (tenant, outlet, product, variant). Boleh
// dihitung ulang kapan saja dari StockMovement. Tanpa id, tanpa hook.
// VariantID memakai "" untuk "tanpa varian" (bukan NULL) agar PK & ON CONFLICT
// sederhana.
type Stock struct {
	TenantID  string `json:"tenant_id" gorm:"primaryKey;type:char(26)"`
	OutletID  string `json:"outlet_id" gorm:"primaryKey;type:char(26)"`
	ProductID string `json:"product_id" gorm:"primaryKey;type:char(26)"`
	// TEXT (bukan char(26)): '' adalah sentinel "tanpa varian", char akan
	// mem-padding-nya jadi 26 spasi.
	VariantID string `json:"variant_id" gorm:"primaryKey;type:text;default:''"`

	Qty         decimal.Decimal `json:"qty" gorm:"type:numeric(14,3);not null;default:0"`
	ReservedQty decimal.Decimal `json:"reserved_qty" gorm:"type:numeric(14,3);not null;default:0"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

func (Stock) TableName() string { return "stocks" }
