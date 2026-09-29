package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Enum sales lapangan (cermin CHECK migrasi 000020/000021).
var (
	VisitPlanStatuses = []string{"planned", "running", "done"}
	VisitResults      = []string{"pending", "order", "no_order", "closed", "rejected"}
	CommissionStatus  = []string{"draft", "approved", "paid"}
)

// VisitPlan adalah rencana kunjungan harian (call plan) seorang sales.
// UNIQUE (tenant, owner, plan_date) — satu rencana per orang per hari.
type VisitPlan struct {
	ID        string    `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID  string    `json:"tenant_id" gorm:"type:char(26);not null;index"`
	OwnerID   string    `json:"owner_id" gorm:"type:char(26);not null;index"`
	PlanDate  time.Time `json:"plan_date" gorm:"type:date"`
	Status    string    `json:"status" gorm:"not null;default:planned"`
	CreatedAt time.Time `json:"created_at"`

	Visits []Visit `json:"visits,omitempty" gorm:"foreignKey:VisitPlanID;references:ID"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (p *VisitPlan) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = ulid.New()
	}
	return
}

// Visit adalah satu kunjungan ke toko pelanggan. Dibuat KLIEN (ULID), sering
// offline → disinkron idempoten lewat /sync/push. Titik lokasi hanya direkam
// saat check-in/check-out (§E.3, UU PDP).
type Visit struct {
	ID            string           `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID      string           `json:"tenant_id" gorm:"type:char(26);not null;index"`
	VisitPlanID   *string          `json:"visit_plan_id" gorm:"type:char(26)"`
	CustomerID    string           `json:"customer_id" gorm:"type:char(26);not null"`
	OwnerID       string           `json:"owner_id" gorm:"type:char(26);not null;index"`
	CheckinAt     *time.Time       `json:"checkin_at"`
	CheckoutAt    *time.Time       `json:"checkout_at"`
	CheckinLat    *decimal.Decimal `json:"checkin_lat" gorm:"type:numeric(9,6)"`
	CheckinLng    *decimal.Decimal `json:"checkin_lng" gorm:"type:numeric(9,6)"`
	PhotoURL      string           `json:"photo_url"`
	Result        string           `json:"result" gorm:"not null;default:pending"`
	NoOrderReason string           `json:"no_order_reason"`
	SaleID        *string          `json:"sale_id" gorm:"type:char(26)"`
	BusinessDate  time.Time        `json:"business_date" gorm:"type:date"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	SyncVersion int64 `json:"sync_version" gorm:"->;column:sync_version"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (v *Visit) BeforeCreate(tx *gorm.DB) (err error) {
	if v.ID == "" {
		v.ID = ulid.New()
	}
	return
}

// SalesTarget adalah target seorang sales pada satu periode.
type SalesTarget struct {
	ID           string    `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID     string    `json:"tenant_id" gorm:"type:char(26);not null;index"`
	UserID       string    `json:"user_id" gorm:"type:char(26);not null;index"`
	PeriodStart  time.Time `json:"period_start" gorm:"type:date"`
	PeriodEnd    time.Time `json:"period_end" gorm:"type:date"`
	TargetAmount int64     `json:"target_amount" gorm:"not null;default:0"`
	TargetVisits int       `json:"target_visits" gorm:"not null;default:0"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (t *SalesTarget) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == "" {
		t.ID = ulid.New()
	}
	return
}

// Commission adalah komisi seorang sales pada satu periode. `base_amount`
// adalah nilai TERTAGIH (pembayaran non-kredit + setoran piutang yang benar
// benar masuk), bukan nilai terkirim.
type Commission struct {
	ID          string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID    string          `json:"tenant_id" gorm:"type:char(26);not null;index"`
	UserID      string          `json:"user_id" gorm:"type:char(26);not null;index"`
	PeriodStart time.Time       `json:"period_start" gorm:"type:date"`
	PeriodEnd   time.Time       `json:"period_end" gorm:"type:date"`
	BaseAmount  int64           `json:"base_amount" gorm:"not null;default:0"`
	Rate        decimal.Decimal `json:"rate" gorm:"type:numeric(7,4);not null"`
	Amount      int64           `json:"amount" gorm:"not null;default:0"`
	Status      string          `json:"status" gorm:"not null;default:draft"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (c *Commission) BeforeCreate(tx *gorm.DB) (err error) {
	if c.ID == "" {
		c.ID = ulid.New()
	}
	return
}
