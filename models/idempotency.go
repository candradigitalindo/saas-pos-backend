package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"gorm.io/gorm"
)

// IdempotencyKey menyimpan hasil satu operasi yang menciptakan uang/stok, agar
// permintaan yang sama (Idempotency-Key sama) mengembalikan hasil lama alih-alih
// mengerjakannya ulang (§8).
type IdempotencyKey struct {
	ID             string    `json:"-" gorm:"primaryKey;type:char(26)"`
	TenantID       string    `json:"-" gorm:"type:char(26)"`
	Scope          string    `json:"-" gorm:"not null"` // 'sale.create', 'sale.refund', ...
	Key            string    `json:"-" gorm:"not null"`
	RequestHash    string    `json:"-" gorm:"not null"`
	ResponseStatus int       `json:"-"`
	ResponseBody   []byte    `json:"-" gorm:"type:jsonb"`
	CreatedAt      time.Time `json:"-"`
	ExpiresAt      time.Time `json:"-"`
}

func (k *IdempotencyKey) BeforeCreate(tx *gorm.DB) (err error) {
	if k.ID == "" {
		k.ID = ulid.New()
	}
	return
}

// ReceiptCounter adalah penghitung nomor struk per outlet per hari usaha.
// Diambil dengan SELECT ... FOR UPDATE lalu next_seq dinaikkan — bukan COUNT(*)
// yang menghasilkan nomor ganda saat dua kasir menutup transaksi bersamaan
// (§13.7). Tanpa id, tanpa hook.
type ReceiptCounter struct {
	TenantID     string    `json:"tenant_id" gorm:"primaryKey;type:char(26)"`
	OutletID     string    `json:"outlet_id" gorm:"primaryKey;type:char(26)"`
	BusinessDate time.Time `json:"business_date" gorm:"primaryKey;type:date"`
	NextSeq      int64     `json:"next_seq" gorm:"not null;default:1"`
}

func (ReceiptCounter) TableName() string { return "receipt_counters" }
