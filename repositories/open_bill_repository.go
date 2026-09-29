package repositories

import (
	"context"
	"errors"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

// Repositori tagihan terbuka (open bill, migrasi 000044).

var ErrOpenBillNotFound = errors.New("tagihan tidak ditemukan")

// ListOpenBills: tagihan BERSTATUS open di satu cabang, terbaru diubah dulu.
func ListOpenBills(ctx context.Context, outletID string) ([]models.OpenBill, error) {
	var rows []models.OpenBill
	err := scopeTenant(ctx, tenantDB(ctx, nil)).
		Where("outlet_id = ? AND status = 'open'", outletID).
		Order("updated_at DESC, id DESC").
		Find(&rows).Error
	return rows, err
}

// FindOpenBillForUpdate memuat tagihan & MENGUNCI barisnya sampai tx selesai:
// dua perangkat yang menyimpan/membayar tagihan yang sama harus antre, supaya
// pemeriksaan versi & status tidak saling mendahului. NO KEY UPDATE — tidak
// ada tabel yang merujuk tagihan, tapi tak ada alasan menahan KEY SHARE.
func FindOpenBillForUpdate(ctx context.Context, tx *gorm.DB, id string) (models.OpenBill, error) {
	var b models.OpenBill
	err := scopeTenant(ctx, tenantDB(ctx, tx)).Clauses(lockNoKeyUpdate()).First(&b, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return b, ErrOpenBillNotFound
	}
	return b, err
}

func CreateOpenBill(ctx context.Context, tx *gorm.DB, b *models.OpenBill) error {
	b.TenantID = currentTenantID(ctx)
	return tenantDB(ctx, tx).Create(b).Error
}

// SaveOpenBill menulis ulang seluruh kolom yang boleh berubah.
func SaveOpenBill(ctx context.Context, tx *gorm.DB, b *models.OpenBill) error {
	return scopeTenant(ctx, tenantDB(ctx, tx)).Model(b).
		Select("label", "customer_id", "order_type", "items", "order_discount", "note", "status",
			"version", "last_op_id", "sale_id", "updated_by", "closed_by", "closed_at", "updated_at").
		Updates(b).Error
}
