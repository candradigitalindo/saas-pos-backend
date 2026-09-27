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

// PaymentClaimRequest: tenant MENGONFIRMASI pembayaran tagihan langganan
// ("sudah saya transfer"). Wajib header Idempotency-Key (§8). Paket baru
// aktif setelah staf keuangan platform menyetujuinya — lihat
// services.SubmitPaymentClaim.
type PaymentClaimRequest struct {
	InvoiceID string `json:"invoice_id" binding:"required,ulid"`
	Amount    int64  `json:"amount" binding:"required,gt=0"`
	Method    string `json:"method" binding:"required,oneof=transfer qris ewallet card cash"`
	// Nama pengirim / nomor referensi — yang dicocokkan staf keuangan dengan
	// mutasi rekening. Wajib: tanpa ini konfirmasi tidak bisa diverifikasi.
	Reference string `json:"reference" binding:"required,min=2,max=100"`
	Note      string `json:"note" binding:"omitempty,max=300"`
}

// PaymentClaimRejectRequest: alasan penolakan konfirmasi (dibaca tenant).
type PaymentClaimRejectRequest struct {
	Reason string `json:"reason" binding:"required,min=3,max=300"`
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
	// Paket yang dibayar tagihan ini; kind "plan_change" = pindah paket yang
	// baru berlaku saat lunas. CreditAmount = potongan sisa paket lama (sudah
	// termasuk di DiscountAmount).
	PlanCode     string `json:"plan_code,omitempty"`
	PlanName     string `json:"plan_name,omitempty"`
	Kind         string `json:"kind"`
	CreditAmount int64  `json:"credit_amount"`
}

// SubscriptionOverviewResponse menggabungkan langganan + tagihan terbuka (bila ada).
type SubscriptionOverviewResponse struct {
	Subscription SubscriptionResponse `json:"subscription"`
	OpenInvoice  *SubInvoiceResponse  `json:"open_invoice,omitempty"`
	// PaymentClaim: konfirmasi TERBARU untuk tagihan terbuka — menunggu
	// verifikasi, atau ditolak (beserta alasannya).
	PaymentClaim *PaymentClaimResponse `json:"payment_claim,omitempty"`
	// PaymentInstructions: rekening tujuan; nil bila belum dikonfigurasi.
	PaymentInstructions *PaymentInstructions `json:"payment_instructions,omitempty"`
}

// SubscriptionCancelResponse melaporkan hasil pembatalan.
type SubscriptionCancelResponse struct {
	RefundAmount int64                `json:"refund_amount"`
	EarnedAmount int64                `json:"earned_amount"`
	MonthsUsed   int                  `json:"months_used"`
	Subscription SubscriptionResponse `json:"subscription"`
}

// PaymentClaimResponse: satu konfirmasi pembayaran langganan.
type PaymentClaimResponse struct {
	ID            string `json:"id"`
	InvoiceID     string `json:"invoice_id"`
	InvoiceNumber string `json:"invoice_number,omitempty"`
	Amount        int64  `json:"amount"`
	Method        string `json:"method"`
	Reference     string `json:"reference"`
	Note          string `json:"note,omitempty"`
	Status        string `json:"status"` // pending | approved | rejected
	RejectReason  string `json:"reject_reason,omitempty"`
	CreatedAt     string `json:"created_at"`
	ReviewedAt    string `json:"reviewed_at,omitempty"`
}

// PlatformPaymentClaimResponse: konfirmasi beserta konteks untuk panel
// internal — usaha pemiliknya, tagihannya, dan paketnya.
type PlatformPaymentClaimResponse struct {
	PaymentClaimResponse
	TenantID     string `json:"tenant_id"`
	BusinessName string `json:"business_name"`
	TenantPhone  string `json:"tenant_phone,omitempty"`
	InvoiceTotal int64  `json:"invoice_total"`
	InvoicePaid  int64  `json:"invoice_paid"`
	PlanName     string `json:"plan_name,omitempty"`
}

// PaymentInstructions: rekening tujuan pembayaran langganan (konfigurasi
// SUBSCRIPTION_BANK_*).
type PaymentInstructions struct {
	BankName      string `json:"bank_name"`
	AccountNumber string `json:"account_number"`
	AccountHolder string `json:"account_holder,omitempty"`
}
