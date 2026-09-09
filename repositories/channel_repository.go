package repositories

import (
	"context"
	"errors"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

// Repositori kanal pesanan online (§5.10). Tabel bertenant + RLS; tidak
// ber-owner_id, jadi hanya lapis 1 (scopeTenant) yang berlaku.

var (
	ErrChannelNotFound        = errors.New("kanal tidak ditemukan")
	ErrChannelProductNotFound = errors.New("pemetaan produk kanal tidak ditemukan")
	ErrChannelOrderNotFound   = errors.New("pesanan kanal tidak ditemukan")
)

// ── Channel ───────────────────────────────────────────────────────────────

// ListChannels mengembalikan seluruh kanal tenant (aktif lebih dulu).
func ListChannels(ctx context.Context) ([]models.Channel, error) {
	var rows []models.Channel
	err := scopeTenant(ctx, tenantDB(ctx, nil)).Order("is_active DESC, name").Find(&rows).Error
	return rows, err
}

// FindChannel memuat satu kanal milik tenant.
func FindChannel(ctx context.Context, tx *gorm.DB, id string) (models.Channel, error) {
	var c models.Channel
	err := scopeTenant(ctx, tenantDB(ctx, tx)).First(&c, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c, ErrChannelNotFound
	}
	return c, err
}

// CreateChannel menyimpan kanal baru.
func CreateChannel(ctx context.Context, tx *gorm.DB, c *models.Channel) error {
	c.TenantID = currentTenantID(ctx)
	return tenantDB(ctx, tx).Create(c).Error
}

// SaveChannel memperbarui kolom kanal yang berubah.
func SaveChannel(ctx context.Context, tx *gorm.DB, c *models.Channel) error {
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.Channel{}).
		Where("id = ?", c.ID).
		Updates(map[string]any{
			"name":             c.Name,
			"merchant_ref":     c.MerchantRef,
			"commission_rate":  c.CommissionRate,
			"price_list_id":    c.PriceListID,
			"integration_mode": c.IntegrationMode,
			"is_active":        c.IsActive,
			"updated_at":       gorm.Expr("now()"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrChannelNotFound
	}
	return nil
}

// DeactivateChannel menandai kanal tidak aktif — "menghapus" kanal tanpa
// membuang riwayat pesanannya (blueprint: mematikan kanal tak boleh mengganggu
// operasi).
func DeactivateChannel(ctx context.Context, tx *gorm.DB, id string) error {
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.Channel{}).
		Where("id = ?", id).
		Updates(map[string]any{"is_active": false, "updated_at": gorm.Expr("now()")})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrChannelNotFound
	}
	return nil
}

// ── Channel product ───────────────────────────────────────────────────────

// ListChannelProducts mengembalikan pemetaan SKU sebuah kanal.
func ListChannelProducts(ctx context.Context, channelID string, limit, offset int) ([]models.ChannelProduct, int64, error) {
	base := func() *gorm.DB {
		return scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.ChannelProduct{})).
			Where("channel_id = ?", channelID)
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.ChannelProduct
	err := base().Order("external_sku").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

// FindChannelProduct memuat satu pemetaan milik tenant.
func FindChannelProduct(ctx context.Context, tx *gorm.DB, id string) (models.ChannelProduct, error) {
	var p models.ChannelProduct
	err := scopeTenant(ctx, tenantDB(ctx, tx)).First(&p, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return p, ErrChannelProductNotFound
	}
	return p, err
}

// FindChannelProductBySKU mencari pemetaan berdasarkan (channel, external_sku).
func FindChannelProductBySKU(ctx context.Context, tx *gorm.DB, channelID, sku string) (models.ChannelProduct, bool, error) {
	var p models.ChannelProduct
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		First(&p, "channel_id = ? AND external_sku = ?", channelID, sku).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return p, false, nil
	}
	if err != nil {
		return p, false, err
	}
	return p, true, nil
}

// ChannelProductPriceMap mengembalikan harga kanal per product_id untuk sebuah
// kanal (hanya yang punya channel_price). Dipakai penetapan harga pesanan kanal.
func ChannelProductPriceMap(ctx context.Context, tx *gorm.DB, channelID string) (map[string]int64, error) {
	var rows []models.ChannelProduct
	if err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("channel_id = ? AND channel_price IS NOT NULL", channelID).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		if r.ChannelPrice != nil {
			out[r.ProductID] = *r.ChannelPrice
		}
	}
	return out, nil
}

// CreateChannelProduct menyimpan pemetaan baru.
func CreateChannelProduct(ctx context.Context, tx *gorm.DB, p *models.ChannelProduct) error {
	p.TenantID = currentTenantID(ctx)
	return tenantDB(ctx, tx).Create(p).Error
}

// SaveChannelProduct memperbarui pemetaan.
func SaveChannelProduct(ctx context.Context, tx *gorm.DB, p *models.ChannelProduct) error {
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.ChannelProduct{}).
		Where("id = ?", p.ID).
		Updates(map[string]any{
			"product_id":    p.ProductID,
			"external_sku":  p.ExternalSKU,
			"channel_price": p.ChannelPrice,
			"is_available":  p.IsAvailable,
			"stock_buffer":  p.StockBuffer,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrChannelProductNotFound
	}
	return nil
}

// DeleteChannelProduct menghapus keras satu pemetaan (tabel tanpa soft delete).
func DeleteChannelProduct(ctx context.Context, tx *gorm.DB, id string) error {
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Delete(&models.ChannelProduct{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrChannelProductNotFound
	}
	return nil
}

// ── Channel order ─────────────────────────────────────────────────────────

// FindChannelOrderByExternal mencari pesanan kanal berdasarkan id eksternal —
// pengaman idempotensi (kanal kadang mengirim pesanan yang sama dua kali).
func FindChannelOrderByExternal(ctx context.Context, tx *gorm.DB, channelID, externalID string) (models.ChannelOrder, bool, error) {
	var o models.ChannelOrder
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		First(&o, "channel_id = ? AND external_order_id = ?", channelID, externalID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return o, false, nil
	}
	if err != nil {
		return o, false, err
	}
	return o, true, nil
}

// CreateChannelOrder menyimpan metadata pesanan kanal.
func CreateChannelOrder(ctx context.Context, tx *gorm.DB, o *models.ChannelOrder) error {
	o.TenantID = currentTenantID(ctx)
	return tx.WithContext(ctx).Create(o).Error
}

// FindChannelOrder memuat satu pesanan kanal milik tenant.
func FindChannelOrder(ctx context.Context, tx *gorm.DB, id string) (models.ChannelOrder, error) {
	var o models.ChannelOrder
	err := scopeTenant(ctx, tenantDB(ctx, tx)).First(&o, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return o, ErrChannelOrderNotFound
	}
	return o, err
}

// ChannelOrderFilter menyaring daftar pesanan kanal.
type ChannelOrderFilter struct {
	ChannelID      string
	ExternalStatus string
}

// ListChannelOrders mengembalikan satu halaman pesanan kanal milik tenant.
func ListChannelOrders(ctx context.Context, f ChannelOrderFilter, limit, offset int) ([]models.ChannelOrder, int64, error) {
	base := func() *gorm.DB {
		q := scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.ChannelOrder{}))
		if f.ChannelID != "" {
			q = q.Where("channel_id = ?", f.ChannelID)
		}
		if f.ExternalStatus != "" {
			q = q.Where("external_status = ?", f.ExternalStatus)
		}
		return q
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.ChannelOrder
	err := base().Order("created_at DESC").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

// SaveChannelOrderStatus memperbarui status & jejak waktu pesanan kanal.
func SaveChannelOrderStatus(ctx context.Context, tx *gorm.DB, o *models.ChannelOrder) error {
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.ChannelOrder{}).
		Where("id = ?", o.ID).
		Updates(map[string]any{
			"external_status": o.ExternalStatus,
			"courier":         o.Courier,
			"tracking_no":     o.TrackingNo,
			"driver_name":     o.DriverName,
			"accepted_at":     o.AcceptedAt,
			"ready_at":        o.ReadyAt,
			"completed_at":    o.CompletedAt,
			"updated_at":      gorm.Expr("now()"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrChannelOrderNotFound
	}
	return nil
}
