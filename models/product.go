package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Product adalah barang/jasa yang dijual. Harga dalam rupiah bulat (BIGINT →
// int64, §3.3); min_stock kuantitas desimal (timbangan).
//
// CategoryID/SKU/Barcode *string: NULL bila kosong — SKU & Barcode terkena
// partial unique index (WHERE ... IS NOT NULL), jadi "" tidak boleh dipakai
// sebagai "tidak ada".
type Product struct {
	ID         string  `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID   string  `json:"tenant_id" gorm:"type:char(26);not null;index"`
	CategoryID *string `json:"category_id" gorm:"type:char(26)"`
	UnitID     string  `json:"unit_id" gorm:"type:char(26);not null"`

	Name    string  `json:"name" gorm:"not null"`
	SKU     *string `json:"sku" gorm:"column:sku"`
	Barcode *string `json:"barcode"`

	SellPrice  int64           `json:"sell_price" gorm:"not null;default:0"`
	CostPrice  int64           `json:"cost_price" gorm:"not null;default:0"` // wajib agar laba bisa dihitung
	TrackStock bool            `json:"track_stock" gorm:"not null;default:true"`
	MinStock   decimal.Decimal `json:"min_stock" gorm:"type:numeric(14,3);not null;default:0"`
	IsActive   bool            `json:"is_active" gorm:"not null;default:true"`
	ImageURL   string          `json:"image_url"`

	// Diisi lewat Joins pada query list/detail; kosong pada operasi tulis.
	Unit     *Unit     `json:"unit,omitempty" gorm:"foreignKey:UnitID;references:ID"`
	Category *Category `json:"category,omitempty" gorm:"foreignKey:CategoryID;references:ID"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (p *Product) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = ulid.New()
	}
	return
}

// ProductVariant adalah varian sebuah produk (Besar/Kecil, level pedas).
// PriceDelta = selisih harga terhadap produk induk.
type ProductVariant struct {
	ID         string  `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID   string  `json:"tenant_id" gorm:"type:char(26);not null;index"`
	ProductID  string  `json:"product_id" gorm:"type:char(26);not null;index"`
	Name       string  `json:"name" gorm:"not null"`
	SKU        *string `json:"sku" gorm:"column:sku"`
	Barcode    *string `json:"barcode"`
	PriceDelta int64   `json:"price_delta" gorm:"not null;default:0"`
	IsActive   bool    `json:"is_active" gorm:"not null;default:true"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (v *ProductVariant) BeforeCreate(tx *gorm.DB) (err error) {
	if v.ID == "" {
		v.ID = ulid.New()
	}
	return
}
