package repositories

import (
	"context"
	"errors"

	"candra/backend-api/database"
	"candra/backend-api/models"

	"gorm.io/gorm"
)

// Tautan struk digital (migrasi 000045).

// SetSaleReceiptToken memasang token HANYA bila belum ada — dua kasir yang
// membagikan struk yang sama bersamaan tidak saling menimpa (yang kalah
// membaca ulang token pemenang). Mengembalikan jumlah baris yang berubah.
func SetSaleReceiptToken(ctx context.Context, tx *gorm.DB, saleID, token string) (int64, error) {
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.Sale{}).
		Where("id = ? AND receipt_token IS NULL", saleID).
		Update("receipt_token", token)
	return res.RowsAffected, res.Error
}

// PublicReceiptByToken memuat penjualan (beserta item & pembayaran) dan
// cabangnya dari token tautan struk. LINTAS TENANT dan tanpa GUC: halaman
// struk publik tidak tahu tenant — tokennya sendiri (128 bit acak) yang
// menjadi kuncinya. Kebijakan RLS permisif saat GUC kosong (lihat keputusan RLS).
func PublicReceiptByToken(ctx context.Context, token string) (models.Sale, models.Outlet, error) {
	var s models.Sale
	var o models.Outlet
	db := database.DB.WithContext(ctx)
	err := db.Preload("Items").Preload("Payments").First(&s, "receipt_token = ?", token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s, o, ErrSaleNotFound
	}
	if err != nil {
		return s, o, err
	}
	err = db.First(&o, "tenant_id = ? AND id = ?", s.TenantID, s.OutletID).Error
	return s, o, err
}
