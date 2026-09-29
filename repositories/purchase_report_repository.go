package repositories

import (
	"context"

	"candra/backend-api/internal/reqctx"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Laporan belanja (GET /reports/purchases): pembelian menurut TANGGAL USAHA
// nota (business_date), pembayaran menurut tanggal usaha pembayarannya.
// Hanya pembelian berstatus received.

// purchaseRange: pembelian received di rentang (opsional per outlet, dalam
// lingkup toko pengguna). Alias tabelnya "pu".
func purchaseRange(ctx context.Context, outletID, from, to string) *gorm.DB {
	q := tenantDB(ctx, nil).Table("purchases pu").
		Where("pu.tenant_id = ? AND pu.status = 'received' AND pu.business_date BETWEEN ?::date AND ?::date",
			reqctx.TenantID(ctx), from, to)
	if outletID != "" {
		q = q.Where("pu.outlet_id = ?", outletID)
	}
	return scopeOutlet(ctx, q, "pu.outlet_id")
}

// PurchaseReportTotals: angka utama laporan belanja.
type PurchaseReportTotals struct {
	NotaCount int64
	Belanja   int64 // Σ total nota di rentang
	// SisaNota: sisa utang SEKARANG dari nota di rentang (yang belum lunas).
	SisaNota int64
	// Pembayaran di rentang (tanggal usaha pembayaran), apa pun tanggal notanya.
	Dibayar   int64
	DariLaci  int64
	DariLain  int64
	UtangKini int64 // seluruh utang yang belum lunas saat ini (tanpa rentang)
}

// PurchaseReportTotalsFor menghitung PurchaseReportTotals.
func PurchaseReportTotalsFor(ctx context.Context, outletID, from, to string) (PurchaseReportTotals, error) {
	var r PurchaseReportTotals
	if err := purchaseRange(ctx, outletID, from, to).
		Select(`COUNT(*) AS nota_count, COALESCE(SUM(pu.total), 0)::bigint AS belanja,
			COALESCE(SUM(pu.total - pu.paid_amount), 0)::bigint AS sisa_nota`).
		Scan(&r).Error; err != nil {
		return r, err
	}

	var bayar struct{ Dibayar, DariLaci, DariLain int64 }
	qb := tenantDB(ctx, nil).Table("purchase_payments pp").
		Where("pp.tenant_id = ? AND pp.business_date BETWEEN ?::date AND ?::date", reqctx.TenantID(ctx), from, to)
	if outletID != "" {
		qb = qb.Where("pp.outlet_id = ?", outletID)
	}
	if err := scopeOutlet(ctx, qb, "pp.outlet_id").
		Select(`COALESCE(SUM(pp.amount), 0)::bigint AS dibayar,
			COALESCE(SUM(pp.amount) FILTER (WHERE pp.source = 'drawer'), 0)::bigint AS dari_laci,
			COALESCE(SUM(pp.amount) FILTER (WHERE pp.source = 'other'), 0)::bigint AS dari_lain`).
		Scan(&bayar).Error; err != nil {
		return r, err
	}
	r.Dibayar, r.DariLaci, r.DariLain = bayar.Dibayar, bayar.DariLaci, bayar.DariLain

	qu := tenantDB(ctx, nil).Table("purchases pu").
		Where("pu.tenant_id = ? AND pu.status = 'received' AND pu.paid_amount < pu.total", reqctx.TenantID(ctx))
	if outletID != "" {
		qu = qu.Where("pu.outlet_id = ?", outletID)
	}
	err := scopeOutlet(ctx, qu, "pu.outlet_id").
		Select("COALESCE(SUM(pu.total - pu.paid_amount), 0)::bigint").Scan(&r.UtangKini).Error
	return r, err
}

// PurchaseReportSupplier: belanja per pemasok di rentang.
type PurchaseReportSupplier struct {
	SupplierID   string
	SupplierName string
	NotaCount    int64
	Belanja      int64
	SisaNota     int64
}

// PurchaseReportBySupplier: belanja per pemasok, terbesar dulu ("" = tanpa pemasok).
func PurchaseReportBySupplier(ctx context.Context, outletID, from, to string) ([]PurchaseReportSupplier, error) {
	var rows []PurchaseReportSupplier
	err := purchaseRange(ctx, outletID, from, to).
		Joins("LEFT JOIN suppliers s ON s.tenant_id = pu.tenant_id AND s.id = pu.supplier_id").
		Select(`COALESCE(pu.supplier_id, '') AS supplier_id, COALESCE(MAX(s.name), '') AS supplier_name,
			COUNT(*) AS nota_count, SUM(pu.total)::bigint AS belanja,
			SUM(pu.total - pu.paid_amount)::bigint AS sisa_nota`).
		Group("COALESCE(pu.supplier_id, '')").
		Order("belanja DESC").
		Scan(&rows).Error
	return rows, err
}

// PurchaseReportDay: belanja satu tanggal usaha.
type PurchaseReportDay struct {
	Tanggal string
	Belanja int64
	Nota    int64
}

// PurchaseReportByDay: belanja per tanggal usaha (hanya hari yang ada nota).
func PurchaseReportByDay(ctx context.Context, outletID, from, to string) ([]PurchaseReportDay, error) {
	var rows []PurchaseReportDay
	err := purchaseRange(ctx, outletID, from, to).
		Select(`to_char(pu.business_date, 'YYYY-MM-DD') AS tanggal, SUM(pu.total)::bigint AS belanja, COUNT(*) AS nota`).
		Group("pu.business_date").
		Order("pu.business_date").
		Scan(&rows).Error
	return rows, err
}

// PurchaseReportProduct: belanja satu barang di rentang.
type PurchaseReportProduct struct {
	ProductID   string
	ProductName string
	UnitName    string
	Qty         decimal.Decimal // satuan dasar
	Nilai       int64
}

// PurchaseReportTopProducts: barang dengan belanja terbesar (paling banyak `n`).
func PurchaseReportTopProducts(ctx context.Context, outletID, from, to string, n int) ([]PurchaseReportProduct, error) {
	var rows []PurchaseReportProduct
	err := purchaseRange(ctx, outletID, from, to).
		Joins("JOIN purchase_items pi ON pi.tenant_id = pu.tenant_id AND pi.purchase_id = pu.id").
		Joins("JOIN products p ON p.tenant_id = pi.tenant_id AND p.id = pi.product_id").
		Joins("LEFT JOIN units un ON un.tenant_id = p.tenant_id AND un.id = p.unit_id").
		Select(`pi.product_id, MAX(p.name) AS product_name, COALESCE(MAX(un.name), '') AS unit_name,
			SUM(pi.qty * pi.unit_conversion) AS qty, SUM(pi.line_total)::bigint AS nilai`).
		Group("pi.product_id").
		Order("nilai DESC").
		Limit(n).
		Scan(&rows).Error
	return rows, err
}

// PurchaseLineRow: satu baris barang pada satu nota — untuk ekspor.
type PurchaseLineRow struct {
	BusinessDate string
	InvoiceNo    string
	SupplierName string
	ProductName  string
	VariantName  string
	Qty          decimal.Decimal
	UnitName     string
	UnitCost     int64
	LineTotal    int64
	NotaTotal    int64
	NotaPaid     int64
	DueDate      string
}

// PurchaseLines: semua baris barang nota di rentang, urut tanggal lalu nota.
func PurchaseLines(ctx context.Context, outletID, from, to string) ([]PurchaseLineRow, error) {
	var rows []PurchaseLineRow
	err := purchaseRange(ctx, outletID, from, to).
		Joins("JOIN purchase_items pi ON pi.tenant_id = pu.tenant_id AND pi.purchase_id = pu.id").
		Joins("JOIN products p ON p.tenant_id = pi.tenant_id AND p.id = pi.product_id").
		Joins("LEFT JOIN product_variants v ON v.tenant_id = pi.tenant_id AND v.id = pi.variant_id").
		Joins("LEFT JOIN units un ON un.tenant_id = p.tenant_id AND un.id = p.unit_id").
		Joins("LEFT JOIN suppliers s ON s.tenant_id = pu.tenant_id AND s.id = pu.supplier_id").
		Select(`to_char(pu.business_date, 'YYYY-MM-DD') AS business_date, COALESCE(pu.invoice_no, '') AS invoice_no,
			COALESCE(s.name, '') AS supplier_name, p.name AS product_name, COALESCE(v.name, '') AS variant_name,
			pi.qty, COALESCE(NULLIF(pi.unit_name, ''), un.name, '') AS unit_name, pi.unit_cost, pi.line_total,
			pu.total AS nota_total, pu.paid_amount AS nota_paid,
			COALESCE(to_char(pu.due_date, 'YYYY-MM-DD'), '') AS due_date`).
		Order("pu.business_date, pu.occurred_at, pu.id, pi.id").
		Scan(&rows).Error
	return rows, err
}

// PurchasePaymentExportRow: satu pembayaran ke pemasok — untuk ekspor.
type PurchasePaymentExportRow struct {
	BusinessDate  string
	SupplierName  string
	InvoiceNo     string
	NotaDate      string
	Amount        int64
	Source        string
	Note          string
	CreatedByName string
}

// PurchasePaymentsInRange: pembayaran menurut tanggal usaha pembayarannya.
func PurchasePaymentsInRange(ctx context.Context, outletID, from, to string) ([]PurchasePaymentExportRow, error) {
	q := tenantDB(ctx, nil).Table("purchase_payments pp").
		Joins("JOIN purchases pu ON pu.tenant_id = pp.tenant_id AND pu.id = pp.purchase_id").
		Joins("LEFT JOIN suppliers s ON s.tenant_id = pu.tenant_id AND s.id = pu.supplier_id").
		Joins("LEFT JOIN users u ON u.tenant_id = pp.tenant_id AND u.id = pp.created_by").
		Where("pp.tenant_id = ? AND pp.business_date BETWEEN ?::date AND ?::date", reqctx.TenantID(ctx), from, to)
	if outletID != "" {
		q = q.Where("pp.outlet_id = ?", outletID)
	}
	var rows []PurchasePaymentExportRow
	err := scopeOutlet(ctx, q, "pp.outlet_id").
		Select(`to_char(pp.business_date, 'YYYY-MM-DD') AS business_date, COALESCE(s.name, '') AS supplier_name,
			COALESCE(pu.invoice_no, '') AS invoice_no, to_char(pu.business_date, 'YYYY-MM-DD') AS nota_date,
			pp.amount, pp.source, COALESCE(pp.note, '') AS note, COALESCE(u.name, '') AS created_by_name`).
		Order("pp.business_date, pp.paid_at, pp.id").
		Scan(&rows).Error
	return rows, err
}
