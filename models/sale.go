package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

var (
	SaleOrderTypes     = []string{"dine_in", "takeaway", "delivery", "pickup"}
	SalePaymentMethods = []string{"cash", "qris", "transfer", "card", "ewallet", "credit"}
)

// Sale adalah satu transaksi kasir. Semua nilai uang BIGINT rupiah bulat, sudah
// dihitung dan dibulatkan per baris lalu dijumlahkan (§3.3). Baris 'returned'
// menyimpan nilai NEGATIF dan menunjuk ReturnOfSaleID.
type Sale struct {
	ID              string  `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID        string  `json:"tenant_id" gorm:"type:char(26);not null;index"`
	OutletID        string  `json:"outlet_id" gorm:"type:char(26);not null"`
	ShiftID         *string `json:"shift_id" gorm:"type:char(26)"`
	ChannelID       *string `json:"channel_id" gorm:"type:char(26)"`
	CustomerID      *string `json:"customer_id" gorm:"type:char(26)"`
	TableID         *string `json:"table_id" gorm:"type:char(26)"`
	ReceiptNo       string  `json:"receipt_no" gorm:"not null"`
	ExternalOrderID *string `json:"external_order_id"`
	IdempotencyKey  string  `json:"-" gorm:"not null"`
	OrderType       string  `json:"order_type" gorm:"not null;default:dine_in"`
	Status          string  `json:"status" gorm:"not null;default:completed"`

	Subtotal       int64 `json:"subtotal" gorm:"not null;default:0"`
	DiscountAmount int64 `json:"discount_amount" gorm:"not null;default:0"`
	TaxAmount      int64 `json:"tax_amount" gorm:"not null;default:0"`
	ServiceAmount  int64 `json:"service_amount" gorm:"not null;default:0"`
	RoundingAmount int64 `json:"rounding_amount" gorm:"not null;default:0"`
	Total          int64 `json:"total" gorm:"not null;default:0"`
	PaidAmount     int64 `json:"paid_amount" gorm:"not null;default:0"`
	ChangeAmount   int64 `json:"change_amount" gorm:"not null;default:0"`
	CostTotal      int64 `json:"cost_total" gorm:"not null;default:0"`

	ReturnOfSaleID  *string    `json:"return_of_sale_id" gorm:"type:char(26)"`
	Note            string     `json:"note"`
	OccurredAt      time.Time  `json:"occurred_at"`
	ClientCreatedAt *time.Time `json:"client_created_at"`
	BusinessDate    time.Time  `json:"business_date" gorm:"type:date"`
	VoidedAt        *time.Time `json:"voided_at"`
	VoidedBy        *string    `json:"voided_by" gorm:"type:char(26)"`
	VoidReason      string     `json:"void_reason"`
	CreatedBy       string     `json:"created_by" gorm:"type:char(26);not null"`
	// ReceiptToken: token acak tautan struk digital (migrasi 000045); kosong
	// sampai struk pertama kali dibagikan. Tidak pernah dikirim di JSON biasa.
	ReceiptToken *string `json:"-" gorm:"column:receipt_token"`

	Items    []SaleItem    `json:"items,omitempty" gorm:"foreignKey:SaleID;references:ID"`
	Payments []SalePayment `json:"payments,omitempty" gorm:"foreignKey:SaleID;references:ID"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *Sale) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == "" {
		s.ID = ulid.New()
	}
	return
}

// SaleItem adalah satu baris keranjang, dengan SNAPSHOT nama/harga/modal saat
// transaksi supaya laporan lama tidak berubah saat master data diubah.
type SaleItem struct {
	ID             string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID       string          `json:"tenant_id" gorm:"type:char(26);not null;index"`
	SaleID         string          `json:"sale_id" gorm:"type:char(26);not null;index"`
	ProductID      string          `json:"product_id" gorm:"type:char(26);not null"`
	VariantID      *string         `json:"variant_id" gorm:"type:char(26)"`
	ProductName    string          `json:"product_name" gorm:"not null"`
	UnitName       string          `json:"unit_name" gorm:"not null"`
	Qty            decimal.Decimal `json:"qty" gorm:"type:numeric(14,3);not null"`
	UnitPrice      int64           `json:"unit_price" gorm:"not null"`
	UnitCost       int64           `json:"unit_cost" gorm:"not null"`
	DiscountAmount int64           `json:"discount_amount" gorm:"not null;default:0"`
	TaxAmount      int64           `json:"tax_amount" gorm:"not null;default:0"`
	LineTotal      int64           `json:"line_total" gorm:"not null"`
	Note           string          `json:"note"`
	// Kemasan yang dijual (000046): Qty dalam kemasan itu, stok bergerak
	// Qty × UnitConversion satuan dasar. Tanpa kemasan: nil & 1.
	ProductUnitID  *string         `json:"product_unit_id" gorm:"type:char(26)"`
	UnitConversion decimal.Decimal `json:"unit_conversion" gorm:"type:numeric(14,6);not null;default:1"`
	CreatedAt      time.Time       `json:"created_at"`
}

func (i *SaleItem) BeforeCreate(tx *gorm.DB) (err error) {
	if i.ID == "" {
		i.ID = ulid.New()
	}
	return
}

// SalePayment adalah satu tender. method='credit' = kasbon → menimbulkan
// Receivable, tetapi tetap dihitung sebagai "dibayar" pada sale.
type SalePayment struct {
	ID        string    `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID  string    `json:"tenant_id" gorm:"type:char(26);not null;index"`
	SaleID    string    `json:"sale_id" gorm:"type:char(26);not null;index"`
	Method    string    `json:"method" gorm:"not null"`
	Amount    int64     `json:"amount" gorm:"not null"`
	Reference string    `json:"reference"`
	FeeAmount int64     `json:"fee_amount" gorm:"not null;default:0"`
	PaidAt    time.Time `json:"paid_at"`
	CreatedAt time.Time `json:"created_at"`
}

func (p *SalePayment) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = ulid.New()
	}
	return
}
