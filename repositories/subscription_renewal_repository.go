package repositories

import (
	"context"
	"errors"
	"time"

	"candra/backend-api/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repositori pekerjaan harian perpanjangan & pengingat langganan, serta
// antrean pengembalian dana (migrasi 000041).
//
// Pencarian kandidat berjalan LINTAS TENANT tanpa GUC (subscription_* adalah
// tabel platform tanpa RLS); setiap tindakan sesudahnya dijalankan service di
// dalam transaksi bertenant milik tenant itu sendiri. tenantID kosong = semua
// tenant; diisi = satu tenant saja (menjalankan ulang satu tenant, dan uji).

// ErrSubRefundNotFound: pengembalian dana tidak ada.
var ErrSubRefundNotFound = errors.New("pengembalian dana tidak ditemukan")

// LockSubscriptionByTenant memuat langganan tenant konteks SEKALIGUS
// menguncinya (FOR UPDATE). Semua jalur yang menerbitkan tagihan melewati
// kunci ini, jadi pekerjaan harian dan pemilik yang menekan "Bayar sekarang"
// bersamaan tidak menerbitkan dua tagihan.
func LockSubscriptionByTenant(ctx context.Context, tx *gorm.DB) (models.Subscription, error) {
	var id string
	err := tenantDB(ctx, tx).Model(&models.Subscription{}).Clauses(lockForUpdate()).
		Where("tenant_id = ?", currentTenantID(ctx)).
		Select("id").Scan(&id).Error
	if err != nil {
		return models.Subscription{}, err
	}
	if id == "" {
		return models.Subscription{}, ErrSubscriptionNotFound
	}
	return FindSubscriptionByTenant(ctx, tx)
}

func sajaTenant(q *gorm.DB, kolom, tenantID string) *gorm.DB {
	if tenantID != "" {
		return q.Where(kolom+" = ?", tenantID)
	}
	return q
}

// RenewalDueTenants: tenant yang langganan BERBAYARnya diperpanjang otomatis,
// berakhir paling lambat `sampai`, dan belum punya tagihan terbuka. Paket
// berharga 0 (data lama "berlangganan" Gratis) tidak pernah ditagih.
func RenewalDueTenants(ctx context.Context, sampai time.Time, tenantID string) ([]string, error) {
	q := database_(ctx).Table("subscriptions s").
		Joins("JOIN plans p ON p.id = s.plan_id AND p.monthly_price > 0").
		Where("s.status IN ('active','past_due') AND s.auto_renew AND s.current_period_end <= ?", sampai).
		Where(`NOT EXISTS (SELECT 1 FROM subscription_invoices i
		        WHERE i.subscription_id = s.id AND i.status IN ('open','overdue'))`)
	var out []string
	err := sajaTenant(q, "s.tenant_id", tenantID).Order("s.current_period_end").Pluck("s.tenant_id", &out).Error
	return out, err
}

// OverdueInvoiceRow: tagihan terbuka yang jatuh temponya sudah lewat.
type OverdueInvoiceRow struct {
	ID       string
	TenantID string
}

// OpenInvoicesPastDue: tagihan 'open' yang due_date-nya sebelum `hariIni`.
func OpenInvoicesPastDue(ctx context.Context, hariIni time.Time, tenantID string) ([]OverdueInvoiceRow, error) {
	q := database_(ctx).Table("subscription_invoices").
		Where("status = 'open' AND due_date < ?", hariIni)
	var out []OverdueInvoiceRow
	err := sajaTenant(q, "tenant_id", tenantID).Select("id, tenant_id").Scan(&out).Error
	return out, err
}

// MarkLapsedPastDue memindahkan langganan 'active' yang periodenya sudah
// berakhir menjadi 'past_due'. Hak paketnya tidak berubah (dihitung dari
// waktu, termasuk masa tenggang); ini supaya statusnya jujur di layar & panel.
func MarkLapsedPastDue(ctx context.Context, now time.Time, tenantID string) (int64, error) {
	q := database_(ctx).Model(&models.Subscription{}).
		Where("status = 'active' AND current_period_end < ?", now)
	res := sajaTenant(q, "tenant_id", tenantID).
		Updates(map[string]any{"status": "past_due", "updated_at": now})
	return res.RowsAffected, res.Error
}

// TrialEndingRow: masa coba yang berakhir dalam rentang pengingat.
type TrialEndingRow struct {
	TenantID    string
	TrialEndsAt time.Time
	PlanName    string
}

// TrialsEndingBetween: langganan 'trial' yang trial_ends_at-nya di (dari, sampai].
func TrialsEndingBetween(ctx context.Context, dari, sampai time.Time, tenantID string) ([]TrialEndingRow, error) {
	q := database_(ctx).Table("subscriptions s").
		Joins("JOIN plans p ON p.id = s.plan_id AND p.monthly_price > 0").
		Where("s.status = 'trial' AND s.trial_ends_at > ? AND s.trial_ends_at <= ?", dari, sampai)
	var out []TrialEndingRow
	err := sajaTenant(q, "s.tenant_id", tenantID).
		Select("s.tenant_id, s.trial_ends_at, p.name AS plan_name").Scan(&out).Error
	return out, err
}

// GraceEndingRow: langganan berbayar yang masa tenggangnya hampir habis dan
// tagihan perpanjangannya belum lunas.
type GraceEndingRow struct {
	TenantID         string
	CurrentPeriodEnd time.Time
	InvoiceNumber    string
	InvoiceTotal     int64
	InvoicePaid      int64
}

// GraceEndingBetween: current_period_end + tenggang jatuh di (dari, sampai]
// dan masih ada tagihan terbuka.
func GraceEndingBetween(ctx context.Context, tenggangHari int, dari, sampai time.Time, tenantID string) ([]GraceEndingRow, error) {
	q := database_(ctx).Table("subscriptions s").
		Joins(`JOIN subscription_invoices i ON i.subscription_id = s.id AND i.status IN ('open','overdue')`).
		Where("s.status IN ('active','past_due')").
		Where("s.current_period_end + make_interval(days => ?) > ?", tenggangHari, dari).
		Where("s.current_period_end + make_interval(days => ?) <= ?", tenggangHari, sampai)
	var out []GraceEndingRow
	err := sajaTenant(q, "s.tenant_id", tenantID).
		Select(`s.tenant_id, s.current_period_end, i.number AS invoice_number,
			i.total_amount AS invoice_total, i.paid_amount AS invoice_paid`).Scan(&out).Error
	return out, err
}

// InsertNoticeOnce mencatat satu pengingat; false bila pengingat yang sama
// (tenant, jenis, ref) sudah pernah tercatat — pemanggil TIDAK mengirim ulang.
func InsertNoticeOnce(ctx context.Context, tx *gorm.DB, n *models.SubscriptionNotice) (bool, error) {
	n.TenantID = currentTenantID(ctx)
	res := tenantDB(ctx, tx).Clauses(clause.OnConflict{DoNothing: true}).Create(n)
	return res.RowsAffected == 1, res.Error
}

// database_ = handle tanpa GUC tenant (pencarian lintas tenant).
func database_(ctx context.Context) *gorm.DB { return tenantDB(ctx, nil) }

// ── Pengembalian dana ────────────────────────────────────────────────────

// LatestRefundForTenant: pengembalian dana terbaru milik tenant konteks.
func LatestRefundForTenant(ctx context.Context, tx *gorm.DB) (models.SubscriptionRefund, error) {
	var r models.SubscriptionRefund
	err := tenantDB(ctx, tx).
		Where("tenant_id = ?", currentTenantID(ctx)).
		Order("created_at DESC, id DESC").First(&r).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r, ErrSubRefundNotFound
	}
	return r, err
}

// PlatformRefundRow: satu pengembalian dana dengan konteks untuk panel.
type PlatformRefundRow struct {
	models.SubscriptionRefund
	BusinessName  string
	TenantPhone   string
	InvoiceNumber string
	InvoicePaid   int64
	PlanName      string
}

// PlatformListRefunds membaca pengembalian dana LINTAS TENANT. status kosong =
// semua kecuali not_needed. Yang menunggu diurutkan terlama dulu (antrean).
func PlatformListRefunds(ctx context.Context, status string, limit int) ([]PlatformRefundRow, error) {
	q := database_(ctx).Table("subscription_refunds r").
		Joins("JOIN tenants t ON t.id = r.tenant_id").
		Joins("JOIN subscription_invoices i ON i.id = r.subscription_invoice_id").
		Joins("LEFT JOIN subscriptions s ON s.id = i.subscription_id").
		Joins("LEFT JOIN plans p ON p.id = COALESCE(i.plan_id, s.plan_id)")
	urut := "r.created_at DESC, r.id DESC"
	if status != "" {
		q = q.Where("r.status = ?", status)
		if status == "pending" {
			urut = "r.created_at ASC, r.id ASC"
		}
	} else {
		q = q.Where("r.status <> 'not_needed'")
	}
	var rows []PlatformRefundRow
	err := q.Select(`r.*, t.business_name, COALESCE(t.phone, '') AS tenant_phone,
		i.number AS invoice_number, i.paid_amount AS invoice_paid, COALESCE(p.name, '') AS plan_name`).
		Order(urut).Limit(limit).Scan(&rows).Error
	return rows, err
}

// FindRefundAnyTenant membaca satu pengembalian tanpa batas tenant — panel
// perlu tahu tenant pemiliknya sebelum membuka transaksi bertenant.
func FindRefundAnyTenant(ctx context.Context, id string) (models.SubscriptionRefund, error) {
	var r models.SubscriptionRefund
	err := database_(ctx).Where("id = ?", id).First(&r).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r, ErrSubRefundNotFound
	}
	return r, err
}

// LockRefund mengunci pengembalian milik tenant konteks (FOR UPDATE) — dua
// staf yang menandai "sudah ditransfer" bersamaan tidak boleh sama-sama lolos.
func LockRefund(ctx context.Context, tx *gorm.DB, id string) (models.SubscriptionRefund, error) {
	var r models.SubscriptionRefund
	err := tenantDB(ctx, tx).Clauses(lockForUpdate()).
		Where("id = ? AND tenant_id = ?", id, currentTenantID(ctx)).First(&r).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r, ErrSubRefundNotFound
	}
	return r, err
}

// MarkRefundPaid menandai pengembalian sudah ditransfer.
func MarkRefundPaid(ctx context.Context, tx *gorm.DB, r *models.SubscriptionRefund, adminID, reference string, now time.Time) error {
	r.Status, r.PaidAt, r.PaidBy, r.PayoutReference = "paid", &now, &adminID, reference
	return tenantDB(ctx, tx).Model(&models.SubscriptionRefund{}).
		Where("id = ? AND tenant_id = ?", r.ID, currentTenantID(ctx)).
		Updates(map[string]any{
			"status": "paid", "paid_at": now, "paid_by": adminID, "payout_reference": reference,
		}).Error
}
