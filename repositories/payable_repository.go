package repositories

import (
	"context"
	"errors"
	"time"

	"candra/backend-api/internal/reqctx"
	"candra/backend-api/internal/ulid"
	"candra/backend-api/models"

	"github.com/shopspring/decimal"
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

// SupplierStat: angka belanja satu pemasok di satu outlet (atau semua outlet
// yang boleh) — untuk daftar Pemasok.
type SupplierStat struct {
	SupplierID     string
	PurchaseCount  int64
	Spent30d       int64 `gorm:"column:spent_30d"`
	LastPurchaseAt *time.Time
	Outstanding    int64
	OverdueCount   int64
}

// SupplierStats mengembalikan SupplierStat per pemasok yang pernah dibeli.
func SupplierStats(ctx context.Context, outletID string, hariIni time.Time) ([]SupplierStat, error) {
	q := tenantDB(ctx, nil).Table("purchases pu").
		Where("pu.tenant_id = ? AND pu.status = 'received' AND pu.supplier_id IS NOT NULL", reqctx.TenantID(ctx))
	if outletID != "" {
		q = q.Where("pu.outlet_id = ?", outletID)
	}
	q = scopeOutlet(ctx, q, "pu.outlet_id")
	var rows []SupplierStat
	err := q.Select(`pu.supplier_id, COUNT(*) AS purchase_count,
			COALESCE(SUM(pu.total) FILTER (WHERE pu.occurred_at >= now() - interval '30 days'), 0)::bigint AS spent_30d,
			MAX(pu.occurred_at) AS last_purchase_at,
			COALESCE(SUM(pu.total - pu.paid_amount), 0)::bigint AS outstanding,
			COUNT(*) FILTER (WHERE pu.paid_amount < pu.total AND pu.due_date < ?::date) AS overdue_count`,
		hariIni.Format("2006-01-02")).
		Group("pu.supplier_id").
		Scan(&rows).Error
	return rows, err
}

// SupplierProduct: satu barang yang pernah dibeli dari pemasok — pembelian
// terakhirnya (harga & satuan beli, bisa kemasan) dan seberapa sering.
type SupplierProduct struct {
	ProductID      string
	ProductName    string
	BaseUnitName   string
	UnitCost       int64           // per satuan beli terakhir (kemasan bila dus)
	UnitName       string          // satuan beli terakhir
	UnitConversion decimal.Decimal // isi satuan beli terakhir dalam satuan dasar
	ProductUnitID  *string
	LastBoughtAt   *time.Time      // nil = belum pernah dibeli dari pemasok ini
	Times          int64           // berapa nota memuat barang ini
	Qty90d         decimal.Decimal `gorm:"column:qty_90d"` // satuan dasar, 90 hari terakhir
	// Utama: pemasok ini pemasok utama barangnya (products.supplier_id, 000050).
	Utama bool
}

// SupplierProducts: barang yang biasa dibeli dari satu pemasok, terakhir
// dibeli dulu (paling banyak 50). Barang yang sudah dihapus tidak ikut —
// daftar ini dipakai untuk memesan lagi.
func SupplierProducts(ctx context.Context, supplierID, outletID string) ([]SupplierProduct, error) {
	outletCond, args := "", []any{reqctx.TenantID(ctx), supplierID}
	if outletID != "" {
		outletCond = " AND pu.outlet_id = ?"
		args = append(args, outletID)
	}
	if ids, terbatas := reqctx.OutletScope(ctx); terbatas {
		if len(ids) == 0 {
			return nil, nil
		}
		outletCond += " AND pu.outlet_id IN ?"
		args = append(args, ids)
	}
	var rows []SupplierProduct
	err := tenantDB(ctx, nil).Raw(`
		WITH baris AS (
			SELECT pi.product_id, pi.purchase_id, pi.qty, pi.unit_conversion, pi.unit_cost, pi.unit_name,
				pi.product_unit_id, pu.occurred_at
			FROM purchase_items pi
			JOIN purchases pu ON pu.tenant_id = pi.tenant_id AND pu.id = pi.purchase_id
			WHERE pi.tenant_id = ? AND pu.supplier_id = ? AND pu.status = 'received'`+outletCond+`
		), agg AS (
			SELECT product_id, COUNT(DISTINCT purchase_id) AS times,
				COALESCE(SUM(qty * unit_conversion) FILTER (WHERE occurred_at >= now() - interval '90 days'), 0) AS qty_90d
			FROM baris GROUP BY product_id
		), terakhir AS (
			SELECT DISTINCT ON (product_id) * FROM baris ORDER BY product_id, occurred_at DESC
		)
		SELECT * FROM (
			SELECT t.product_id, p.name AS product_name, COALESCE(un.name, '') AS base_unit_name,
				t.unit_cost, t.unit_name, t.unit_conversion, t.product_unit_id,
				t.occurred_at AS last_bought_at, a.times, a.qty_90d, COALESCE(p.supplier_id = ?, false) AS utama
			FROM terakhir t
			JOIN agg a ON a.product_id = t.product_id
			JOIN products p ON p.tenant_id = ? AND p.id = t.product_id AND p.deleted_at IS NULL
			LEFT JOIN units un ON un.tenant_id = p.tenant_id AND un.id = p.unit_id
			UNION ALL
			-- Pemasok utamanya pemasok ini, tapi belum pernah dibeli darinya:
			-- harga terakhir = harga modal, satuan beli = satuan dasar.
			SELECT p.id, p.name, COALESCE(un.name, ''), p.cost_price, COALESCE(un.name, ''), 1, NULL,
				NULL, 0, 0, true
			FROM products p
			LEFT JOIN units un ON un.tenant_id = p.tenant_id AND un.id = p.unit_id
			WHERE p.tenant_id = ? AND p.supplier_id = ? AND p.deleted_at IS NULL
				AND p.id NOT IN (SELECT product_id FROM baris)
		) x
		ORDER BY last_bought_at DESC NULLS LAST, product_name
		LIMIT 50`, append(args, supplierID, reqctx.TenantID(ctx), reqctx.TenantID(ctx), supplierID)...).Scan(&rows).Error
	return rows, err
}
