package models

import (
	"encoding/json"
	"time"

	"candra/backend-api/internal/ulid"

	"gorm.io/gorm"
)

// Model tabel sistem §5.14.

// Enum audit (cermin CHECK migrasi 000032).
var (
	AuditActorTypes      = []string{"user", "partner_user", "system", "admin"}
	OutboxStatuses       = []string{"pending", "processing", "done", "failed", "dead"}
	NotificationChannels = []string{"whatsapp", "email"}
)

// AuditLog adalah jejak audit LINTAS LINGKUP: `TenantID` nil untuk aksi
// platform/mitra, `ActorType` membedakan user tenant, akun mitra, sistem, admin.
//
// `TargetTable`/`TargetID` sengaja TANPA foreign key (§5.16): log harus tetap
// ada walau baris rujukannya dipartisi atau diarsipkan.
type AuditLog struct {
	ID          string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID    *string         `json:"tenant_id" gorm:"type:char(26)"`
	ActorType   string          `json:"actor_type" gorm:"not null"`
	ActorID     *string         `json:"actor_id" gorm:"type:char(26)"`
	Action      string          `json:"action" gorm:"not null"`
	TargetTable *string         `json:"target_table"`
	TargetID    *string         `json:"target_id" gorm:"type:char(26)"`
	BeforeData  json.RawMessage `json:"before_data,omitempty" gorm:"type:jsonb"`
	AfterData   json.RawMessage `json:"after_data,omitempty" gorm:"type:jsonb"`
	IPAddress   *string         `json:"ip_address" gorm:"type:inet"`
	UserAgent   string          `json:"user_agent"`
	OccurredAt  time.Time       `json:"occurred_at"`
}

func (a *AuditLog) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == "" {
		a.ID = ulid.New()
	}
	return
}

// OutboxEvent adalah pola outbox untuk pengiriman andal (notifikasi/webhook
// keluar). Produsen & konsumennya menyusul; tabelnya sudah ada agar relasi &
// index-nya tidak hilang diam-diam (§5.16).
type OutboxEvent struct {
	ID          string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID    *string         `json:"tenant_id" gorm:"type:char(26)"`
	Topic       string          `json:"topic" gorm:"not null"`
	Payload     json.RawMessage `json:"payload" gorm:"type:jsonb;not null"`
	Status      string          `json:"status" gorm:"not null;default:pending"`
	Attempts    int             `json:"attempts" gorm:"not null;default:0"`
	LastError   string          `json:"last_error"`
	AvailableAt time.Time       `json:"available_at"`
	CreatedAt   time.Time       `json:"created_at"`
	ProcessedAt *time.Time      `json:"processed_at"`
}

func (e *OutboxEvent) BeforeCreate(tx *gorm.DB) (err error) {
	if e.ID == "" {
		e.ID = ulid.New()
	}
	return
}

// NotificationTemplate: `TenantID` nil = template bawaan sistem.
type NotificationTemplate struct {
	ID       string  `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID *string `json:"tenant_id" gorm:"type:char(26)"`
	Code     string  `json:"code" gorm:"not null"`
	Channel  string  `json:"channel" gorm:"not null"`
	Subject  string  `json:"subject"`
	Body     string  `json:"body" gorm:"not null"`
}

func (t *NotificationTemplate) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == "" {
		t.ID = ulid.New()
	}
	return
}
