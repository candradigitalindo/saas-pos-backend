package models

import (
	"time"

	"candra/backend-api/internal/ulid"

	"gorm.io/gorm"
)

// Enum CRM yang sah (cermin CHECK migrasi 000017). Dipakai lapisan validasi.
var (
	PipelineKinds  = []string{"freelance", "field_sales", "general"}
	DealStatuses   = []string{"open", "won", "lost"}
	ActivityKinds  = []string{"call", "chat", "meeting", "visit", "task", "note"}
	ActivityStatus = []string{"planned", "done", "canceled"}
)

// LeadSource adalah kanal asal prospek (WhatsApp, Instagram, Referral, ...).
// Tabelnya tidak menyimpan created_at/updated_at — data konfigurasi ringan.
type LeadSource struct {
	ID       string `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID string `json:"tenant_id" gorm:"type:char(26);not null;index"`
	Name     string `json:"name" gorm:"not null"`
	IsActive bool   `json:"is_active" gorm:"not null;default:true"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (s *LeadSource) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == "" {
		s.ID = ulid.New()
	}
	return
}

// TableName tetap eksplisit.
func (LeadSource) TableName() string { return "lead_sources" }

// Pipeline adalah corong prospek yang tahapnya bisa diatur tenant.
type Pipeline struct {
	ID        string `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID  string `json:"tenant_id" gorm:"type:char(26);not null;index"`
	Name      string `json:"name" gorm:"not null"`
	Kind      string `json:"kind" gorm:"not null"` // salah satu PipelineKinds
	IsDefault bool   `json:"is_default" gorm:"not null;default:false"`

	Stages []PipelineStage `json:"stages,omitempty" gorm:"foreignKey:PipelineID;references:ID"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (p *Pipeline) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = ulid.New()
	}
	return
}

// PipelineStage adalah satu tahap pipeline, dengan urutan & probabilitas 0..1.
type PipelineStage struct {
	ID          string  `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID    string  `json:"tenant_id" gorm:"type:char(26);not null;index"`
	PipelineID  string  `json:"pipeline_id" gorm:"type:char(26);not null;index"`
	Name        string  `json:"name" gorm:"not null"`
	SortOrder   int     `json:"sort_order" gorm:"not null"`
	Probability float64 `json:"probability" gorm:"type:numeric(7,4);not null;default:0"`
	IsWon       bool    `json:"is_won" gorm:"not null;default:false"`
	IsLost      bool    `json:"is_lost" gorm:"not null;default:false"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (s *PipelineStage) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == "" {
		s.ID = ulid.New()
	}
	return
}

// Deal adalah satu peluang penjualan. `owner_id` = lapis visibilitas ketiga
// (§6): sales hanya melihat deal miliknya kecuali punya crm.lead.view.all.
// `lost_reason` adalah data paling berharga & paling sering dilupakan.
type Deal struct {
	ID                string     `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID          string     `json:"tenant_id" gorm:"type:char(26);not null;index"`
	PipelineID        string     `json:"pipeline_id" gorm:"type:char(26);not null"`
	StageID           string     `json:"stage_id" gorm:"type:char(26);not null"`
	CustomerID        *string    `json:"customer_id" gorm:"type:char(26)"`
	LeadSourceID      *string    `json:"lead_source_id" gorm:"type:char(26)"`
	OwnerID           string     `json:"owner_id" gorm:"type:char(26);not null;index"`
	Title             string     `json:"title" gorm:"not null"`
	Value             int64      `json:"value" gorm:"not null;default:0"`
	ExpectedCloseDate *time.Time `json:"expected_close_date" gorm:"type:date"`
	Status            string     `json:"status" gorm:"not null;default:open"`
	LostReason        string     `json:"lost_reason"`
	ClosedAt          *time.Time `json:"closed_at"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	SyncVersion int64 `json:"sync_version" gorm:"->;column:sync_version"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (d *Deal) BeforeCreate(tx *gorm.DB) (err error) {
	if d.ID == "" {
		d.ID = ulid.New()
	}
	return
}

// Activity adalah aktivitas / pengingat follow-up (telepon, chat, kunjungan,
// tugas). Follow-up yang lupa dikerjakan = kebocoran omzet terbesar di segmen
// CRM.
type Activity struct {
	ID          string     `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID    string     `json:"tenant_id" gorm:"type:char(26);not null;index"`
	Kind        string     `json:"kind" gorm:"not null"` // salah satu ActivityKinds
	Subject     string     `json:"subject" gorm:"not null"`
	Body        string     `json:"body"`
	OwnerID     string     `json:"owner_id" gorm:"type:char(26);not null;index"`
	CustomerID  *string    `json:"customer_id" gorm:"type:char(26)"`
	DealID      *string    `json:"deal_id" gorm:"type:char(26)"`
	DueAt       *time.Time `json:"due_at"`
	CompletedAt *time.Time `json:"completed_at"`
	Status      string     `json:"status" gorm:"not null;default:planned"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	SyncVersion int64 `json:"sync_version" gorm:"->;column:sync_version"`
}

// BeforeCreate meng-generate ULID bila ID belum diisi.
func (a *Activity) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == "" {
		a.ID = ulid.New()
	}
	return
}

// CRMDocumentCounter adalah penghitung nomor dokumen per (tenant, jenis).
type CRMDocumentCounter struct {
	TenantID string `gorm:"primaryKey;type:char(26)"`
	DocType  string `gorm:"primaryKey"`
	NextSeq  int64  `gorm:"not null;default:1"`
}

// TableName tetap eksplisit.
func (CRMDocumentCounter) TableName() string { return "crm_document_counters" }
