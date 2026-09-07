package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"gorm.io/gorm"
)

// DeferredRevenueEntry adalah satu bulan pengakuan pendapatan diterima di muka
// (§13.4). Uang prabayar N bulan dipecah jadi N baris; `recognized_at` diisi
// pekerjaan harian saat bulannya tiba. `amount` boleh negatif: baris penyesuaian
// yang ditulis saat pembatalan agar total yang diakui persis = uang yang benar
// benar menjadi hak platform (dibayar − dikembalikan).
type DeferredRevenueEntry struct {
	ID                    string     `json:"id" gorm:"primaryKey;type:char(26)"`
	SubscriptionInvoiceID string     `json:"subscription_invoice_id" gorm:"type:char(26);not null"`
	TenantID              string     `json:"tenant_id" gorm:"type:char(26);not null"`
	RecognitionMonth      time.Time  `json:"recognition_month" gorm:"type:date"` // tanggal 1 bulan pengakuan
	Amount                int64      `json:"amount" gorm:"not null"`
	RecognizedAt          *time.Time `json:"recognized_at"` // NULL = belum diakui
	CreatedAt             time.Time  `json:"created_at"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (e *DeferredRevenueEntry) BeforeCreate(tx *gorm.DB) (err error) {
	if e.ID == "" {
		e.ID = ulid.New()
	}
	return
}

// SubscriptionRefund adalah jejak audit pengembalian dana saat pembatalan di
// tengah masa prabayar. Nilainya dihitung ulang pada harga bulanan NORMAL
// (blueprint aturan 3): refund = maks(0, dibayar − bulan_terpakai × harga_bulanan).
type SubscriptionRefund struct {
	ID                    string    `json:"id" gorm:"primaryKey;type:char(26)"`
	SubscriptionInvoiceID string    `json:"subscription_invoice_id" gorm:"type:char(26);not null"`
	TenantID              string    `json:"tenant_id" gorm:"type:char(26);not null"`
	Amount                int64     `json:"amount" gorm:"not null"`
	MonthsUsed            int       `json:"months_used" gorm:"not null"`
	Reason                string    `json:"reason"`
	RefundedAt            time.Time `json:"refunded_at"`
	CreatedAt             time.Time `json:"created_at"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (r *SubscriptionRefund) BeforeCreate(tx *gorm.DB) (err error) {
	if r.ID == "" {
		r.ID = ulid.New()
	}
	return
}
