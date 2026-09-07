package structs

import "encoding/json"

// DTO langganan & tagihan platform (Fase 7, §5.13). Semua nilai uang int64
// rupiah bulat; tarif diskon string desimal.

// ── Request ───────────────────────────────────────────────────────────────

// SubscriptionStartRequest memilih paket + masa langganan (1/3/6/9/12 bulan).
type SubscriptionStartRequest struct {
	PlanCode   string `json:"plan_code" binding:"required"`
	TermMonths int    `json:"term_months" binding:"required,oneof=1 3 6 9 12"`
}

// SubscriptionChangePlanRequest untuk naik/turun paket di tengah masa (prorata).
type SubscriptionChangePlanRequest struct {
	PlanCode   string `json:"plan_code" binding:"required"`
	TermMonths int    `json:"term_months" binding:"required,oneof=1 3 6 9 12"`
}

// SubscriptionCancelRequest membatalkan langganan di tengah masa.
type SubscriptionCancelRequest struct {
	Reason string `json:"reason" binding:"omitempty,max=255"`
}

// SubscriptionPaymentRequest membayar sebuah tagihan langganan. Wajib header
// Idempotency-Key (§8).
type SubscriptionPaymentRequest struct {
	InvoiceID string `json:"invoice_id" binding:"required,ulid"`
	Amount    int64  `json:"amount" binding:"required,gt=0"`
	Method    string `json:"method" binding:"required,oneof=cash transfer card qris ewallet"`
	Reference string `json:"reference" binding:"omitempty,max=100"`
}

// ── Response ──────────────────────────────────────────────────────────────

// PlanTermPrice adalah harga sebuah paket untuk satu masa langganan.
type PlanTermPrice struct {
	TermMonths     int    `json:"term_months"`
	DiscountRate   string `json:"discount_rate"`
	GrossAmount    int64  `json:"gross_amount"`
	DiscountAmount int64  `json:"discount_amount"`
	TotalAmount    int64  `json:"total_amount"`
}

// PlanResponse adalah satu paket beserta tabel harga per masa.
type PlanResponse struct {
	Code                   string          `json:"code"`
	Name                   string          `json:"name"`
	MonthlyPrice           int64           `json:"monthly_price"`
	MaxOutlets             *int            `json:"max_outlets"`
	MaxUsers               *int            `json:"max_users"`
	MaxProducts            *int            `json:"max_products"`
	MaxMonthlyTransactions *int            `json:"max_monthly_transactions"`
	Features               json.RawMessage `json:"features"`
	TermPrices             []PlanTermPrice `json:"term_prices"`
}

// SubscriptionResponse adalah status langganan tenant.
type SubscriptionResponse struct {
	ID                 string `json:"id"`
	PlanCode           string `json:"plan_code"`
	PlanName           string `json:"plan_name"`
	TermMonths         int    `json:"term_months"`
	DiscountRate       string `json:"discount_rate"`
	Status             string `json:"status"`
	TrialEndsAt        string `json:"trial_ends_at,omitempty"`
	CurrentPeriodStart string `json:"current_period_start"`
	CurrentPeriodEnd   string `json:"current_period_end"`
	AutoRenew          bool   `json:"auto_renew"`
	CanceledAt         string `json:"canceled_at,omitempty"`
	CancelReason       string `json:"cancel_reason,omitempty"`
}

// SubInvoiceResponse adalah satu tagihan langganan.
type SubInvoiceResponse struct {
	ID             string `json:"id"`
	Number         string `json:"number"`
	TermMonths     int    `json:"term_months"`
	PeriodStart    string `json:"period_start"`
	PeriodEnd      string `json:"period_end"`
	GrossAmount    int64  `json:"gross_amount"`
	DiscountAmount int64  `json:"discount_amount"`
	TotalAmount    int64  `json:"total_amount"`
	PaidAmount     int64  `json:"paid_amount"`
	DueDate        string `json:"due_date"`
	Status         string `json:"status"`
	PaidAt         string `json:"paid_at,omitempty"`
}

// SubscriptionOverviewResponse menggabungkan langganan + tagihan terbuka (bila ada).
type SubscriptionOverviewResponse struct {
	Subscription SubscriptionResponse `json:"subscription"`
	OpenInvoice  *SubInvoiceResponse  `json:"open_invoice,omitempty"`
}

// SubscriptionCancelResponse melaporkan hasil pembatalan.
type SubscriptionCancelResponse struct {
	RefundAmount int64                `json:"refund_amount"`
	EarnedAmount int64                `json:"earned_amount"`
	MonthsUsed   int                  `json:"months_used"`
	Subscription SubscriptionResponse `json:"subscription"`
}
