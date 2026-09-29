package repositories

import (
	"context"
	"errors"
	"time"

	"candra/backend-api/models"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

var ErrCustomerNotFound = errors.New("pelanggan tidak ditemukan")

func customerSearch(search string) (string, []any) {
	if search == "" {
		return "", nil
	}
	p := "%" + escapeLike(search) + "%"
	return "name ILIKE ? OR phone ILIKE ? OR code ILIKE ?", []any{p, p, p}
}

// ListCustomers mengambil satu halaman pelanggan milik tenant konteks.
func ListCustomers(ctx context.Context, search string, limit, offset int) ([]models.Customer, int64, error) {
	where, args := customerSearch(search)
	return paginateTenant[models.Customer](ctx, where, args, "name ASC, id ASC", limit, offset)
}

// FindCustomerInTenant memuat satu pelanggan milik tenant konteks. tx opsional.
//
// TANPA lapis visibilitas — dipakai jalur transaksi internal (checkout, dokumen
// CRM) yang boleh menunjuk pelanggan mana pun di tenant. Untuk endpoint yang
// menghadap pengguna, pakai FindCustomerVisible.
func FindCustomerInTenant(ctx context.Context, tx *gorm.DB, id string, out *models.Customer) error {
	err := firstTenant(ctx, tx, id, out)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrCustomerNotFound
	}
	return err
}

// ListCustomersVisible seperti ListCustomers tetapi juga menerapkan lapis 3
// (visibilitas kepemilikan, §6): sales hanya melihat pelanggan miliknya + yang
// belum ber-owner. Dipakai GET /customers.
func ListCustomersVisible(ctx context.Context, search string, limit, offset int) ([]models.Customer, int64, error) {
	where, args := customerSearch(search)
	build := func() *gorm.DB {
		q := scopeVisibility(ctx, scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.Customer{})), "customers")
		if where != "" {
			q = q.Where(where, args...)
		}
		return q
	}
	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []models.Customer{}, 0, nil
	}
	rows := make([]models.Customer, 0, limit)
	err := build().Order("name ASC, id ASC").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

// FindCustomerVisible memuat satu pelanggan yang TERLIHAT oleh user konteks
// (lapis 1 + lapis 3). Dipakai GET /customers/:id.
func FindCustomerVisible(ctx context.Context, id string, out *models.Customer) error {
	err := scopeVisibility(ctx, scopeTenant(ctx, tenantDB(ctx, nil)), "customers").
		First(out, "customers.id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrCustomerNotFound
	}
	return err
}

// CreateCustomer menyimpan pelanggan baru (TenantID diisi pemanggil). tx opsional.
func CreateCustomer(ctx context.Context, tx *gorm.DB, row *models.Customer) error {
	return createTenant(ctx, tx, row)
}

// UpdateCustomer menyimpan perubahan atribut pelanggan. tx opsional.
func UpdateCustomer(ctx context.Context, tx *gorm.DB, row *models.Customer) error {
	err := updateTenantColumns(ctx, tx, row,
		"code", "name", "phone", "email", "address", "type",
		"price_list_id", "owner_id", "credit_limit", "credit_term_days", "latitude", "longitude", "note")
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrCustomerNotFound
	}
	return err
}

// DeleteCustomer menandai pelanggan terhapus (soft delete). tx opsional.
func DeleteCustomer(ctx context.Context, tx *gorm.DB, id string) error {
	err := softDeleteTenant[models.Customer](ctx, tx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrCustomerNotFound
	}
	return err
}

// OutstandingReceivableTotal mengembalikan total sisa piutang seorang pelanggan
// (untuk cek batas kredit sebelum menerima kasbon baru). tx opsional.
func OutstandingReceivableTotal(ctx context.Context, tx *gorm.DB, customerID string) (int64, error) {
	var total int64
	err := scopeTenant(ctx, tenantDB(ctx, tx).Model(&models.Receivable{})).
		Where("customer_id = ? AND status IN ?", customerID, []string{"open", "partial"}).
		Select("COALESCE(SUM(amount - paid_amount), 0)").
		Scan(&total).Error
	return total, err
}

// CustomerSpendRow adalah ringkasan belanja satu pelanggan.
type CustomerSpendRow struct {
	CustomerID string
	VisitCount int64      // jumlah nota 'completed' (retur bukan kedatangan)
	TotalSpent int64      // Σ total nota 'completed' + 'returned' (retur negatif)
	LastVisit  *time.Time // occurred_at nota 'completed' terakhir
}

// CustomerSpending merangkum belanja pelanggan-pelanggan `ids`: berapa kali
// datang, total belanja bersih, dan kapan terakhir datang.
//
// Aturannya SAMA dengan laporan (lihat RefreshDailySummary): uang dijumlahkan
// atas 'completed' DAN 'returned' — baris retur bernilai negatif sehingga
// mengurangi totalnya — sedangkan kedatangan hanya menghitung 'completed'.
// Void ('canceled') tidak ikut sama sekali.
//
// Mengikuti lingkup toko pengguna (scopeOutlet): staf cabang A melihat belanja
// pelanggan DI CABANG A, bukan omzet cabang lain yang tidak boleh ia lihat.
// Satu query untuk satu halaman daftar, bukan satu query per pelanggan.
func CustomerSpending(ctx context.Context, ids []string) (map[string]CustomerSpendRow, error) {
	out := make(map[string]CustomerSpendRow, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	q := tenantDB(ctx, nil).Table("sales").
		Where("tenant_id = ? AND customer_id IN ? AND status IN ('completed', 'returned')",
			currentTenantID(ctx), ids)
	q = scopeOutlet(ctx, q, "outlet_id")
	var rows []CustomerSpendRow
	err := q.Select(`
		customer_id,
		COUNT(*) FILTER (WHERE status = 'completed')           AS visit_count,
		COALESCE(SUM(total), 0)                                AS total_spent,
		MAX(occurred_at) FILTER (WHERE status = 'completed')   AS last_visit`).
		Group("customer_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.CustomerID] = r
	}
	return out, nil
}

// CustomerOutstanding: sisa kasbon seorang pelanggan, dan bagiannya yang
// sudah lewat jatuh tempo.
type CustomerOutstanding struct {
	CustomerID  string
	Outstanding int64
	Overdue     int64
}

// CustomerReceivableOutstanding menjumlahkan sisa kasbon BELUM LUNAS per
// pelanggan — status 'open' DAN 'partial' (sudah dicicil sebagian). Dulu hanya
// 'open', sehingga kasbon yang baru dicicil sekali lenyap dari sisa kasbon
// pelanggannya. Piutang tidak berlapis toko — kasbon melekat pada pelanggan,
// bukan pada cabang tempat ia berutang.
func CustomerReceivableOutstanding(ctx context.Context, ids []string, hariIni time.Time) (map[string]CustomerOutstanding, error) {
	out := make(map[string]CustomerOutstanding, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []CustomerOutstanding
	err := tenantDB(ctx, nil).Table("receivables").
		Where("tenant_id = ? AND customer_id IN ? AND status IN ('open', 'partial')", currentTenantID(ctx), ids).
		Select(`customer_id, COALESCE(SUM(amount - paid_amount), 0)::bigint AS outstanding,
			COALESCE(SUM(amount - paid_amount) FILTER (WHERE due_date < ?::date), 0)::bigint AS overdue`,
			hariIni.Format("2006-01-02")).
		Group("customer_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.CustomerID] = r
	}
	return out, nil
}

// CustomerTopProduct: barang yang sering dibeli seorang pelanggan.
type CustomerTopProduct struct {
	ProductID   string
	ProductName string
	UnitName    string // satuan dasar
	Times       int64  // berapa nota memuatnya
	Qty         decimal.Decimal
	LastAt      time.Time
}

// CustomerTopProducts: `limit` barang yang paling sering dibeli pelanggan
// (nota 'completed'; varian digabung ke barangnya; jumlah dalam satuan dasar),
// di toko yang boleh dilihat pengguna — sama dengan angka belanjanya.
func CustomerTopProducts(ctx context.Context, customerID string, limit int) ([]CustomerTopProduct, error) {
	q := tenantDB(ctx, nil).Table("sale_items si").
		Joins("JOIN sales s ON s.tenant_id = si.tenant_id AND s.id = si.sale_id").
		Joins("LEFT JOIN products p ON p.tenant_id = si.tenant_id AND p.id = si.product_id").
		Joins("LEFT JOIN units un ON un.tenant_id = p.tenant_id AND un.id = p.unit_id").
		Where("si.tenant_id = ? AND s.customer_id = ? AND s.status = 'completed'", currentTenantID(ctx), customerID)
	q = scopeOutlet(ctx, q, "s.outlet_id")
	var rows []CustomerTopProduct
	err := q.Select(`si.product_id,
			COALESCE(MAX(p.name), (array_agg(si.product_name ORDER BY s.occurred_at DESC))[1]) AS product_name,
			COALESCE(MAX(un.name), '') AS unit_name,
			COUNT(DISTINCT s.id) AS times,
			SUM(si.qty * si.unit_conversion) AS qty,
			MAX(s.occurred_at) AS last_at`).
		Group("si.product_id").
		Order("times DESC, last_at DESC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}
