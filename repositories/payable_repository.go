package repositories

import (
	"context"
	"errors"
	"time"

	"candra/backend-api/internal/reqctx"
	"candra/backend-api/internal/ulid"
	"candra/backend-api/models"

	"gorm.io/gorm"
)

// LockPurchaseInTenant mengunci baris pembelian (FOR UPDATE) lalu memuatnya.
// Wajib dipakai di dalam tx sebelum mengubah paid_amount: dua pembayaran
// bersamaan tidak boleh sama-sama membaca sisa utang yang lama.
func LockPurchaseInTenant(ctx context.Context, tx *gorm.DB, id string, out *models.Purchase) error {
	if err := lockTenantRow[models.Purchase](ctx, tx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPurchaseNotFound
		}
		return err
	}
	return FindPurchaseInTenant(ctx, tx, id, out)
}

// SetPurchasePaid menyetel paid_amount pembelian (sudah dikunci pemanggil).
func SetPurchasePaid(ctx context.Context, tx *gorm.DB, id string, paid int64) error {
	return scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.Purchase{}).
		Where("id = ?", id).
		Updates(map[string]any{"paid_amount": paid, "updated_at": gorm.Expr("now()")}).Error
}

// CreatePurchasePayment menyimpan satu pembayaran pembelian.
func CreatePurchasePayment(ctx context.Context, tx *gorm.DB, p *models.PurchasePayment) error {
	return createTenant(ctx, tx, p)
}

// PurchasePaymentRow: satu pembayaran dengan nama pencatatnya.
type PurchasePaymentRow struct {
	models.PurchasePayment
	CreatedByName string
}

// ListPurchasePayments mengembalikan pembayaran satu pembelian, terlama dulu.
func ListPurchasePayments(ctx context.Context, purchaseID string) ([]PurchasePaymentRow, error) {
	var rows []PurchasePaymentRow
	err := tenantDB(ctx, nil).Table("purchase_payments pp").
		Joins("LEFT JOIN users u ON u.tenant_id = pp.tenant_id AND u.id = pp.created_by").
		Where("pp.tenant_id = ? AND pp.purchase_id = ?", reqctx.TenantID(ctx), purchaseID).
		Select("pp.*, COALESCE(u.name, '') AS created_by_name").
		Order("pp.paid_at ASC, pp.id ASC").
		Scan(&rows).Error
	return rows, err
}

// PayableSupplier: utang ke satu pemasok (atau "tanpa pemasok" bila
// SupplierID kosong) di satu outlet.
type PayableSupplier struct {
	SupplierID    string
	SupplierName  string
	Outstanding   int64
	Count         int64
	OverdueCount  int64
	OverdueAmount int64
	DueSoonCount  int64 // jatuh tempo hari ini s.d. `segera` hari lagi
	DueSoonAmount int64
	NearestDue    *time.Time
	OldestAt      time.Time
}

// PayablesSummary merangkum utang pemasok SELURUH pembelian belum lunas satu
// outlet (atau semua outlet yang boleh): per pemasok, terbesar dulu. `hariIni`
// = tanggal usaha outlet, untuk menghitung yang sudah lewat jatuh tempo.
func PayablesSummary(ctx context.Context, outletID string, hariIni time.Time, segera int) ([]PayableSupplier, error) {
	q := tenantDB(ctx, nil).Table("purchases pu").
		Joins("LEFT JOIN suppliers s ON s.tenant_id = pu.tenant_id AND s.id = pu.supplier_id").
		Where("pu.tenant_id = ? AND pu.status = 'received' AND pu.paid_amount < pu.total", reqctx.TenantID(ctx))
	if outletID != "" {
		q = q.Where("pu.outlet_id = ?", outletID)
	}
	q = scopeOutlet(ctx, q, "pu.outlet_id")
	var rows []PayableSupplier
	hari, batas := hariIni.Format("2006-01-02"), hariIni.AddDate(0, 0, segera).Format("2006-01-02")
	err := q.Select(`COALESCE(pu.supplier_id, '') AS supplier_id, COALESCE(MAX(s.name), '') AS supplier_name,
			SUM(pu.total - pu.paid_amount)::bigint AS outstanding, COUNT(*) AS count,
			COUNT(*) FILTER (WHERE pu.due_date < ?::date) AS overdue_count,
			COALESCE(SUM(pu.total - pu.paid_amount) FILTER (WHERE pu.due_date < ?::date), 0)::bigint AS overdue_amount,
			COUNT(*) FILTER (WHERE pu.due_date BETWEEN ?::date AND ?::date) AS due_soon_count,
			COALESCE(SUM(pu.total - pu.paid_amount) FILTER (WHERE pu.due_date BETWEEN ?::date AND ?::date), 0)::bigint AS due_soon_amount,
			MIN(pu.due_date) AS nearest_due, MIN(pu.occurred_at) AS oldest_at`, hari, hari, hari, batas, hari, batas).
		Group("COALESCE(pu.supplier_id, '')").
		Order("outstanding DESC").
		Scan(&rows).Error
	return rows, err
}

// PayableCandidate: satu pembelian belum lunas yang jatuh temponya sudah
// dekat/lewat — kandidat pengingat.
type PayableCandidate struct {
	ID           string
	TenantID     string
	OutletID     string
	InvoiceNo    string
	SupplierName string
	DueDate      time.Time
	Outstanding  int64
}

// PayableReminderCandidates mengembalikan pembelian belum lunas LINTAS tenant
// (atau satu tenant) yang jatuh temponya ≤ batas. Batasnya longgar (tanggal
// UTC); pemanggil menyaring ulang dengan tanggal usaha outlet masing-masing.
func PayableReminderCandidates(ctx context.Context, batas time.Time, tenantID string) ([]PayableCandidate, error) {
	q := tenantDB(ctx, nil).Table("purchases pu").
		Joins("LEFT JOIN suppliers s ON s.tenant_id = pu.tenant_id AND s.id = pu.supplier_id").
		Where("pu.status = 'received' AND pu.paid_amount < pu.total AND pu.due_date IS NOT NULL AND pu.due_date <= ?::date",
			batas.Format("2006-01-02"))
	if tenantID != "" {
		q = q.Where("pu.tenant_id = ?", tenantID)
	}
	var rows []PayableCandidate
	err := q.Select(`pu.id, pu.tenant_id, pu.outlet_id, COALESCE(pu.invoice_no, '') AS invoice_no,
			COALESCE(s.name, '') AS supplier_name, pu.due_date, (pu.total - pu.paid_amount)::bigint AS outstanding`).
		Order("pu.tenant_id, pu.due_date, pu.id").
		Scan(&rows).Error
	return rows, err
}

// InsertPayableNoticeOnce mencatat pengingat (pembelian, jenis) sekali saja;
// false bila sudah pernah tercatat.
func InsertPayableNoticeOnce(ctx context.Context, tx *gorm.DB, purchaseID, kind string) (bool, error) {
	var id string
	err := tenantDB(ctx, tx).Raw(`
		INSERT INTO payable_notices (id, tenant_id, purchase_id, kind) VALUES (?, ?, ?, ?)
		ON CONFLICT (tenant_id, purchase_id, kind) DO NOTHING RETURNING id`,
		ulid.New(), reqctx.TenantID(ctx), purchaseID, kind).Scan(&id).Error
	return id != "", err
}
