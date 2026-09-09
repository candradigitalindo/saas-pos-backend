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

// Enum tambahan (Fase 11b, migrasi 000024/000025).
var (
	ChannelEventStatuses = []string{"pending", "processing", "done", "failed", "dead"}
	ChannelFeeKinds      = []string{"commission", "service", "shipping_subsidy", "merchant_promo", "tax", "other"}
	SettlementStatuses   = []string{"open", "matched", "mismatch", "closed"}
	StockSyncStatuses    = []string{"pending", "sent", "failed"}
)

// ChannelEvent adalah peristiwa MENTAH dari kanal (webhook/polling). Webhook
// tidak memproses — hanya menaruh baris di sini lalu balas 200. Pekerja
// asinkron yang memprosesnya. `UNIQUE (tenant, channel, event_type,
// external_ref)` = dedup pengiriman ganda.
type ChannelEvent struct {
	ID          string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID    string          `json:"tenant_id" gorm:"type:char(26);not null;index"`
	ChannelID   string          `json:"channel_id" gorm:"type:char(26);not null;index"`
	EventType   string          `json:"event_type" gorm:"not null"`
	ExternalRef string          `json:"external_ref"`
	Payload     json.RawMessage `json:"payload" gorm:"type:jsonb;not null"`
	Status      string          `json:"status" gorm:"not null;default:pending"`
	Attempts    int             `json:"attempts" gorm:"not null;default:0"`
	LastError   string          `json:"last_error"`
	ReceivedAt  time.Time       `json:"received_at"`
	ProcessedAt *time.Time      `json:"processed_at"`
}

func (e *ChannelEvent) BeforeCreate(tx *gorm.DB) (err error) {
	if e.ID == "" {
		e.ID = ulid.New()
	}
	return
}

// ChannelFee adalah satu komponen potongan kanal atas sebuah penjualan
// (komisi, biaya layanan, subsidi ongkir, promo, pajak). Rincian untuk
// rekonsiliasi settlement; total tetap masuk `sale_payments.fee_amount`.
type ChannelFee struct {
	ID        string    `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID  string    `json:"tenant_id" gorm:"type:char(26);not null;index"`
	SaleID    string    `json:"sale_id" gorm:"type:char(26);not null;index"`
	Kind      string    `json:"kind" gorm:"not null"`
	Amount    int64     `json:"amount" gorm:"not null"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

func (f *ChannelFee) BeforeCreate(tx *gorm.DB) (err error) {
	if f.ID == "" {
		f.ID = ulid.New()
	}
	return
}

// ChannelSettlement mencocokkan nilai pesanan sebuah periode dengan uang yang
// benar-benar masuk rekening.
type ChannelSettlement struct {
	ID             string     `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID       string     `json:"tenant_id" gorm:"type:char(26);not null;index"`
	ChannelID      string     `json:"channel_id" gorm:"type:char(26);not null"`
	PeriodStart    time.Time  `json:"period_start" gorm:"type:date"`
	PeriodEnd      time.Time  `json:"period_end" gorm:"type:date"`
	GrossAmount    int64      `json:"gross_amount" gorm:"not null;default:0"`
	FeeAmount      int64      `json:"fee_amount" gorm:"not null;default:0"`
	NetAmount      int64      `json:"net_amount" gorm:"not null;default:0"`
	ReceivedAmount *int64     `json:"received_amount"`
	Status         string     `json:"status" gorm:"not null;default:open"`
	ReceivedAt     *time.Time `json:"received_at"`
	Note           string     `json:"note"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (s *ChannelSettlement) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == "" {
		s.ID = ulid.New()
	}
	return
}

// ChannelStockSync adalah satu permintaan pembaruan stok ke kanal dalam antrean.
type ChannelStockSync struct {
	ID           string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID     string          `json:"tenant_id" gorm:"type:char(26);not null;index"`
	ChannelID    string          `json:"channel_id" gorm:"type:char(26);not null;index"`
	ProductID    string          `json:"product_id" gorm:"type:char(26);not null"`
	RequestedQty decimal.Decimal `json:"requested_qty" gorm:"type:numeric(14,3);not null"`
	Status       string          `json:"status" gorm:"not null;default:pending"`
	Attempts     int             `json:"attempts" gorm:"not null;default:0"`
	LastError    string          `json:"last_error"`
	QueuedAt     time.Time       `json:"queued_at"`
	SentAt       *time.Time      `json:"sent_at"`
}

func (s *ChannelStockSync) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == "" {
		s.ID = ulid.New()
	}
	return
}
