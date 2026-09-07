package models

import (
	"encoding/json"
	"time"

	"candra/backend-api/internal/ulid"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Enum yang sah (cermin CHECK migrasi 000015). Dipakai lapisan validasi service.
var (
	SubscriptionStatuses  = []string{"trial", "active", "past_due", "canceled", "expired"}
	SubscriptionTerms     = []int{1, 3, 6, 9, 12}
	SubInvoiceStatuses    = []string{"open", "paid", "overdue", "void", "refunded"}
	SubscriptionAddonCode = []string{"crm_freelance", "crm_sales", "online_channel"}
)

// Plan adalah paket langganan platform. Tabel PLATFORM (tanpa tenant_id, tanpa
// RLS). `Features` JSON bebas: {"qris":true,"crm_freelance":false}.
type Plan struct {
	ID                     string          `json:"id" gorm:"primaryKey;type:char(26)"`
	Code                   string          `json:"code" gorm:"not null;uniqueIndex"`
	Name                   string          `json:"name" gorm:"not null"`
	MonthlyPrice           int64           `json:"monthly_price" gorm:"not null"`
	MaxOutlets             *int            `json:"max_outlets"`
	MaxUsers               *int            `json:"max_users"`
	MaxProducts            *int            `json:"max_products"`
	MaxMonthlyTransactions *int            `json:"max_monthly_transactions"`
	Features               json.RawMessage `json:"features" gorm:"type:jsonb;not null;default:'{}'"`
	IsActive               bool            `json:"is_active" gorm:"not null;default:true"`
	CreatedAt              time.Time       `json:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (p *Plan) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = ulid.New()
	}
	return
}

// PlanTermDiscount adalah tangga diskon prabayar (1/3/6/9/12 bulan). Global.
type PlanTermDiscount struct {
	ID           string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TermMonths   int             `json:"term_months" gorm:"not null;uniqueIndex"`
	DiscountRate decimal.Decimal `json:"discount_rate" gorm:"type:numeric(7,4);not null"` // 0.1670 = 16,7%
	IsActive     bool            `json:"is_active" gorm:"not null;default:true"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (d *PlanTermDiscount) BeforeCreate(tx *gorm.DB) (err error) {
	if d.ID == "" {
		d.ID = ulid.New()
	}
	return
}

// Subscription adalah langganan satu tenant. UNIQUE(tenant_id) — satu per tenant.
// `tenant_id` adalah penunjuk pelanggan, bukan pembatas akses (tabel platform).
type Subscription struct {
	ID                 string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID           string          `json:"tenant_id" gorm:"type:char(26);not null;uniqueIndex"`
	PlanID             string          `json:"plan_id" gorm:"type:char(26);not null"`
	TermMonths         int             `json:"term_months" gorm:"not null;default:1"`
	DiscountRate       decimal.Decimal `json:"discount_rate" gorm:"type:numeric(7,4);not null;default:0"`
	Status             string          `json:"status" gorm:"not null;default:trial"`
	TrialEndsAt        *time.Time      `json:"trial_ends_at"`
	CurrentPeriodStart time.Time       `json:"current_period_start"`
	CurrentPeriodEnd   time.Time       `json:"current_period_end"`
	AutoRenew          bool            `json:"auto_renew" gorm:"not null;default:true"`
	CanceledAt         *time.Time      `json:"canceled_at"`
	CancelReason       string          `json:"cancel_reason"`

	Plan *Plan `json:"plan,omitempty" gorm:"foreignKey:PlanID;references:ID"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (s *Subscription) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == "" {
		s.ID = ulid.New()
	}
	return
}

// SubscriptionAddon adalah add-on berbayar (CRM Freelance/Sales, Kanal Online).
// Dibuat tabelnya di Fase 7; logika penagihannya menyusul.
type SubscriptionAddon struct {
	ID             string     `json:"id" gorm:"primaryKey;type:char(26)"`
	SubscriptionID string     `json:"subscription_id" gorm:"type:char(26);not null"`
	AddonCode      string     `json:"addon_code" gorm:"not null"`
	Quantity       int        `json:"quantity" gorm:"not null;default:1"`
	UnitPrice      int64      `json:"unit_price" gorm:"not null"`
	StartedAt      time.Time  `json:"started_at"`
	EndedAt        *time.Time `json:"ended_at"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (a *SubscriptionAddon) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == "" {
		a.ID = ulid.New()
	}
	return
}

// SubscriptionInvoice adalah tagihan langganan platform (BUKAN invoice pelanggan
// modul CRM). Semua nilai uang BIGINT rupiah bulat.
type SubscriptionInvoice struct {
	ID             string     `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID       string     `json:"tenant_id" gorm:"type:char(26);not null"`
	SubscriptionID string     `json:"subscription_id" gorm:"type:char(26);not null"`
	Number         string     `json:"number" gorm:"not null;uniqueIndex"`
	TermMonths     int        `json:"term_months" gorm:"not null"`
	PeriodStart    time.Time  `json:"period_start" gorm:"type:date"`
	PeriodEnd      time.Time  `json:"period_end" gorm:"type:date"`
	GrossAmount    int64      `json:"gross_amount" gorm:"not null"`
	DiscountAmount int64      `json:"discount_amount" gorm:"not null;default:0"`
	TotalAmount    int64      `json:"total_amount" gorm:"not null"`
	PaidAmount     int64      `json:"paid_amount" gorm:"not null;default:0"`
	DueDate        time.Time  `json:"due_date" gorm:"type:date"`
	Status         string     `json:"status" gorm:"not null;default:open"`
	PaidAt         *time.Time `json:"paid_at"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (i *SubscriptionInvoice) BeforeCreate(tx *gorm.DB) (err error) {
	if i.ID == "" {
		i.ID = ulid.New()
	}
	return
}

// SubscriptionPayment adalah satu pembayaran atas SubscriptionInvoice.
type SubscriptionPayment struct {
	ID                    string    `json:"id" gorm:"primaryKey;type:char(26)"`
	SubscriptionInvoiceID string    `json:"subscription_invoice_id" gorm:"type:char(26);not null"`
	Amount                int64     `json:"amount" gorm:"not null"`
	Method                string    `json:"method" gorm:"not null"`
	Reference             string    `json:"reference"`
	GatewayFee            int64     `json:"gateway_fee" gorm:"not null;default:0"`
	PaidAt                time.Time `json:"paid_at"`
	CreatedAt             time.Time `json:"created_at"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (p *SubscriptionPayment) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = ulid.New()
	}
	return
}
