package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// PriceListKinds adalah nilai `kind` yang sah (cermin CHECK migrasi 000005).
var PriceListKinds = []string{"retail", "wholesale", "member", "channel"}

// PriceList adalah satu daftar harga: eceran, grosir, member, atau per kanal.
// Hanya boleh ada satu IsDefault per tenant (partial unique index).
type PriceList struct {
	ID        string  `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID  string  `json:"tenant_id" gorm:"type:char(26);not null;index"`
	Name      string  `json:"name" gorm:"not null"`
	Kind      string  `json:"kind" gorm:"not null"`            // salah satu PriceListKinds
	ChannelID *string `json:"channel_id" gorm:"type:char(26)"` // diisi bila kind='channel'; FK menyusul Fase 11a
	IsDefault bool    `json:"is_default" gorm:"not null;default:false"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// SyncVersion diisi pemicu `bump_sync_version` (migrasi 000014); read-only.
	SyncVersion int64 `json:"sync_version" gorm:"->;column:sync_version"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (p *PriceList) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = ulid.New()
	}
	return
}

// ProductPrice adalah harga sebuah produk (opsional per varian) pada sebuah
// daftar harga, dengan ambang kuantitas MinQty untuk harga bertingkat.
// Tidak ber-soft-delete (baris dihapus keras saat harga dicabut).
type ProductPrice struct {
	ID          string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID    string          `json:"tenant_id" gorm:"type:char(26);not null;index"`
	ProductID   string          `json:"product_id" gorm:"type:char(26);not null;index"`
	VariantID   *string         `json:"variant_id" gorm:"type:char(26)"`
	PriceListID string          `json:"price_list_id" gorm:"type:char(26);not null;index"`
	MinQty      decimal.Decimal `json:"min_qty" gorm:"type:numeric(14,3);not null;default:1"`
	Price       int64           `json:"price" gorm:"not null"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// SyncVersion diisi pemicu `bump_sync_version` (migrasi 000014); read-only.
	SyncVersion int64 `json:"sync_version" gorm:"->;column:sync_version"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (p *ProductPrice) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = ulid.New()
	}
	return
}
