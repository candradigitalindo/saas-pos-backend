package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"gorm.io/gorm"
)

// Shift adalah sesi kas satu outlet. Hanya boleh ada satu shift 'open' per outlet
// (partial unique index uq_shifts_open_per_outlet).
type Shift struct {
	ID           string     `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID     string     `json:"tenant_id" gorm:"type:char(26);not null;index"`
	OutletID     string     `json:"outlet_id" gorm:"type:char(26);not null"`
	OpenedBy     string     `json:"opened_by" gorm:"type:char(26);not null"`
	ClosedBy     *string    `json:"closed_by" gorm:"type:char(26)"`
	OpenedAt     time.Time  `json:"opened_at"`
	ClosedAt     *time.Time `json:"closed_at"`
	BusinessDate time.Time  `json:"business_date" gorm:"type:date"`
	OpeningCash  int64      `json:"opening_cash" gorm:"not null;default:0"`
	ExpectedCash int64      `json:"expected_cash" gorm:"not null;default:0"` // dihitung saat tutup
	CountedCash  *int64     `json:"counted_cash"`                            // hasil hitung fisik
	Difference   *int64     `json:"difference"`                              // counted - expected
	Note         string     `json:"note"`
	Status       string     `json:"status" gorm:"not null;default:open"` // 'open' | 'closed'

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *Shift) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == "" {
		s.ID = ulid.New()
	}
	return
}

// CashMovement adalah kas masuk/keluar non-penjualan pada sebuah shift.
type CashMovement struct {
	ID           string    `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID     string    `json:"tenant_id" gorm:"type:char(26);not null;index"`
	OutletID     string    `json:"outlet_id" gorm:"type:char(26);not null"`
	ShiftID      string    `json:"shift_id" gorm:"type:char(26);not null;index"`
	Direction    string    `json:"direction" gorm:"not null"` // 'in' | 'out'
	Amount       int64     `json:"amount" gorm:"not null"`
	Reason       string    `json:"reason" gorm:"not null"`
	OccurredAt   time.Time `json:"occurred_at"`
	BusinessDate time.Time `json:"business_date" gorm:"type:date"`
	CreatedBy    string    `json:"created_by" gorm:"type:char(26);not null"`
	CreatedAt    time.Time `json:"created_at"`
}

func (m *CashMovement) BeforeCreate(tx *gorm.DB) (err error) {
	if m.ID == "" {
		m.ID = ulid.New()
	}
	return
}
