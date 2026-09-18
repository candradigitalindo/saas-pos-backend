package repositories

import (
	"context"
	"errors"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

// ErrProductNotFound dikembalikan bila produk tidak ada di tenant konteks.
var ErrProductNotFound = errors.New("produk tidak ditemukan")

// ProductFilter menampung filter opsional untuk daftar produk.
type ProductFilter struct {
	Search     string // cocok ke name (memakai index trigram)
	CategoryID string // "" = semua
	IsActive   *bool  // nil = semua
}

// buildProductWhere menyusun klausa WHERE dari filter. Kolom dikualifikasi
// "products." untuk aman saat query melakukan JOIN ke units/categories.
func buildProductWhere(f ProductFilter) (string, []any) {
	conds := make([]string, 0, 3)
	args := make([]any, 0, 3)

	if f.Search != "" {
		conds = append(conds, "products.name ILIKE ?")
		args = append(args, "%"+escapeLike(f.Search)+"%")
	}
	if f.CategoryID != "" {
		conds = append(conds, "products.category_id = ?")
		args = append(args, f.CategoryID)
	}
	if f.IsActive != nil {
		conds = append(conds, "products.is_active = ?")
		args = append(args, *f.IsActive)
	}

	if len(conds) == 0 {
		return "", nil
	}
	where := conds[0]
	for _, c := range conds[1:] {
		where += " AND " + c
	}
	return where, args
}

// ListProducts mengambil satu halaman produk milik tenant konteks, dengan nama
// unit & kategori ikut (LEFT JOIN, satu round-trip — CONVENTIONS §2).
//
// Pencarian `Search` memakai index GIN trigram di products.name (target
// blueprint: < 200 ms pada 500+ produk).
func ListProducts(ctx context.Context, f ProductFilter, limit, offset int) ([]models.Product, int64, error) {
	where, args := buildProductWhere(f)

	// Count: filter hanya menyentuh kolom products → tanpa JOIN.
	countQ := scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.Product{}))
	if where != "" {
		countQ = countQ.Where(where, args...)
	}
	var total int64
	if err := countQ.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []models.Product{}, 0, nil
	}

	rows := make([]models.Product, 0, limit)
	q := scopeTenantOn(ctx, tenantDB(ctx, nil), "products").
		Joins("Unit").
		Joins("Category").
		Order("products.id DESC").
		Limit(limit).
		Offset(offset)
	if where != "" {
		q = q.Where(where, args...)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// FindProductInTenant memuat satu produk (dengan Unit & Category) milik tenant
// konteks. tx opsional.
// SetProductImageURL menyimpan alamat foto sebuah barang (kosong = tanpa foto).
//
// Memakai Update kolom tunggal, bukan Save model penuh: menyimpan seluruh
// model berarti menulis ulang harga dan stok minimum dari salinan yang dibaca
// sebelumnya, dan mengunggah foto tidak boleh diam-diam mengembalikan harga
// yang baru saja diubah orang lain.
func SetProductImageURL(ctx context.Context, id, alamat string) error {
	return tenantDB(ctx, nil).Model(&models.Product{}).
		Where("id = ? AND tenant_id = ?", id, currentTenantID(ctx)).
		Update("image_url", alamat).Error
}

func FindProductInTenant(ctx context.Context, tx *gorm.DB, id string, out *models.Product) error {
	err := scopeTenantOn(ctx, tenantDB(ctx, tx), "products").
		Joins("Unit").
		Joins("Category").
		Where("products.id = ?", id).
		First(out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrProductNotFound
	}
	return err
}

// CreateProduct menyimpan produk baru (TenantID diisi pemanggil). tx opsional.
func CreateProduct(ctx context.Context, tx *gorm.DB, row *models.Product) error {
	// Omit association agar GORM tidak ikut menyisipkan/mengubah unit/kategori.
	return tenantDB(ctx, tx).Omit("Unit", "Category").Create(row).Error
}

// CreateProductsBulk menyisipkan banyak produk sekaligus dalam satu transaksi
// (dipakai impor CSV, setelah controller memvalidasi tiap baris). Semua-atau-
// tidak: bila satu baris gagal di database, seluruh batch batal.
func CreateProductsBulk(ctx context.Context, rows []models.Product) error {
	if len(rows) == 0 {
		return nil
	}
	return WithTenant(ctx, func(tx *gorm.DB) error {
		return tx.Omit("Unit", "Category").CreateInBatches(rows, 200).Error
	})
}

// UpdateProduct menyimpan perubahan atribut produk milik tenant konteks. tx
// opsional. Kolom identitas (tenant_id, id) & sync_version tidak ikut.
func UpdateProduct(ctx context.Context, tx *gorm.DB, row *models.Product) error {
	res := scopeTenant(ctx, tenantDB(ctx, tx)).
		Model(row).
		Omit("Unit", "Category").
		Select(
			"category_id", "unit_id", "name", "sku", "barcode",
			"sell_price", "cost_price", "track_stock", "min_stock",
			"is_active", "image_url",
		).
		Updates(row)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrProductNotFound
	}
	return nil
}

// DeleteProduct menandai produk terhapus (soft delete). tx opsional.
func DeleteProduct(ctx context.Context, tx *gorm.DB, id string) error {
	err := softDeleteTenant[models.Product](ctx, tx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrProductNotFound
	}
	return err
}

// ExistingProductCodes mengembalikan himpunan SKU dan barcode yang SUDAH ada
// (di antara baris hidup) untuk daftar kandidat — dipakai impor CSV agar bisa
// menandai baris duplikat sebelum menyisipkan. Nilai kosong diabaikan.
func ExistingProductCodes(ctx context.Context, skus, barcodes []string) (existSKU, existBarcode map[string]bool, err error) {
	existSKU = map[string]bool{}
	existBarcode = map[string]bool{}

	if len(skus) > 0 {
		var found []string
		if err = scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.Product{})).
			Where("sku IN ?", skus).Pluck("sku", &found).Error; err != nil {
			return nil, nil, err
		}
		for _, s := range found {
			existSKU[s] = true
		}
	}
	if len(barcodes) > 0 {
		var found []string
		if err = scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.Product{})).
			Where("barcode IN ?", barcodes).Pluck("barcode", &found).Error; err != nil {
			return nil, nil, err
		}
		for _, b := range found {
			existBarcode[b] = true
		}
	}
	return existSKU, existBarcode, nil
}

// UnitLookup mengembalikan peta nama-satuan → id untuk seluruh satuan tenant.
// Dipakai impor CSV yang menyebut satuan dengan namanya.
func UnitLookup(ctx context.Context) (map[string]string, error) {
	var units []models.Unit
	if err := scopeTenant(ctx, tenantDB(ctx, nil)).
		Select("id", "name").Find(&units).Error; err != nil {
		return nil, err
	}
	m := make(map[string]string, len(units))
	for _, u := range units {
		m[u.Name] = u.ID
	}
	return m, nil
}

// ProductsByIDs memuat produk (dengan Unit) untuk sekumpulan id — dipakai
// checkout untuk membuat SNAPSHOT nama/harga/modal. tx wajib (di dalam transaksi
// checkout). Mengembalikan peta id → produk.
func ProductsByIDs(ctx context.Context, tx *gorm.DB, ids []string) (map[string]models.Product, error) {
	out := map[string]models.Product{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []models.Product
	err := scopeTenantOn(ctx, tenantDB(ctx, tx), "products").
		Joins("Unit").
		Where("products.id IN ?", ids).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, p := range rows {
		out[p.ID] = p
	}
	return out, nil
}

// ProductVariantsByIDs memuat varian untuk sekumpulan id. tx wajib.
func ProductVariantsByIDs(ctx context.Context, tx *gorm.DB, ids []string) (map[string]models.ProductVariant, error) {
	out := map[string]models.ProductVariant{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []models.ProductVariant
	if err := scopeTenant(ctx, tenantDB(ctx, tx)).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, v := range rows {
		out[v.ID] = v
	}
	return out, nil
}

// CategoryLookup mengembalikan peta nama-kategori → id untuk seluruh kategori tenant.
func CategoryLookup(ctx context.Context) (map[string]string, error) {
	var cats []models.Category
	if err := scopeTenant(ctx, tenantDB(ctx, nil)).
		Select("id", "name").Find(&cats).Error; err != nil {
		return nil, err
	}
	m := make(map[string]string, len(cats))
	for _, c := range cats {
		m[c.Name] = c.ID
	}
	return m, nil
}
