package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Enum dokumen CRM (cermin CHECK migrasi 000018/000019).
var (
	QuotationStatuses = []string{"draft", "sent", "accepted", "rejected", "expired"}
	ProjectStatuses   = []string{"active", "on_hold", "completed", "canceled"}
	InvoiceStatuses   = []string{"draft", "sent", "partial", "paid", "overdue", "void"}
	InvoicePayMethods = []string{"cash", "qris", "transfer", "card", "ewallet"}
)

// Quotation adalah penawaran ke pelanggan. Disetujui → otomatis jadi Project.
type Quotation struct {
	ID             string     `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID       string     `json:"tenant_id" gorm:"type:char(26);not null;index"`
	Number         string     `json:"number" gorm:"not null"`
	CustomerID     string     `json:"customer_id" gorm:"type:char(26);not null"`
	DealID         *string    `json:"deal_id" gorm:"type:char(26)"`
	OwnerID        string     `json:"owner_id" gorm:"type:char(26);not null;index"`
	ValidUntil     *time.Time `json:"valid_until" gorm:"type:date"`
	Status         string     `json:"status" gorm:"not null;default:draft"`
	Subtotal       int64      `json:"subtotal" gorm:"not null;default:0"`
	DiscountAmount int64      `json:"discount_amount" gorm:"not null;default:0"`
	TaxAmount      int64      `json:"tax_amount" gorm:"not null;default:0"`
	Total          int64      `json:"total" gorm:"not null;default:0"`
	Note           string     `json:"note"`
	AcceptedAt     *time.Time `json:"accepted_at"`

	Items []QuotationItem `json:"items,omitempty" gorm:"foreignKey:QuotationID;references:ID"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (q *Quotation) BeforeCreate(tx *gorm.DB) (err error) {
	if q.ID == "" {
		q.ID = ulid.New()
	}
	return
}

// QuotationItem adalah satu baris penawaran; product_id opsional (baris jasa
// bebas teks). Harga di-SNAPSHOT saat dibuat.
type QuotationItem struct {
	ID             string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID       string          `json:"tenant_id" gorm:"type:char(26);not null;index"`
	QuotationID    string          `json:"quotation_id" gorm:"type:char(26);not null;index"`
	ProductID      *string         `json:"product_id" gorm:"type:char(26)"`
	Description    string          `json:"description" gorm:"not null"`
	Qty            decimal.Decimal `json:"qty" gorm:"type:numeric(14,3);not null"`
	UnitPrice      int64           `json:"unit_price" gorm:"not null"`
	DiscountAmount int64           `json:"discount_amount" gorm:"not null;default:0"`
	LineTotal      int64           `json:"line_total" gorm:"not null"`
}

func (i *QuotationItem) BeforeCreate(tx *gorm.DB) (err error) {
	if i.ID == "" {
		i.ID = ulid.New()
	}
	return
}

// Project adalah pekerjaan freelance dengan termin pembayaran. `contract_value`
// biasanya = total penawaran yang disetujui.
type Project struct {
	ID            string     `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID      string     `json:"tenant_id" gorm:"type:char(26);not null;index"`
	CustomerID    string     `json:"customer_id" gorm:"type:char(26);not null"`
	QuotationID   *string    `json:"quotation_id" gorm:"type:char(26)"`
	OwnerID       string     `json:"owner_id" gorm:"type:char(26);not null;index"`
	Name          string     `json:"name" gorm:"not null"`
	StartDate     *time.Time `json:"start_date" gorm:"type:date"`
	DueDate       *time.Time `json:"due_date" gorm:"type:date"`
	Status        string     `json:"status" gorm:"not null;default:active"`
	ContractValue int64      `json:"contract_value" gorm:"not null;default:0"`

	Tasks    []ProjectTask    `json:"tasks,omitempty" gorm:"foreignKey:ProjectID;references:ID"`
	Expenses []ProjectExpense `json:"expenses,omitempty" gorm:"foreignKey:ProjectID;references:ID"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (p *Project) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = ulid.New()
	}
	return
}

// ProjectTask adalah satu deliverable / tugas proyek.
type ProjectTask struct {
	ID        string     `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID  string     `json:"tenant_id" gorm:"type:char(26);not null;index"`
	ProjectID string     `json:"project_id" gorm:"type:char(26);not null;index"`
	Title     string     `json:"title" gorm:"not null"`
	DueDate   *time.Time `json:"due_date" gorm:"type:date"`
	DoneAt    *time.Time `json:"done_at"`
	SortOrder int        `json:"sort_order" gorm:"not null;default:0"`
}

func (t *ProjectTask) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == "" {
		t.ID = ulid.New()
	}
	return
}

// ProjectExpense adalah biaya proyek (vendor, transport, cetak). Dipakai
// menghitung laba per proyek — dan menjadi `cost_total` penjualan saat invoice
// proyek lunas.
type ProjectExpense struct {
	ID          string    `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID    string    `json:"tenant_id" gorm:"type:char(26);not null;index"`
	ProjectID   string    `json:"project_id" gorm:"type:char(26);not null;index"`
	Description string    `json:"description" gorm:"not null"`
	Amount      int64     `json:"amount" gorm:"not null"`
	SpentAt     time.Time `json:"spent_at" gorm:"type:date"`
	ReceiptURL  string    `json:"receipt_url"`
}

func (e *ProjectExpense) BeforeCreate(tx *gorm.DB) (err error) {
	if e.ID == "" {
		e.ID = ulid.New()
	}
	return
}

// Invoice adalah tagihan PELANGGAN (bukan tagihan langganan platform). Dibayar
// bertahap; `sale_id` terisi saat lunas → tercatat sebagai satu penjualan di
// tabel `sales` yang sama dengan POS.
type Invoice struct {
	ID             string    `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID       string    `json:"tenant_id" gorm:"type:char(26);not null;index"`
	Number         string    `json:"number" gorm:"not null"`
	CustomerID     string    `json:"customer_id" gorm:"type:char(26);not null"`
	ProjectID      *string   `json:"project_id" gorm:"type:char(26)"`
	QuotationID    *string   `json:"quotation_id" gorm:"type:char(26)"`
	OwnerID        string    `json:"owner_id" gorm:"type:char(26);not null;index"`
	IssueDate      time.Time `json:"issue_date" gorm:"type:date"`
	DueDate        time.Time `json:"due_date" gorm:"type:date"`
	Status         string    `json:"status" gorm:"not null;default:draft"`
	Subtotal       int64     `json:"subtotal" gorm:"not null;default:0"`
	DiscountAmount int64     `json:"discount_amount" gorm:"not null;default:0"`
	TaxAmount      int64     `json:"tax_amount" gorm:"not null;default:0"`
	Total          int64     `json:"total" gorm:"not null;default:0"`
	PaidAmount     int64     `json:"paid_amount" gorm:"not null;default:0"`
	TermLabel      string    `json:"term_label"`
	SaleID         *string   `json:"sale_id" gorm:"type:char(26)"`

	Items    []InvoiceItem    `json:"items,omitempty" gorm:"foreignKey:InvoiceID;references:ID"`
	Payments []InvoicePayment `json:"payments,omitempty" gorm:"foreignKey:InvoiceID;references:ID"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (i *Invoice) BeforeCreate(tx *gorm.DB) (err error) {
	if i.ID == "" {
		i.ID = ulid.New()
	}
	return
}

// InvoiceItem — satu baris invoice; product_id opsional (baris jasa bebas teks).
type InvoiceItem struct {
	ID          string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID    string          `json:"tenant_id" gorm:"type:char(26);not null;index"`
	InvoiceID   string          `json:"invoice_id" gorm:"type:char(26);not null;index"`
	ProductID   *string         `json:"product_id" gorm:"type:char(26)"`
	Description string          `json:"description" gorm:"not null"`
	Qty         decimal.Decimal `json:"qty" gorm:"type:numeric(14,3);not null"`
	UnitPrice   int64           `json:"unit_price" gorm:"not null"`
	LineTotal   int64           `json:"line_total" gorm:"not null"`
}

func (i *InvoiceItem) BeforeCreate(tx *gorm.DB) (err error) {
	if i.ID == "" {
		i.ID = ulid.New()
	}
	return
}

// InvoicePayment adalah satu pembayaran bertahap. `business_date` dihitung dari
// zona outlet default tenant di server. Pelunasan memicu pencatatan penjualan.
type InvoicePayment struct {
	ID           string    `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID     string    `json:"tenant_id" gorm:"type:char(26);not null;index"`
	InvoiceID    string    `json:"invoice_id" gorm:"type:char(26);not null;index"`
	Amount       int64     `json:"amount" gorm:"not null"`
	Method       string    `json:"method" gorm:"not null"`
	PaidAt       time.Time `json:"paid_at"`
	BusinessDate time.Time `json:"business_date" gorm:"type:date"`
	ProofURL     string    `json:"proof_url"`
	CreatedAt    time.Time `json:"created_at"`
}

func (p *InvoicePayment) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = ulid.New()
	}
	return
}
