package structs

// DTO SDM — karyawan, jadwal, libur, absensi, cuti (Fase 13, §5.11).

// ── Employee ──────────────────────────────────────────────────────────────

type EmployeeCreateRequest struct {
	UserID            string `json:"user_id" binding:"omitempty,ulid"`
	OutletID          string `json:"outlet_id" binding:"required,ulid"`
	EmployeeNo        string `json:"employee_no" binding:"omitempty,max=40"`
	FullName          string `json:"full_name" binding:"required,min=1,max=150"`
	Phone             string `json:"phone" binding:"omitempty,max=30"`
	Email             string `json:"email" binding:"omitempty,email"`
	Position          string `json:"position" binding:"omitempty,max=80"`
	EmploymentStatus  string `json:"employment_status" binding:"omitempty,oneof=permanent contract probation daily"`
	WageType          string `json:"wage_type" binding:"required,oneof=monthly daily hourly"`
	BaseWage          int64  `json:"base_wage" binding:"omitempty,gte=0"`
	PayrollPeriodType string `json:"payroll_period_type" binding:"omitempty,oneof=daily weekly biweekly monthly"`
	BankName          string `json:"bank_name" binding:"omitempty,max=60"`
	BankAccountNo     string `json:"bank_account_no" binding:"omitempty,max=40"`
	BankAccountName   string `json:"bank_account_name" binding:"omitempty,max=120"`
	JoinedAt          string `json:"joined_at" binding:"required"` // YYYY-MM-DD
}

type EmployeeUpdateRequest struct {
	FullName          *string `json:"full_name" binding:"omitempty,min=1,max=150"`
	Phone             *string `json:"phone" binding:"omitempty,max=30"`
	Email             *string `json:"email" binding:"omitempty"`
	Position          *string `json:"position" binding:"omitempty,max=80"`
	EmploymentStatus  *string `json:"employment_status" binding:"omitempty,oneof=permanent contract probation daily"`
	WageType          *string `json:"wage_type" binding:"omitempty,oneof=monthly daily hourly"`
	BaseWage          *int64  `json:"base_wage" binding:"omitempty,gte=0"`
	PayrollPeriodType *string `json:"payroll_period_type" binding:"omitempty,oneof=daily weekly biweekly monthly"`
	BankName          *string `json:"bank_name" binding:"omitempty,max=60"`
	BankAccountNo     *string `json:"bank_account_no" binding:"omitempty,max=40"`
	BankAccountName   *string `json:"bank_account_name" binding:"omitempty,max=120"`
	ResignedAt        *string `json:"resigned_at" binding:"omitempty"`
	IsActive          *bool   `json:"is_active"`
}

type EmployeeResponse struct {
	ID                string `json:"id"`
	UserID            string `json:"user_id,omitempty"`
	OutletID          string `json:"outlet_id"`
	EmployeeNo        string `json:"employee_no,omitempty"`
	FullName          string `json:"full_name"`
	Phone             string `json:"phone,omitempty"`
	Position          string `json:"position,omitempty"`
	EmploymentStatus  string `json:"employment_status"`
	WageType          string `json:"wage_type"`
	BaseWage          int64  `json:"base_wage"`
	PayrollPeriodType string `json:"payroll_period_type"`
	JoinedAt          string `json:"joined_at"`
	ResignedAt        string `json:"resigned_at,omitempty"`
	IsActive          bool   `json:"is_active"`
}

// ── Work schedule ─────────────────────────────────────────────────────────

type WorkScheduleRequest struct {
	Weekday              int    `json:"weekday" binding:"gte=0,lte=6"`
	IsWorkingDay         *bool  `json:"is_working_day"`
	StartTime            string `json:"start_time" binding:"omitempty"` // HH:MM
	EndTime              string `json:"end_time" binding:"omitempty"`
	BreakMinutes         int    `json:"break_minutes" binding:"omitempty,gte=0"`
	LateToleranceMinutes int    `json:"late_tolerance_minutes" binding:"omitempty,gte=0"`
	EffectiveFrom        string `json:"effective_from" binding:"required"` // YYYY-MM-DD
}

type WorkScheduleResponse struct {
	ID                   string `json:"id"`
	Weekday              int    `json:"weekday"`
	IsWorkingDay         bool   `json:"is_working_day"`
	StartTime            string `json:"start_time,omitempty"`
	EndTime              string `json:"end_time,omitempty"`
	BreakMinutes         int    `json:"break_minutes"`
	LateToleranceMinutes int    `json:"late_tolerance_minutes"`
	EffectiveFrom        string `json:"effective_from"`
}

// ── Holiday ───────────────────────────────────────────────────────────────

type HolidayRequest struct {
	OutletID    string `json:"outlet_id" binding:"omitempty,ulid"`
	HolidayDate string `json:"holiday_date" binding:"required"` // YYYY-MM-DD
	Name        string `json:"name" binding:"required,min=1,max=120"`
	IsPaid      *bool  `json:"is_paid"`
}

type HolidayResponse struct {
	ID          string `json:"id"`
	OutletID    string `json:"outlet_id,omitempty"`
	HolidayDate string `json:"holiday_date"`
	Name        string `json:"name"`
	IsPaid      bool   `json:"is_paid"`
}

// ── Attendance ────────────────────────────────────────────────────────────

type AttendanceRecordRequest struct {
	ID         string `json:"id" binding:"omitempty,ulid"`
	EmployeeID string `json:"employee_id" binding:"required,ulid"`
	Kind       string `json:"kind" binding:"required,oneof=in out"`
	OccurredAt string `json:"occurred_at" binding:"omitempty"` // RFC3339
	PhotoURL   string `json:"photo_url" binding:"omitempty,max=500"`
	Latitude   string `json:"latitude" binding:"omitempty"`
	Longitude  string `json:"longitude" binding:"omitempty"`
	DeviceID   string `json:"device_id" binding:"omitempty,max=80"`
}

type AttendanceResponse struct {
	ID           string `json:"id"`
	EmployeeID   string `json:"employee_id"`
	OutletID     string `json:"outlet_id"`
	Kind         string `json:"kind"`
	OccurredAt   string `json:"occurred_at"`
	BusinessDate string `json:"business_date"`
	Source       string `json:"source"`
	PhotoURL     string `json:"photo_url,omitempty"`
}

type CorrectionRequest struct {
	AttendanceID  string `json:"attendance_id" binding:"required,ulid"`
	NewOccurredAt string `json:"new_occurred_at" binding:"required"` // RFC3339
	Reason        string `json:"reason" binding:"required,min=3,max=255"`
}

// CorrectionAdjustment opsional saat menyetujui koreksi untuk hari di periode
// gaji yang sudah dikunci — menghasilkan payroll_adjustment ke periode berikutnya.
type CorrectionAdjustment struct {
	Category string `json:"category" binding:"required,oneof=earning deduction"`
	Amount   int64  `json:"amount" binding:"required,gt=0"`
	Name     string `json:"name" binding:"required,min=1,max=120"`
}

type CorrectionApproveRequest struct {
	Adjustment *CorrectionAdjustment `json:"adjustment" binding:"omitempty"`
}

type CorrectionResponse struct {
	ID            string `json:"id"`
	AttendanceID  string `json:"attendance_id"`
	NewOccurredAt string `json:"new_occurred_at"`
	Reason        string `json:"reason"`
	Status        string `json:"status"`
	ApprovedAt    string `json:"approved_at,omitempty"`
}

// ── Leave ─────────────────────────────────────────────────────────────────

type LeaveRequestCreate struct {
	EmployeeID string `json:"employee_id" binding:"required,ulid"`
	Kind       string `json:"kind" binding:"required,oneof=permit sick leave unpaid"`
	StartDate  string `json:"start_date" binding:"required"`
	EndDate    string `json:"end_date" binding:"required"`
	Days       string `json:"days" binding:"required"` // "1" atau "0.5"
	Reason     string `json:"reason" binding:"omitempty,max=255"`
}

type LeaveApproveRequest struct {
	IsPaid       *bool  `json:"is_paid"` // snapshot kebijakan; default true untuk leave/sick, false untuk unpaid
	RejectReason string `json:"reject_reason" binding:"omitempty,max=255"`
}

type LeaveRequestResponse struct {
	ID         string `json:"id"`
	EmployeeID string `json:"employee_id"`
	Kind       string `json:"kind"`
	StartDate  string `json:"start_date"`
	EndDate    string `json:"end_date"`
	Days       string `json:"days"`
	IsPaid     bool   `json:"is_paid"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
}
