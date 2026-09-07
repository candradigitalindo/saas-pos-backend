package structs

// DTO CRM sales lapangan — kunjungan, target, komisi (Fase 10, §5.9).

// ── Visit plan ────────────────────────────────────────────────────────────

type VisitPlanCreateRequest struct {
	PlanDate    string   `json:"plan_date" binding:"required"`      // YYYY-MM-DD
	OwnerID     string   `json:"owner_id" binding:"omitempty,ulid"` // kosong → diri sendiri
	CustomerIDs []string `json:"customer_ids" binding:"required,min=1,dive,ulid"`
}

type VisitPlanResponse struct {
	ID       string          `json:"id"`
	OwnerID  string          `json:"owner_id"`
	PlanDate string          `json:"plan_date"`
	Status   string          `json:"status"`
	Visits   []VisitResponse `json:"visits,omitempty"`
}

// ── Visit ─────────────────────────────────────────────────────────────────

// VisitUpsertRequest dipakai POST /visits dan op sync 'visit.upsert'. `id` ULID
// klien; kolom waktu opsional (perangkat mengisinya offline).
type VisitUpsertRequest struct {
	ID            string `json:"id" binding:"omitempty,ulid"`
	VisitPlanID   string `json:"visit_plan_id" binding:"omitempty,ulid"`
	CustomerID    string `json:"customer_id" binding:"required,ulid"`
	CheckinAt     string `json:"checkin_at" binding:"omitempty"`  // RFC3339
	CheckoutAt    string `json:"checkout_at" binding:"omitempty"` // RFC3339
	CheckinLat    string `json:"checkin_lat" binding:"omitempty"`
	CheckinLng    string `json:"checkin_lng" binding:"omitempty"`
	PhotoURL      string `json:"photo_url" binding:"omitempty,max=500"`
	Result        string `json:"result" binding:"omitempty,oneof=pending order no_order closed rejected"`
	NoOrderReason string `json:"no_order_reason" binding:"omitempty,max=255"`
	SaleID        string `json:"sale_id" binding:"omitempty,ulid"`
}

type VisitCheckoutRequest struct {
	Result        string `json:"result" binding:"required,oneof=order no_order closed rejected"`
	NoOrderReason string `json:"no_order_reason" binding:"omitempty,max=255"`
	CheckoutAt    string `json:"checkout_at" binding:"omitempty"`
	PhotoURL      string `json:"photo_url" binding:"omitempty,max=500"`
	SaleID        string `json:"sale_id" binding:"omitempty,ulid"`
}

type VisitResponse struct {
	ID            string `json:"id"`
	VisitPlanID   string `json:"visit_plan_id,omitempty"`
	CustomerID    string `json:"customer_id"`
	OwnerID       string `json:"owner_id"`
	CheckinAt     string `json:"checkin_at,omitempty"`
	CheckoutAt    string `json:"checkout_at,omitempty"`
	CheckinLat    string `json:"checkin_lat,omitempty"`
	CheckinLng    string `json:"checkin_lng,omitempty"`
	PhotoURL      string `json:"photo_url,omitempty"`
	Result        string `json:"result"`
	NoOrderReason string `json:"no_order_reason,omitempty"`
	SaleID        string `json:"sale_id,omitempty"`
	BusinessDate  string `json:"business_date"`
}

// ── Sales target ──────────────────────────────────────────────────────────

type SalesTargetRequest struct {
	UserID       string `json:"user_id" binding:"required,ulid"`
	PeriodStart  string `json:"period_start" binding:"required"` // YYYY-MM-DD
	PeriodEnd    string `json:"period_end" binding:"required"`
	TargetAmount int64  `json:"target_amount" binding:"omitempty,gte=0"`
	TargetVisits int    `json:"target_visits" binding:"omitempty,gte=0"`
}

type SalesTargetResponse struct {
	ID             string `json:"id"`
	UserID         string `json:"user_id"`
	PeriodStart    string `json:"period_start"`
	PeriodEnd      string `json:"period_end"`
	TargetAmount   int64  `json:"target_amount"`
	TargetVisits   int    `json:"target_visits"`
	AchievedAmount int64  `json:"achieved_amount"` // nilai tertagih pada periode
	AchievedVisits int    `json:"achieved_visits"` // kunjungan selesai pada periode
}

// ── Commission ────────────────────────────────────────────────────────────

type CommissionComputeRequest struct {
	UserID      string `json:"user_id" binding:"required,ulid"`
	PeriodStart string `json:"period_start" binding:"required"` // YYYY-MM-DD
	PeriodEnd   string `json:"period_end" binding:"required"`
	Rate        string `json:"rate" binding:"required"` // pecahan: "0.05"
}

type CommissionResponse struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
	BaseAmount  int64  `json:"base_amount"`
	Rate        string `json:"rate"`
	Amount      int64  `json:"amount"`
	Status      string `json:"status"`
}
