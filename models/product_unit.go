package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ProductUnit adalah satu kemasan barang (migrasi 000046): "dus isi 40".
// Conversion = isi kemasan dalam SATUAN DASAR barang (> 1). SellPrice nil =
// isi × harga jual barang. Stok selalu dihitung dalam satuan dasar.
type ProductUnit struct {
	ID         string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID   string          `json:"tenant_id" gorm:"type:char(26);not null"`
	ProductID  string          `json:"product_id" gorm:"type:char(26);not null"`
	UnitID     string          `json:"unit_id" gorm:"type:char(26);not null"`
	Conversion decimal.Decimal `json:"conversion" gorm:"type:numeric(14,6);not null"`
	SellPrice  *int64          `json:"sell_price"`
	Barcode    *string         `json:"barcode"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// SyncVersion diisi pemicu `bump_sync_version`; read-only.
	SyncVersion int64 `json:"sync_version" gorm:"->;column:sync_version"`

	// Unit dimuat lewat Preload untuk nama satuan.
	Unit *Unit `json:"unit,omitempty" gorm:"foreignKey:UnitID;references:ID"`
}

func (p *ProductUnit) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = ulid.New()
	}
	return
}

// HargaKemasan: harga jual satu kemasan — harga tersendiri, atau isi × harga
// jual barang (dibulatkan ke rupiah).
func (p ProductUnit) HargaKemasan(hargaJual int64) int64 {
	if p.SellPrice != nil {
		return *p.SellPrice
	}
	return p.Conversion.Mul(decimal.NewFromInt(hargaJual)).Round(0).IntPart()
}
