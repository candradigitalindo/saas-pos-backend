package models

import (
	"encoding/json"
	"time"

	"candra/backend-api/internal/ulid"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Enum kanal (cermin CHECK migrasi 000022/000023).
var (
	ChannelKinds       = []string{"pos", "marketplace", "delivery_app", "conversation"}
	ChannelIntegration = []string{"manual", "csv", "api"}
	// ChannelOrderStatuses adalah status pesanan kanal yang kita kelola pada
	// `sales.status` (subset enum §5.5 yang relevan untuk alur kanal).
	ChannelOrderStatuses = []string{"pending", "accepted", "preparing", "ready", "shipped", "completed", "rejected", "canceled"}
)

// Channel adalah satu kanal penjualan online milik tenant, terikat ke satu
// outlet. `CredentialsEncrypted` disiapkan untuk adaptor API (Fase 11b) —
// belum dipakai di 11a. Tidak pernah dikembalikan lewat API.
type Channel struct {
	ID                   string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID             string          `json:"tenant_id" gorm:"type:char(26);not null;index"`
	OutletID             string          `json:"outlet_id" gorm:"type:char(26);not null"`
	Kind                 string          `json:"kind" gorm:"not null"`
	Provider             string          `json:"provider" gorm:"not null"`
	Name                 string          `json:"name" gorm:"not null"`
	MerchantRef          string          `json:"merchant_ref"`
	CredentialsEncrypted []byte          `json:"-" gorm:"column:credentials_encrypted"`
	CommissionRate       decimal.Decimal `json:"commission_rate" gorm:"type:numeric(7,4);not null;default:0"`
	PriceListID          *string         `json:"price_list_id" gorm:"type:char(26)"`
	IntegrationMode      string          `json:"integration_mode" gorm:"not null;default:manual"`
	IsActive             bool            `json:"is_active" gorm:"not null;default:true"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (c *Channel) BeforeCreate(tx *gorm.DB) (err error) {
	if c.ID == "" {
		c.ID = ulid.New()
	}
	return
}

// ChannelProduct memetakan SKU kanal ke produk internal. `channel_price`
// menimpa harga master untuk kanal itu bila diisi.
type ChannelProduct struct {
	ID                string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID          string          `json:"tenant_id" gorm:"type:char(26);not null;index"`
	ChannelID         string          `json:"channel_id" gorm:"type:char(26);not null;index"`
	ProductID         string          `json:"product_id" gorm:"type:char(26);not null;index"`
	VariantID         *string         `json:"variant_id" gorm:"type:char(26)"`
	ExternalSKU       string          `json:"external_sku" gorm:"not null"`
	ExternalProductID string          `json:"external_product_id"`
	ChannelPrice      *int64          `json:"channel_price"`
	IsAvailable       bool            `json:"is_available" gorm:"not null;default:true"`
	StockBuffer       decimal.Decimal `json:"stock_buffer" gorm:"type:numeric(14,3);not null;default:0"`
	LastSyncedAt      *time.Time      `json:"last_synced_at"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (p *ChannelProduct) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = ulid.New()
	}
	return
}

// TableName tetap eksplisit.
func (ChannelProduct) TableName() string { return "channel_products" }

// ChannelOrder adalah metadata khas kanal atas sebuah `sales`. Satu pesanan =
// satu sales = satu ChannelOrder; `UNIQUE (tenant, channel, external_order_id)`
// mencegah pesanan ganda menjadi dua penjualan.
type ChannelOrder struct {
	ID              string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID        string          `json:"tenant_id" gorm:"type:char(26);not null;index"`
	ChannelID       string          `json:"channel_id" gorm:"type:char(26);not null;index"`
	SaleID          string          `json:"sale_id" gorm:"type:char(26);not null"`
	ExternalOrderID string          `json:"external_order_id" gorm:"not null"`
	ExternalStatus  string          `json:"external_status"`
	BuyerName       string          `json:"buyer_name"`
	BuyerPhone      string          `json:"buyer_phone"`
	ShippingAddress string          `json:"shipping_address"`
	Courier         string          `json:"courier"`
	TrackingNo      string          `json:"tracking_no"`
	DriverName      string          `json:"driver_name"`
	AcceptedAt      *time.Time      `json:"accepted_at"`
	ReadyAt         *time.Time      `json:"ready_at"`
	CompletedAt     *time.Time      `json:"completed_at"`
	RawPayload      json.RawMessage `json:"raw_payload,omitempty" gorm:"type:jsonb"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (o *ChannelOrder) BeforeCreate(tx *gorm.DB) (err error) {
	if o.ID == "" {
		o.ID = ulid.New()
	}
	return
}
