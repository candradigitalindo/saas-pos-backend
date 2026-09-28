package repositories

import (
	"context"
	"errors"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

var ErrVariantNotFound = errors.New("varian tidak ditemukan")

// ListProductVariants: varian satu barang (aktif & nonaktif), urut dibuat.
func ListProductVariants(ctx context.Context, tx *gorm.DB, productID string) ([]models.ProductVariant, error) {
	var out []models.ProductVariant
	err := scopeTenant(ctx, tenantDB(ctx, tx)).Where("product_id = ?", productID).
		Order("created_at, id").Find(&out).Error
	return out, err
}

// VariantsForProducts: varian beberapa barang sekaligus (menu aplikasi antar).
func VariantsForProducts(ctx context.Context, tx *gorm.DB, productIDs []string) (map[string][]models.ProductVariant, error) {
	out := map[string][]models.ProductVariant{}
	if len(productIDs) == 0 {
		return out, nil
	}
	var rows []models.ProductVariant
	if err := scopeTenant(ctx, tenantDB(ctx, tx)).Where("product_id IN ?", productIDs).
		Order("created_at, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, v := range rows {
		out[v.ProductID] = append(out[v.ProductID], v)
	}
	return out, nil
}

func FindProductVariant(ctx context.Context, tx *gorm.DB, productID, id string) (models.ProductVariant, error) {
	var v models.ProductVariant
	err := scopeTenant(ctx, tenantDB(ctx, tx)).First(&v, "product_id = ? AND id = ?", productID, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return v, ErrVariantNotFound
	}
	return v, err
}

// FindVariantByCode: varian ber-SKU/barcode itu (pesanan kanal & pemindai).
func FindVariantByCode(ctx context.Context, tx *gorm.DB, code string) (models.ProductVariant, bool, error) {
	var rows []models.ProductVariant
	err := scopeTenant(ctx, tenantDB(ctx, tx)).Where("sku = ? OR barcode = ?", code, code).Limit(2).Find(&rows).Error
	if err != nil || len(rows) != 1 {
		return models.ProductVariant{}, false, err
	}
	return rows[0], true, nil
}

// VariantCodeTaken: SKU/barcode sudah dipakai varian lain.
func VariantCodeTaken(ctx context.Context, tx *gorm.DB, code, exceptID string) (bool, error) {
	var n int64
	err := scopeTenant(ctx, tenantDB(ctx, tx).Model(&models.ProductVariant{})).
		Where("(sku = ? OR barcode = ?) AND id <> ?", code, code, exceptID).Count(&n).Error
	return n > 0, err
}

func CreateProductVariant(ctx context.Context, tx *gorm.DB, v *models.ProductVariant) error {
	v.TenantID = currentTenantID(ctx)
	return tenantDB(ctx, tx).Create(v).Error
}

func UpdateProductVariant(ctx context.Context, tx *gorm.DB, v *models.ProductVariant) error {
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Model(v).
		Select("name", "sku", "barcode", "price_delta", "is_active").Updates(v)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrVariantNotFound
	}
	return nil
}

// DeleteProductVariant: hapus lunak — penjualan lama tetap merujuknya.
func DeleteProductVariant(ctx context.Context, tx *gorm.DB, productID, id string) error {
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Where("product_id = ?", productID).Delete(&models.ProductVariant{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrVariantNotFound
	}
	return nil
}
