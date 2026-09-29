package repositories

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"

	"gorm.io/gorm"
)

var ErrSaleNotFound = errors.New("transaksi tidak ditemukan")

// NextReceiptSeq mengambil nomor urut struk berikutnya untuk (outlet, hari
// usaha) secara ATOMIK lewat satu UPSERT: sisipkan baris penghitung bila belum
// ada, kalau sudah ada naikkan next_seq — lalu kembalikan nilai sebelum
// dinaikkan. Bukan COUNT(*) (nomor ganda saat dua kasir bersamaan, §13.7), dan
// bukan SELECT-lalu-INSERT terpisah (dua checkout pertama-hari yang paralel
// sama-sama menyisipkan → pelanggaran primary key → checkout gagal 500).
//
// ON CONFLICT DO UPDATE mengunci baris; transaksi paralel menunggu giliran lalu
// membaca next_seq yang sudah dinaikkan — tiap pemanggil mendapat nomor unik.
func NextReceiptSeq(ctx context.Context, tx *gorm.DB, outletID string, businessDate time.Time) (int64, error) {
	tid := currentTenantID(ctx)

	var seq int64
	err := tx.WithContext(ctx).Raw(`
		INSERT INTO receipt_counters (tenant_id, outlet_id, business_date, next_seq)
		VALUES (?, ?, ?, 2)
		ON CONFLICT (tenant_id, outlet_id, business_date)
		DO UPDATE SET next_seq = receipt_counters.next_seq + 1
		RETURNING next_seq - 1
	`, tid, outletID, businessDate).Scan(&seq).Error
	if err != nil {
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
	// Search: potongan nomor nota (tanpa peduli huruf besar).
	Search string
	// Method: hanya transaksi yang punya pembayaran dengan cara ini
	// (cash | qris | credit | ...).
	Method string
	// CustomerID: hanya transaksi pelanggan ini (riwayat belanja pelanggan).
	CustomerID string
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
	if q := strings.TrimSpace(f.Search); q != "" {
		conds, args = append(conds, "receipt_no ILIKE ?"), append(args, "%"+escapeLike(q)+"%")
	}
	if f.Method != "" {
		conds = append(conds, "EXISTS (SELECT 1 FROM sale_payments sp WHERE sp.sale_id = sales.id AND sp.method = ?)")
		args = append(args, f.Method)
	}
	if f.CustomerID != "" {
		conds, args = append(conds, "customer_id = ?"), append(args, f.CustomerID)
	}
	where := ""
	for i, c := range conds {
		if i > 0 {
			where += " AND "
		}
		where += c
	}
	where, args = whereOutlet(ctx, where, args, "outlet_id")
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
	q = scopeOutlet(ctx, q, "outlet_id")
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

// AttachSaleLines memuat item & pembayaran untuk SATU HALAMAN transaksi dalam
// dua kueri (bukan satu per baris) — riwayat menampilkan isi belanja dan cara
// bayarnya langsung di daftar.
func AttachSaleLines(ctx context.Context, sales []models.Sale) error {
	if len(sales) == 0 {
		return nil
	}
	ids := make([]string, len(sales))
	idx := make(map[string]int, len(sales))
	for i, s := range sales {
		ids[i], idx[s.ID] = s.ID, i
	}
	var items []models.SaleItem
	if err := scopeTenant(ctx, tenantDB(ctx, nil)).Where("sale_id IN ?", ids).
		Order("created_at, id").Find(&items).Error; err != nil {
		return err
	}
	var pays []models.SalePayment
	if err := scopeTenant(ctx, tenantDB(ctx, nil)).Where("sale_id IN ?", ids).
		Order("paid_at, id").Find(&pays).Error; err != nil {
		return err
	}
	for _, it := range items {
		sales[idx[it.SaleID]].Items = append(sales[idx[it.SaleID]].Items, it)
	}
	for _, p := range pays {
		sales[idx[p.SaleID]].Payments = append(sales[idx[p.SaleID]].Payments, p)
	}
	return nil
}

// SaleNames: nama pelanggan & kasir (pembuat) untuk satu halaman transaksi.
func SaleNames(ctx context.Context, sales []models.Sale) (pelanggan, kasir map[string]string, err error) {
	pelanggan = map[string]string{}
	var custIDs, userIDs []string
	for _, s := range sales {
		if s.CustomerID != nil {
			custIDs = append(custIDs, *s.CustomerID)
		}
		if s.CreatedBy != "" {
			userIDs = append(userIDs, s.CreatedBy)
		}
	}
	type baris struct{ ID, Name string }
	if len(custIDs) > 0 {
		var rows []baris
		if err = scopeTenant(ctx, tenantDB(ctx, nil).Table("customers")).
			Where("id IN ?", custIDs).Select("id, name").Scan(&rows).Error; err != nil {
			return
		}
		for _, r := range rows {
			pelanggan[r.ID] = r.Name
		}
	}
	kasir, err = UserNames(ctx, userIDs)
	return
}

// UserNames: nama pengguna (kasir) per id, satu kueri.
func UserNames(ctx context.Context, ids []string) (map[string]string, error) {
	nama := map[string]string{}
	if len(ids) == 0 {
		return nama, nil
	}
	var rows []struct{ ID, Name string }
	if err := scopeTenant(ctx, tenantDB(ctx, nil).Table("users")).
		Where("id IN ?", ids).Select("id, name").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		nama[r.ID] = r.Name
	}
	return nama, nil
}

// SaleReturnLinks menghubungkan retur dengan penjualan asalnya, dua arah:
// asal[idRetur] = nomor nota penjualan yang diretur; diretur[idAsal] = nomor
// nota returnya. Tanpa ini baris retur di riwayat tidak menyebut nota mana yang
// dikembalikan, dan penjualan yang sudah diretur tampak "Lunas" biasa.
func SaleReturnLinks(ctx context.Context, sales []models.Sale) (asal, diretur map[string]string, err error) {
	asal, diretur = map[string]string{}, map[string]string{}
	if len(sales) == 0 {
		return
	}
	ids := make([]string, 0, len(sales))
	var asalIDs []string
	for _, s := range sales {
		ids = append(ids, s.ID)
		if s.ReturnOfSaleID != nil {
			asalIDs = append(asalIDs, *s.ReturnOfSaleID)
		}
	}
	if len(asalIDs) > 0 {
		var rows []struct{ ID, ReceiptNo string }
		if err = scopeTenant(ctx, tenantDB(ctx, nil).Table("sales")).
			Where("id IN ?", asalIDs).Select("id, receipt_no").Scan(&rows).Error; err != nil {
			return
		}
		nota := make(map[string]string, len(rows))
		for _, r := range rows {
			nota[r.ID] = r.ReceiptNo
		}
		for _, s := range sales {
			if s.ReturnOfSaleID != nil {
				asal[s.ID] = nota[*s.ReturnOfSaleID]
			}
		}
	}
	var rets []struct{ ReturnOfSaleID, ReceiptNo string }
	if err = scopeTenant(ctx, tenantDB(ctx, nil).Table("sales")).
		Where("return_of_sale_id IN ? AND status = 'returned'", ids).
		Select("return_of_sale_id, receipt_no").Scan(&rets).Error; err != nil {
		return
	}
	for _, r := range rets {
		diretur[r.ReturnOfSaleID] = r.ReceiptNo
	}
	return
}

// SaleScope membatasi ringkasan penjualan: satu hari usaha, satu shift, atau
// keduanya. Kosong = tidak dibatasi pada dimensi itu.
type SaleScope struct {
	OutletID     string
	BusinessDate string // "YYYY-MM-DD"
	ShiftID      string
}

func (s SaleScope) apply(ctx context.Context, q *gorm.DB, kolom string) *gorm.DB {
	if s.OutletID != "" {
		q = q.Where(kolom+"outlet_id = ?", s.OutletID)
	}
	if s.BusinessDate != "" {
		q = q.Where(kolom+"business_date = ?", s.BusinessDate)
	}
	if s.ShiftID != "" {
		q = q.Where(kolom+"shift_id = ?", s.ShiftID)
	}
	return scopeOutlet(ctx, q, kolom+"outlet_id")
}

// SaleDayCounts: jumlah & nilai transaksi per status dalam satu SaleScope.
type SaleDayCounts struct {
	SalesCount    int64
	SalesTotal    int64
	ReturnsCount  int64
	ReturnsTotal  int64 // bernilai negatif (baris retur)
	CanceledCount int64
	CanceledTotal int64
}

// CountSales merangkum transaksi dalam satu SaleScope per status.
func CountSales(ctx context.Context, sc SaleScope) (SaleDayCounts, error) {
	q := sc.apply(ctx, scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.Sale{})), "")
	var c SaleDayCounts
	err := q.Select(`
		COUNT(*) FILTER (WHERE status = 'completed')                 AS sales_count,
		COALESCE(SUM(total) FILTER (WHERE status = 'completed'), 0)  AS sales_total,
		COUNT(*) FILTER (WHERE status = 'returned')                  AS returns_count,
		COALESCE(SUM(total) FILTER (WHERE status = 'returned'), 0)   AS returns_total,
		COUNT(*) FILTER (WHERE status = 'canceled')                  AS canceled_count,
		COALESCE(SUM(total) FILTER (WHERE status = 'canceled'), 0)   AS canceled_total`).
		Scan(&c).Error
	return c, err
}

// MethodTotal: uang masuk lewat satu cara bayar.
type MethodTotal struct {
	Method     string
	SalesCount int64
	Amount     int64
}

// SalesByMethod: uang masuk per cara bayar dari penjualan 'completed' dalam
// satu SaleScope, terbesar dulu. Rumusnya sama dengan laporan
// (SalesByPaymentMethod): kembalian dikurangkan dari baris tunai, dihitung
// terpisah supaya transaksi dengan beberapa baris pembayaran tidak
// menguranginya berkali-kali.
func SalesByMethod(ctx context.Context, sc SaleScope) ([]MethodTotal, error) {
	q := tenantDB(ctx, nil).
		Table("sale_payments sp").
		Joins("JOIN sales s ON s.tenant_id = sp.tenant_id AND s.id = sp.sale_id").
		Where("s.tenant_id = ? AND s.status = 'completed'", currentTenantID(ctx))
	var rows []MethodTotal
	if err := sc.apply(ctx, q, "s.").Select(`
		sp.method                   AS method,
		COUNT(DISTINCT sp.sale_id)  AS sales_count,
		COALESCE(SUM(sp.amount), 0) AS amount`).
		Group("sp.method").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	var kembalian int64
	kq := sc.apply(ctx, scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.Sale{})), "").
		Where("status = 'completed'")
	if err := kq.Select("COALESCE(SUM(change_amount), 0)").Scan(&kembalian).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].Method == "cash" {
			rows[i].Amount -= kembalian
		}
	}
	sort.SliceStable(rows, func(a, b int) bool { return rows[a].Amount > rows[b].Amount })
	return rows, nil
}
