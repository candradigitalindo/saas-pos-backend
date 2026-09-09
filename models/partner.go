package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Model Program Mitra Penjual (Fase 12, §16, blueprint Bagian G).
//
// Semua tabel di sini adalah lingkup PLATFORM — sejajar dengan plans/
// subscriptions, BUKAN data operasional tenant. Tidak ada RLS; isolasi antar
// mitra dijaga filter `partner_id` eksplisit di repo. Uang int64 rupiah bulat;
// tarif desimal.

// Enum (cermin CHECK migrasi 000026–000028).
var (
	PartnerKinds               = []string{"agen", "afiliasi"}
	PartnerStatuses            = []string{"pending", "active", "suspended", "terminated"}
	PartnerLeadStatuses        = []string{"baru", "dihubungi", "demo", "daftar", "gagal"}
	PartnerAttributionStatuses = []string{"active", "lapsed", "disputed", "revoked"}
	PartnerCommissionStatuses  = []string{"held", "approved", "paid", "clawed_back"}
	PartnerPayoutStatuses      = []string{"draft", "paid", "void"}
	PartnerDisputeStatuses     = []string{"open", "upheld", "rejected"}
)

// PartnerTier adalah tingkat mitra + aturan komisinya (blueprint G.5 — dapat
// diubah tanpa menyentuh kode).
type PartnerTier struct {
	ID                string          `json:"id" gorm:"primaryKey;type:char(26)"`
	Code              string          `json:"code" gorm:"not null;uniqueIndex"`
	Name              string          `json:"name" gorm:"not null"`
	Kind              string          `json:"kind" gorm:"not null"`
	CommissionRate    decimal.Decimal `json:"commission_rate" gorm:"type:numeric(7,4);not null;default:0"`
	Recurring         bool            `json:"recurring" gorm:"not null;default:true"`
	OneTimeMonths     int             `json:"one_time_months" gorm:"not null;default:0"`
	ActivationMinTxn  int             `json:"activation_min_txn" gorm:"not null;default:30"`
	ActivationMinDays int             `json:"activation_min_days" gorm:"not null;default:30"`
	AttributionDays   int             `json:"attribution_days" gorm:"not null;default:60"`
	ClawbackDays      int             `json:"clawback_days" gorm:"not null;default:90"`
	IsActive          bool            `json:"is_active" gorm:"not null;default:true"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

func (t *PartnerTier) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == "" {
		t.ID = ulid.New()
	}
	return
}

// Partner adalah satu mitra penjual (agen daerah atau afiliasi). `ReferralCode`
// unik global — dipakai calon tenant saat mendaftar. Kredensial rekening/NPWP
// tidak pernah dikembalikan lewat portal mitra.
type Partner struct {
	ID                 string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TierID             string          `json:"tier_id" gorm:"type:char(26);not null"`
	Kind               string          `json:"kind" gorm:"not null"`
	Name               string          `json:"name" gorm:"not null"`
	Region             string          `json:"region"`
	ReferralCode       string          `json:"referral_code" gorm:"not null;uniqueIndex"`
	Status             string          `json:"status" gorm:"not null;default:pending"`
	BankAccount        string          `json:"-"`
	TaxID              string          `json:"-" gorm:"column:tax_id"`
	TaxWithholdingRate decimal.Decimal `json:"tax_withholding_rate" gorm:"type:numeric(7,4);not null;default:0"`
	JoinedAt           *time.Time      `json:"joined_at" gorm:"type:date"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`

	Tier *PartnerTier `json:"tier,omitempty" gorm:"foreignKey:TierID"`
}

func (p *Partner) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = ulid.New()
	}
	return
}

// PartnerUser adalah akun login mitra — jalur autentikasi TERPISAH dari user
// tenant (blueprint G.8). Satu mitra boleh punya beberapa.
type PartnerUser struct {
	ID           string     `json:"id" gorm:"primaryKey;type:char(26)"`
	PartnerID    string     `json:"partner_id" gorm:"type:char(26);not null;index"`
	Name         string     `json:"name" gorm:"not null"`
	Email        string     `json:"email" gorm:"not null"`
	Username     string     `json:"username" gorm:"not null;uniqueIndex"`
	PasswordHash string     `json:"-" gorm:"column:password_hash;not null"`
	IsActive     bool       `json:"is_active" gorm:"not null;default:true"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`

	Partner *Partner `json:"partner,omitempty" gorm:"foreignKey:PartnerID"`
}

func (u *PartnerUser) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == "" {
		u.ID = ulid.New()
	}
	return
}

// PartnerLead adalah prospek yang didaftarkan mitra. Masa atribusi mulai
// berjalan saat prospek dibuat.
type PartnerLead struct {
	ID                   string    `json:"id" gorm:"primaryKey;type:char(26)"`
	PartnerID            string    `json:"partner_id" gorm:"type:char(26);not null;index"`
	BusinessName         string    `json:"business_name" gorm:"not null"`
	ContactName          string    `json:"contact_name"`
	ContactPhone         string    `json:"contact_phone"`
	City                 string    `json:"city"`
	BusinessType         string    `json:"business_type"`
	Status               string    `json:"status" gorm:"not null;default:baru"`
	RegisteredTenantID   *string   `json:"registered_tenant_id" gorm:"type:char(26)"`
	AttributionExpiresAt time.Time `json:"attribution_expires_at" gorm:"type:date"`
	Note                 string    `json:"note"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

func (l *PartnerLead) BeforeCreate(tx *gorm.DB) (err error) {
	if l.ID == "" {
		l.ID = ulid.New()
	}
	return
}

// PartnerReferral menautkan mitra ke tenant. Dibuat SEKALI saat tenant mendaftar
// memakai kode referral valid; `UNIQUE(tenant_id)` — dasar seluruh komisi.
type PartnerReferral struct {
	ID                string     `json:"id" gorm:"primaryKey;type:char(26)"`
	PartnerID         string     `json:"partner_id" gorm:"type:char(26);not null;index"`
	TenantID          string     `json:"tenant_id" gorm:"type:char(26);not null;uniqueIndex"`
	LeadID            *string    `json:"lead_id" gorm:"type:char(26)"`
	ReferralCode      string     `json:"referral_code" gorm:"not null"`
	AttributedAt      time.Time  `json:"attributed_at"`
	AttributionStatus string     `json:"attribution_status" gorm:"not null;default:active"`
	ActivatedAt       *time.Time `json:"activated_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func (r *PartnerReferral) BeforeCreate(tx *gorm.DB) (err error) {
	if r.ID == "" {
		r.ID = ulid.New()
	}
	return
}

// PartnerCommission adalah satu perhitungan komisi: satu faktur langganan
// DIBAYAR × mitra. `UNIQUE(subscription_invoice_id, partner_id)` = hitung ulang
// idempoten.
type PartnerCommission struct {
	ID                    string          `json:"id" gorm:"primaryKey;type:char(26)"`
	PartnerID             string          `json:"partner_id" gorm:"type:char(26);not null;index"`
	TenantID              string          `json:"tenant_id" gorm:"type:char(26);not null"`
	SubscriptionInvoiceID string          `json:"subscription_invoice_id" gorm:"type:char(26);not null"`
	PeriodStart           time.Time       `json:"period_start" gorm:"type:date"`
	PeriodEnd             time.Time       `json:"period_end" gorm:"type:date"`
	BaseAmount            int64           `json:"base_amount" gorm:"not null"`
	Rate                  decimal.Decimal `json:"rate" gorm:"type:numeric(7,4);not null"`
	Amount                int64           `json:"amount" gorm:"not null"`
	Status                string          `json:"status" gorm:"not null;default:held"`
	PayoutID              *string         `json:"payout_id" gorm:"type:char(26)"`
	Note                  string          `json:"note"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
}

func (c *PartnerCommission) BeforeCreate(tx *gorm.DB) (err error) {
	if c.ID == "" {
		c.ID = ulid.New()
	}
	return
}

// PartnerPayout adalah pencairan periodik: bruto − clawback − pajak = neto.
type PartnerPayout struct {
	ID             string     `json:"id" gorm:"primaryKey;type:char(26)"`
	PartnerID      string     `json:"partner_id" gorm:"type:char(26);not null;index"`
	PeriodStart    time.Time  `json:"period_start" gorm:"type:date"`
	PeriodEnd      time.Time  `json:"period_end" gorm:"type:date"`
	GrossAmount    int64      `json:"gross_amount" gorm:"not null"`
	ClawbackAmount int64      `json:"clawback_amount" gorm:"not null;default:0"`
	TaxAmount      int64      `json:"tax_amount" gorm:"not null;default:0"`
	NetAmount      int64      `json:"net_amount" gorm:"not null"`
	Status         string     `json:"status" gorm:"not null;default:draft"`
	TransferProof  string     `json:"transfer_proof"`
	PaidAt         *time.Time `json:"paid_at"`
	Note           string     `json:"note"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (p *PartnerPayout) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = ulid.New()
	}
	return
}

// PartnerDispute adalah sengketa atribusi + keputusan admin (blueprint G.2 #5).
type PartnerDispute struct {
	ID         string     `json:"id" gorm:"primaryKey;type:char(26)"`
	PartnerID  string     `json:"partner_id" gorm:"type:char(26);not null;index"`
	TenantID   *string    `json:"tenant_id" gorm:"type:char(26)"`
	ReferralID *string    `json:"referral_id" gorm:"type:char(26)"`
	Reason     string     `json:"reason" gorm:"not null"`
	Status     string     `json:"status" gorm:"not null;default:open"`
	Resolution string     `json:"resolution"`
	ResolvedAt *time.Time `json:"resolved_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (d *PartnerDispute) BeforeCreate(tx *gorm.DB) (err error) {
	if d.ID == "" {
		d.ID = ulid.New()
	}
	return
}

// PartnerMerchantAccessLog mencatat setiap kali mitra membuka data (terbatas)
// merchant binaannya (blueprint G.8).
type PartnerMerchantAccessLog struct {
	ID            int64     `json:"id" gorm:"primaryKey"`
	PartnerID     string    `json:"partner_id" gorm:"type:char(26);not null"`
	PartnerUserID string    `json:"partner_user_id" gorm:"type:char(26);not null"`
	TenantID      string    `json:"tenant_id" gorm:"type:char(26);not null"`
	Action        string    `json:"action" gorm:"not null"`
	At            time.Time `json:"at"`
}

func (PartnerMerchantAccessLog) TableName() string { return "partner_merchant_access_log" }
