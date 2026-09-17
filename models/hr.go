package models

import (
	"time"

	"candra/backend-api/internal/timez"
	"candra/backend-api/internal/ulid"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Enum SDM (cermin CHECK migrasi 000029/000030).
var (
	EmploymentStatuses  = []string{"permanent", "contract", "probation", "daily"}
	WageTypes           = []string{"monthly", "daily", "hourly"}
	PayrollPeriodTypes  = []string{"daily", "weekly", "biweekly", "monthly"}
	AttendanceKinds     = []string{"in", "out"}
	AttendanceDayStatus = []string{"present", "late", "leave", "sick", "permit", "holiday", "off", "absent"}
	LeaveKinds          = []string{"permit", "sick", "leave", "unpaid"}
	LeaveStatuses       = []string{"pending", "approved", "rejected", "canceled"}
)

// Employee adalah karyawan tenant. `user_id` NULL untuk karyawan tanpa akun
// aplikasi (mis. juru masak). Data pribadi tunduk UU PDP.
type Employee struct {
	ID       string  `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID string  `json:"tenant_id" gorm:"type:char(26);not null;index"`
	UserID   *string `json:"user_id" gorm:"type:char(26)"`
	OutletID string  `json:"outlet_id" gorm:"type:char(26);not null"`
	// EmployeeNo *string: NULL bila kosong. Kolom ini terkena partial unique
	// index `WHERE employee_no IS NOT NULL`, jadi "" TIDAK boleh dipakai sebagai
	// "tidak ada" — karyawan kedua tanpa nomor akan bentrok dengan yang pertama.
	// Pola yang sama dipakai SKU/Barcode di models/product.go.
	EmployeeNo        *string    `json:"employee_no"`
	FullName          string     `json:"full_name" gorm:"not null"`
	Phone             string     `json:"phone"`
	Email             string     `json:"email"`
	IDNumber          string     `json:"id_number" gorm:"column:id_number"`
	NPWP              string     `json:"npwp" gorm:"column:npwp"`
	Address           string     `json:"address"`
	BirthDate         *time.Time `json:"birth_date" gorm:"type:date"`
	Position          string     `json:"position"`
	EmploymentStatus  string     `json:"employment_status" gorm:"not null;default:permanent"`
	WageType          string     `json:"wage_type" gorm:"not null"`
	BaseWage          int64      `json:"base_wage" gorm:"not null;default:0"`
	PayrollPeriodType string     `json:"payroll_period_type" gorm:"not null;default:monthly"`
	BankName          string     `json:"bank_name"`
	BankAccountNo     string     `json:"bank_account_no"`
	BankAccountName   string     `json:"bank_account_name"`
	JoinedAt          time.Time  `json:"joined_at" gorm:"type:date"`
	ResignedAt        *time.Time `json:"resigned_at" gorm:"type:date"`
	IsActive          bool       `json:"is_active" gorm:"not null;default:true"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	SyncVersion int64 `json:"sync_version" gorm:"->;column:sync_version"`
}

func (e *Employee) BeforeCreate(tx *gorm.DB) (err error) {
	if e.ID == "" {
		e.ID = ulid.New()
	}
	return
}

// WorkSchedule adalah pola jam kerja per hari-dalam-minggu, berlaku dari
// `effective_from`. weekday 0 = Minggu. end_time < start_time = lewat tengah malam.
type WorkSchedule struct {
	ID                   string       `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID             string       `json:"tenant_id" gorm:"type:char(26);not null;index"`
	EmployeeID           string       `json:"employee_id" gorm:"type:char(26);not null;index"`
	Weekday              int          `json:"weekday" gorm:"not null"`
	IsWorkingDay         bool         `json:"is_working_day" gorm:"not null;default:true"`
	StartTime            *timez.Clock `json:"start_time" gorm:"type:time"`
	EndTime              *timez.Clock `json:"end_time" gorm:"type:time"`
	BreakMinutes         int          `json:"break_minutes" gorm:"not null;default:0"`
	LateToleranceMinutes int          `json:"late_tolerance_minutes" gorm:"not null;default:0"`
	EffectiveFrom        time.Time    `json:"effective_from" gorm:"type:date"`
	EffectiveTo          *time.Time   `json:"effective_to" gorm:"type:date"`
	CreatedAt            time.Time    `json:"created_at"`
}

func (s *WorkSchedule) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == "" {
		s.ID = ulid.New()
	}
	return
}

// Holiday adalah hari libur; outlet_id NULL = berlaku semua outlet.
type Holiday struct {
	ID          string    `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID    string    `json:"tenant_id" gorm:"type:char(26);not null;index"`
	OutletID    *string   `json:"outlet_id" gorm:"type:char(26)"`
	HolidayDate time.Time `json:"holiday_date" gorm:"type:date"`
	Name        string    `json:"name" gorm:"not null"`
	IsPaid      bool      `json:"is_paid" gorm:"not null;default:true"`
	CreatedAt   time.Time `json:"created_at"`
}

func (h *Holiday) BeforeCreate(tx *gorm.DB) (err error) {
	if h.ID == "" {
		h.ID = ulid.New()
	}
	return
}

// Attendance adalah SATU ketukan absen (in/out) — buku besar, hanya tambah.
// Dibuat klien (ULID). `business_date` dihitung server dari zona outlet.
type Attendance struct {
	ID              string           `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID        string           `json:"tenant_id" gorm:"type:char(26);not null;index"`
	EmployeeID      string           `json:"employee_id" gorm:"type:char(26);not null;index"`
	OutletID        string           `json:"outlet_id" gorm:"type:char(26);not null"`
	Kind            string           `json:"kind" gorm:"not null"`
	OccurredAt      time.Time        `json:"occurred_at"`
	BusinessDate    time.Time        `json:"business_date" gorm:"type:date"`
	Source          string           `json:"source" gorm:"not null;default:manual"`
	PhotoURL        string           `json:"photo_url"`
	Latitude        *decimal.Decimal `json:"latitude" gorm:"type:numeric(9,6)"`
	Longitude       *decimal.Decimal `json:"longitude" gorm:"type:numeric(9,6)"`
	DeviceID        string           `json:"device_id"`
	ClientCreatedAt *time.Time       `json:"client_created_at"`
	Reason          string           `json:"reason"`
	ApprovedBy      *string          `json:"approved_by" gorm:"type:char(26)"`
	CreatedBy       string           `json:"created_by" gorm:"type:char(26);not null"`
	CreatedAt       time.Time        `json:"created_at"`

	SyncVersion int64 `json:"sync_version" gorm:"->;column:sync_version"`
}

func (a *Attendance) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == "" {
		a.ID = ulid.New()
	}
	return
}

// AttendanceCorrection mengoreksi waktu sebuah Attendance TANPA mengubah baris
// asli. Satu absensi hanya boleh punya satu koreksi disetujui.
type AttendanceCorrection struct {
	ID            string     `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID      string     `json:"tenant_id" gorm:"type:char(26);not null;index"`
	AttendanceID  string     `json:"attendance_id" gorm:"type:char(26);not null;index"`
	NewOccurredAt time.Time  `json:"new_occurred_at"`
	Reason        string     `json:"reason" gorm:"not null"`
	RequestedBy   string     `json:"requested_by" gorm:"type:char(26);not null"`
	ApprovedBy    *string    `json:"approved_by" gorm:"type:char(26)"`
	ApprovedAt    *time.Time `json:"approved_at"`
	Status        string     `json:"status" gorm:"not null;default:pending"`
	CreatedAt     time.Time  `json:"created_at"`
}

func (c *AttendanceCorrection) BeforeCreate(tx *gorm.DB) (err error) {
	if c.ID == "" {
		c.ID = ulid.New()
	}
	return
}

// AttendanceDay adalah CACHE status harian seorang karyawan — boleh dihitung
// ulang kapan saja dari attendances + koreksi + jadwal + libur + cuti.
type AttendanceDay struct {
	TenantID          string     `json:"tenant_id" gorm:"primaryKey;type:char(26)"`
	EmployeeID        string     `json:"employee_id" gorm:"primaryKey;type:char(26)"`
	BusinessDate      time.Time  `json:"business_date" gorm:"primaryKey;type:date"`
	Status            string     `json:"status" gorm:"not null"`
	ScheduledStart    *time.Time `json:"scheduled_start"`
	ScheduledEnd      *time.Time `json:"scheduled_end"`
	FirstIn           *time.Time `json:"first_in"`
	LastOut           *time.Time `json:"last_out"`
	LateMinutes       int        `json:"late_minutes" gorm:"not null;default:0"`
	EarlyLeaveMinutes int        `json:"early_leave_minutes" gorm:"not null;default:0"`
	WorkMinutes       int        `json:"work_minutes" gorm:"not null;default:0"`
	OvertimeMinutes   int        `json:"overtime_minutes" gorm:"not null;default:0"`
	LeaveRequestID    *string    `json:"leave_request_id" gorm:"type:char(26)"`
	ComputedAt        time.Time  `json:"computed_at"`
}

func (AttendanceDay) TableName() string { return "attendance_days" }

// LeaveRequest — pengajuan cuti/izin. `is_paid` di-SNAPSHOT saat disetujui.
type LeaveRequest struct {
	ID            string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID      string          `json:"tenant_id" gorm:"type:char(26);not null;index"`
	EmployeeID    string          `json:"employee_id" gorm:"type:char(26);not null;index"`
	Kind          string          `json:"kind" gorm:"not null"`
	StartDate     time.Time       `json:"start_date" gorm:"type:date"`
	EndDate       time.Time       `json:"end_date" gorm:"type:date"`
	Days          decimal.Decimal `json:"days" gorm:"type:numeric(5,1);not null"`
	Reason        string          `json:"reason"`
	AttachmentURL string          `json:"attachment_url"`
	IsPaid        bool            `json:"is_paid" gorm:"not null"`
	Status        string          `json:"status" gorm:"not null;default:pending"`
	ApprovedBy    *string         `json:"approved_by" gorm:"type:char(26)"`
	ApprovedAt    *time.Time      `json:"approved_at"`
	RejectReason  string          `json:"reject_reason"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`

	SyncVersion int64 `json:"sync_version" gorm:"->;column:sync_version"`
}

func (l *LeaveRequest) BeforeCreate(tx *gorm.DB) (err error) {
	if l.ID == "" {
		l.ID = ulid.New()
	}
	return
}

// LeaveBalance — kuota & pemakaian cuti tahunan.
type LeaveBalance struct {
	TenantID        string          `json:"tenant_id" gorm:"primaryKey;type:char(26)"`
	EmployeeID      string          `json:"employee_id" gorm:"primaryKey;type:char(26)"`
	Year            int             `json:"year" gorm:"primaryKey"`
	QuotaDays       decimal.Decimal `json:"quota_days" gorm:"type:numeric(5,1);not null;default:12"`
	UsedDays        decimal.Decimal `json:"used_days" gorm:"type:numeric(5,1);not null;default:0"`
	CarriedOverDays decimal.Decimal `json:"carried_over_days" gorm:"type:numeric(5,1);not null;default:0"`
}

func (LeaveBalance) TableName() string { return "leave_balances" }
