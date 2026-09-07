package repositories

import (
	"context"
	"errors"
	"time"

	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"

	"gorm.io/gorm"
)

var ErrSaleNotFound = errors.New("transaksi tidak ditemukan")

// NextReceiptSeq mengambil nomor urut struk berikutnya untuk (outlet, hari
// usaha), mengunci baris penghitung SELECT ... FOR UPDATE lalu menaikkannya.
// Bukan COUNT(*) — itu menghasilkan nomor ganda saat dua kasir bersamaan (§13.7).
func NextReceiptSeq(ctx context.Context, tx *gorm.DB, outletID string, businessDate time.Time) (int64, error) {
	tid := currentTenantID(ctx)

	var ctr models.ReceiptCounter
	err := tx.WithContext(ctx).
		Clauses(lockForUpdate()).
		Where("tenant_id = ? AND outlet_id = ? AND business_date = ?", tid, outletID, businessDate).
		First(&ctr).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Baris belum ada: buat dengan next_seq=2 dan pakai 1 untuk struk ini.
		ctr = models.ReceiptCounter{TenantID: tid, OutletID: outletID, BusinessDate: businessDate, NextSeq: 2}
		if err := tx.WithContext(ctx).Create(&ctr).Error; err != nil {
			return 0, err
		}
		return 1, nil
	}
	if err != nil {
		return 0, err
	}

	seq := ctr.NextSeq
	if err := tx.WithContext(ctx).Model(&models.ReceiptCounter{}).
		Where("tenant_id = ? AND outlet_id = ? AND business_date = ?", tid, outletID, businessDate).
		UpdateColumn("next_seq", seq+1).Error; err != nil {
		return 0, err
	}
	return seq, nil
}

// CreateSale menyimpan sale + item + payment dalam satu insert bersarang.
// tenant_id di-stempel dari context ke sale DAN ke tiap item/payment (RLS
// WITH CHECK menolak baris tanpa tenant_id yang cocok).
func CreateSale(ctx context.Context, tx *gorm.DB, sale *models.Sale) error {
	tid := currentTenantID(ctx)
	sale.TenantID = tid
	for i := range sale.Items {
		sale.Items[i].TenantID = tid
	}
	for i := range sale.Payments {
		sale.Payments[i].TenantID = tid
	}
	return tx.WithContext(ctx).Create(sale).Error
}

// FindSaleInTenant memuat satu sale beserta item & pembayarannya. tx opsional.
func FindSaleInTenant(ctx context.Context, tx *gorm.DB, id string, out *models.Sale) error {
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Preload("Items").
		Preload("Payments").
		First(out, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrSaleNotFound
	}
	return err
}

// FindReturnSale memuat transaksi retur (status 'returned') yang menunjuk
// origSaleID, bila ada. tx opsional. Mengembalikan ErrSaleNotFound bila belum ada.
func FindReturnSale(ctx context.Context, tx *gorm.DB, origSaleID string, out *models.Sale) error {
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("return_of_sale_id = ? AND status = 'returned'", origSaleID).
		First(out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrSaleNotFound
	}
	return err
}

// SaleFilter menampung filter daftar transaksi.
type SaleFilter struct {
	OutletID     string
	BusinessDate string // "YYYY-MM-DD"
	Status       string
	ShiftID      string
}

// ListSales mengembalikan satu halaman transaksi milik tenant konteks (tanpa
// item/pembayaran — ambil detail lewat FindSaleInTenant).
func ListSales(ctx context.Context, f SaleFilter, limit, offset int) ([]models.Sale, int64, error) {
	conds := []string{}
	args := []any{}
	if f.OutletID != "" {
		conds, args = append(conds, "outlet_id = ?"), append(args, f.OutletID)
	}
	if f.BusinessDate != "" {
		conds, args = append(conds, "business_date = ?"), append(args, f.BusinessDate)
	}
	if f.Status != "" {
		conds, args = append(conds, "status = ?"), append(args, f.Status)
	}
	if f.ShiftID != "" {
		conds, args = append(conds, "shift_id = ?"), append(args, f.ShiftID)
	}
	where := ""
	for i, c := range conds {
		if i > 0 {
			where += " AND "
		}
		where += c
	}
	return paginateTenant[models.Sale](ctx, where, args, "occurred_at DESC, id DESC", limit, offset)
}

// MarkSaleCanceled menandai sale dibatalkan (void) — status + jejak voided_*.
func MarkSaleCanceled(ctx context.Context, tx *gorm.DB, id, reason string) error {
	now := time.Now().UTC()
	uid := reqctx.UserID(ctx)
	res := scopeTenant(ctx, tenantDB(ctx, tx)).
		Model(&models.Sale{}).
		Where("id = ? AND status = 'completed'", id).
		Updates(map[string]any{
			"status":      "canceled",
			"voided_at":   now,
			"voided_by":   uid,
			"void_reason": reason,
			"updated_at":  now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrSaleNotFound // tidak ada / sudah tidak 'completed'
	}
	return nil
}

// SalesSummary adalah agregat transaksi untuk verifikasi laporan (§16 Fase 3 DoD).
type SalesSummary struct {
	SalesCount     int64 `json:"sales_count"`
	Gross          int64 `json:"gross"` // Σ subtotal (status completed)
	DiscountAmount int64 `json:"discount_amount"`
	TaxAmount      int64 `json:"tax_amount"`
	ServiceAmount  int64 `json:"service_amount"`
	Net            int64 `json:"net"` // Σ total
	CostTotal      int64 `json:"cost_total"`
	GrossProfit    int64 `json:"gross_profit"` // net - cost
	Paid           int64 `json:"paid"`         // Σ paid_amount
	ChangeTotal    int64 `json:"change_total"`
}

// SummarizeSales menghitung agregat transaksi berstatus 'completed' pada rentang
// business_date [from, to] (inklusif), opsional per outlet.
func SummarizeSales(ctx context.Context, outletID, from, to string) (SalesSummary, error) {
	q := scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.Sale{})).
		Where("status = 'completed' AND business_date BETWEEN ? AND ?", from, to)
	if outletID != "" {
		q = q.Where("outlet_id = ?", outletID)
	}
	var s SalesSummary
	err := q.Select(`
		COUNT(*)                              AS sales_count,
		COALESCE(SUM(subtotal), 0)            AS gross,
		COALESCE(SUM(discount_amount), 0)     AS discount_amount,
		COALESCE(SUM(tax_amount), 0)          AS tax_amount,
		COALESCE(SUM(service_amount), 0)      AS service_amount,
		COALESCE(SUM(total), 0)               AS net,
		COALESCE(SUM(cost_total), 0)          AS cost_total,
		COALESCE(SUM(total - cost_total), 0)  AS gross_profit,
		COALESCE(SUM(paid_amount), 0)         AS paid,
		COALESCE(SUM(change_amount), 0)       AS change_total`).
		Scan(&s).Error
	return s, err
}
