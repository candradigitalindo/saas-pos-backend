package structs

// DTO Program Mitra Penjual (Fase 12, blueprint Bagian G). Uang int64 rupiah
// bulat; tarif string desimal. Kredensial mitra (rekening/NPWP) tidak pernah
// dikembalikan lewat portal.

// ── Auth mitra ───────────────────────────────────────────────────────────

type PartnerLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type PartnerAuthResponse struct {
	AccessToken string              `json:"access_token"`
	ExpiresAt   string              `json:"expires_at"`
	Partner     PartnerResponse     `json:"partner"`
	User        PartnerUserResponse `json:"user"`
}

type PartnerUserResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

// ── Tingkat & mitra (panel internal) ─────────────────────────────────────

type PartnerTierRequest struct {
	Code              string `json:"code" binding:"required,min=2,max=40"`
	Name              string `json:"name" binding:"required,min=2,max=80"`
	Kind              string `json:"kind" binding:"required,oneof=agen afiliasi"`
	CommissionRate    string `json:"commission_rate" binding:"required"` // "0.15" = 15%
	Recurring         *bool  `json:"recurring"`
	OneTimeMonths     int    `json:"one_time_months" binding:"omitempty,gte=0"`
	ActivationMinTxn  int    `json:"activation_min_txn" binding:"omitempty,gte=0"`
	ActivationMinDays int    `json:"activation_min_days" binding:"omitempty,gte=0"`
	AttributionDays   int    `json:"attribution_days" binding:"omitempty,gte=0"`
	ClawbackDays      int    `json:"clawback_days" binding:"omitempty,gte=0"`
}

type PartnerTierResponse struct {
	ID                string `json:"id"`
	Code              string `json:"code"`
	Name              string `json:"name"`
	Kind              string `json:"kind"`
	CommissionRate    string `json:"commission_rate"`
	Recurring         bool   `json:"recurring"`
	OneTimeMonths     int    `json:"one_time_months"`
	ActivationMinTxn  int    `json:"activation_min_txn"`
	ActivationMinDays int    `json:"activation_min_days"`
	AttributionDays   int    `json:"attribution_days"`
	ClawbackDays      int    `json:"clawback_days"`
	IsActive          bool   `json:"is_active"`
}

type PartnerCreateRequest struct {
	TierCode           string `json:"tier_code" binding:"required"`
	Name               string `json:"name" binding:"required,min=2,max=120"`
	Region             string `json:"region" binding:"omitempty,max=80"`
	ReferralCode       string `json:"referral_code" binding:"omitempty,min=3,max=40"` // kosong → dibuatkan
	BankAccount        string `json:"bank_account" binding:"omitempty,max=80"`
	TaxID              string `json:"tax_id" binding:"omitempty,max=40"`
	TaxWithholdingRate string `json:"tax_withholding_rate" binding:"omitempty"`
	// Akun login pertama.
	UserName     string `json:"user_name" binding:"required,min=2,max=120"`
	UserEmail    string `json:"user_email" binding:"required,email"`
	UserUsername string `json:"user_username" binding:"required,min=3,max=60"`
	UserPassword string `json:"user_password" binding:"omitempty,min=8"` // kosong → dibuatkan
}

type PartnerResponse struct {
	ID           string `json:"id"`
	TierCode     string `json:"tier_code"`
	Kind         string `json:"kind"`
	Name         string `json:"name"`
	Region       string `json:"region,omitempty"`
	ReferralCode string `json:"referral_code"`
	Status       string `json:"status"`
	JoinedAt     string `json:"joined_at,omitempty"`
}

// PartnerCreateResponse memuat kredensial awal SEKALI (tak bisa diambil lagi).
type PartnerCreateResponse struct {
	Partner           PartnerResponse `json:"partner"`
	UserUsername      string          `json:"user_username"`
	GeneratedPassword string          `json:"generated_password,omitempty"`
}

// ── Prospek ──────────────────────────────────────────────────────────────

type PartnerLeadRequest struct {
	BusinessName string `json:"business_name" binding:"required,min=2,max=120"`
	ContactName  string `json:"contact_name" binding:"omitempty,max=120"`
	ContactPhone string `json:"contact_phone" binding:"omitempty,max=30"`
	City         string `json:"city" binding:"omitempty,max=80"`
	BusinessType string `json:"business_type" binding:"omitempty,max=40"`
	Note         string `json:"note" binding:"omitempty,max=300"`
}

type PartnerLeadResponse struct {
	ID                   string `json:"id"`
	BusinessName         string `json:"business_name"`
	ContactName          string `json:"contact_name,omitempty"`
	ContactPhone         string `json:"contact_phone,omitempty"`
	City                 string `json:"city,omitempty"`
	Status               string `json:"status"`
	RegisteredTenantID   string `json:"registered_tenant_id,omitempty"`
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
	TenantID              string `json:"tenant_id"`
	SubscriptionInvoiceID string `json:"subscription_invoice_id"`
	PeriodStart           string `json:"period_start"`
	PeriodEnd             string `json:"period_end"`
	BaseAmount            int64  `json:"base_amount"`
	Rate                  string `json:"rate"`
	Amount                int64  `json:"amount"`
	Status                string `json:"status"`
	PayoutID              string `json:"payout_id,omitempty"`
}

type PartnerPayoutResponse struct {
	ID             string `json:"id"`
	PeriodStart    string `json:"period_start"`
	PeriodEnd      string `json:"period_end"`
	GrossAmount    int64  `json:"gross_amount"`
	ClawbackAmount int64  `json:"clawback_amount"`
	TaxAmount      int64  `json:"tax_amount"`
	NetAmount      int64  `json:"net_amount"`
	Status         string `json:"status"`
	TransferProof  string `json:"transfer_proof,omitempty"`
	PaidAt         string `json:"paid_at,omitempty"`
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

type PartnerCommissionRunRequest struct {
	PeriodStart string `json:"period_start" binding:"required"` // YYYY-MM-DD
	PeriodEnd   string `json:"period_end" binding:"required"`
}

type PartnerCommissionRunResult struct {
	From                string `json:"from"`
	To                  string `json:"to"`
	ReferralsSeen       int    `json:"referrals_seen"`
	Activated           int    `json:"activated"`
	Computed            int    `json:"computed"`    // baris komisi ditulis/diperbarui
	ClawedBack          int    `json:"clawed_back"` // baris komisi ditarik
	SkippedNoActivation int    `json:"skipped_no_activation"`
}

type PartnerPayoutRunRequest struct {
	PartnerID   string `json:"partner_id" binding:"required,ulid"`
	PeriodStart string `json:"period_start" binding:"required"`
	PeriodEnd   string `json:"period_end" binding:"required"`
}

type PartnerPayoutMarkPaidRequest struct {
	TransferProof string `json:"transfer_proof" binding:"required,min=2,max=200"`
}
