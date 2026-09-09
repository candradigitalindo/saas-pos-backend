package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"candra/backend-api/database"
	"candra/backend-api/internal/ulid"
	"candra/backend-api/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repositori outbox notifikasi (§5.14, migrasi 000032).
//
// Pola outbox: peristiwa DITULIS dalam transaksi bisnis yang sama dengan
// perubahan datanya, lalu dikirim belakangan oleh pekerja. Itu yang membuat
// "tagihan tersimpan tapi notifikasinya tidak pernah terkirim" — atau
// sebaliknya — tidak mungkin terjadi.
//
// Tabel LINTAS LINGKUP tanpa RLS: `tenant_id` boleh NULL untuk peristiwa
// platform. Pekerja membacanya lintas tenant.

// MaxOutboxAttempts sebelum sebuah peristiwa masuk antrean mati (§ "Dead
// letter: setelah 10 percobaan → status='dead', muncul di panel admin").
const MaxOutboxAttempts = 10

var ErrOutboxEventNotFound = errors.New("peristiwa outbox tidak ditemukan")

// EnqueueOutbox menulis satu peristiwa. tx WAJIB diisi bila dipanggil dari
// dalam transaksi bisnis — itulah inti pola outbox.
func EnqueueOutbox(ctx context.Context, tx *gorm.DB, tenantID *string, topic string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	db := database.DB
	if tx != nil {
		db = tx
	}
	return db.WithContext(ctx).Create(&models.OutboxEvent{
		TenantID: tenantID, Topic: topic, Payload: raw,
		Status: "pending", AvailableAt: time.Now().UTC(),
	}).Error
}

// ClaimDueOutbox mengambil peristiwa yang SUDAH waktunya dikirim, mengunci
// barisnya (FOR UPDATE SKIP LOCKED) sehingga beberapa pekerja aman berjalan
// bersamaan. LINTAS tenant.
func ClaimDueOutbox(ctx context.Context, tx *gorm.DB, limit int) ([]models.OutboxEvent, error) {
	var rows []models.OutboxEvent
	err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Where("status IN ('pending','failed') AND attempts < ? AND available_at <= now()", MaxOutboxAttempts).
		Order("available_at").Limit(limit).Find(&rows).Error
	return rows, err
}

// SaveOutboxResult menulis hasil satu percobaan pengiriman.
//
// Gagal → penundaan BERTAHAP (backoff eksponensial, dibatasi 1 jam) supaya
// penyedia notifikasi yang sedang bermasalah tidak dihujani percobaan ulang.
func SaveOutboxResult(ctx context.Context, tx *gorm.DB, id, status, lastErr string, attempts int) error {
	upd := map[string]any{"status": status, "attempts": attempts, "last_error": lastErr}
	switch status {
	case "done":
		upd["processed_at"] = time.Now().UTC()
	case "failed":
		tunda := time.Duration(1<<uint(min(attempts, 6))) * time.Minute // 2,4,8,…,64 mnt
		if tunda > time.Hour {
			tunda = time.Hour
		}
		upd["available_at"] = time.Now().UTC().Add(tunda)
	}
	return tx.WithContext(ctx).Model(&models.OutboxEvent{}).
		Where("id = ?", id).Updates(upd).Error
}

// ListOutboxEvents dipakai panel internal untuk melihat antrean & antrean mati.
func ListOutboxEvents(ctx context.Context, status, topic string, limit int) ([]models.OutboxEvent, error) {
	q := database.DB.WithContext(ctx)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if topic != "" {
		q = q.Where("topic = ?", topic)
	}
	var rows []models.OutboxEvent
	err := q.Order("created_at DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

// RetryOutboxEvent mengembalikan peristiwa 'dead' ke antrean setelah masalahnya
// diperbaiki — tombol yang dibutuhkan panel admin agar antrean mati tidak jadi
// kuburan.
func RetryOutboxEvent(ctx context.Context, id string) error {
	res := database.DB.WithContext(ctx).Model(&models.OutboxEvent{}).
		Where("id = ? AND status IN ('dead','failed')", id).
		Updates(map[string]any{
			"status": "pending", "attempts": 0, "last_error": "",
			"available_at": time.Now().UTC(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrOutboxEventNotFound
	}
	return nil
}

// ── Template notifikasi ──────────────────────────────────────────────────

// FindNotificationTemplate mencari template milik tenant lebih dulu; bila tidak
// ada, jatuh ke template BAWAAN SISTEM (tenant_id NULL).
func FindNotificationTemplate(ctx context.Context, tenantID *string, code, channel string) (models.NotificationTemplate, bool, error) {
	var t models.NotificationTemplate
	q := database.DB.WithContext(ctx).Where("code = ? AND channel = ?", code, channel)
	if tenantID != nil {
		q = q.Where("tenant_id = ? OR tenant_id IS NULL", *tenantID).
			Order("tenant_id NULLS LAST") // milik tenant menang atas bawaan
	} else {
		q = q.Where("tenant_id IS NULL")
	}
	err := q.First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return t, false, nil
	}
	if err != nil {
		return t, false, err
	}
	return t, true, nil
}

// UpsertNotificationTemplate menyimpan template (bawaan sistem bila TenantID nil).
//
// Memakai SQL mentah, bukan clause.OnConflict GORM: index uniknya berbentuk
// EKSPRESI atas tenant_id yang NULL (lihat migrasi 000032) — target ON CONFLICT
// harus ditulis persis seperti itu agar PostgreSQL mengenalinya. Daftar kolom
// biasa akan ditolak "no unique or exclusion constraint matching".
func UpsertNotificationTemplate(ctx context.Context, t *models.NotificationTemplate) error {
	if t.ID == "" {
		t.ID = ulid.New()
	}
	return database.DB.WithContext(ctx).Exec(`
		INSERT INTO notification_templates (id, tenant_id, code, channel, subject, body)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (COALESCE(tenant_id, ''), code, channel)
		DO UPDATE SET subject = EXCLUDED.subject, body = EXCLUDED.body`,
		t.ID, t.TenantID, t.Code, t.Channel, t.Subject, t.Body).Error
}
