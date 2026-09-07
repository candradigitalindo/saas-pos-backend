package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"gorm.io/gorm"
)

// Business type & status yang sah untuk Tenant (cermin CHECK di migrasi 000003).
// Dipakai lapisan validasi service agar pesan errornya bisa dijelaskan, bukan
// hanya "violates check constraint".
var (
	TenantBusinessTypes = []string{"retail", "fnb", "service", "wholesale", "other"}
	TenantStatuses      = []string{"trial", "active", "past_due", "suspended", "closed"}
)

// Tenant adalah satu usaha (pelanggan SaaS). Akar semua data operasional.
//
// Bukan "tabel bertenant": tidak punya kolom tenant_id. Isolasinya dijaga RLS
// berbasis kolom id (migrasi 000004) — sebuah query yang di-scope ke tenant X
// hanya bisa melihat baris tenants dengan id = X.
type Tenant struct {
	ID           string `json:"id" gorm:"primaryKey;type:char(26)"`
	BusinessName string `json:"business_name" gorm:"not null"`
	BusinessType string `json:"business_type" gorm:"not null"` // salah satu TenantBusinessTypes
	OwnerName    string `json:"owner_name" gorm:"not null"`
	Phone        string `json:"phone" gorm:"not null"`
	Email        string `json:"email"`
	NPWP         string `json:"npwp" gorm:"column:npwp"`
	NIB          string `json:"nib" gorm:"column:nib"`
	Status       string `json:"status" gorm:"not null;default:trial"` // salah satu TenantStatuses

	// Diisi saat registrasi bila memakai kode referral mitra. FK ke partners
	// menyusul di Fase 12; untuk sekarang hanya kolom.
	ReferredByPartnerID string `json:"referred_by_partner_id" gorm:"column:referred_by_partner_id"`
	ReferralCodeUsed    string `json:"referral_code_used"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (t *Tenant) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == "" {
		t.ID = ulid.New()
	}
	return
}
