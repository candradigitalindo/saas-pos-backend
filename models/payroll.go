package models

import (
	"encoding/json"
	"time"

	"candra/backend-api/internal/ulid"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Enum penggajian (cermin CHECK migrasi 000031).
var (
	PayrollRuleTypes      = []string{"tunjangan_tetap", "potongan_telat", "potongan_alpa", "upah_lembur", "bonus_kehadiran", "bonus_target", "potongan_kasbon", "komponen_manual"}
	PayrollRuleCategories = []string{"earning", "deduction"}
	PayrollPeriodStatuses = []string{"draft", "calculated", "locked", "paid", "canceled"}
	PayslipStatuses       = []string{"draft", "locked", "paid"}
	AdvanceStatuses       = []string{"pending", "approved", "disbursed", "settled", "canceled"}
)

// PayrollRule adalah satu komponen gaji yang berlaku pada rentang tanggal.
// `params` JSONB (dikecualikan dari aturan "JSONB hanya muatan luar", §5.11):
// bentuknya divalidasi per `type` di service, tak pernah di-query SQL, dan
// disalin ke `payslip_lines.params_snapshot` saat dipakai.
type PayrollRule struct {
	ID            string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID      string          `json:"tenant_id" gorm:"type:char(26);not null;index"`
	Code          string          `json:"code" gorm:"not null"`
	Name          string          `json:"name" gorm:"not null"`
	Type          string          `json:"type" gorm:"not null"`
	Category      string          `json:"category" gorm:"not null"`
	Params        json.RawMessage `json:"params" gorm:"type:jsonb;not null"`
	TargetType    string          `json:"target_type" gorm:"not null;default:all"`
	TargetID      *string         `json:"target_id" gorm:"type:char(26)"`
	Priority      int             `json:"priority" gorm:"not null;default:100"`
	EffectiveFrom time.Time       `json:"effective_from" gorm:"type:date"`
	EffectiveTo   *time.Time      `json:"effective_to" gorm:"type:date"`
	IsActive      bool            `json:"is_active" gorm:"not null;default:true"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

func (r *PayrollRule) BeforeCreate(tx *gorm.DB) (err error) {
	if r.ID == "" {
		r.ID = ulid.New()
	}
	return
}

// PayrollPeriod adalah satu siklus gaji. Dihitung → dikunci → dibayar.
type PayrollPeriod struct {
	ID             string     `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID       string     `json:"tenant_id" gorm:"type:char(26);not null;index"`
	OutletID       *string    `json:"outlet_id" gorm:"type:char(26)"`
	PeriodType     string     `json:"period_type" gorm:"not null"`
	StartDate      time.Time  `json:"start_date" gorm:"type:date"`
	EndDate        time.Time  `json:"end_date" gorm:"type:date"`
	PayDate        *time.Time `json:"pay_date" gorm:"type:date"`
	Status         string     `json:"status" gorm:"not null;default:draft"`
	TotalGross     int64      `json:"total_gross" gorm:"not null;default:0"`
	TotalDeduction int64      `json:"total_deduction" gorm:"not null;default:0"`
	TotalNet       int64      `json:"total_net" gorm:"not null;default:0"`
	CalculatedAt   *time.Time `json:"calculated_at"`
	LockedAt       *time.Time `json:"locked_at"`
	LockedBy       *string    `json:"locked_by" gorm:"type:char(26)"`
	PaidAt         *time.Time `json:"paid_at"`
	CreatedBy      string     `json:"created_by" gorm:"type:char(26);not null"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (p *PayrollPeriod) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = ulid.New()
	}
	return
}

// Payslip adalah slip gaji satu karyawan pada satu periode. net tak pernah minus;
// kekurangan potongan jadi `carried_debt` untuk periode berikutnya.
type Payslip struct {
	ID              string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID        string          `json:"tenant_id" gorm:"type:char(26);not null;index"`
	PayrollPeriodID string          `json:"payroll_period_id" gorm:"type:char(26);not null;index"`
	EmployeeID      string          `json:"employee_id" gorm:"type:char(26);not null"`
	GrossAmount     int64           `json:"gross_amount" gorm:"not null;default:0"`
	DeductionAmount int64           `json:"deduction_amount" gorm:"not null;default:0"`
	NetAmount       int64           `json:"net_amount" gorm:"not null;default:0"`
	CarriedDebt     int64           `json:"carried_debt" gorm:"not null;default:0"`
	PresentDays     decimal.Decimal `json:"present_days" gorm:"type:numeric(5,1);not null;default:0"`
	LateCount       int             `json:"late_count" gorm:"not null;default:0"`
	AbsentDays      decimal.Decimal `json:"absent_days" gorm:"type:numeric(5,1);not null;default:0"`
	LeaveDays       decimal.Decimal `json:"leave_days" gorm:"type:numeric(5,1);not null;default:0"`
	OvertimeMinutes int             `json:"overtime_minutes" gorm:"not null;default:0"`
	Status          string          `json:"status" gorm:"not null;default:draft"`
	PaidAt          *time.Time      `json:"paid_at"`
	PaymentMethod   string          `json:"payment_method"`
	Note            string          `json:"note"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`

	Lines []PayslipLine `json:"lines,omitempty" gorm:"foreignKey:PayslipID;references:ID"`
}

func (s *Payslip) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == "" {
		s.ID = ulid.New()
	}
	return
}

// PayslipLine adalah satu baris slip dengan SNAPSHOT nama/tipe/params aturan.
type PayslipLine struct {
	ID             string           `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID       string           `json:"tenant_id" gorm:"type:char(26);not null;index"`
	PayslipID      string           `json:"payslip_id" gorm:"type:char(26);not null;index"`
	RuleID         *string          `json:"rule_id" gorm:"type:char(26)"`
	Name           string           `json:"name" gorm:"not null"`
	Category       string           `json:"category" gorm:"not null"`
	RuleType       string           `json:"rule_type"`
	ParamsSnapshot json.RawMessage  `json:"params_snapshot,omitempty" gorm:"type:jsonb"`
	BasisNote      string           `json:"basis_note"`
	Quantity       *decimal.Decimal `json:"quantity" gorm:"type:numeric(14,3)"`
	Amount         int64            `json:"amount" gorm:"not null"`
	SortOrder      int              `json:"sort_order" gorm:"not null;default:0"`
}

func (l *PayslipLine) BeforeCreate(tx *gorm.DB) (err error) {
	if l.ID == "" {
		l.ID = ulid.New()
	}
	return
}

// PayrollAdjustment adalah koreksi terlambat: dibuat setelah periode dikunci,
// dibebankan ke periode BERIKUTNYA.
type PayrollAdjustment struct {
	ID               string    `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID         string    `json:"tenant_id" gorm:"type:char(26);not null;index"`
	EmployeeID       string    `json:"employee_id" gorm:"type:char(26);not null;index"`
	OriginPeriodID   string    `json:"origin_period_id" gorm:"type:char(26);not null"`
	TargetPeriodID   *string   `json:"target_period_id" gorm:"type:char(26)"`
	Name             string    `json:"name" gorm:"not null"`
	Category         string    `json:"category" gorm:"not null"`
	Amount           int64     `json:"amount" gorm:"not null"`
	Reason           string    `json:"reason" gorm:"not null"`
	AppliedPayslipID *string   `json:"applied_payslip_id" gorm:"type:char(26)"`
	CreatedBy        string    `json:"created_by" gorm:"type:char(26);not null"`
	CreatedAt        time.Time `json:"created_at"`
}

func (a *PayrollAdjustment) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == "" {
		a.ID = ulid.New()
	}
	return
}

// EmployeeAdvance adalah kasbon karyawan; dipotong bertahap lewat gaji.
type EmployeeAdvance struct {
	ID                string     `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID          string     `json:"tenant_id" gorm:"type:char(26);not null;index"`
	EmployeeID        string     `json:"employee_id" gorm:"type:char(26);not null;index"`
	Amount            int64      `json:"amount" gorm:"not null"`
	Remaining         int64      `json:"remaining" gorm:"not null"`
	InstallmentAmount int64      `json:"installment_amount" gorm:"not null"`
	Reason            string     `json:"reason"`
	Status            string     `json:"status" gorm:"not null;default:pending"`
	ApprovedBy        *string    `json:"approved_by" gorm:"type:char(26)"`
	ApprovedAt        *time.Time `json:"approved_at"`
	DisbursedAt       *time.Time `json:"disbursed_at"`
	CashMovementID    *string    `json:"cash_movement_id" gorm:"type:char(26)"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func (a *EmployeeAdvance) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == "" {
		a.ID = ulid.New()
	}
	return
}

// AdvanceRepayment adalah satu cicilan pelunasan kasbon (biasanya lewat slip).
type AdvanceRepayment struct {
	ID           string    `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID     string    `json:"tenant_id" gorm:"type:char(26);not null;index"`
	AdvanceID    string    `json:"advance_id" gorm:"type:char(26);not null;index"`
	PayslipID    *string   `json:"payslip_id" gorm:"type:char(26)"`
	Amount       int64     `json:"amount" gorm:"not null"`
	PaidAt       time.Time `json:"paid_at"`
	BusinessDate time.Time `json:"business_date" gorm:"type:date"`
}

func (r *AdvanceRepayment) BeforeCreate(tx *gorm.DB) (err error) {
	if r.ID == "" {
		r.ID = ulid.New()
	}
	return
}
