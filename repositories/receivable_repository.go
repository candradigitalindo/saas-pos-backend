package repositories

import (
	"context"
	"errors"
	"time"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

var ErrReceivableNotFound = errors.New("piutang tidak ditemukan")

// CreateReceivable menyimpan piutang baru (dari kasbon / invoice). tx opsional.
func CreateReceivable(ctx context.Context, tx *gorm.DB, row *models.Receivable) error {
	return createTenant(ctx, tx, row)
}

// FindReceivableInTenant memuat satu piutang. tx opsional.
func FindReceivableInTenant(ctx context.Context, tx *gorm.DB, id string, out *models.Receivable) error {
	err := firstTenant(ctx, tx, id, out)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrReceivableNotFound
	}
	return err
}

// FindReceivableBySource memuat piutang berdasarkan sumbernya (mis. sale id).
// Dipakai saat void/refund penjualan kasbon.
func FindReceivableBySource(ctx context.Context, tx *gorm.DB, sourceTable, sourceID string, out *models.Receivable) error {
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("source_table = ? AND source_id = ?", sourceTable, sourceID).
		First(out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrReceivableNotFound
	}
	return err
}

// StatusBelumLunas: saringan status "unpaid" = 'open' + 'partial'. Kasbon
// yang sudah dicicil berstatus 'partial' — menyaring 'open' saja membuatnya
// hilang dari daftar "belum lunas" padahal sisanya masih ada.
const StatusBelumLunas = "unpaid"

// ListReceivables mengembalikan satu halaman piutang (opsional per pelanggan /
// status), berpaginasi. Status "unpaid" = open + partial, TERLAMA dulu —
// urutan yang sama dengan pelunasan setoran pelanggan; selain itu terbaru dulu.
func ListReceivables(ctx context.Context, customerID, status string, limit, offset int) ([]models.Receivable, int64, error) {
	conds, args := []string{}, []any{}
	if customerID != "" {
		conds, args = append(conds, "customer_id = ?"), append(args, customerID)
	}
	urut := "created_at DESC, id DESC"
	switch status {
	case "":
	case StatusBelumLunas:
		conds = append(conds, "status IN ('open', 'partial')")
		urut = "created_at ASC, id ASC"
	default:
		conds, args = append(conds, "status = ?"), append(args, status)
	}
	where := ""
	for i, c := range conds {
		if i > 0 {
			where += " AND "
		}
		where += c
	}
	return paginateTenant[models.Receivable](ctx, where, args, urut, limit, offset)
}

// ReceivableSaleRef: nota asal sebuah kasbon.
type ReceivableSaleRef struct {
	ID           string
	ReceiptNo    string
	BusinessDate time.Time
	OutletName   string
}

// ReceivableSaleRefs memuat nomor nota, tanggal usaha, dan nama toko untuk
// penjualan `saleIDs` — satu query untuk satu halaman kasbon.
func ReceivableSaleRefs(ctx context.Context, saleIDs []string) (map[string]ReceivableSaleRef, error) {
	out := make(map[string]ReceivableSaleRef, len(saleIDs))
	if len(saleIDs) == 0 {
		return out, nil
	}
	var rows []ReceivableSaleRef
	err := tenantDB(ctx, nil).Table("sales s").
		Joins("JOIN outlets o ON o.tenant_id = s.tenant_id AND o.id = s.outlet_id").
		Where("s.tenant_id = ? AND s.id IN ?", currentTenantID(ctx), saleIDs).
		Select("s.id, s.receipt_no, s.business_date, o.name AS outlet_name").
		Scan(&rows).Error
	for _, r := range rows {
		out[r.ID] = r
	}
	return out, err
}

// LockUnpaidReceivablesOfCustomer mengunci (FOR UPDATE) lalu memuat seluruh
// kasbon belum lunas seorang pelanggan, TERLAMA dulu — urutan pelunasan
// setoran. Urutan kunci yang tetap juga mencegah dua setoran bersamaan saling
// menunggu (deadlock).
func LockUnpaidReceivablesOfCustomer(ctx context.Context, tx *gorm.DB, customerID string) ([]models.Receivable, error) {
	var rows []models.Receivable
	err := scopeTenant(ctx, tx).Clauses(lockForUpdate()).
		Where("customer_id = ? AND status IN ('open', 'partial')", customerID).
		Order("created_at ASC, id ASC").
		Find(&rows).Error
	return rows, err
}

// SetReceivablePaid menyetel paid_amount & status kasbon (sudah dikunci).
func SetReceivablePaid(ctx context.Context, tx *gorm.DB, id string, paid int64, status string) error {
	return scopeTenant(ctx, tx).Model(&models.Receivable{}).
		Where("id = ?", id).
		Updates(map[string]any{"paid_amount": paid, "status": status, "updated_at": gorm.Expr("now()")}).Error
}

// CreateReceivablePayment menyimpan satu setoran kasbon.
func CreateReceivablePayment(ctx context.Context, tx *gorm.DB, p *models.ReceivablePayment) error {
	return createTenant(ctx, tx, p)
}

// SetReceivableDueDate mengubah jatuh tempo kasbon BELUM LUNAS (nil = tanpa
// jatuh tempo). ErrReceivableSettled bila sudah lunas / dihapusbukukan.
func SetReceivableDueDate(ctx context.Context, tx *gorm.DB, id string, due *time.Time) (models.Receivable, error) {
	var rec models.Receivable
	if err := scopeTenant(ctx, tx).Clauses(lockForUpdate()).First(&rec, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return rec, ErrReceivableNotFound
		}
		return rec, err
	}
	if rec.Status == "paid" || rec.Status == "written_off" {
		return rec, ErrReceivableSettled
	}
	rec.DueDate = due
	return rec, scopeTenant(ctx, tx).Model(&models.Receivable{}).
		Where("id = ?", id).
		Updates(map[string]any{"due_date": due, "updated_at": gorm.Expr("now()")}).Error
}

// ReceivableCustomer: kasbon belum lunas seorang pelanggan, dirangkum.
type ReceivableCustomer struct {
	CustomerID    string
	CustomerName  string
	Phone         string
	CreditLimit   int64
	Outstanding   int64
	Count         int64
	OverdueCount  int64
	OverdueAmount int64
	DueSoonCount  int64 // jatuh tempo hari ini s.d. `segera` hari lagi
	DueSoonAmount int64
	NearestDue    *time.Time
	OldestAt      time.Time
	LastPaidAt    *time.Time
}

// ReceivablesSummary merangkum kasbon belum lunas SELURUH tenant per
// pelanggan: yang lewat jatuh tempo terbesar dulu, lalu sisa terbesar — urutan
// siapa yang perlu ditagih lebih dulu. `hariIni` = tanggal usaha toko.
//
// Pelanggan yang sudah dihapus tetap ikut (kasbonnya tetap harus ditagih).
// Seperti GET /receivables, dibatasi izin receivable.manage saja — tanpa lapis
// kepemilikan CRM: yang mengurus kasbon harus melihat semua kasbon.
func ReceivablesSummary(ctx context.Context, hariIni time.Time, segera int) ([]ReceivableCustomer, error) {
	hari, batas := hariIni.Format("2006-01-02"), hariIni.AddDate(0, 0, segera).Format("2006-01-02")
	var rows []ReceivableCustomer
	err := tenantDB(ctx, nil).Table("receivables r").
		Joins("JOIN customers c ON c.tenant_id = r.tenant_id AND c.id = r.customer_id").
		Where("r.tenant_id = ? AND r.status IN ('open', 'partial')", currentTenantID(ctx)).
		Select(`r.customer_id, MAX(c.name) AS customer_name, COALESCE(MAX(c.phone), '') AS phone,
			MAX(c.credit_limit) AS credit_limit,
			SUM(r.amount - r.paid_amount)::bigint AS outstanding, COUNT(*) AS count,
			COUNT(*) FILTER (WHERE r.due_date < ?::date) AS overdue_count,
			COALESCE(SUM(r.amount - r.paid_amount) FILTER (WHERE r.due_date < ?::date), 0)::bigint AS overdue_amount,
			COUNT(*) FILTER (WHERE r.due_date BETWEEN ?::date AND ?::date) AS due_soon_count,
			COALESCE(SUM(r.amount - r.paid_amount) FILTER (WHERE r.due_date BETWEEN ?::date AND ?::date), 0)::bigint AS due_soon_amount,
			MIN(r.due_date) AS nearest_due, MIN(r.created_at) AS oldest_at,
			(SELECT MAX(rp.paid_at) FROM receivable_payments rp
			   JOIN receivables r2 ON r2.tenant_id = rp.tenant_id AND r2.id = rp.receivable_id
			  WHERE r2.tenant_id = r.tenant_id AND r2.customer_id = r.customer_id) AS last_paid_at`,
			hari, hari, hari, batas, hari, batas).
		Group("r.tenant_id, r.customer_id").
		Order("overdue_amount DESC, outstanding DESC, customer_name ASC").
		Scan(&rows).Error
	return rows, err
}

// CustomerPaymentRow: satu setoran kasbon seorang pelanggan.
type CustomerPaymentRow struct {
	models.ReceivablePayment
	ReceiptNo     string
	CollectedName string
}

// ListCustomerReceivablePayments: setoran kasbon seorang pelanggan, terbaru
// dulu (paling banyak `limit`).
func ListCustomerReceivablePayments(ctx context.Context, customerID string, limit int) ([]CustomerPaymentRow, error) {
	var rows []CustomerPaymentRow
	err := tenantDB(ctx, nil).Table("receivable_payments rp").
		Joins("JOIN receivables r ON r.tenant_id = rp.tenant_id AND r.id = rp.receivable_id").
		Joins("LEFT JOIN sales s ON r.source_table = 'sales' AND s.tenant_id = r.tenant_id AND s.id = r.source_id").
		Joins("LEFT JOIN users u ON u.tenant_id = rp.tenant_id AND u.id = rp.collected_by").
		Where("rp.tenant_id = ? AND r.customer_id = ?", currentTenantID(ctx), customerID).
		Select("rp.*, COALESCE(s.receipt_no, '') AS receipt_no, COALESCE(u.name, '') AS collected_name").
		Order("rp.paid_at DESC, rp.id DESC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}

// AddReceivablePayment mencatat pembayaran cicilan dan memperbarui
// paid_amount/status piutang, DI DALAM tx pemanggil (WithTenant — pemanggil
// menyimpan catatan idempotensinya di transaksi yang sama). Baris piutang
// dikunci lebih dulu. Menolak bila amount melebihi sisa (ErrOverpay).
func AddReceivablePayment(ctx context.Context, tx *gorm.DB, pay *models.ReceivablePayment) (models.Receivable, error) {
	var rec models.Receivable
	err := func() error {
		if err := scopeTenant(ctx, tx).Clauses(lockForUpdate()).
			First(&rec, "id = ?", pay.ReceivableID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrReceivableNotFound
			}
			return err
		}
		if rec.Status == "paid" || rec.Status == "written_off" {
			return ErrReceivableSettled
		}
		if pay.Amount > rec.Outstanding() {
			return ErrOverpay
		}

		pay.TenantID = currentTenantID(ctx)
		if err := tx.WithContext(ctx).Create(pay).Error; err != nil {
			return err
		}

		newPaid := rec.PaidAmount + pay.Amount
		newStatus := "partial"
		if newPaid >= rec.Amount {
			newStatus = "paid"
		}
		rec.PaidAmount, rec.Status = newPaid, newStatus
		return scopeTenant(ctx, tx).Model(&models.Receivable{}).
			Where("id = ?", rec.ID).
			Updates(map[string]any{"paid_amount": newPaid, "status": newStatus, "updated_at": gorm.Expr("now()")}).Error
	}()
	return rec, err
}

var (
	ErrReceivableSettled = errors.New("piutang sudah lunas atau dihapusbukukan")
	ErrOverpay           = errors.New("nominal pembayaran melebihi sisa piutang")
)
