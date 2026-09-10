package repositories

import (
	"context"
	"math"

	"candra/backend-api/models"
)

// Repositori sinkronisasi offline (§10).
//
// Arah SERVER → KLIEN (pull). Klien menyimpan sebuah kursor `sync_version` dan
// meminta semua baris yang lebih baru. Untuk tiap entitas kita ambil satu
// halaman `WHERE sync_version > since ORDER BY sync_version LIMIT n`; baris yang
// sudah soft-delete masuk ke daftar `deleted` (klien ikut menghapus), baris
// hard-delete datang dari `sync_tombstones`.
//
// Semua query BACA tanpa GUC → wajib lewat scopeTenant (lapis 1 isolasi, §6).

// PullChanges menampung satu halaman perubahan untuk klien.
type PullChanges struct {
	Cursor  int64 `json:"cursor"`   // kursor baru; klien menyimpannya DIKURANGI jeda aman
	HasMore bool  `json:"has_more"` // true → masih ada halaman, tarik lagi

	Categories    []models.Category       `json:"categories"`
	Units         []models.Unit           `json:"units"`
	Products      []models.Product        `json:"products"`
	Variants      []models.ProductVariant `json:"product_variants"`
	PriceLists    []models.PriceList      `json:"price_lists"`
	ProductPrices []models.ProductPrice   `json:"product_prices"`
	Customers     []models.Customer       `json:"customers"`
	Stocks        []models.Stock          `json:"stocks"` // snapshot per outlet, bukan delta
	Deleted       map[string][]string     `json:"deleted"`
}

// pullPage mengambil satu halaman baris entitas T dengan sync_version > since
// (termasuk yang soft-delete, lewat Unscoped). Mengembalikan baris HIDUP,
// sync_version tertinggi di halaman, apakah halaman penuh (limit tercapai →
// mungkin masih ada), dan id baris yang sudah soft-delete.
func pullPage[T any](
	ctx context.Context, since int64, limit int,
	extract func(T) (syncVersion int64, deleted bool, id string),
) (live []T, maxSV int64, full bool, deletedIDs []string, err error) {
	var rows []T
	err = scopeTenant(ctx, tenantDB(ctx, nil)).
		Unscoped().
		Where("sync_version > ?", since).
		Order("sync_version").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, 0, false, nil, err
	}
	full = len(rows) == limit
	for _, r := range rows {
		sv, del, id := extract(r)
		if sv > maxSV {
			maxSV = sv
		}
		if del {
			deletedIDs = append(deletedIDs, id)
		} else {
			live = append(live, r)
		}
	}
	return live, maxSV, full, deletedIDs, nil
}

// pullTombstones mengambil satu halaman batu nisan hard-delete.
func pullTombstones(ctx context.Context, since int64, limit int) ([]models.SyncTombstone, int64, bool, error) {
	var rows []models.SyncTombstone
	if err := scopeTenant(ctx, tenantDB(ctx, nil)).
		Where("sync_version > ?", since).
		Order("sync_version").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, 0, false, err
	}
	var maxSV int64
	for _, r := range rows {
		if r.SyncVersion > maxSV {
			maxSV = r.SyncVersion
		}
	}
	return rows, maxSV, len(rows) == limit, nil
}

// pullStocks mengembalikan snapshot saldo stok satu outlet (dibatasi limit).
// Bukan delta — angka di klien memang perkiraan (§10), klien mengganti seluruh
// tabel stoknya.
func pullStocks(ctx context.Context, outletID string, limit int) ([]models.Stock, error) {
	q := scopeTenant(ctx, tenantDB(ctx, nil)).Model(&models.Stock{})
	if outletID != "" {
		q = q.Where("outlet_id = ?", outletID)
	}
	var rows []models.Stock
	err := q.Order("product_id, variant_id").Limit(limit).Find(&rows).Error
	return rows, err
}

// GetPullChanges merakit satu halaman pull lintas seluruh entitas yang
// disinkronkan + batu nisan + snapshot stok, lalu menghitung kursor berikutnya.
//
// Kursor: bila SATU entitas pun halamannya penuh, kursor = sync_version terkecil
// di antara halaman-halaman penuh itu — titik lanjut yang aman (setiap baris
// dengan sync_version ≤ kursor pasti sudah terkirim). Bila tak ada yang penuh,
// kursor = sync_version tertinggi yang terlihat (klien maju dan berhenti).
func GetPullChanges(ctx context.Context, outletID string, since int64, limit int) (PullChanges, error) {
	out := PullChanges{Deleted: map[string][]string{}}

	var maxSV int64
	anyFull := false
	truncMin := int64(math.MaxInt64)

	// track menyerap hasil satu halaman ke dalam perhitungan kursor.
	track := func(pageMaxSV int64, full bool, table string, delIDs []string) {
		if pageMaxSV > maxSV {
			maxSV = pageMaxSV
		}
		if len(delIDs) > 0 {
			out.Deleted[table] = append(out.Deleted[table], delIDs...)
		}
		if full {
			anyFull = true
			if pageMaxSV < truncMin {
				truncMin = pageMaxSV
			}
		}
	}

	var err error
	var sv int64
	var full bool
	var del []string

	if out.Categories, sv, full, del, err = pullPage(ctx, since, limit, func(c models.Category) (int64, bool, string) {
		return c.SyncVersion, c.DeletedAt.Valid, c.ID
	}); err != nil {
		return out, err
	}
	track(sv, full, "categories", del)

	if out.Units, sv, full, del, err = pullPage(ctx, since, limit, func(u models.Unit) (int64, bool, string) {
		return u.SyncVersion, u.DeletedAt.Valid, u.ID
	}); err != nil {
		return out, err
	}
	track(sv, full, "units", del)

	if out.Products, sv, full, del, err = pullPage(ctx, since, limit, func(p models.Product) (int64, bool, string) {
		return p.SyncVersion, p.DeletedAt.Valid, p.ID
	}); err != nil {
		return out, err
	}
	track(sv, full, "products", del)

	if out.Variants, sv, full, del, err = pullPage(ctx, since, limit, func(v models.ProductVariant) (int64, bool, string) {
		return v.SyncVersion, v.DeletedAt.Valid, v.ID
	}); err != nil {
		return out, err
	}
	track(sv, full, "product_variants", del)

	if out.PriceLists, sv, full, del, err = pullPage(ctx, since, limit, func(pl models.PriceList) (int64, bool, string) {
		return pl.SyncVersion, pl.DeletedAt.Valid, pl.ID
	}); err != nil {
		return out, err
	}
	track(sv, full, "price_lists", del)

	if out.ProductPrices, sv, full, del, err = pullPage(ctx, since, limit, func(pp models.ProductPrice) (int64, bool, string) {
		return pp.SyncVersion, false, pp.ID // product_prices hard-delete (tanpa deleted_at)
	}); err != nil {
		return out, err
	}
	track(sv, full, "product_prices", del)

	if out.Customers, sv, full, del, err = pullPage(ctx, since, limit, func(c models.Customer) (int64, bool, string) {
		return c.SyncVersion, c.DeletedAt.Valid, c.ID
	}); err != nil {
		return out, err
	}
	track(sv, full, "customers", del)

	// Batu nisan hard-delete.
	tombs, tsv, tfull, err := pullTombstones(ctx, since, limit)
	if err != nil {
		return out, err
	}
	for _, tb := range tombs {
		out.Deleted[tb.TargetTable] = append(out.Deleted[tb.TargetTable], tb.RowID)
	}
	track(tsv, tfull, "", nil)

	// Snapshot stok (tidak ikut kursor).
	if out.Stocks, err = pullStocks(ctx, outletID, limit); err != nil {
		return out, err
	}

	if anyFull {
		out.Cursor, out.HasMore = truncMin, true
	} else {
		out.Cursor, out.HasMore = maxSV, false
		if out.Cursor < since {
			out.Cursor = since
		}
	}

	// Irisan kosong dikirim sebagai [] , bukan null.
	//
	// encoding/json memarshal irisan nil jadi `null`, dan klien yang wajar
	// membaca kontrak `[]Category` akan langsung memanggil `.length` di
	// atasnya. Bagi warung yang BARU didaftarkan — belum punya kategori, belum
	// punya pelanggan — hampir semua daftar di sini nil, jadi bug itu justru
	// mengenai pengguna pertama, bukan kasus pinggiran.
	nonNil(&out)
	return out, nil
}

// nonNil memastikan setiap daftar pada PullChanges berupa array kosong alih-alih
// nil, supaya JSON-nya `[]` dan bukan `null`. Lihat catatan di GetPullChanges.
func nonNil(p *PullChanges) {
	if p.Categories == nil {
		p.Categories = []models.Category{}
	}
	if p.Units == nil {
		p.Units = []models.Unit{}
	}
	if p.Products == nil {
		p.Products = []models.Product{}
	}
	if p.Variants == nil {
		p.Variants = []models.ProductVariant{}
	}
	if p.PriceLists == nil {
		p.PriceLists = []models.PriceList{}
	}
	if p.ProductPrices == nil {
		p.ProductPrices = []models.ProductPrice{}
	}
	if p.Customers == nil {
		p.Customers = []models.Customer{}
	}
	if p.Stocks == nil {
		p.Stocks = []models.Stock{}
	}
	if p.Deleted == nil {
		p.Deleted = map[string][]string{}
	}
}
