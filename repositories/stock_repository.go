package repositories

import (
	"context"
	"sort"
	"strings"
	"time"

	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StockKey mengidentifikasi satu saldo stok.
type StockKey struct {
	OutletID, ProductID, VariantID string
}

// LockStocks mengunci (SELECT ... FOR UPDATE) baris stok untuk pasangan
// (outlet, product) yang diberikan, DIURUT product_id MENAIK agar urutan
// penguncian global konsisten dan tidak terjadi deadlock (§7, §13.1 langkah 2).
//
// Baris yang belum ada TIDAK dibuat di sini — pemanggil menganggap saldo 0.
// Mengembalikan peta dari (outlet|product|variant) ke Stock.
func LockStocks(ctx context.Context, tx *gorm.DB, outletID string, productIDsSortedAsc []string) (map[StockKey]models.Stock, error) {
	out := map[StockKey]models.Stock{}
	if len(productIDsSortedAsc) == 0 {
		return out, nil
	}

	var rows []models.Stock
	err := tx.WithContext(ctx).
		Clauses(lockForUpdate()).
		Where("tenant_id = ? AND outlet_id = ? AND product_id IN ?",
			reqctx.TenantID(ctx), outletID, productIDsSortedAsc).
		Order("product_id ASC, variant_id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[StockKey{r.OutletID, r.ProductID, r.VariantID}] = r
	}
	return out, nil
}

// EnsureStockRows memastikan baris saldo `stocks` ADA untuk setiap key (dibuat
// dengan qty 0 bila belum), SEBELUM LockStocks dipanggil. Mengembalikan key
// yang baru dibuat oleh panggilan ini.
//
// Kenapa perlu: SELECT ... FOR UPDATE hanya bisa mengunci baris yang sudah
// ada. Untuk barang yang belum pernah bergerak, dua transaksi bersamaan
// sama-sama "mengunci" nol baris, sama-sama menganggap saldo awal 0, lalu
// UpsertStockQty yang kedua menimpa hasil yang pertama — 12 pembelian
// bersamaan pernah berakhir dengan stok 1. INSERT ... ON CONFLICT DO NOTHING
// menunggu transaksi lain yang sedang menyisipkan key yang sama selesai,
// sehingga LockStocks sesudahnya selalu menemukan baris untuk dikunci dan
// membaca saldo yang sudah di-commit.
//
// Disisipkan URUT (product_id, variant_id) — urutan yang sama dengan
// LockStocks — supaya dua transaksi tidak saling menunggu dalam lingkaran.
func EnsureStockRows(ctx context.Context, tx *gorm.DB, outletID string, keys []StockKey) (map[StockKey]bool, error) {
	created := map[StockKey]bool{}
	if len(keys) == 0 {
		return created, nil
	}
	uniq := make(map[StockKey]struct{}, len(keys))
	sorted := make([]StockKey, 0, len(keys))
	for _, k := range keys {
		k.OutletID = outletID
		if _, ok := uniq[k]; ok {
			continue
		}
		uniq[k] = struct{}{}
		sorted = append(sorted, k)
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].ProductID != sorted[j].ProductID {
			return sorted[i].ProductID < sorted[j].ProductID
		}
		return sorted[i].VariantID < sorted[j].VariantID
	})

	tid := reqctx.TenantID(ctx)
	var sb strings.Builder
	args := make([]any, 0, len(sorted)*4)
	sb.WriteString("INSERT INTO stocks (tenant_id, outlet_id, product_id, variant_id) VALUES ")
	for i, k := range sorted {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("(?, ?, ?, ?)")
		args = append(args, tid, outletID, k.ProductID, k.VariantID)
	}
	sb.WriteString(" ON CONFLICT (tenant_id, outlet_id, product_id, variant_id) DO NOTHING" +
		" RETURNING product_id, variant_id")

	var rows []struct {
		ProductID string
		VariantID string
	}
	if err := tx.WithContext(ctx).Raw(sb.String(), args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		created[StockKey{OutletID: outletID, ProductID: r.ProductID, VariantID: r.VariantID}] = true
	}
	return created, nil
}

// UpsertStockQty menyetel saldo cache stok ke qty (hasil hitung dari
// balance_after gerakan terakhir). ON CONFLICT pada PK komposit → qty ditimpa,
// updated_at diperbarui ke waktu server.
func UpsertStockQty(ctx context.Context, tx *gorm.DB, outletID, productID, variantID string, qty decimal.Decimal) error {
	row := models.Stock{
		TenantID:  reqctx.TenantID(ctx),
		OutletID:  outletID,
		ProductID: productID,
		VariantID: variantID,
		Qty:       qty,
	}
	return tx.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "tenant_id"}, {Name: "outlet_id"}, {Name: "product_id"}, {Name: "variant_id"},
			},
			DoUpdates: clause.Assignments(map[string]any{
				"qty":        gorm.Expr("EXCLUDED.qty"),
				"updated_at": gorm.Expr("now()"),
			}),
		}).
		Create(&row).Error
}

// RecordMovements menyisipkan gerakan stok (append-only). tenant_id di-stempel
// dari context — pemanggil (service) tidak perlu mengisinya di tiap baris.
func RecordMovements(ctx context.Context, tx *gorm.DB, moves []models.StockMovement) error {
	if len(moves) == 0 {
		return nil
	}
	tid := currentTenantID(ctx)
	for i := range moves {
		moves[i].TenantID = tid
	}
	return tx.WithContext(ctx).Create(&moves).Error
}

// StockQtyMap mengembalikan saldo cache banyak barang sekaligus untuk satu
// outlet, dalam SATU query (bukan CurrentStockQty per barang). Key yang tidak
// punya baris tidak ada di peta — pemanggil menganggapnya 0. tx opsional.
func StockQtyMap(ctx context.Context, tx *gorm.DB, outletID string, productIDs []string) (map[StockKey]decimal.Decimal, error) {
	out := map[StockKey]decimal.Decimal{}
	if len(productIDs) == 0 {
		return out, nil
	}
	var rows []models.Stock
	if err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("outlet_id = ? AND product_id IN ?", outletID, productIDs).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[StockKey{OutletID: r.OutletID, ProductID: r.ProductID, VariantID: r.VariantID}] = r.Qty
	}
	return out, nil
}

// CurrentStockQty mengembalikan saldo cache untuk satu (outlet, product, variant),
// 0 bila belum ada baris. tx opsional.
func CurrentStockQty(ctx context.Context, tx *gorm.DB, outletID, productID, variantID string) (decimal.Decimal, error) {
	var row models.Stock
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("outlet_id = ? AND product_id = ? AND variant_id = ?", outletID, productID, variantID).
		First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return decimal.Zero, nil
	}
	if err != nil {
		return decimal.Zero, err
	}
	return row.Qty, nil
}

// StockFilter adalah penyaring daftar saldo stok.
type StockFilter struct {
	OutletID string
	LowOnly  bool // hanya yang qty ≤ min_stock
	// Search mencocokkan nama, SKU, atau barcode barang (tanpa peduli huruf
	// besar). Dulu tidak ada: halaman Stok hanya menerima 100 baris per
	// halaman, jadi mencari di sisi klien diam-diam melewatkan sisanya.
	Search string
	// ProductIDs membatasi ke barang tertentu — daftar Barang memakainya untuk
	// kolom stok halaman yang sedang tampil saja.
	ProductIDs []string
	// Status menyaring satu keadaan — batasnya sama dengan StockSummary:
	// safe | low | out | negative | idle. Kosong = semua.
	Status string
	// Sort: name (bawaan) | urgent | value | sold. Lihat stockOrder.
	Sort string
}

// Keadaan & urutan yang dikenali ListStocks. Nilai lain ditolak controller
// (422), bukan diam-diam diabaikan.
var (
	StockStatuses = map[string]bool{"safe": true, "low": true, "out": true, "negative": true, "idle": true}
	StockSorts    = map[string]bool{"name": true, "urgent": true, "value": true, "sold": true}
)

// Jendela "laku per hari" dan batas "tidak laku". 30 hari: cukup panjang
// untuk meratakan hari ramai/sepi, cukup pendek untuk mengikuti musim.
const stockWindow = "30 days"

// stockMoveMatch mencocokkan gerakan stok dengan baris saldo `stocks`.
// Stok dihitung di tingkat BARANG (variant_id kosong), sedangkan gerakan penjualan
// varian tetap membawa variant_id-nya — jadi baris tingkat barang menghitung
// semua gerakan barang itu.
const stockMoveMatch = `m.tenant_id = stocks.tenant_id AND m.outlet_id = stocks.outlet_id
	AND m.product_id = stocks.product_id
	AND (stocks.variant_id = '' OR m.variant_id = stocks.variant_id)`

// stockIdleCond: masih ada barangnya, TIDAK terjual sama sekali 30 hari
// terakhir, padahal sudah tercatat di toko lebih dari 30 hari (barang yang baru
// datang minggu ini bukan "tidak laku").
var stockIdleCond = `stocks.qty > 0
	AND NOT EXISTS (SELECT 1 FROM stock_movements m WHERE ` + stockMoveMatch + `
		AND m.kind = 'sale' AND m.occurred_at >= now() - interval '` + stockWindow + `')
	AND EXISTS (SELECT 1 FROM stock_movements m WHERE ` + stockMoveMatch + `
		AND m.occurred_at < now() - interval '` + stockWindow + `')`

// stockStatusCond menerjemahkan StockFilter.Status ke WHERE — batasnya SAMA
// dengan hitungan StockSummary, supaya angka di kartu ringkasan selalu sama
// dengan jumlah baris setelah disaring.
func stockStatusCond(status string) string {
	switch status {
	case "safe":
		return "stocks.qty > p.min_stock AND stocks.qty > 0"
	case "low":
		return "stocks.qty > 0 AND stocks.qty <= p.min_stock"
	case "out":
		return "stocks.qty = 0"
	case "negative":
		return "stocks.qty < 0"
	case "idle":
		return stockIdleCond
	}
	return ""
}

// stockOrder: urutan daftar saldo. "urgent" menaikkan yang butuh tindakan —
// minus, habis, lalu hampir habis dari yang paling jauh di bawah batasnya —
// sehingga halaman pertama selalu berisi yang perlu diurus hari ini.
func stockOrder(sort string) string {
	switch sort {
	case "urgent":
		return `CASE WHEN stocks.qty < 0 THEN 0 WHEN stocks.qty = 0 THEN 1
			WHEN stocks.qty <= p.min_stock THEN 2 ELSE 3 END,
			stocks.qty / NULLIF(p.min_stock, 0) ASC NULLS LAST, p.name ASC, stocks.product_id ASC`
	case "value":
		return "GREATEST(stocks.qty, 0) * p.cost_price DESC, p.name ASC, stocks.product_id ASC"
	case "sold":
		return "v.sold_30d DESC, p.name ASC, stocks.product_id ASC"
	}
	return "p.name ASC, stocks.product_id ASC"
}

// stockBase menyusun query dasar saldo stok (JOIN produk, bertenant, sesuai
// lingkup toko pengguna) untuk satu outlet atau semua outlet yang boleh.
func stockBase(ctx context.Context, outletID string) *gorm.DB {
	q := tenantDB(ctx, nil).
		Table("stocks").
		Joins("JOIN products p ON p.tenant_id = stocks.tenant_id AND p.id = stocks.product_id AND p.deleted_at IS NULL").
		Where("stocks.tenant_id = ?", reqctx.TenantID(ctx))
	if outletID != "" {
		q = q.Where("stocks.outlet_id = ?", outletID)
	}
	return scopeOutlet(ctx, q, "stocks.outlet_id")
}

// ListStocks mengembalikan saldo stok (lihat StockFilter), berpaginasi, dengan
// nama produk & satuan.
func ListStocks(ctx context.Context, f StockFilter, limit, offset int) ([]StockRow, int64, error) {
	// build menyusun query dasar (JOIN produk & satuan) yang sama untuk count
	// maupun ambil-halaman.
	build := func() *gorm.DB {
		q := stockBase(ctx, f.OutletID).
			Joins("JOIN units u ON u.tenant_id = p.tenant_id AND u.id = p.unit_id")
		if f.LowOnly {
			q = q.Where("stocks.qty <= p.min_stock")
		}
		if cond := stockStatusCond(f.Status); cond != "" {
			q = q.Where(cond)
		}
		if f.Search != "" {
			pola := "%" + escapeLike(f.Search) + "%"
			q = q.Where("(p.name ILIKE ? OR p.sku ILIKE ? OR p.barcode ILIKE ?)", pola, pola, pola)
		}
		if len(f.ProductIDs) > 0 {
			q = q.Where("stocks.product_id IN ?", f.ProductIDs)
		}
		return q
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []StockRow{}, 0, nil
	}

	// Laku 30 hari = keluar bersih lewat penjualan, dikurangi pembatalan &
	// retur, ditambah yang terpakai sebagai bahan resep. Terakhir terjual
	// diambil dari indeks (outlet, barang, waktu DESC) — berhenti di penjualan
	// pertama yang ditemui, tidak memindai seluruh riwayat.
	var rows []StockRow
	err := build().
		Joins("LEFT JOIN categories c ON c.tenant_id = p.tenant_id AND c.id = p.category_id AND c.deleted_at IS NULL").
		Joins(`LEFT JOIN LATERAL (
			SELECT GREATEST(COALESCE(-SUM(m.qty_delta), 0), 0) AS sold_30d
			FROM stock_movements m
			WHERE ` + stockMoveMatch + `
				AND m.kind IN ('sale', 'void', 'refund', 'recipe')
				AND m.occurred_at >= now() - interval '` + stockWindow + `'
		) v ON true`).
		Select(`stocks.outlet_id, stocks.product_id, stocks.variant_id, stocks.qty, stocks.reserved_qty,
			p.name AS product_name, p.min_stock, u.name AS unit_name,
			COALESCE(p.sku, '') AS sku, COALESCE(p.image_url, '') AS image_url,
			p.category_id, COALESCE(c.name, '') AS category_name, p.cost_price,
			ROUND(GREATEST(stocks.qty, 0) * p.cost_price)::bigint AS stock_value,
			v.sold_30d,
			(SELECT m.occurred_at FROM stock_movements m WHERE ` + stockMoveMatch + ` AND m.kind = 'sale'
				ORDER BY m.occurred_at DESC LIMIT 1) AS last_sold_at`).
		Order(stockOrder(f.Sort)).
		Limit(limit).Offset(offset).
		Scan(&rows).Error
	return rows, total, err
}

// StockRow adalah baris saldo stok yang diperkaya untuk ditampilkan.
type StockRow struct {
	OutletID    string          `json:"outlet_id"`
	ProductID   string          `json:"product_id"`
	VariantID   string          `json:"variant_id"`
	Qty         decimal.Decimal `json:"qty"`
	ReservedQty decimal.Decimal `json:"reserved_qty"`
	ProductName string          `json:"product_name"`
	MinStock    decimal.Decimal `json:"min_stock"`
	UnitName    string          `json:"unit_name"`

	SKU          string
	ImageURL     string
	CategoryID   *string
	CategoryName string
	CostPrice    int64
	StockValue   int64           // max(qty,0) × harga modal
	Sold30d      decimal.Decimal `gorm:"column:sold_30d"` // keluar bersih 30 hari terakhir
	LastSoldAt   *time.Time
}

// MovementsByRef mengembalikan gerakan stok yang dihasilkan sebuah dokumen
// (mis. semua kind='sale' dari satu penjualan), untuk dibalik saat void/refund.
// tx opsional.
func MovementsByRef(ctx context.Context, tx *gorm.DB, refTable, refID, kind string) ([]models.StockMovement, error) {
	var rows []models.StockMovement
	q := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("ref_table = ? AND ref_id = ?", refTable, refID)
	if kind != "" {
		q = q.Where("kind = ?", kind)
	}
	err := q.Order("product_id ASC").Find(&rows).Error
	return rows, err
}

// ListStockMovements mengembalikan kartu stok satu produk (opsional per outlet),
// terbaru dulu, berpaginasi.
func ListStockMovements(ctx context.Context, productID, outletID string, limit, offset int) ([]models.StockMovement, int64, error) {
	where := "product_id = ?"
	args := []any{productID}
	if outletID != "" {
		where += " AND outlet_id = ?"
		args = append(args, outletID)
	}
	where, args = whereOutlet(ctx, where, args, "outlet_id")
	return paginateTenant[models.StockMovement](ctx, where, args, "occurred_at DESC, id DESC", limit, offset)
}

// StockSummaryRow adalah hasil StockSummary: jumlah baris saldo per keadaan
// dan nilai stok (rupiah, harga modal).
type StockSummaryRow struct {
	Total      int64
	Safe       int64
	Low        int64
	Out        int64
	Negative   int64
	StockValue int64
	// Idle: masih ada barangnya tapi tidak terjual 30 hari (lihat
	// stockIdleCond) — modal yang tertahan di rak. Bisa beririsan dengan
	// safe/low; bukan keadaan kelima.
	Idle      int64
	IdleValue int64
}

// StockSummary merangkum saldo stok SELURUH barang (bukan satu halaman):
// berapa yang aman, hampir habis, habis, dan minus, plus nilai stok menurut
// harga modal.
//
// Batasannya sama persis dengan lencana di halaman Stok:
//   - negative (perlu dicocokkan): qty < 0 — penjualan tidak pernah diblokir
//     stok, jadi saldo minus itu sah dan menandakan catatan yang perlu dicek;
//   - out (habis): qty = 0;
//   - low (hampir habis): 0 < qty ≤ min_stock;
//   - safe: sisanya.
//
// Nilai stok memakai GREATEST(qty, 0): saldo minus tidak "mengurangi" nilai
// barang yang ada di rak — ia hanya berarti catatannya belum cocok.
func StockSummary(ctx context.Context, outletID string) (StockSummaryRow, error) {
	var r StockSummaryRow
	err := stockBase(ctx, outletID).Select(`
		COUNT(*)                                                         AS total,
		COUNT(*) FILTER (WHERE stocks.qty > p.min_stock AND stocks.qty > 0) AS safe,
		COUNT(*) FILTER (WHERE stocks.qty > 0 AND stocks.qty <= p.min_stock) AS low,
		COUNT(*) FILTER (WHERE stocks.qty = 0)                           AS out,
		COUNT(*) FILTER (WHERE stocks.qty < 0)                           AS negative,
		COALESCE(ROUND(SUM(GREATEST(stocks.qty, 0) * p.cost_price)), 0)::bigint AS stock_value,
		COUNT(*) FILTER (WHERE ` + stockIdleCond + `)                         AS idle,
		COALESCE(ROUND(SUM(stocks.qty * p.cost_price) FILTER (WHERE ` + stockIdleCond + `)), 0)::bigint AS idle_value`).
		Scan(&r).Error
	return r, err
}
