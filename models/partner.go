package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Model Program Mitra Penjual (Fase 12, §5.12, blueprint Bagian G).
//
// Semua tabel di sini adalah lingkup PLATFORM — sejajar dengan plans/
// subscriptions, BUKAN data operasional tenant. Tidak ada RLS dan FK-nya
// TUNGGAL (§5.17); isolasi antar mitra dijaga filter `partner_id` eksplisit di
// repo. Uang int64 rupiah bulat; tarif desimal.

// Enum (cermin CHECK migrasi 000026–000028; nilai mengikuti §5.12).
var (
	PartnerKinds              = []string{"affiliate", "agent"}
	PartnerStatuses           = []string{"pending", "verified", "active", "suspended", "terminated"}
	PartnerLeadStatuses       = []string{"new", "contacted", "demo", "registered", "activated", "lost"}
	PartnerReferralStatuses   = []string{"pending", "active", "ended", "disputed", "revoked"}
	PartnerCommissionStatuses = []string{"held", "approved", "paid", "clawed_back", "canceled"}
	PartnerPayoutStatuses     = []string{"draft", "approved", "paid", "failed"}
	PartnerDisputeStatuses    = []string{"open", "accepted", "rejected"}
	PartnerMaterialKinds      = []string{"brochure", "video", "template", "pricelist"}
)

// PartnerTier adalah tingkat mitra + aturan komisinya (blueprint G.5 — dapat
// diubah tanpa menyentuh kode). `RecurringMonths` nil = komisi berjalan selama
// merchant masih berlangganan.
//
// Empat kolom ambang (ActivationMin*, AttributionDays, ClawbackDays) adalah
// TAMBAHAN di luar DDL §5.12 — blueprint G.2 #3 & #4 mewajibkan perilakunya
// tetapi §5.12 tidak memberi mereka tempat. Lihat komentar migrasi 000026.
type PartnerTier struct {
	ID                 string          `json:"id" gorm:"primaryKey;type:char(26)"`
	Name               string          `json:"name" gorm:"not null;uniqueIndex"`
	Kind               string          `json:"kind" gorm:"not null"`
	RecurringRate      decimal.Decimal `json:"recurring_rate" gorm:"type:numeric(7,4);not null"`
	RecurringMonths    *int            `json:"recurring_months"`
	ActivationBonus    int64           `json:"activation_bonus" gorm:"not null;default:0"`
	MinActiveMerchants int             `json:"min_active_merchants" gorm:"not null;default:0"`
	ActivationMinTxn   int             `json:"activation_min_txn" gorm:"not null;default:30"`
	ActivationMinDays  int             `json:"activation_min_days" gorm:"not null;default:30"`
	AttributionDays    int             `json:"attribution_days" gorm:"not null;default:60"`
	ClawbackDays       int             `json:"clawback_days" gorm:"not null;default:90"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

func (t *PartnerTier) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == "" {
		t.ID = ulid.New()
	}
	return
}

// Partner adalah satu mitra penjual (agen daerah atau afiliasi). `ReferralCode`
// unik global — dipakai calon tenant saat mendaftar. Data rekening/identitas
// tidak pernah dikembalikan lewat portal.
//
// TIDAK ADA kolom upline/parent: batas satu tingkat dikunci di skema, bukan
// hanya di perjanjian (§5.12).
type Partner struct {
	ID                 string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TierID             string          `json:"tier_id" gorm:"type:char(26);not null"`
	Kind               string          `json:"kind" gorm:"not null"`
	Name               string          `json:"name" gorm:"not null"`
	Phone              string          `json:"phone" gorm:"not null"`
	Email              string          `json:"email"`
	Region             string          `json:"region"`
	ReferralCode       string          `json:"referral_code" gorm:"not null;uniqueIndex"`
	IDNumber           string          `json:"-" gorm:"column:id_number"`
	NPWP               string          `json:"-" gorm:"column:npwp"`
	BankName           string          `json:"-"`
	BankAccountNo      string          `json:"-"`
	BankAccountName    string          `json:"-"`
	Status             string          `json:"status" gorm:"not null;default:pending"`
	VerifiedAt         *time.Time      `json:"verified_at"`
	JoinedAt           *time.Time      `json:"joined_at"`
	TaxWithholdingRate decimal.Decimal `json:"tax_withholding_rate" gorm:"type:numeric(7,4);not null;default:0"`
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
// tenant (blueprint G.8). Login memakai EMAIL (unik selama belum dihapus).
type PartnerUser struct {
	ID           string         `json:"id" gorm:"primaryKey;type:char(26)"`
	PartnerID    string         `json:"partner_id" gorm:"type:char(26);not null;index"`
	Name         string         `json:"name" gorm:"not null"`
	Email        string         `json:"email" gorm:"not null"`
	Phone        string         `json:"phone"`
	PasswordHash string         `json:"-" gorm:"column:password_hash;not null"`
	IsActive     bool           `json:"is_active" gorm:"not null;default:true"`
	LastLoginAt  *time.Time     `json:"last_login_at"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`

	Partner *Partner `json:"partner,omitempty" gorm:"foreignKey:PartnerID"`
}

func (u *PartnerUser) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == "" {
		u.ID = ulid.New()
	}
	return
}

// PartnerLead adalah prospek yang didaftarkan mitra. Masa atribusi mulai
// berjalan saat prospek dibuat (blueprint G.2 #5).
type PartnerLead struct {
	ID                   string    `json:"id" gorm:"primaryKey;type:char(26)"`
	PartnerID            string    `json:"partner_id" gorm:"type:char(26);not null;index"`
	BusinessName         string    `json:"business_name" gorm:"not null"`
	ContactName          string    `json:"contact_name"`
	Phone                string    `json:"phone" gorm:"not null"`
	City                 string    `json:"city"`
	BusinessType         string    `json:"business_type"`
	Note                 string    `json:"note"`
	Status               string    `json:"status" gorm:"not null;default:new"`
	AttributionExpiresAt time.Time `json:"attribution_expires_at"`
	ConvertedTenantID    *string   `json:"converted_tenant_id" gorm:"type:char(26)"`
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
	ID                 string     `json:"id" gorm:"primaryKey;type:char(26)"`
	PartnerID          string     `json:"partner_id" gorm:"type:char(26);not null;index"`
	TenantID           string     `json:"tenant_id" gorm:"type:char(26);not null;uniqueIndex"`
	LeadID             *string    `json:"lead_id" gorm:"type:char(26)"`
	ReferralCode       string     `json:"referral_code" gorm:"not null"`
	AttributedAt       time.Time  `json:"attributed_at"`
	ActivatedAt        *time.Time `json:"activated_at"`
	CommissionStartsAt *time.Time `json:"commission_starts_at"`
	CommissionEndsAt   *time.Time `json:"commission_ends_at"`
	Status             string     `json:"status" gorm:"not null;default:pending"`
}

func (r *PartnerReferral) BeforeCreate(tx *gorm.DB) (err error) {
	if r.ID == "" {
		r.ID = ulid.New()
	}
	return
}

// PartnerCommission adalah satu perhitungan komisi: (referral × faktur langganan
// × bulan). `UNIQUE(referral_id, subscription_invoice_id, period_month)` =
// hitung ulang idempoten.
type PartnerCommission struct {
	ID                    string          `json:"id" gorm:"primaryKey;type:char(26)"`
	PartnerID             string          `json:"partner_id" gorm:"type:char(26);not null;index"`
	ReferralID            string          `json:"referral_id" gorm:"type:char(26);not null"`
	SubscriptionInvoiceID string          `json:"subscription_invoice_id" gorm:"type:char(26);not null"`
	PeriodMonth           time.Time       `json:"period_month" gorm:"type:date"`
	BaseAmount            int64           `json:"base_amount" gorm:"not null"`
	Rate                  decimal.Decimal `json:"rate" gorm:"type:numeric(7,4);not null"`
	Amount                int64           `json:"amount" gorm:"not null"`
	Status                string          `json:"status" gorm:"not null;default:held"`
	PayoutID              *string         `json:"payout_id" gorm:"type:char(26)"`
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
	ID               string     `json:"id" gorm:"primaryKey;type:char(26)"`
	PartnerID        string     `json:"partner_id" gorm:"type:char(26);not null;index"`
	PeriodStart      time.Time  `json:"period_start" gorm:"type:date"`
	PeriodEnd        time.Time  `json:"period_end" gorm:"type:date"`
	GrossAmount      int64      `json:"gross_amount" gorm:"not null"`
	TaxAmount        int64      `json:"tax_amount" gorm:"not null;default:0"`
	ClawbackAmount   int64      `json:"clawback_amount" gorm:"not null;default:0"`
	NetAmount        int64      `json:"net_amount" gorm:"not null"`
	Status           string     `json:"status" gorm:"not null;default:draft"`
	PaidAt           *time.Time `json:"paid_at"`
	TransferProofURL string     `json:"transfer_proof_url"`
	TaxSlipURL       string     `json:"tax_slip_url"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func (p *PartnerPayout) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = ulid.New()
	}
	return
}

// PartnerTarget adalah target & pencapaian merchant aktif per periode
// (blueprint G.4 — papan peringkat BERBASIS MERCHANT AKTIF, bukan pendaftaran).
type PartnerTarget struct {
	ID                string    `json:"id" gorm:"primaryKey;type:char(26)"`
	PartnerID         string    `json:"partner_id" gorm:"type:char(26);not null;index"`
	PeriodStart       time.Time `json:"period_start" gorm:"type:date"`
	PeriodEnd         time.Time `json:"period_end" gorm:"type:date"`
	TargetMerchants   int       `json:"target_merchants" gorm:"not null;default:0"`
	AchievedMerchants int       `json:"achieved_merchants" gorm:"not null;default:0"`
}

func (t *PartnerTarget) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == "" {
		t.ID = ulid.New()
	}
	return
}

// PartnerMaterial adalah materi jualan berversi (brosur, video, template pesan,
// daftar harga resmi) — mencegah mitra mengarang janji fitur (blueprint G.4).
type PartnerMaterial struct {
	ID        string    `json:"id" gorm:"primaryKey;type:char(26)"`
	Title     string    `json:"title" gorm:"not null"`
	Kind      string    `json:"kind" gorm:"not null"`
	FileURL   string    `json:"file_url" gorm:"not null"`
	Version   int       `json:"version" gorm:"not null;default:1"`
	MinTierID *string   `json:"min_tier_id" gorm:"type:char(26)"`
	IsActive  bool      `json:"is_active" gorm:"not null;default:true"`
	CreatedAt time.Time `json:"created_at"`
}

func (m *PartnerMaterial) BeforeCreate(tx *gorm.DB) (err error) {
	if m.ID == "" {
		m.ID = ulid.New()
	}
	return
}

// PartnerTraining adalah modul pelatihan; wajib untuk tingkat agen (G.6 #3).
type PartnerTraining struct {
	ID         string `json:"id" gorm:"primaryKey;type:char(26)"`
	Title      string `json:"title" gorm:"not null"`
	ContentURL string `json:"content_url"`
	IsRequired bool   `json:"is_required" gorm:"not null;default:false"`
	SortOrder  int    `json:"sort_order" gorm:"not null;default:0"`
}

func (t *PartnerTraining) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == "" {
		t.ID = ulid.New()
	}
	return
}

// PartnerTrainingRecord = tabel penghubung murni: PK-nya pasangan kunci asing,
// tanpa kolom id (§5.17).
type PartnerTrainingRecord struct {
	PartnerUserID string     `json:"partner_user_id" gorm:"primaryKey;type:char(26)"`
	TrainingID    string     `json:"training_id" gorm:"primaryKey;type:char(26)"`
	CompletedAt   *time.Time `json:"completed_at"`
	Score         *int       `json:"score"`
}

func (PartnerTrainingRecord) TableName() string { return "partner_training_records" }

// PartnerDispute adalah sengketa atribusi + keputusan admin (blueprint G.2 #5:
// "Sengketa diputus manual oleh admin, dan keputusannya dicatat").
type PartnerDispute struct {
	ID                string     `json:"id" gorm:"primaryKey;type:char(26)"`
	ClaimantPartnerID string     `json:"claimant_partner_id" gorm:"type:char(26);not null;index"`
	TenantID          *string    `json:"tenant_id" gorm:"type:char(26)"`
	LeadID            *string    `json:"lead_id" gorm:"type:char(26)"`
	Reason            string     `json:"reason" gorm:"not null"`
	Status            string     `json:"status" gorm:"not null;default:open"`
	DecidedBy         *string    `json:"decided_by" gorm:"type:char(26)"`
	DecidedAt         *time.Time `json:"decided_at"`
	DecisionNote      string     `json:"decision_note"`
	CreatedAt         time.Time  `json:"created_at"`
}

func (d *PartnerDispute) BeforeCreate(tx *gorm.DB) (err error) {
	if d.ID == "" {
		d.ID = ulid.New()
	}
	return
}
