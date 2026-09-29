package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Recipe adalah bill-of-material sebuah menu jadi (F&B): saat menu terjual,
// stok bahan baku dipotong sesuai RecipeItem. Satu resep per produk.
type Recipe struct {
	ID        string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID  string          `json:"tenant_id" gorm:"type:char(26);not null;index"`
	ProductID string          `json:"product_id" gorm:"type:char(26);not null"`
	YieldQty  decimal.Decimal `json:"yield_qty" gorm:"type:numeric(14,3);not null;default:1"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (r *Recipe) BeforeCreate(tx *gorm.DB) (err error) {
	if r.ID == "" {
		r.ID = ulid.New()
	}
	return
}

// RecipeItem adalah satu bahan baku dalam sebuah resep. Tabel ringkas: tanpa
// timestamp, tanpa soft delete.
type RecipeItem struct {
	ID                  string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID            string          `json:"tenant_id" gorm:"type:char(26);not null;index"`
	RecipeID            string          `json:"recipe_id" gorm:"type:char(26);not null;index"`
	IngredientProductID string          `json:"ingredient_product_id" gorm:"type:char(26);not null"`
	Qty                 decimal.Decimal `json:"qty" gorm:"type:numeric(14,3);not null"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (ri *RecipeItem) BeforeCreate(tx *gorm.DB) (err error) {
	if ri.ID == "" {
		ri.ID = ulid.New()
	}
	return
}
