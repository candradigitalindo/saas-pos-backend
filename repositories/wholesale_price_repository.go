package repositories

import (
	"context"
	"errors"

	"candra/backend-api/helpers"
	"candra/backend-api/models"

	"gorm.io/gorm"
)

// Harga grosir per jumlah = baris product_prices pada daftar harga DEFAULT
// tenant, tanpa varian (variant_id NULL). Daftar harga lain (member, grosir
// per pelanggan) memakai tabel yang sama tapi belum dibaca di sini.

// EnsureDefaultPriceList mengembalikan daftar harga default tenant, membuatnya
// ("Harga umum", kind retail) bila belum ada. Dua pembuatan bersamaan dijaga
// indeks unik parsial — yang kalah membaca ulang milik pemenang.
func EnsureDefaultPriceList(ctx context.Context, tx *gorm.DB) (models.PriceList, error) {
	var pl models.PriceList
	err := scopeTenant(ctx, tenantDB(ctx, tx)).Where("is_default").First(&pl).Error
	if err == nil {
		return pl, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return pl, err
	}
	pl = models.PriceList{TenantID: currentTenantID(ctx), Name: "Harga umum", Kind: "retail", IsDefault: true}
	err = tenantDB(ctx, tx).Create(&pl).Error
	if helpers.IsDuplicateEntryError(err) {
		err = scopeTenant(ctx, tenantDB(ctx, tx)).Where("is_default").First(&pl).Error
	}
	return pl, err
}

// WholesaleTiers: tingkat harga grosir per barang, min_qty TERBESAR dulu
// (pemilihan harga cukup mengambil yang pertama cocok).
func WholesaleTiers(ctx context.Context, tx *gorm.DB, productIDs []string) (map[string][]models.ProductPrice, error) {
	out := map[string][]models.ProductPrice{}
	if len(productIDs) == 0 {
		return out, nil
	}
	var rows []models.ProductPrice
	err := scopeTenantOn(ctx, tenantDB(ctx, tx), "product_prices").
		Joins("JOIN price_lists pl ON pl.tenant_id = product_prices.tenant_id AND pl.id = product_prices.price_list_id").
		Where("pl.is_default AND pl.deleted_at IS NULL AND product_prices.variant_id IS NULL AND product_prices.product_id IN ?", productIDs).
		Order("product_prices.min_qty DESC").
		Find(&rows).Error
	for _, r := range rows {
		out[r.ProductID] = append(out[r.ProductID], r)
	}
	return out, err
}

// ReplaceWholesaleTiers mengganti SELURUH tingkat grosir satu barang. Hapus
// lalu sisipkan — bukan upsert: variant_id NULL membuat UNIQUE tidak pernah
// bentrok, jadi ON CONFLICT tidak bisa diandalkan.
func ReplaceWholesaleTiers(ctx context.Context, tx *gorm.DB, productID, priceListID string, tiers []models.ProductPrice) error {
	if err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("product_id = ? AND price_list_id = ? AND variant_id IS NULL", productID, priceListID).
		Delete(&models.ProductPrice{}).Error; err != nil {
		return err
	}
	for i := range tiers {
		tiers[i].TenantID, tiers[i].ProductID, tiers[i].PriceListID = currentTenantID(ctx), productID, priceListID
	}
	if len(tiers) == 0 {
		return nil
	}
	return tenantDB(ctx, tx).Create(&tiers).Error
}
