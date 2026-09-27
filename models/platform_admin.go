package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"gorm.io/gorm"
)

// PlatformAdmin adalah akun staf INTERNAL penyedia SaaS (panel internal,
// blueprint G.5) — realm ketiga, terpisah penuh dari user tenant dan akun mitra
// (migrasi 000035). Login memakai email.
//
// Perannya tetap, tidak dinamis: penggunanya segelintir orang, dan aturan siapa
// boleh mencairkan uang mitra tidak boleh bisa diubah dari dalam panel oleh
// salah satu dari mereka sendiri.
type PlatformAdmin struct {
	ID           string         `json:"id" gorm:"primaryKey;type:char(26)"`
	Name         string         `json:"name" gorm:"not null"`
	Email        string         `json:"email" gorm:"not null"`
	PasswordHash string         `json:"-" gorm:"column:password_hash;not null"`
	Role         string         `json:"role" gorm:"not null"`
	IsActive     bool           `json:"is_active" gorm:"not null;default:true"`
	LastLoginAt  *time.Time     `json:"last_login_at"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

func (a *PlatformAdmin) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == "" {
		a.ID = ulid.New()
	}
	return
}

// PlatformAdminRoles adalah peran yang sah (cermin CHECK migrasi 000035).
var PlatformAdminRoles = []string{"superadmin", "operator", "finance", "support"}

// Kemampuan per peran. Dipakai middlewares.RequirePlatform(...) — daftar
// eksplisit, bukan tabel izin, karena jumlah peran & aksinya kecil dan tetap.
const (
	CapPartnerVerify  = "partner.verify"  // verifikasi & aktifkan mitra, atur tingkat
	CapPartnerFinance = "partner.finance" // jalankan komisi, setujui, cairkan
	CapPartnerDispute = "partner.dispute" // putuskan sengketa atribusi
	CapPlatformAdmin  = "platform.admin"  // kelola akun admin lain
	CapPlatformRead   = "platform.read"   // membaca seluruh panel
	// CapBillingVerify: memverifikasi konfirmasi pembayaran langganan tenant
	// (menyetujui = mengaktifkan paket berbayar). Wewenang keuangan — bukan
	// operator yang memverifikasi mitra.
	CapBillingVerify = "billing.verify"
)

// platformCaps memetakan peran → kemampuan.
var platformCaps = map[string][]string{
	"superadmin": {CapPartnerVerify, CapPartnerFinance, CapPartnerDispute, CapPlatformAdmin, CapPlatformRead, CapBillingVerify},
	"operator":   {CapPartnerVerify, CapPartnerDispute, CapPlatformRead},
	"finance":    {CapPartnerFinance, CapPlatformRead, CapBillingVerify},
	"support":    {CapPlatformRead},
}

// PlatformCan melaporkan apakah sebuah peran admin punya kemampuan tertentu.
func PlatformCan(role, capability string) bool {
	for _, c := range platformCaps[role] {
		if c == capability {
			return true
		}
	}
	return false
}

// PlatformCapabilities mengembalikan seluruh kemampuan sebuah peran (untuk
// ditampilkan di panel).
func PlatformCapabilities(role string) []string { return platformCaps[role] }
