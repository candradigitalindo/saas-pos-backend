package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"gorm.io/gorm"
)

var CustomerTypes = []string{"person", "company", "store"}

// Customer adalah pelanggan tenant. credit_limit membatasi total kasbon.
type Customer struct {
	ID          string   `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID    string   `json:"tenant_id" gorm:"type:char(26);not null;index"`
	Code        *string  `json:"code"`
	Name        string   `json:"name" gorm:"not null"`
	Phone       *string  `json:"phone"` // unik per tenant di antara baris hidup
	Email       string   `json:"email"`
	Address     string   `json:"address"`
	Type        string   `json:"type" gorm:"not null;default:person"`
	PriceListID *string  `json:"price_list_id" gorm:"type:char(26)"`
	OwnerID     *string  `json:"owner_id" gorm:"type:char(26)"` // CRM: sales pemilik data
	CreditLimit int64    `json:"credit_limit" gorm:"not null;default:0"`
	Latitude    *float64 `json:"latitude" gorm:"type:numeric(9,6)"`
	Longitude   *float64 `json:"longitude" gorm:"type:numeric(9,6)"`
	Note        string   `json:"note"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (c *Customer) BeforeCreate(tx *gorm.DB) (err error) {
	if c.ID == "" {
		c.ID = ulid.New()
	}
	return
}

// ContactPerson adalah narahubung pada pelanggan berbentuk perusahaan/toko.
type ContactPerson struct {
	ID         string `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID   string `json:"tenant_id" gorm:"type:char(26);not null;index"`
	CustomerID string `json:"customer_id" gorm:"type:char(26);not null;index"`
	Name       string `json:"name" gorm:"not null"`
	Position   string `json:"position"`
	Phone      string `json:"phone"`
	Email      string `json:"email"`
	IsPrimary  bool   `json:"is_primary" gorm:"not null;default:false"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (c *ContactPerson) BeforeCreate(tx *gorm.DB) (err error) {
	if c.ID == "" {
		c.ID = ulid.New()
	}
	return
}

var ReceivableStatuses = []string{"open", "partial", "paid", "written_off"}

// Receivable adalah piutang: timbul dari penjualan kasbon (payment method
// 'credit') atau invoice. Selalu satu baris per (source_table, source_id).
type Receivable struct {
	ID          string     `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID    string     `json:"tenant_id" gorm:"type:char(26);not null;index"`
	CustomerID  string     `json:"customer_id" gorm:"type:char(26);not null;index"`
	SourceTable string     `json:"source_table" gorm:"not null"` // 'sales' | 'invoices'
	SourceID    string     `json:"source_id" gorm:"type:char(26);not null"`
	Amount      int64      `json:"amount" gorm:"not null"`
	PaidAmount  int64      `json:"paid_amount" gorm:"not null;default:0"`
	DueDate     *time.Time `json:"due_date" gorm:"type:date"`
	Status      string     `json:"status" gorm:"not null;default:open"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (r *Receivable) BeforeCreate(tx *gorm.DB) (err error) {
	if r.ID == "" {
		r.ID = ulid.New()
	}
	return
}

// Outstanding mengembalikan sisa piutang yang belum dibayar.
func (r Receivable) Outstanding() int64 { return r.Amount - r.PaidAmount }

// ReceivablePayment adalah satu pembayaran cicilan/pelunasan piutang.
type ReceivablePayment struct {
	ID           string    `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID     string    `json:"tenant_id" gorm:"type:char(26);not null;index"`
	ReceivableID string    `json:"receivable_id" gorm:"type:char(26);not null;index"`
	Amount       int64     `json:"amount" gorm:"not null"`
	Method       string    `json:"method" gorm:"not null"`
	PaidAt       time.Time `json:"paid_at"`
	BusinessDate time.Time `json:"business_date" gorm:"type:date"`
	CollectedBy  *string   `json:"collected_by" gorm:"type:char(26)"`
	ProofURL     string    `json:"proof_url"`

	CreatedAt time.Time `json:"created_at"`
}

func (p *ReceivablePayment) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = ulid.New()
	}
	return
}
