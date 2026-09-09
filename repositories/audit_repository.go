package repositories

import (
	"context"

	"candra/backend-api/database"
	"candra/backend-api/models"
)

// Repositori jejak audit (§5.14). LINTAS LINGKUP: `tenant_id` boleh NULL untuk
// aksi platform/mitra, jadi tanpa scopeTenant — pemanggil mengisi tenant_id
// eksplisit. Tabelnya tanpa RLS agar proses platform tetap bisa membacanya.

// AuditEntry adalah satu baris jejak audit yang akan ditulis.
type AuditEntry struct {
	TenantID    *string
	ActorType   string // user | partner_user | system | admin
	ActorID     *string
	Action      string
	TargetTable *string
	TargetID    *string
}

// WriteAuditLogs menulis BANYAK baris jejak audit dalam SATU insert.
//
// Batch, bukan per baris: portal mitra mencatat satu baris per merchant yang
// dilihat, dan satu insert per merchant membuat pembukaan halaman jadi N query.
func WriteAuditLogs(ctx context.Context, entries []AuditEntry) error {
	if len(entries) == 0 {
		return nil
	}
	rows := make([]models.AuditLog, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, models.AuditLog{
			TenantID: e.TenantID, ActorType: e.ActorType, ActorID: e.ActorID,
			Action: e.Action, TargetTable: e.TargetTable, TargetID: e.TargetID,
		})
	}
	return database.DB.WithContext(ctx).Create(&rows).Error
}

// CountAuditLogs menghitung baris audit yang cocok — dipakai test & panel audit.
func CountAuditLogs(ctx context.Context, actorType, action string, tenantID *string) (int64, error) {
	q := database.DB.WithContext(ctx).Model(&models.AuditLog{})
	if actorType != "" {
		q = q.Where("actor_type = ?", actorType)
	}
	if action != "" {
		q = q.Where("action = ?", action)
	}
	if tenantID != nil {
		q = q.Where("tenant_id = ?", *tenantID)
	}
	var n int64
	err := q.Count(&n).Error
	return n, err
}
