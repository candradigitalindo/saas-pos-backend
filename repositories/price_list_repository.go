package repositories

import (
	"context"
	"errors"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

// Daftar harga KHUSUS (member, reseller) — price_lists selain yang default.
// Harga per barang disimpan sebagai product_prices tanpa varian; jumlah
// minimalnya 1 (harga khusus berlaku sejak barang pertama).

var ErrPriceListNotFound = errors.New("daftar harga tidak ditemukan")

// ListPriceLists: semua daftar harga tenant yang belum dihapus, default dulu.
func ListPriceLists(ctx context.Context) ([]models.PriceList, error) {
	var rows []models.PriceList
	err := scopeTenant(ctx, tenantDB(ctx, nil)).Order("is_default DESC, name").Find(&rows).Error
	return rows, err
}

// FindSpecialPriceList: daftar harga KHUSUS (bukan default) yang masih ada.
func FindSpecialPriceList(ctx context.Context, tx *gorm.DB, id string) (models.PriceList, error) {
	var pl models.PriceList
	err := scopeTenant(ctx, tenantDB(ctx, tx)).Where("NOT is_default").First(&pl, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return pl, ErrPriceListNotFound
	}
	return pl, err
}

func CreatePriceList(ctx context.Context, tx *gorm.DB, pl *models.PriceList) error {
	pl.TenantID = currentTenantID(ctx)
	return tenantDB(ctx, tx).Create(pl).Error
}

// DeletePriceList menghapus lunak daftar harga khusus. Harga barangnya ikut
// dihapus (keras) supaya tidak menggantung; pelanggan yang memakainya kembali
// ke harga umum karena daftar yang terhapus diabaikan saat checkout.
func DeletePriceList(ctx context.Context, tx *gorm.DB, id string) error {
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Where("id = ? AND NOT is_default", id).Delete(&models.PriceList{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrPriceListNotFound
	}
	return scopeTenant(ctx, tenantDB(ctx, tx)).Where("price_list_id = ?", id).Delete(&models.ProductPrice{}).Error
}

// ListTiers: tingkat harga satu daftar harga (yang masih ada) per barang,
// tanpa varian, min_qty TERBESAR dulu.
func ListTiers(ctx context.Context, tx *gorm.DB, listID string, productIDs []string) (map[string][]models.ProductPrice, error) {
	out := map[string][]models.ProductPrice{}
	if len(productIDs) == 0 || listID == "" {
		return out, nil
	}
	var rows []models.ProductPrice
	err := scopeTenantOn(ctx, tenantDB(ctx, tx), "product_prices").
		Joins("JOIN price_lists pl ON pl.tenant_id = product_prices.tenant_id AND pl.id = product_prices.price_list_id").
		Where("pl.id = ? AND pl.deleted_at IS NULL AND product_prices.variant_id IS NULL AND product_prices.product_id IN ?", listID, productIDs).
		Order("product_prices.min_qty DESC").
		Find(&rows).Error
	for _, r := range rows {
		out[r.ProductID] = append(out[r.ProductID], r)
	}
	return out, err
}

// SpecialPrices: harga khusus satu barang di semua daftar harga khusus yang masih ada.
func SpecialPrices(ctx context.Context, tx *gorm.DB, productID string) ([]models.ProductPrice, error) {
	var rows []models.ProductPrice
	err := scopeTenantOn(ctx, tenantDB(ctx, tx), "product_prices").
		Joins("JOIN price_lists pl ON pl.tenant_id = product_prices.tenant_id AND pl.id = product_prices.price_list_id").
		Where("NOT pl.is_default AND pl.deleted_at IS NULL AND product_prices.variant_id IS NULL AND product_prices.product_id = ?", productID).
		Order("pl.name").
		Find(&rows).Error
	return rows, err
}

// ReplaceSpecialPrices mengganti SELURUH harga khusus satu barang (semua
// daftar harga khusus) — hapus lalu sisip, seperti harga grosir.
func ReplaceSpecialPrices(ctx context.Context, tx *gorm.DB, productID string, rows []models.ProductPrice) error {
	if err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("product_id = ? AND variant_id IS NULL AND price_list_id IN (?)", productID,
			scopeTenant(ctx, tenantDB(ctx, tx).Model(&models.PriceList{}).Unscoped()).Where("NOT is_default").Select("id")).
		Delete(&models.ProductPrice{}).Error; err != nil {
		return err
	}
	for i := range rows {
		rows[i].TenantID, rows[i].ProductID = currentTenantID(ctx), productID
	}
	if len(rows) == 0 {
		return nil
	}
	return tenantDB(ctx, tx).Create(&rows).Error
}
