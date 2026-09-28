package repositories

import (
	"context"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

// Kemasan barang (product_units, migrasi 000046).

// ProductUnitsFor: kemasan aktif per barang, isi terkecil dulu, beserta satuannya.
func ProductUnitsFor(ctx context.Context, tx *gorm.DB, productIDs []string) (map[string][]models.ProductUnit, error) {
	out := map[string][]models.ProductUnit{}
	if len(productIDs) == 0 {
		return out, nil
	}
	var rows []models.ProductUnit
	err := scopeTenant(ctx, tenantDB(ctx, tx)).Preload("Unit").
		Where("product_id IN ?", productIDs).Order("conversion").Find(&rows).Error
	for _, r := range rows {
		out[r.ProductID] = append(out[r.ProductID], r)
	}
	return out, err
}

// ProductUnitsByIDs: kemasan aktif per id (checkout/pembelian).
func ProductUnitsByIDs(ctx context.Context, tx *gorm.DB, ids []string) (map[string]models.ProductUnit, error) {
	out := map[string]models.ProductUnit{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []models.ProductUnit
	err := scopeTenant(ctx, tenantDB(ctx, tx)).Preload("Unit").Where("id IN ?", ids).Find(&rows).Error
	for _, r := range rows {
		out[r.ID] = r
	}
	return out, err
}

// SaveProductUnits menyamakan kemasan satu barang dengan `rows` (kunci:
// unit_id). Kemasan yang sudah ada DIUBAH di tempat — id-nya tetap, jadi
// keranjang offline yang merujuknya tidak patah; yang tidak disebut dihapus
// lunak; yang baru disisipkan.
func SaveProductUnits(ctx context.Context, tx *gorm.DB, productID string, rows []models.ProductUnit) error {
	var ada []models.ProductUnit
	if err := scopeTenant(ctx, tenantDB(ctx, tx)).Where("product_id = ?", productID).Find(&ada).Error; err != nil {
		return err
	}
	perSatuan := map[string]models.ProductUnit{}
	for _, a := range ada {
		perSatuan[a.UnitID] = a
	}
	dipakai := map[string]bool{}
	for _, r := range rows {
		dipakai[r.UnitID] = true
		if lama, ok := perSatuan[r.UnitID]; ok {
			if err := scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.ProductUnit{}).Where("id = ?", lama.ID).
				Updates(map[string]any{"conversion": r.Conversion, "sell_price": r.SellPrice, "barcode": r.Barcode}).Error; err != nil {
				return err
			}
			continue
		}
		r.TenantID, r.ProductID = currentTenantID(ctx), productID
		if err := tenantDB(ctx, tx).Create(&r).Error; err != nil {
			return err
		}
	}
	for _, a := range ada {
		if !dipakai[a.UnitID] {
			if err := scopeTenant(ctx, tenantDB(ctx, tx)).Delete(&models.ProductUnit{}, "id = ?", a.ID).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// PackagingCodeTaken: barcode sudah dipakai kemasan aktif lain (selain
// kemasan barang `exceptProductID` bersatuan `exceptUnitID`).
func PackagingCodeTaken(ctx context.Context, tx *gorm.DB, code, exceptProductID, exceptUnitID string) (bool, error) {
	var n int64
	err := scopeTenant(ctx, tenantDB(ctx, tx).Model(&models.ProductUnit{})).
		Where("barcode = ? AND NOT (product_id = ? AND unit_id = ?)", code, exceptProductID, exceptUnitID).
		Count(&n).Error
	return n > 0, err
}
