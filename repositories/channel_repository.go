package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"candra/backend-api/database"
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

// ChannelStatRow: agregat pesanan satu kanal (lihat ChannelStats).
type ChannelStatRow struct {
	ChannelID     string
	OrderCount    int64
	GrossAmount   int64
	TotalAmount   int64
	FeeAmount     int64
	CanceledCount int64
	LastOrderAt   *time.Time
}

// ChannelStatsSince merangkum pesanan tiap kanal sejak `since` (waktu pesanan,
// bukan waktu dicatat — impor CSV membawa tanggal aslinya). Pesanan terakhir
// dihitung tanpa batas waktu: "terakhir 3 bulan lalu" juga informasi.
// Satu kueri untuk semua kanal.
func ChannelStatsSince(ctx context.Context, since time.Time) (map[string]ChannelStatRow, error) {
	tid := currentTenantID(ctx)
	var rows []ChannelStatRow
	err := tenantDB(ctx, nil).Raw(`
		SELECT co.channel_id,
		  COUNT(*) FILTER (WHERE s.status = 'completed' AND s.occurred_at >= ?)                        AS order_count,
		  COALESCE(SUM(s.subtotal) FILTER (WHERE s.status = 'completed' AND s.occurred_at >= ?), 0)    AS gross_amount,
		  COALESCE(SUM(s.total) FILTER (WHERE s.status = 'completed' AND s.occurred_at >= ?), 0)       AS total_amount,
		  COALESCE(SUM(f.fee) FILTER (WHERE s.status = 'completed' AND s.occurred_at >= ?), 0)         AS fee_amount,
		  COUNT(*) FILTER (WHERE s.status = 'canceled' AND s.occurred_at >= ?)                         AS canceled_count,
		  MAX(s.occurred_at)                                                                           AS last_order_at
		FROM channel_orders co
		JOIN sales s ON s.tenant_id = co.tenant_id AND s.id = co.sale_id
		LEFT JOIN (
		  SELECT sale_id, SUM(fee_amount) AS fee FROM sale_payments WHERE tenant_id = ? GROUP BY sale_id
		) f ON f.sale_id = s.id
		WHERE co.tenant_id = ?
		GROUP BY co.channel_id`,
		since, since, since, since, since, tid, tid).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]ChannelStatRow, len(rows))
	for _, r := range rows {
		out[r.ChannelID] = r
	}
	return out, nil
}

// SalesForChannelOrders memuat penjualan di balik pesanan kanal, lengkap dengan
// item & pembayarannya, dalam kueri tetap (bukan satu kueri per pesanan).
func SalesForChannelOrders(ctx context.Context, orders []models.ChannelOrder) (map[string]models.Sale, error) {
	out := map[string]models.Sale{}
	if len(orders) == 0 {
		return out, nil
	}
	ids := make([]string, 0, len(orders))
	for _, o := range orders {
		ids = append(ids, o.SaleID)
	}
	var sales []models.Sale
	if err := scopeTenant(ctx, tenantDB(ctx, nil)).Where("id IN ?", ids).Find(&sales).Error; err != nil {
		return nil, err
	}
	if err := AttachSaleLines(ctx, sales); err != nil {
		return nil, err
	}
	for _, s := range sales {
		out[s.ID] = s
	}
	return out, nil
}

// FindProductIDByCode mencari barang dari SKU atau barcode-nya — untuk impor
// laporan kanal yang tidak mengenal ID internal. Kosong bila tidak ada; galat
// bila kodenya dipakai lebih dari satu barang (tebakan di sini berarti stok
// barang yang salah ikut terpotong).
func FindProductIDByCode(ctx context.Context, tx *gorm.DB, code string) (string, error) {
	var ids []string
	err := scopeTenant(ctx, tenantDB(ctx, tx).Model(&models.Product{})).
		Where("sku = ? OR barcode = ?", code, code).Limit(2).Pluck("id", &ids).Error
	if err != nil {
		return "", err
	}
	switch len(ids) {
	case 0:
		return "", nil
	case 1:
		return ids[0], nil
	default:
		return "", ErrAmbiguousProductCode
	}
}

// ErrAmbiguousProductCode: satu SKU/barcode dipakai beberapa barang.
var ErrAmbiguousProductCode = errors.New("kode barang dipakai lebih dari satu barang")

// SaveChannelConnection menyimpan kredensial terenkripsi & status sambungan API.
func SaveChannelConnection(ctx context.Context, tx *gorm.DB, c *models.Channel) error {
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.Channel{}).
		Where("id = ?", c.ID).
		Updates(map[string]any{
			"provider":              c.Provider,
			"merchant_ref":          c.MerchantRef,
			"integration_mode":      c.IntegrationMode,
			"credentials_encrypted": c.CredentialsEncrypted,
			"webhook_token":         c.WebhookToken,
			"connection_status":     c.ConnectionStatus,
			"connection_checked_at": c.ConnectionCheckedAt,
			"connection_error":      c.ConnectionError,
			"updated_at":            gorm.Expr("now()"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrChannelNotFound
	}
	return nil
}

// FindChannelByWebhookToken mencari kanal dari token alamat webhook-nya,
// LINTAS tenant (webhook datang tanpa sesi) — seperti FindChannelByProviderRef.
func FindChannelByWebhookToken(ctx context.Context, token string) (models.Channel, error) {
	var c models.Channel
	err := database.DB.WithContext(ctx).Where("webhook_token = ?", token).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c, ErrChannelNotFound
	}
	return c, err
}

// ChannelByIDAnyTenant memuat kanal tanpa sesi (pekerja lintas tenant).
func ChannelByIDAnyTenant(ctx context.Context, id string) (models.Channel, error) {
	var c models.Channel
	err := database.DB.WithContext(ctx).First(&c, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c, ErrChannelNotFound
	}
	return c, err
}

// ChannelProductsForProduct: semua pemetaan satu barang di kanal (satu barang
// bisa dijual lewat lebih dari satu SKU/listing).
func ChannelProductsForProduct(ctx context.Context, tx *gorm.DB, channelID, productID string) ([]models.ChannelProduct, error) {
	var out []models.ChannelProduct
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("channel_id = ? AND product_id = ?", channelID, productID).Order("external_sku").Find(&out).Error
	return out, err
}

// SetChannelProductRef menyimpan pengenal penyedia (hasil pencocokan) dan
// waktu sinkron stok terakhir.
func SetChannelProductRef(ctx context.Context, tx *gorm.DB, id, ref string) error {
	return scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.ChannelProduct{}).
		Where("id = ?", id).Update("external_product_id", ref).Error
}

func MarkChannelProductSynced(ctx context.Context, tx *gorm.DB, ids []string, at time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.ChannelProduct{}).
		Where("id IN ?", ids).Update("last_synced_at", at).Error
}

// StokKanalRingkas: angka ringkas sinkron stok satu kanal.
type StokKanalRingkas struct {
	Terpetakan int64
	BerRef     int64
	Menunggu   int64
	Gagal      int64
	TerakhirAt *time.Time
	GalatAkhir string
	SKUGalat   string
}

func ChannelStockSummary(ctx context.Context, channelID string) (StokKanalRingkas, error) {
	var r StokKanalRingkas
	db := tenantDB(ctx, nil)
	if err := scopeTenant(ctx, db.Model(&models.ChannelProduct{})).Where("channel_id = ?", channelID).Count(&r.Terpetakan).Error; err != nil {
		return r, err
	}
	if err := scopeTenant(ctx, db.Model(&models.ChannelProduct{})).
		Where("channel_id = ? AND COALESCE(external_product_id, '') <> ''", channelID).Count(&r.BerRef).Error; err != nil {
		return r, err
	}
	if err := scopeTenant(ctx, db.Model(&models.ChannelStockSync{})).
		Where("channel_id = ? AND status = 'pending'", channelID).Count(&r.Menunggu).Error; err != nil {
		return r, err
	}
	// Gagal = barang yang antrean TERAKHIR-nya gagal (bukan riwayat lama).
	var gagal []struct {
		LastError   string
		ExternalSKU string
	}
	err := db.Raw(`
		SELECT s.last_error, COALESCE((SELECT cp.external_sku FROM channel_products cp
		       WHERE cp.tenant_id = s.tenant_id AND cp.channel_id = s.channel_id AND cp.product_id = s.product_id
		       ORDER BY cp.external_sku LIMIT 1), '') AS external_sku
		FROM channel_stock_syncs s
		WHERE s.tenant_id = ? AND s.channel_id = ? AND s.status = 'failed'
		  AND s.queued_at = (SELECT max(t.queued_at) FROM channel_stock_syncs t
		      WHERE t.tenant_id = s.tenant_id AND t.channel_id = s.channel_id AND t.product_id = s.product_id)
		ORDER BY s.queued_at DESC`, currentTenantID(ctx), channelID).Scan(&gagal).Error
	if err != nil {
		return r, err
	}
	r.Gagal = int64(len(gagal))
	if len(gagal) > 0 {
		r.GalatAkhir, r.SKUGalat = gagal[0].LastError, gagal[0].ExternalSKU
	}
	var akhir sql.NullTime
	if err := scopeTenant(ctx, db.Model(&models.ChannelStockSync{})).
		Where("channel_id = ? AND status = 'sent'", channelID).Select("MAX(sent_at)").Row().Scan(&akhir); err != nil {
		return r, err
	}
	if akhir.Valid {
		r.TerakhirAt = &akhir.Time
	}
	return r, nil
}

// ChannelsForPull: kanal aktif LINTAS tenant yang tersambung ke penyedia
// dengan tarikan berkala (pekerja, tanpa sesi).
func ChannelsForPull(ctx context.Context, providers []string) ([]models.Channel, error) {
	var out []models.Channel
	err := database.DB.WithContext(ctx).
		Where("integration_mode = 'api' AND is_active AND provider IN ? AND connection_status = 'connected'", providers).
		Order("id").Find(&out).Error
	return out, err
}

// LastChannelEventAt: kapan peristiwa terakhir dari kanal ini diterima.
func LastChannelEventAt(ctx context.Context, channelID string) (*time.Time, error) {
	// MAX tanpa baris = NULL → nil ("belum ada pesanan otomatis").
	var t sql.NullTime
	err := scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.ChannelEvent{})).
		Where("channel_id = ?", channelID).Select("MAX(received_at)").Row().Scan(&t)
	if err != nil || !t.Valid {
		return nil, err
	}
	return &t.Time, nil
}

// FindChannelForUpdate memuat kanal & MENGUNCI barisnya sampai tx selesai —
// dipakai saat token penyedia diperbarui: refresh token Shopee hanya bisa
// dipakai sekali, jadi dua pembaruan bersamaan harus antre. NO KEY UPDATE,
// bukan UPDATE: tarikan pesanan menyisipkan channel_events (FK ke kanal ini)
// dari koneksi lain selama kunci dipegang — FOR UPDATE membuatnya menunggu
// kunci itu sampai statement_timeout.
func FindChannelForUpdate(ctx context.Context, tx *gorm.DB, id string) (models.Channel, error) {
	var c models.Channel
	err := scopeTenant(ctx, tenantDB(ctx, tx)).Clauses(lockNoKeyUpdate()).First(&c, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c, ErrChannelNotFound
	}
	return c, err
}
