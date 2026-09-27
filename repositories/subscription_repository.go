package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/internal/ulid"
	"candra/backend-api/models"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Repositori langganan & tagihan platform (§5.13).
//
// Ini tabel PLATFORM — TIDAK ada Row Level Security. `plans` &
// `plan_term_discounts` global; sisanya menyimpan `tenant_id` sebagai penunjuk
// pelanggan. Setiap query yang menghadap tenant WAJIB memfilter
// `tenant_id = <konteks>` sendiri — itu satu-satunya penjaga isolasi di sini.

var ErrSubscriptionNotFound = errors.New("langganan tidak ditemukan")
var ErrSubInvoiceNotFound = errors.New("tagihan langganan tidak ditemukan")

// ── Katalog paket ─────────────────────────────────────────────────────────

// ListActivePlans mengembalikan paket aktif, termurah lebih dulu.
func ListActivePlans(ctx context.Context) ([]models.Plan, error) {
	var rows []models.Plan
	err := tenantDB(ctx, nil).Where("is_active = ?", true).Order("monthly_price").Find(&rows).Error
	return rows, err
}

// FindPlanByCode mencari paket berdasarkan kode ('basic','pro',...).
func FindPlanByCode(ctx context.Context, tx *gorm.DB, code string) (models.Plan, error) {
	var p models.Plan
	err := tenantDB(ctx, tx).Where("code = ?", code).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return p, fmt.Errorf("%w: paket %q tidak dikenal", helpers.ErrValidation, code)
	}
	return p, err
}

// FindPlanByID memuat satu paket berdasarkan id.
func FindPlanByID(ctx context.Context, tx *gorm.DB, id string) (models.Plan, error) {
	var p models.Plan
	err := tenantDB(ctx, tx).Where("id = ?", id).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return p, fmt.Errorf("%w: paket tidak ditemukan", helpers.ErrNotFound)
	}
	return p, err
}

// TermDiscountRate mengembalikan tarif diskon prabayar untuk `term` bulan.
// Term tanpa baris (mis. 1 bulan) → 0.
func TermDiscountRate(ctx context.Context, tx *gorm.DB, term int) (decimal.Decimal, error) {
	var d models.PlanTermDiscount
	err := tenantDB(ctx, tx).Where("term_months = ? AND is_active = ?", term, true).First(&d).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return decimal.Zero, nil
	}
	if err != nil {
		return decimal.Zero, err
	}
	return d.DiscountRate, nil
}

// ── Langganan ─────────────────────────────────────────────────────────────

// FindSubscriptionByTenant memuat langganan tenant konteks (+ Plan).
func FindSubscriptionByTenant(ctx context.Context, tx *gorm.DB) (models.Subscription, error) {
	var s models.Subscription
	err := tenantDB(ctx, tx).
		Preload("Plan").
		Where("tenant_id = ?", currentTenantID(ctx)).
		First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s, ErrSubscriptionNotFound
	}
	return s, err
}

// CreateSubscription menyimpan langganan baru.
func CreateSubscription(ctx context.Context, tx *gorm.DB, s *models.Subscription) error {
	s.TenantID = currentTenantID(ctx)
	return tx.WithContext(ctx).Create(s).Error
}

// SaveSubscription menyimpan seluruh kolom langganan (kecuali id & created_at).
func SaveSubscription(ctx context.Context, tx *gorm.DB, s *models.Subscription) error {
	return tx.WithContext(ctx).
		Model(&models.Subscription{}).
		Where("id = ? AND tenant_id = ?", s.ID, currentTenantID(ctx)).
		Updates(map[string]any{
			"plan_id":              s.PlanID,
			"term_months":          s.TermMonths,
			"discount_rate":        s.DiscountRate,
			"status":               s.Status,
			"trial_ends_at":        s.TrialEndsAt,
			"current_period_start": s.CurrentPeriodStart,
			"current_period_end":   s.CurrentPeriodEnd,
			"auto_renew":           s.AutoRenew,
			"canceled_at":          s.CanceledAt,
			"cancel_reason":        s.CancelReason,
			"updated_at":           time.Now().UTC(),
		}).Error
}

// ── Tagihan ───────────────────────────────────────────────────────────────

// NextSubInvoiceNumber mengambil nomor faktur berurutan berikutnya (SUB-000001).
func NextSubInvoiceNumber(ctx context.Context, tx *gorm.DB) (string, error) {
	var n int64
	if err := tx.WithContext(ctx).Raw("SELECT nextval('subscription_invoice_seq')").Scan(&n).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("SUB-%06d", n), nil
}

// CreateSubInvoice menyimpan tagihan baru. Relasi Plan (bila diisi pemanggil
// untuk respons) tidak ikut disimpan.
func CreateSubInvoice(ctx context.Context, tx *gorm.DB, in *models.SubscriptionInvoice) error {
	in.TenantID = currentTenantID(ctx)
	return tx.WithContext(ctx).Omit("Plan").Create(in).Error
}

// FindSubInvoiceForTenant memuat satu tagihan milik tenant konteks.
func FindSubInvoiceForTenant(ctx context.Context, tx *gorm.DB, id string) (models.SubscriptionInvoice, error) {
	var in models.SubscriptionInvoice
	err := tenantDB(ctx, tx).Preload("Plan").
		Where("id = ? AND tenant_id = ?", id, currentTenantID(ctx)).
		First(&in).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return in, ErrSubInvoiceNotFound
	}
	return in, err
}

// LockSubInvoiceForTenant memuat SEKALIGUS mengunci (FOR UPDATE) satu tagihan
// milik tenant konteks. Mengirim konfirmasi, menyetujui pembayaran, dan
// membatalkan tagihan sama-sama melewati kunci ini, jadi tidak ada konfirmasi
// yang tertinggal pada tagihan yang baru saja dibatalkan.
func LockSubInvoiceForTenant(ctx context.Context, tx *gorm.DB, id string) (models.SubscriptionInvoice, error) {
	var in models.SubscriptionInvoice
	err := tenantDB(ctx, tx).Clauses(lockForUpdate()).
		Where("id = ? AND tenant_id = ?", id, currentTenantID(ctx)).
		First(&in).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return in, ErrSubInvoiceNotFound
	}
	if err != nil {
		return in, err
	}
	if in.PlanID != nil {
		var p models.Plan
		if err := tenantDB(ctx, tx).Where("id = ?", *in.PlanID).First(&p).Error; err == nil {
			in.Plan = &p
		}
	}
	return in, nil
}

// OpenSubInvoiceForTenant mengembalikan tagihan terbuka (open/overdue) terbaru
// milik tenant, bila ada.
func OpenSubInvoiceForTenant(ctx context.Context, tx *gorm.DB) (models.SubscriptionInvoice, bool, error) {
	var in models.SubscriptionInvoice
	err := tenantDB(ctx, tx).Preload("Plan").
		Where("tenant_id = ? AND status IN ('open','overdue')", currentTenantID(ctx)).
		Order("created_at DESC").
		First(&in).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return in, false, nil
	}
	if err != nil {
		return in, false, err
	}
	return in, true, nil
}

// LatestPaidSubInvoice mengembalikan tagihan berbayar terbaru milik tenant.
func LatestPaidSubInvoice(ctx context.Context, tx *gorm.DB) (models.SubscriptionInvoice, error) {
	var in models.SubscriptionInvoice
	err := tenantDB(ctx, tx).Preload("Plan").
		Where("tenant_id = ? AND status = 'paid'", currentTenantID(ctx)).
		Order("paid_at DESC").
		First(&in).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return in, ErrSubInvoiceNotFound
	}
	return in, err
}

// ListSubInvoicesForTenant mengembalikan halaman tagihan milik tenant, terbaru dulu.
func ListSubInvoicesForTenant(ctx context.Context, limit, offset int) ([]models.SubscriptionInvoice, int64, error) {
	q := tenantDB(ctx, nil).Model(&models.SubscriptionInvoice{}).
		Where("tenant_id = ?", currentTenantID(ctx))
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.SubscriptionInvoice
	err := q.Preload("Plan").Order("created_at DESC").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

// SaveSubInvoice menyimpan kolom yang berubah pada sebuah tagihan.
func SaveSubInvoice(ctx context.Context, tx *gorm.DB, in *models.SubscriptionInvoice) error {
	return tx.WithContext(ctx).
		Model(&models.SubscriptionInvoice{}).
		Where("id = ? AND tenant_id = ?", in.ID, currentTenantID(ctx)).
		Updates(map[string]any{
			"discount_amount": in.DiscountAmount,
			"total_amount":    in.TotalAmount,
			"paid_amount":     in.PaidAmount,
			"status":          in.Status,
			"paid_at":         in.PaidAt,
			// Periode bisa digeser saat lunas (tagihan masa coba, lihat
			// services.terapkanPembayaran).
			"period_start": in.PeriodStart,
			"period_end":   in.PeriodEnd,
			"updated_at":   time.Now().UTC(),
		}).Error
}

// CreateSubPayment menyimpan satu pembayaran tagihan.
func CreateSubPayment(ctx context.Context, tx *gorm.DB, p *models.SubscriptionPayment) error {
	return tx.WithContext(ctx).Create(p).Error
}

// ── Pendapatan diterima di muka ───────────────────────────────────────────

// CreateDeferredEntries menyimpan sekumpulan baris pengakuan.
func CreateDeferredEntries(ctx context.Context, tx *gorm.DB, rows []models.DeferredRevenueEntry) error {
	if len(rows) == 0 {
		return nil
	}
	tid := currentTenantID(ctx)
	for i := range rows {
		rows[i].TenantID = tid
	}
	return tx.WithContext(ctx).Create(&rows).Error
}

// DeleteUnrecognizedDeferred menghapus baris pengakuan yang belum diakui untuk
// sebuah tagihan (dipakai saat pembatalan / ganti paket). Mengembalikan jumlah baris.
func DeleteUnrecognizedDeferred(ctx context.Context, tx *gorm.DB, invoiceID string) (int64, error) {
	res := tx.WithContext(ctx).
		Where("subscription_invoice_id = ? AND tenant_id = ? AND recognized_at IS NULL", invoiceID, currentTenantID(ctx)).
		Delete(&models.DeferredRevenueEntry{})
	return res.RowsAffected, res.Error
}

// UpsertDeferredAdjustment menambahkan `amount` ke baris pengakuan bulan `month`
// untuk sebuah tagihan (membuatnya bila belum ada), dan menandainya SUDAH
// diakui. Dipakai saat pembatalan / ganti paket agar total yang diakui persis
// sama dengan uang yang benar-benar menjadi hak platform.
func UpsertDeferredAdjustment(ctx context.Context, tx *gorm.DB, invoiceID string, month time.Time, amount int64) error {
	return tx.WithContext(ctx).Exec(`
		INSERT INTO deferred_revenue_entries
			(id, subscription_invoice_id, tenant_id, recognition_month, amount, recognized_at, created_at)
		VALUES (?, ?, ?, ?, ?, now(), now())
		ON CONFLICT (subscription_invoice_id, recognition_month) DO UPDATE SET
			amount        = deferred_revenue_entries.amount + EXCLUDED.amount,
			recognized_at = now()`,
		ulid.New(), invoiceID, currentTenantID(ctx), month.Format("2006-01-02"), amount,
	).Error
}

// SumRecognizedDeferred menjumlahkan pendapatan yang SUDAH diakui untuk sebuah tagihan.
func SumRecognizedDeferred(ctx context.Context, tx *gorm.DB, invoiceID string) (int64, error) {
	var total int64
	err := tx.WithContext(ctx).
		Model(&models.DeferredRevenueEntry{}).
		Where("subscription_invoice_id = ? AND recognized_at IS NOT NULL", invoiceID).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error
	return total, err
}

// RecognizeDueRevenue mengakui semua baris yang bulan pengakuannya sudah tiba
// (§13.4). GLOBAL — lintas tenant, dipanggil pekerjaan harian. Mengembalikan
// jumlah baris yang baru diakui.
func RecognizeDueRevenue(ctx context.Context) (int64, error) {
	res := tenantDB(ctx, nil).Exec(`
		UPDATE deferred_revenue_entries
		SET recognized_at = now()
		WHERE recognized_at IS NULL
		  AND recognition_month <= date_trunc('month', now())::date`)
	return res.RowsAffected, res.Error
}

// RecognizedRevenueBetween menjumlahkan pendapatan yang SUDAH diakui pada rentang
// bulan [from, to]. tenantID kosong = lintas seluruh tenant (laporan platform).
func RecognizedRevenueBetween(ctx context.Context, tenantID, from, to string) (int64, error) {
	q := tenantDB(ctx, nil).
		Model(&models.DeferredRevenueEntry{}).
		Where("recognized_at IS NOT NULL AND recognition_month BETWEEN ? AND ?", from, to)
	if tenantID != "" {
		q = q.Where("tenant_id = ?", tenantID)
	}
	var total int64
	err := q.Select("COALESCE(SUM(amount), 0)").Scan(&total).Error
	return total, err
}

// ── Pengembalian dana ─────────────────────────────────────────────────────

// CreateSubRefund menyimpan jejak audit pengembalian dana.
func CreateSubRefund(ctx context.Context, tx *gorm.DB, r *models.SubscriptionRefund) error {
	r.TenantID = currentTenantID(ctx)
	return tx.WithContext(ctx).Create(r).Error
}

// TenantIDFromCtx mengekspos tenant_id konteks untuk service langganan (tabel
// platform tak ber-RLS, jadi service memfilternya sendiri).
func TenantIDFromCtx(ctx context.Context) string { return reqctx.TenantID(ctx) }
