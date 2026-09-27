package structs

// DTO Program Mitra Penjual (Fase 12, §5.12, blueprint Bagian G). Uang int64
// rupiah bulat; tarif string desimal. Data rekening/identitas mitra tidak pernah
// dikembalikan lewat portal.

// ── Auth mitra ───────────────────────────────────────────────────────────

// PartnerLoginRequest — portal mitra login memakai EMAIL (§5.12).
type PartnerLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type PartnerAuthResponse struct {
	AccessToken string              `json:"access_token"`
	ExpiresAt   string              `json:"expires_at"`
	Partner     PartnerResponse     `json:"partner"`
	User        PartnerUserResponse `json:"user"`
}

type PartnerUserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone,omitempty"`
}

// PartnerMeResponse: GET /partner/me — mitra & akun yang sedang masuk.
type PartnerMeResponse struct {
	Partner PartnerResponse     `json:"partner"`
	User    PartnerUserResponse `json:"user"`
}

// ── Tingkat & mitra (panel internal) ─────────────────────────────────────

type PartnerTierRequest struct {
	Name               string `json:"name" binding:"required,min=2,max=80"`
	Kind               string `json:"kind" binding:"required,oneof=affiliate agent"`
	RecurringRate      string `json:"recurring_rate" binding:"required"` // "0.15" = 15%
	RecurringMonths    *int   `json:"recurring_months"`                  // nil = selama merchant aktif
	ActivationBonus    int64  `json:"activation_bonus" binding:"omitempty,gte=0"`
	MinActiveMerchants int    `json:"min_active_merchants" binding:"omitempty,gte=0"`
	ActivationMinTxn   int    `json:"activation_min_txn" binding:"omitempty,gte=0"`
	ActivationMinDays  int    `json:"activation_min_days" binding:"omitempty,gte=0"`
	AttributionDays    int    `json:"attribution_days" binding:"omitempty,gte=0"`
	ClawbackDays       int    `json:"clawback_days" binding:"omitempty,gte=0"`
}

type PartnerTierResponse struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Kind               string `json:"kind"`
	RecurringRate      string `json:"recurring_rate"`
	RecurringMonths    *int   `json:"recurring_months"`
	ActivationBonus    int64  `json:"activation_bonus"`
	MinActiveMerchants int    `json:"min_active_merchants"`
	ActivationMinTxn   int    `json:"activation_min_txn"`
	ActivationMinDays  int    `json:"activation_min_days"`
	AttributionDays    int    `json:"attribution_days"`
	ClawbackDays       int    `json:"clawback_days"`
}

type PartnerCreateRequest struct {
	TierName           string `json:"tier_name" binding:"required"`
	Name               string `json:"name" binding:"required,min=2,max=120"`
	Phone              string `json:"phone" binding:"required,min=5,max=30"`
	Email              string `json:"email" binding:"omitempty,email"`
	Region             string `json:"region" binding:"omitempty,max=80"`
	ReferralCode       string `json:"referral_code" binding:"omitempty,min=3,max=40"` // kosong → dibuatkan
	IDNumber           string `json:"id_number" binding:"omitempty,max=40"`
	NPWP               string `json:"npwp" binding:"omitempty,max=40"`
	BankName           string `json:"bank_name" binding:"omitempty,max=60"`
	BankAccountNo      string `json:"bank_account_no" binding:"omitempty,max=40"`
	BankAccountName    string `json:"bank_account_name" binding:"omitempty,max=120"`
	TaxWithholdingRate string `json:"tax_withholding_rate" binding:"omitempty"`
	// Akun login pertama (login memakai email).
	UserName     string `json:"user_name" binding:"required,min=2,max=120"`
	UserEmail    string `json:"user_email" binding:"required,email"`
	UserPhone    string `json:"user_phone" binding:"omitempty,max=30"`
	UserPassword string `json:"user_password" binding:"omitempty,min=8"` // kosong → dibuatkan
}

type PartnerResponse struct {
	ID           string `json:"id"`
	TierName     string `json:"tier_name"`
	Kind         string `json:"kind"`
	Name         string `json:"name"`
	Phone        string `json:"phone,omitempty"`
	Email        string `json:"email,omitempty"`
	Region       string `json:"region,omitempty"`
	ReferralCode string `json:"referral_code"`
	Status       string `json:"status"`
	VerifiedAt   string `json:"verified_at,omitempty"`
	JoinedAt     string `json:"joined_at,omitempty"`
}

// PartnerCreateResponse memuat kredensial awal SEKALI (tak bisa diambil lagi).
type PartnerCreateResponse struct {
	Partner           PartnerResponse `json:"partner"`
	UserEmail         string          `json:"user_email"`
	GeneratedPassword string          `json:"generated_password,omitempty"`
}

// ── Prospek ──────────────────────────────────────────────────────────────

type PartnerLeadRequest struct {
	BusinessName string `json:"business_name" binding:"required,min=2,max=120"`
	ContactName  string `json:"contact_name" binding:"omitempty,max=120"`
	Phone        string `json:"phone" binding:"required,min=5,max=30"`
	City         string `json:"city" binding:"omitempty,max=80"`
	BusinessType string `json:"business_type" binding:"omitempty,max=40"`
	Note         string `json:"note" binding:"omitempty,max=300"`
}

type PartnerLeadResponse struct {
	ID                   string `json:"id"`
	BusinessName         string `json:"business_name"`
	ContactName          string `json:"contact_name,omitempty"`
	Phone                string `json:"phone,omitempty"`
	City                 string `json:"city,omitempty"`
	Status               string `json:"status"`
	ConvertedTenantID    string `json:"converted_tenant_id,omitempty"`
	AttributionExpiresAt string `json:"attribution_expires_at"`
	CreatedAt            string `json:"created_at"`
}

// ── Merchant binaan (tampilan TERBATAS, blueprint G.8) ──────────────────
//
// HANYA field berikut. Tidak pernah omzet, produk, harga, pelanggan, transaksi.
type PartnerMerchantResponse struct {
	TenantID           string `json:"tenant_id"`
	BusinessName       string `json:"business_name"`
	SubscriptionStatus string `json:"subscription_status"`
	CurrentPeriodEnd   string `json:"current_period_end,omitempty"` // tanggal jatuh tempo
	IsActive           bool   `json:"is_active"`
	AttributedAt       string `json:"attributed_at"`
	Activated          bool   `json:"activated"`
}

// ── Komisi & pencairan ──────────────────────────────────────────────────

type PartnerCommissionResponse struct {
	ID                    string `json:"id"`
	ReferralID            string `json:"referral_id"`
	SubscriptionInvoiceID string `json:"subscription_invoice_id"`
	PeriodMonth           string `json:"period_month"`
	BaseAmount            int64  `json:"base_amount"`
	Rate                  string `json:"rate"`
	Amount                int64  `json:"amount"`
	Status                string `json:"status"`
	PayoutID              string `json:"payout_id,omitempty"`
}

type PartnerPayoutResponse struct {
	ID               string `json:"id"`
	PeriodStart      string `json:"period_start"`
	PeriodEnd        string `json:"period_end"`
	GrossAmount      int64  `json:"gross_amount"`
	ClawbackAmount   int64  `json:"clawback_amount"`
	TaxAmount        int64  `json:"tax_amount"`
	NetAmount        int64  `json:"net_amount"`
	Status           string `json:"status"`
	TransferProofURL string `json:"transfer_proof_url,omitempty"`
	TaxSlipURL       string `json:"tax_slip_url,omitempty"`
	PaidAt           string `json:"paid_at,omitempty"`
}

type PartnerDashboardResponse struct {
	LeadsByStatus      map[string]int         `json:"leads_by_status"`
	MerchantsTotal     int                    `json:"merchants_total"`
	MerchantsActive    int                    `json:"merchants_active"`
	CommissionHeld     int64                  `json:"commission_held"`
	CommissionApproved int64                  `json:"commission_approved"`
	CommissionPaid     int64                  `json:"commission_paid"`
	LastPayout         *PartnerPayoutResponse `json:"last_payout,omitempty"`
}

// ── Mesin komisi (panel internal / CLI) ────────────────────────────────

type PartnerCommissionRunResult struct {
	From                string `json:"from"`
	To                  string `json:"to"`
	ReferralsSeen       int    `json:"referrals_seen"`
	Activated           int    `json:"activated"`
	Computed            int    `json:"computed"`
	ClawedBack          int    `json:"clawed_back"`
	SkippedNoActivation int    `json:"skipped_no_activation"`
}

type PartnerPayoutMarkPaidRequest struct {
	TransferProofURL string `json:"transfer_proof_url" binding:"required,min=2,max=300"`
	TaxSlipURL       string `json:"tax_slip_url" binding:"omitempty,max=300"`
}

// ── Pelengkap: target, materi, pelatihan, sengketa (migrasi 000028) ──────

type PartnerTargetRequest struct {
	PartnerID       string `json:"partner_id" binding:"required,ulid"`
	PeriodStart     string `json:"period_start" binding:"required"` // YYYY-MM-DD
	PeriodEnd       string `json:"period_end" binding:"required"`
	TargetMerchants int    `json:"target_merchants" binding:"gte=0"`
}

type PartnerTargetResponse struct {
	ID                string `json:"id"`
	PeriodStart       string `json:"period_start"`
	PeriodEnd         string `json:"period_end"`
	TargetMerchants   int    `json:"target_merchants"`
	AchievedMerchants int    `json:"achieved_merchants"`
}

type PartnerMaterialRequest struct {
	Title     string `json:"title" binding:"required,min=2,max=160"`
	Kind      string `json:"kind" binding:"required,oneof=brochure video template pricelist"`
	FileURL   string `json:"file_url" binding:"required,url"`
	Version   int    `json:"version" binding:"omitempty,gte=1"`
	MinTierID string `json:"min_tier_id" binding:"omitempty,ulid"` // kosong = semua tingkat
}

type PartnerMaterialResponse struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Kind     string `json:"kind"`
	FileURL  string `json:"file_url"`
	Version  int    `json:"version"`
	IsActive bool   `json:"is_active"`
}

type PartnerTrainingRequest struct {
	Title      string `json:"title" binding:"required,min=2,max=160"`
	ContentURL string `json:"content_url" binding:"omitempty,url"`
	IsRequired bool   `json:"is_required"`
	SortOrder  int    `json:"sort_order" binding:"omitempty,gte=0"`
}

type PartnerTrainingResponse struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	ContentURL  string `json:"content_url,omitempty"`
	IsRequired  bool   `json:"is_required"`
	SortOrder   int    `json:"sort_order"`
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completed_at,omitempty"`
	Score       *int   `json:"score,omitempty"`
}

type PartnerTrainingCompleteRequest struct {
	Score *int `json:"score" binding:"omitempty,gte=0,lte=100"`
}

type PartnerDisputeRequest struct {
	TenantID string `json:"tenant_id" binding:"omitempty,ulid"`
	LeadID   string `json:"lead_id" binding:"omitempty,ulid"`
	Reason   string `json:"reason" binding:"required,min=10,max=1000"`
}

type PartnerDisputeResolveRequest struct {
	Status       string `json:"status" binding:"required,oneof=accepted rejected"`
	DecisionNote string `json:"decision_note" binding:"required,min=5,max=1000"`
}

type PartnerDisputeResponse struct {
	ID                string `json:"id"`
	ClaimantPartnerID string `json:"claimant_partner_id"`
	TenantID          string `json:"tenant_id,omitempty"`
	LeadID            string `json:"lead_id,omitempty"`
	Reason            string `json:"reason"`
	Status            string `json:"status"`
	DecisionNote      string `json:"decision_note,omitempty"`
	DecidedAt         string `json:"decided_at,omitempty"`
	CreatedAt         string `json:"created_at"`
}
