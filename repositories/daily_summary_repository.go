package repositories

import (
	"context"
	"sort"
	"time"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

// Repositori agregat laporan harian (§5.14). Dua jalur pemeliharaan:
//
//   - ApplySaleToSummary  — dipanggil di dalam transaksi checkout, menambah
//     kontribusi satu penjualan ke baris ringkasannya secara inkremental
//     (UPSERT). Cepat: satu baris tersentuh.
//   - RefreshDailySummary — menghitung ulang SELURUH baris kanal untuk satu
//     (outlet, hari usaha) dari tabel `sales`. Dipakai void/retur dan perintah
//     rebuild manual. Selalu benar karena membaca ulang sumber kebenaran.
//
// Semua jalur BACA (dashboard, laporan) memakai tenantDB tanpa GUC, jadi tiap
// query WAJIB memfilter tenant_id secara eksplisit (lapis 1 isolasi, §6).

// SummaryAggregate menampung delapan besaran ringkasan. Dipakai baik untuk total
// keseluruhan maupun—lewat penyematan—untuk baris terkelompok.
type SummaryAggregate struct {
	SalesCount     int64 `json:"sales_count"`
	GrossAmount    int64 `json:"gross_amount"`
	DiscountAmount int64 `json:"discount_amount"`
	TaxAmount      int64 `json:"tax_amount"`
	NetAmount      int64 `json:"net_amount"`
	CostAmount     int64 `json:"cost_amount"`
	FeeAmount      int64 `json:"fee_amount"`
	GrossProfit    int64 `json:"gross_profit"`
}

// SummaryGroupRow adalah satu baris agregat berlabel kunci (tanggal atau kanal).
type SummaryGroupRow struct {
	Key string `json:"key"`
	SummaryAggregate
}

// summarySelectCols dipakai ulang oleh query agregasi tabel ringkasan.
const summarySelectCols = `
	COALESCE(SUM(sales_count), 0)     AS sales_count,
	COALESCE(SUM(gross_amount), 0)    AS gross_amount,
	COALESCE(SUM(discount_amount), 0) AS discount_amount,
	COALESCE(SUM(tax_amount), 0)      AS tax_amount,
	COALESCE(SUM(net_amount), 0)      AS net_amount,
	COALESCE(SUM(cost_amount), 0)     AS cost_amount,
	COALESCE(SUM(fee_amount), 0)      AS fee_amount,
	COALESCE(SUM(gross_profit), 0)    AS gross_profit`

// ApplySaleToSummary menambah kontribusi satu penjualan 'completed' ke baris
// ringkasan (tenant, outlet, business_date, channel). Idempotensi transaksi
// dijamin oleh Idempotency-Key checkout di lapisan atas — fungsi ini tidak
// dipanggil dua kali untuk penjualan yang sama.
//
// Dipanggil DI DALAM transaksi checkout (GUC app.tenant_id aktif).
func ApplySaleToSummary(ctx context.Context, tx *gorm.DB, sale *models.Sale) error {
	channel := ""
	if sale.ChannelID != nil {
		channel = *sale.ChannelID
	}
	var fee int64
	for _, p := range sale.Payments {
		fee += p.FeeAmount
	}
	profit := sale.Total - sale.CostTotal - fee

	return tx.WithContext(ctx).Exec(`
		INSERT INTO daily_sales_summaries
			(tenant_id, outlet_id, business_date, channel_id,
			 sales_count, gross_amount, discount_amount, tax_amount,
			 net_amount, cost_amount, fee_amount, gross_profit, updated_at)
		VALUES (?, ?, ?, ?, 1, ?, ?, ?, ?, ?, ?, ?, now())
		ON CONFLICT (tenant_id, outlet_id, business_date, channel_id) DO UPDATE SET
			sales_count     = daily_sales_summaries.sales_count     + 1,
			gross_amount    = daily_sales_summaries.gross_amount    + EXCLUDED.gross_amount,
			discount_amount = daily_sales_summaries.discount_amount + EXCLUDED.discount_amount,
			tax_amount      = daily_sales_summaries.tax_amount      + EXCLUDED.tax_amount,
			net_amount      = daily_sales_summaries.net_amount      + EXCLUDED.net_amount,
			cost_amount     = daily_sales_summaries.cost_amount     + EXCLUDED.cost_amount,
			fee_amount      = daily_sales_summaries.fee_amount      + EXCLUDED.fee_amount,
			gross_profit    = daily_sales_summaries.gross_profit    + EXCLUDED.gross_profit,
			updated_at      = now()`,
		sale.TenantID, sale.OutletID, sale.BusinessDate, channel,
		sale.Subtotal, sale.DiscountAmount, sale.TaxAmount,
		sale.Total, sale.CostTotal, fee, profit,
	).Error
}

// RefreshDailySummary membangun ulang baris ringkasan satu (outlet, hari usaha)
// dari tabel `sales` + `sale_payments`. Hapus dulu baris lama untuk hari itu,
// lalu isi ulang dari agregat per kanal.
//
// Nilai uang dijumlahkan atas status 'completed' DAN 'returned' — baris retur
// bernilai negatif sehingga otomatis mengurangi hari terjadinya retur; jumlah
// transaksi hanya menghitung yang 'completed'. Void ('canceled') tidak ikut,
// jadi menghitung ulang setelah void langsung menghapus kontribusinya.
//
// Dipanggil DI DALAM transaksi (GUC app.tenant_id aktif). DELETE dan INSERT
// dijalankan sebagai dua statement terpisah (protokol extended pgx tidak
// mengizinkan banyak statement dalam satu Exec).
func RefreshDailySummary(ctx context.Context, tx *gorm.DB, outletID string, businessDate time.Time) error {
	tid := currentTenantID(ctx)
	day := businessDate.Format("2006-01-02")

	if err := tx.WithContext(ctx).Exec(`
		DELETE FROM daily_sales_summaries
		WHERE tenant_id = ? AND outlet_id = ? AND business_date = ?`,
		tid, outletID, day,
	).Error; err != nil {
		return err
	}

	return tx.WithContext(ctx).Exec(`
		INSERT INTO daily_sales_summaries
			(tenant_id, outlet_id, business_date, channel_id,
			 sales_count, gross_amount, discount_amount, tax_amount,
			 net_amount, cost_amount, fee_amount, gross_profit, updated_at)
		SELECT
			s.tenant_id, s.outlet_id, s.business_date, COALESCE(s.channel_id, ''),
			COUNT(*) FILTER (WHERE s.status = 'completed'),
			COALESCE(SUM(s.subtotal), 0),
			COALESCE(SUM(s.discount_amount), 0),
			COALESCE(SUM(s.tax_amount), 0),
			COALESCE(SUM(s.total), 0),
			COALESCE(SUM(s.cost_total), 0),
			COALESCE(SUM(f.fee), 0),
			COALESCE(SUM(s.total), 0) - COALESCE(SUM(s.cost_total), 0) - COALESCE(SUM(f.fee), 0),
			now()
		FROM sales s
		LEFT JOIN (
			SELECT sale_id, SUM(fee_amount) AS fee
			FROM sale_payments
			WHERE tenant_id = ?
			GROUP BY sale_id
		) f ON f.sale_id = s.id
		WHERE s.tenant_id = ? AND s.outlet_id = ? AND s.business_date = ?
		  AND s.status IN ('completed', 'returned')
		GROUP BY s.tenant_id, s.outlet_id, s.business_date, COALESCE(s.channel_id, '')`,
		tid, tid, outletID, day,
	).Error
}

// OutletIDsWithSales mengembalikan id outlet unik yang punya transaksi pada
// rentang business_date [from, to] — dipakai rebuild "semua outlet".
func OutletIDsWithSales(ctx context.Context, from, to string) ([]string, error) {
	tid := currentTenantID(ctx)
	var ids []string
	err := tenantDB(ctx, nil).
		Model(&models.Sale{}).
		Where("tenant_id = ? AND business_date BETWEEN ? AND ?", tid, from, to).
		Distinct().
		Pluck("outlet_id", &ids).Error
	return ids, err
}

// SummaryTotalsBetween menjumlahkan tabel ringkasan pada rentang [from, to],
// opsional per outlet. Inti jalur cepat dashboard & laporan laba.
func SummaryTotalsBetween(ctx context.Context, outletID, from, to string) (SummaryAggregate, error) {
	var agg SummaryAggregate
	err := summaryScope(ctx, outletID, from, to).
		Select(summarySelectCols).
		Scan(&agg).Error
	return agg, err
}

// SummaryByChannel memecah tabel ringkasan per channel_id pada rentang.
func SummaryByChannel(ctx context.Context, outletID, from, to string) ([]SummaryGroupRow, error) {
	var rows []SummaryGroupRow
	err := summaryScope(ctx, outletID, from, to).
		Select("channel_id AS key, " + summarySelectCols).
		Group("channel_id").
		Order("net_amount DESC").
		Scan(&rows).Error
	return rows, err
}

// SummaryByDate memecah tabel ringkasan per business_date pada rentang.
func SummaryByDate(ctx context.Context, outletID, from, to string) ([]SummaryGroupRow, error) {
	var rows []SummaryGroupRow
	err := summaryScope(ctx, outletID, from, to).
		Select("to_char(business_date, 'YYYY-MM-DD') AS key, " + summarySelectCols).
		Group("business_date").
		Order("business_date").
		Scan(&rows).Error
	return rows, err
}

// summaryScope membangun query dasar bertenant atas daily_sales_summaries.
func summaryScope(ctx context.Context, outletID, from, to string) *gorm.DB {
	q := tenantDB(ctx, nil).
		Model(&models.DailySalesSummary{}).
		Where("tenant_id = ? AND business_date BETWEEN ? AND ?", currentTenantID(ctx), from, to)
	if outletID != "" {
		q = q.Where("outlet_id = ?", outletID)
	}
	return q
}

// SalesByCashier mengelompokkan penjualan 'completed' per kasir (created_by),
// dibaca langsung dari `sales` karena kasir bukan dimensi tabel ringkasan.
func SalesByCashier(ctx context.Context, outletID, from, to string) ([]SummaryGroupRow, error) {
	tid := currentTenantID(ctx)
	q := tenantDB(ctx, nil).
		Table("sales s").
		Joins("JOIN users u ON u.id = s.created_by AND u.tenant_id = ?", tid).
		Where("s.tenant_id = ? AND s.business_date BETWEEN ? AND ? AND s.status = 'completed'", tid, from, to)
	if outletID != "" {
		q = q.Where("s.outlet_id = ?", outletID)
	}
	var rows []SummaryGroupRow
	err := q.Select(`
		u.name AS key,
		COUNT(*)                                  AS sales_count,
		COALESCE(SUM(s.subtotal), 0)              AS gross_amount,
		COALESCE(SUM(s.discount_amount), 0)       AS discount_amount,
		COALESCE(SUM(s.tax_amount), 0)            AS tax_amount,
		COALESCE(SUM(s.total), 0)                 AS net_amount,
		COALESCE(SUM(s.cost_total), 0)            AS cost_amount,
		0                                         AS fee_amount,
		COALESCE(SUM(s.total - s.cost_total), 0)  AS gross_profit`).
		Group("u.name").
		Order("net_amount DESC").
		Scan(&rows).Error
	return rows, err
}

// SalesByPaymentMethod mengelompokkan tender penjualan 'completed' per metode.
// net_amount di sini berarti "nominal yang benar-benar masuk lewat metode itu";
// kolom modal & laba tidak berlaku untuk dimensi ini.
//
// KEMBALIAN DIKURANGKAN dari baris 'cash'. Yang tercatat di sale_payments adalah
// uang yang DISERAHKAN pembeli, jadi tanpa pengurangan ini baris Tunai selalu
// lebih besar dari penjualannya sendiri — pemilik melihat rincian yang tidak
// pernah berjumlah sama dengan totalnya, lalu berhenti percaya pada laporannya.
// Kembalian selalu diambil dari laci tunai, apa pun metode pembayarannya.
func SalesByPaymentMethod(ctx context.Context, outletID, from, to string) ([]SummaryGroupRow, error) {
	tid := currentTenantID(ctx)
	q := tenantDB(ctx, nil).
		Table("sale_payments sp").
		Joins("JOIN sales s ON s.id = sp.sale_id").
		Where("s.tenant_id = ? AND s.business_date BETWEEN ? AND ? AND s.status = 'completed'", tid, from, to)
	if outletID != "" {
		q = q.Where("s.outlet_id = ?", outletID)
	}
	var rows []SummaryGroupRow
	err := q.Select(`
		sp.method                          AS key,
		COUNT(DISTINCT sp.sale_id)         AS sales_count,
		0                                  AS gross_amount,
		0                                  AS discount_amount,
		0                                  AS tax_amount,
		COALESCE(SUM(sp.amount), 0)        AS net_amount,
		0                                  AS cost_amount,
		COALESCE(SUM(sp.fee_amount), 0)    AS fee_amount,
		0                                  AS gross_profit`).
		Group("sp.method").
		Order("net_amount DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	// Kembalian dihitung terpisah (bukan lewat join di atas) supaya transaksi
	// dengan lebih dari satu baris pembayaran tidak menghitungnya berkali-kali.
	kq := scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.Sale{})).
		Where("business_date BETWEEN ? AND ? AND status = 'completed'", from, to)
	if outletID != "" {
		kq = kq.Where("outlet_id = ?", outletID)
	}
	var kembalian int64
	if err := kq.Select("COALESCE(SUM(change_amount), 0)").Scan(&kembalian).Error; err != nil {
		return nil, err
	}

	if kembalian > 0 {
		for i := range rows {
			if rows[i].Key == "cash" {
				rows[i].NetAmount -= kembalian
				break
			}
		}
		// Urutan bisa berubah setelah pengurangan; laporan menampilkan yang
		// terbesar lebih dulu.
		sort.SliceStable(rows, func(a, b int) bool {
			return rows[a].NetAmount > rows[b].NetAmount
		})
	}

	return rows, nil
}
