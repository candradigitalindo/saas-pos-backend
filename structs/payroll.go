package structs

import "encoding/json"

// DTO penggajian (Fase 13, §5.11, §13.6).

// ── Payroll rule ──────────────────────────────────────────────────────────

type PayrollRuleRequest struct {
	Code          string          `json:"code" binding:"required,min=1,max=40"`
	Name          string          `json:"name" binding:"required,min=1,max=120"`
	Type          string          `json:"type" binding:"required"`
	Category      string          `json:"category" binding:"required,oneof=earning deduction"`
	Params        json.RawMessage `json:"params" binding:"required"`
	TargetType    string          `json:"target_type" binding:"omitempty,oneof=all role employee"`
	TargetID      string          `json:"target_id" binding:"omitempty,ulid"`
	Priority      int             `json:"priority" binding:"omitempty,gte=0"`
	EffectiveFrom string          `json:"effective_from" binding:"required"`
	EffectiveTo   string          `json:"effective_to" binding:"omitempty"`
}

type PayrollRuleResponse struct {
	ID            string          `json:"id"`
	Code          string          `json:"code"`
	Name          string          `json:"name"`
	Type          string          `json:"type"`
	Category      string          `json:"category"`
	Params        json.RawMessage `json:"params"`
	TargetType    string          `json:"target_type"`
	TargetID      string          `json:"target_id,omitempty"`
	Priority      int             `json:"priority"`
	EffectiveFrom string          `json:"effective_from"`
	EffectiveTo   string          `json:"effective_to,omitempty"`
	IsActive      bool            `json:"is_active"`
}

// ── Payroll period ────────────────────────────────────────────────────────

type PayrollPeriodRequest struct {
	OutletID   string `json:"outlet_id" binding:"omitempty,ulid"`
	PeriodType string `json:"period_type" binding:"required,oneof=daily weekly biweekly monthly"`
	StartDate  string `json:"start_date" binding:"required"`
	EndDate    string `json:"end_date" binding:"required"`
}

type PayrollPeriodResponse struct {
	ID             string `json:"id"`
	OutletID       string `json:"outlet_id,omitempty"`
	PeriodType     string `json:"period_type"`
	StartDate      string `json:"start_date"`
	EndDate        string `json:"end_date"`
	Status         string `json:"status"`
	TotalGross     int64  `json:"total_gross"`
	TotalDeduction int64  `json:"total_deduction"`
	TotalNet       int64  `json:"total_net"`
	CalculatedAt   string `json:"calculated_at,omitempty"`
	LockedAt       string `json:"locked_at,omitempty"`
	PaidAt         string `json:"paid_at,omitempty"`
}

// ── Payslip ───────────────────────────────────────────────────────────────

type PayslipLineResponse struct {
	Name      string `json:"name"`
	Category  string `json:"category"`
	RuleType  string `json:"rule_type,omitempty"`
	BasisNote string `json:"basis_note,omitempty"`
	Quantity  string `json:"quantity,omitempty"`
	Amount    int64  `json:"amount"`
	SortOrder int    `json:"sort_order"`
}

type PayslipResponse struct {
	ID              string                `json:"id"`
	PayrollPeriodID string                `json:"payroll_period_id"`
	EmployeeID      string                `json:"employee_id"`
	GrossAmount     int64                 `json:"gross_amount"`
	DeductionAmount int64                 `json:"deduction_amount"`
	NetAmount       int64                 `json:"net_amount"`
	CarriedDebt     int64                 `json:"carried_debt"`
	PresentDays     string                `json:"present_days"`
	LateCount       int                   `json:"late_count"`
	AbsentDays      string                `json:"absent_days"`
	LeaveDays       string                `json:"leave_days"`
	OvertimeMinutes int                   `json:"overtime_minutes"`
	Status          string                `json:"status"`
	Lines           []PayslipLineResponse `json:"lines,omitempty"`
}

// ── Employee advance (kasbon) ─────────────────────────────────────────────

type AdvanceCreateRequest struct {
	EmployeeID        string `json:"employee_id" binding:"required,ulid"`
	Amount            int64  `json:"amount" binding:"required,gt=0"`
	InstallmentAmount int64  `json:"installment_amount" binding:"required,gt=0"`
	Reason            string `json:"reason" binding:"omitempty,max=255"`
}

type AdvanceResponse struct {
	ID                string `json:"id"`
	EmployeeID        string `json:"employee_id"`
	Amount            int64  `json:"amount"`
	Remaining         int64  `json:"remaining"`
	InstallmentAmount int64  `json:"installment_amount"`
	Status            string `json:"status"`
	Reason            string `json:"reason,omitempty"`
}
