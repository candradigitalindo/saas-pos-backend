package models

import (
	"time"

	"candra/backend-api/internal/timez"
	"candra/backend-api/internal/ulid"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// OutletTypes adalah nilai `type` yang sah (cermin CHECK di migrasi 000003).
// vehicle = stok kanvas (mobil keliling).
var OutletTypes = []string{"store", "kitchen", "warehouse", "vehicle"}

// Outlet adalah cabang/gerai milik satu tenant.
//
// Zona waktu ada DI SINI, bukan di tenant: satu usaha bisa punya cabang di
// Makassar (WITA) dan Jayapura (WIT), dan laporan harian tiap cabang harus benar
// menurut zonanya sendiri (§3.2, §5.2).
type Outlet struct {
	ID       string `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID string `json:"tenant_id" gorm:"type:char(26);not null;index"`
	Name     string `json:"name" gorm:"not null"`
	Type     string `json:"type" gorm:"not null;default:store"` // salah satu OutletTypes
	Address  string `json:"address"`
	Phone    string `json:"phone"`

	// Timezone: nama IANA (Asia/Jakarta | Asia/Makassar | Asia/Jayapura).
	// Divalidasi di service dengan timez.IsSupportedTimezone.
	Timezone string `json:"timezone" gorm:"not null;default:Asia/Jakarta"`
	// BusinessDayStart: batas awal hari usaha, untuk usaha yang tutup lewat
	// tengah malam. Kolom TIME; timez.Clock menjamin round-trip pgx yang andal.
	// Pakai DayStartOffset() untuk pergeserannya sebagai Duration.
	BusinessDayStart timez.Clock `json:"business_day_start" gorm:"type:time;not null"`

	Currency          string          `json:"currency" gorm:"not null;default:IDR"`
	TaxEnabled        bool            `json:"tax_enabled" gorm:"not null;default:false"`
	TaxRate           decimal.Decimal `json:"tax_rate" gorm:"type:numeric(7,4);not null;default:0"` // pecahan: 0.1100 = 11%
	TaxInclusive      bool            `json:"tax_inclusive" gorm:"not null;default:true"`
	ServiceChargeRate decimal.Decimal `json:"service_charge_rate" gorm:"type:numeric(7,4);not null;default:0"`
	ReceiptHeader     string          `json:"receipt_header"`
	ReceiptFooter     string          `json:"receipt_footer"`
	IsActive          bool            `json:"is_active" gorm:"not null;default:true"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (o *Outlet) BeforeCreate(tx *gorm.DB) (err error) {
	if o.ID == "" {
		o.ID = ulid.New()
	}
	return
}

// DayStartOffset mengembalikan pergeseran awal hari usaha dari tengah malam,
// siap dipakai timez.BusinessDate / timez.DayRangeUTC.
func (o Outlet) DayStartOffset() time.Duration {
	return o.BusinessDayStart.Duration()
}
