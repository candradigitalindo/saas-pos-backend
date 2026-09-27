package repositories

import (
	"context"
	"errors"
	"time"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

// Repositori konfirmasi pembayaran langganan (migrasi 000039).
//
// Jalur TENANT (kirim & baca konfirmasinya sendiri) memakai tenantDB dengan
// GUC tenant dan filter tenant_id eksplisit. Jalur PANEL membaca lintas tenant
// tanpa GUC (kebijakan RLS permisif saat GUC kosong), lalu MENGUNCI baris di
// dalam transaksi bertenant milik konfirmasi itu sendiri.

// ErrSubClaimNotFound: konfirmasi pembayaran tidak ada (atau milik tenant lain).
var ErrSubClaimNotFound = errors.New("konfirmasi pembayaran tidak ditemukan")

// CreateSubClaim menyimpan konfirmasi baru milik tenant konteks. tx wajib
// (dipanggil di dalam WithTenant bersama catatan idempotensinya).
func CreateSubClaim(ctx context.Context, tx *gorm.DB, c *models.SubscriptionPaymentClaim) error {
	c.TenantID = currentTenantID(ctx)
	return tenantDB(ctx, tx).Create(c).Error
}

// LatestSubClaimForInvoice mengembalikan konfirmasi TERBARU untuk satu tagihan
// milik tenant konteks, atau ErrSubClaimNotFound.
func LatestSubClaimForInvoice(ctx context.Context, tx *gorm.DB, invoiceID string) (models.SubscriptionPaymentClaim, error) {
	var c models.SubscriptionPaymentClaim
	err := tenantDB(ctx, tx).
		Where("tenant_id = ? AND subscription_invoice_id = ?", currentTenantID(ctx), invoiceID).
		Order("created_at DESC, id DESC").
		First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c, ErrSubClaimNotFound
	}
	return c, err
}

// ListSubClaimsForTenant: konfirmasi milik tenant konteks, terbaru dulu.
func ListSubClaimsForTenant(ctx context.Context, limit int) ([]models.SubscriptionPaymentClaim, error) {
	var rows []models.SubscriptionPaymentClaim
	err := tenantDB(ctx, nil).
		Where("tenant_id = ?", currentTenantID(ctx)).
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

// PlatformSubClaimRow: satu konfirmasi beserta konteks yang dibutuhkan staf
// keuangan untuk memutuskannya tanpa membuka layar lain.
type PlatformSubClaimRow struct {
	models.SubscriptionPaymentClaim
	BusinessName  string
	TenantPhone   string
	InvoiceNumber string
	InvoiceTotal  int64
	InvoicePaid   int64
	PlanName      string
}

// PlatformListSubClaims membaca konfirmasi LINTAS TENANT (panel internal).
// status kosong = semua. Yang menunggu diurutkan terlama dulu (antrean kerja);
// sisanya terbaru dulu (riwayat).
func PlatformListSubClaims(ctx context.Context, status string, limit int) ([]PlatformSubClaimRow, error) {
	q := tenantDB(ctx, nil).
		Table("subscription_payment_claims c").
		Joins("JOIN tenants t ON t.id = c.tenant_id").
		Joins("JOIN subscription_invoices i ON i.id = c.subscription_invoice_id").
		Joins("LEFT JOIN subscriptions s ON s.id = i.subscription_id").
		Joins("LEFT JOIN plans p ON p.id = s.plan_id")
	urut := "c.created_at DESC, c.id DESC"
	if status != "" {
		q = q.Where("c.status = ?", status)
		if status == "pending" {
			urut = "c.created_at ASC, c.id ASC"
		}
	}
	var rows []PlatformSubClaimRow
	err := q.Select(`c.*, t.business_name, COALESCE(t.phone, '') AS tenant_phone,
		i.number AS invoice_number, i.total_amount AS invoice_total, i.paid_amount AS invoice_paid,
		COALESCE(p.name, '') AS plan_name`).
		Order(urut).Limit(limit).
		Scan(&rows).Error
	return rows, err
}

// FindSubClaimAnyTenant membaca satu konfirmasi tanpa batas tenant — panel
// perlu tahu tenant pemiliknya sebelum membuka transaksi bertenant.
func FindSubClaimAnyTenant(ctx context.Context, id string) (models.SubscriptionPaymentClaim, error) {
	var c models.SubscriptionPaymentClaim
	err := tenantDB(ctx, nil).Where("id = ?", id).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c, ErrSubClaimNotFound
	}
	return c, err
}

// LockSubClaim mengunci konfirmasi milik tenant konteks di dalam tx — dua
// staf yang menekan "Setujui" bersamaan tidak boleh mencatat pembayaran dua
// kali.
func LockSubClaim(ctx context.Context, tx *gorm.DB, id string) (models.SubscriptionPaymentClaim, error) {
	var c models.SubscriptionPaymentClaim
	err := tenantDB(ctx, tx).Clauses(lockForUpdate()).
		Where("id = ? AND tenant_id = ?", id, currentTenantID(ctx)).
		First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c, ErrSubClaimNotFound
	}
	return c, err
}

// ReviewSubClaim menyimpan keputusan atas konfirmasi (disetujui/ditolak).
func ReviewSubClaim(ctx context.Context, tx *gorm.DB, c *models.SubscriptionPaymentClaim, status, adminID, reason string, paymentID *string) error {
	now := time.Now().UTC()
	c.Status, c.ReviewedBy, c.ReviewedAt, c.RejectReason, c.SubscriptionPaymentID = status, &adminID, &now, reason, paymentID
	return tenantDB(ctx, tx).Model(c).Updates(map[string]any{
		"status": status, "reviewed_by": adminID, "reviewed_at": now,
		"reject_reason": reason, "subscription_payment_id": paymentID, "updated_at": now,
	}).Error
}
