package repositories

import (
	"context"
	"errors"
	"time"

	"candra/backend-api/database"
	"candra/backend-api/internal/ulid"
	"candra/backend-api/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repositori kanal Fase 11b — kotak masuk peristiwa, biaya, settlement, antrean
// sinkron stok (§5.10).
//
// Beberapa fungsi LINTAS-TENANT (webhook tanpa auth, pekerja latar): memakai
// database.DB langsung tanpa scopeTenant, dengan tenant_id/kolom eksplisit.

var ErrChannelSettlementNotFound = errors.New("settlement kanal tidak ditemukan")

// FindChannelByProviderRef mencari kanal berdasarkan (provider, merchant_ref)
// LINTAS tenant — dipakai webhook untuk menautkan peristiwa ke tenant.
func FindChannelByProviderRef(ctx context.Context, provider, merchantRef string) (models.Channel, error) {
	var c models.Channel
	err := database.DB.WithContext(ctx).
		Where("provider = ? AND merchant_ref = ?", provider, merchantRef).
		First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c, ErrChannelNotFound
	}
	return c, err
}

// InsertChannelEvent menyimpan peristiwa mentah, idempoten lewat
// UNIQUE (tenant, channel, event_type, external_ref). Mengembalikan created.
func InsertChannelEvent(ctx context.Context, tenantID, channelID, eventType, externalRef string, payload []byte) (bool, error) {
	// ReceivedAt WAJIB diisi: time.Time nol ditulis GORM apa adanya ("0001-01-01")
	// dan menimpa DEFAULT now() — antrean pekerja (urut received_at) dan
	// "pesanan otomatis terakhir" lalu kehilangan waktunya.
	row := models.ChannelEvent{
		ID: ulid.New(), TenantID: tenantID, ChannelID: channelID,
		EventType: eventType, ExternalRef: externalRef, Payload: payload, Status: "pending",
		ReceivedAt: time.Now().UTC(),
	}
	res := database.DB.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "channel_id"}, {Name: "event_type"}, {Name: "external_ref"}},
			DoNothing: true,
		}).
		Create(&row)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// MaxEventAttempts adalah batas percobaan sebelum sebuah peristiwa ditandai
// 'dead' (antrean mati) dan tidak diambil lagi oleh pekerja.
const MaxEventAttempts = 5

// ClaimPendingEvents mengambil peristiwa yang siap diproses LINTAS tenant,
// mengunci barisnya (FOR UPDATE SKIP LOCKED).
func ClaimPendingEvents(ctx context.Context, tx *gorm.DB, limit int) ([]models.ChannelEvent, error) {
	var rows []models.ChannelEvent
	err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Where("status IN ('pending','failed') AND attempts < ?", MaxEventAttempts).
		Order("received_at").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

// SaveChannelEventResult menulis hasil pemrosesan satu peristiwa.
func SaveChannelEventResult(ctx context.Context, tx *gorm.DB, id, status, lastErr string, attempts int) error {
	upd := map[string]any{"status": status, "attempts": attempts, "last_error": lastErr}
	if status == "done" {
		now := time.Now().UTC()
		upd["processed_at"] = now
	}
	return tx.WithContext(ctx).Model(&models.ChannelEvent{}).Where("id = ?", id).Updates(upd).Error
}

// ListChannelEvents mengembalikan peristiwa sebuah kanal milik tenant konteks.
func ListChannelEvents(ctx context.Context, channelID, status string, limit, offset int) ([]models.ChannelEvent, int64, error) {
	build := func() *gorm.DB {
		q := scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.ChannelEvent{}))
		if channelID != "" {
			q = q.Where("channel_id = ?", channelID)
		}
		if status != "" {
			q = q.Where("status = ?", status)
		}
		return q
	}
	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.ChannelEvent
	err := build().Order("received_at DESC").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

// AnyTenantUserID mengembalikan id user pertama sebuah tenant — dipakai pekerja
// latar sebagai `created_by` penjualan hasil ingest kanal. tx boleh nil (baca
// lepas). Query LINTAS-tenant: filter tenant_id eksplisit, tanpa scopeTenant.
func AnyTenantUserID(ctx context.Context, tx *gorm.DB, tenantID string) (string, error) {
	var id string
	err := tenantDB(ctx, tx).Model(&models.User{}).
		Where("tenant_id = ? AND is_active = ?", tenantID, true).
		Order("created_at").Limit(1).Pluck("id", &id).Error
	if err != nil {
		return "", err
	}
	if id == "" {
		return "", errors.New("tenant tanpa user aktif")
	}
	return id, nil
}

// ── Channel fee ───────────────────────────────────────────────────────────

func CreateChannelFee(ctx context.Context, tx *gorm.DB, f *models.ChannelFee) error {
	f.TenantID = currentTenantID(ctx)
	return tx.WithContext(ctx).Create(f).Error
}

// ChannelFeesForSale mengembalikan rincian biaya sebuah penjualan.
func ChannelFeesForSale(ctx context.Context, tx *gorm.DB, saleID string) ([]models.ChannelFee, error) {
	var rows []models.ChannelFee
	err := scopeTenant(ctx, tenantDB(ctx, tx)).Where("sale_id = ?", saleID).Order("kind").Find(&rows).Error
	return rows, err
}

// ReplaceChannelFees mengganti seluruh rincian biaya sebuah penjualan —
// idempoten untuk pemrosesan peristiwa yang mungkin diulang.
func ReplaceChannelFees(ctx context.Context, tx *gorm.DB, saleID string, fees []models.ChannelFee) error {
	tid := currentTenantID(ctx)
	if err := tx.WithContext(ctx).Where("tenant_id = ? AND sale_id = ?", tid, saleID).
		Delete(&models.ChannelFee{}).Error; err != nil {
		return err
	}
	for i := range fees {
		fees[i].ID = ulid.New()
		fees[i].TenantID = tid
		fees[i].SaleID = saleID
	}
	if len(fees) == 0 {
		return nil
	}
	return tx.WithContext(ctx).Create(&fees).Error
}

// ── Channel settlement ────────────────────────────────────────────────────

// SettlementTotalsForChannel menjumlahkan nilai kotor & biaya kanal untuk
// penjualan 'completed' pada rentang business_date.
func SettlementTotalsForChannel(ctx context.Context, tx *gorm.DB, channelID, from, to string) (gross, fee int64, err error) {
	tid := currentTenantID(ctx)
	if err = tenantDB(ctx, tx).Model(&models.Sale{}).
		Where("tenant_id = ? AND channel_id = ? AND status = 'completed' AND business_date BETWEEN ? AND ?", tid, channelID, from, to).
		Select("COALESCE(SUM(total), 0)").Scan(&gross).Error; err != nil {
		return
	}
	err = tenantDB(ctx, tx).
		Table("channel_fees cf").
		Joins("JOIN sales s ON s.id = cf.sale_id").
		Where("cf.tenant_id = ? AND s.channel_id = ? AND s.status = 'completed' AND s.business_date BETWEEN ? AND ?", tid, channelID, from, to).
		Select("COALESCE(SUM(cf.amount), 0)").Scan(&fee).Error
	return
}

// FindSettlement memuat settlement sebuah (channel, period_start) milik tenant.
// periodStart adalah string YYYY-MM-DD (kolom bertipe DATE).
func FindSettlement(ctx context.Context, tx *gorm.DB, channelID, periodStart string) (models.ChannelSettlement, bool, error) {
	var s models.ChannelSettlement
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		First(&s, "channel_id = ? AND period_start = ?", channelID, periodStart).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s, false, nil
	}
	if err != nil {
		return s, false, err
	}
	return s, true, nil
}

func UpsertSettlement(ctx context.Context, tx *gorm.DB, s *models.ChannelSettlement) error {
	tid := currentTenantID(ctx)
	s.TenantID = tid
	var existing models.ChannelSettlement
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		First(&existing, "channel_id = ? AND period_start = ?", s.ChannelID, s.PeriodStart.Format("2006-01-02")).Error
	if err == nil {
		s.ID = existing.ID
		return tenantDB(ctx, tx).Model(&models.ChannelSettlement{}).Where("id = ?", existing.ID).Updates(map[string]any{
			"period_end": s.PeriodEnd, "gross_amount": s.GrossAmount, "fee_amount": s.FeeAmount,
			"net_amount": s.NetAmount, "received_amount": s.ReceivedAmount, "status": s.Status,
			"received_at": s.ReceivedAt, "note": s.Note, "updated_at": gorm.Expr("now()"),
		}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return tenantDB(ctx, tx).Create(s).Error
}

func ListSettlements(ctx context.Context, channelID string) ([]models.ChannelSettlement, error) {
	var rows []models.ChannelSettlement
	q := scopeTenant(ctx, tenantDB(ctx, nil))
	if channelID != "" {
		q = q.Where("channel_id = ?", channelID)
	}
	err := q.Order("period_start DESC").Find(&rows).Error
	return rows, err
}

// ── Channel stock sync ────────────────────────────────────────────────────

// QueueStockSyncOnce: antre sinkron stok satu barang bila belum ada yang
// menunggu (pemetaan baru/diubah, gerakan stok). qty = stok saat diantre;
// yang dikirim dihitung ulang saat dikirim.
func QueueStockSyncOnce(ctx context.Context, tenantID, channelID, productID string, qty string) error {
	return database.DB.WithContext(ctx).Exec(`
		INSERT INTO channel_stock_syncs (id, tenant_id, channel_id, product_id, requested_qty, status, queued_at)
		SELECT ?, ?, ?, ?, ?, 'pending', now()
		WHERE NOT EXISTS (SELECT 1 FROM channel_stock_syncs
			WHERE tenant_id = ? AND channel_id = ? AND product_id = ? AND status = 'pending')`,
		ulid.New(), tenantID, channelID, productID, qty, tenantID, channelID, productID).Error
}

// ProductsNeedingStockSync: barang TERPETAKAN (ber-ref penyedia) di kanal
// yang belum pernah diantre, atau stoknya bergerak (stock_movements) setelah
// antrean terakhirnya — dan tidak sedang menunggu. Lintas tenant (pekerja).
// Patokan antrean terakhir, bukan kiriman terakhir: SKU yang gagal permanen
// tidak diantre ulang sampai stoknya bergerak lagi.
func ProductsNeedingStockSync(ctx context.Context, tenantID, channelID, outletID string) ([]string, error) {
	var ids []string
	err := database.DB.WithContext(ctx).Raw(`
		SELECT DISTINCT cp.product_id FROM channel_products cp
		WHERE cp.tenant_id = ? AND cp.channel_id = ? AND COALESCE(cp.external_product_id, '') <> ''
		  AND NOT EXISTS (SELECT 1 FROM channel_stock_syncs s
		      WHERE s.tenant_id = cp.tenant_id AND s.channel_id = cp.channel_id AND s.product_id = cp.product_id AND s.status = 'pending')
		  AND (
		    NOT EXISTS (SELECT 1 FROM channel_stock_syncs s
		        WHERE s.tenant_id = cp.tenant_id AND s.channel_id = cp.channel_id AND s.product_id = cp.product_id)
		    OR EXISTS (SELECT 1 FROM stock_movements m
		        WHERE m.tenant_id = cp.tenant_id AND m.outlet_id = ? AND m.product_id = cp.product_id
		          AND m.created_at > (SELECT max(s.queued_at) FROM channel_stock_syncs s
		              WHERE s.tenant_id = cp.tenant_id AND s.channel_id = cp.channel_id AND s.product_id = cp.product_id)))`,
		tenantID, channelID, outletID).Scan(&ids).Error
	return ids, err
}

// ClaimPendingStockSyncs mengunci antrean yang siap dikirim; kecuali = baris
// yang sudah dicoba di putaran ini (dicoba lagi di putaran pekerja berikutnya,
// bukan beruntun dalam hitungan milidetik).
func ClaimPendingStockSyncs(ctx context.Context, tx *gorm.DB, limit int, kecuali ...string) ([]models.ChannelStockSync, error) {
	var rows []models.ChannelStockSync
	err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Where("status = 'pending' AND attempts < ?", MaxEventAttempts).
		Where("id NOT IN ?", append([]string{""}, kecuali...)).
		// Yang belum pernah dicoba dulu — baris yang terus gagal tidak
		// menghalangi kiriman baru.
		Order("attempts").Order("queued_at").Limit(limit).Find(&rows).Error
	return rows, err
}

func SaveStockSyncResult(ctx context.Context, tx *gorm.DB, id, status, lastErr string, attempts int, qty ...string) error {
	upd := map[string]any{"status": status, "attempts": attempts, "last_error": lastErr}
	if len(qty) > 0 && qty[0] != "" {
		upd["requested_qty"] = qty[0]
	}
	if status == "sent" {
		now := time.Now().UTC()
		upd["sent_at"] = now
	}
	return tx.WithContext(ctx).Model(&models.ChannelStockSync{}).Where("id = ?", id).Updates(upd).Error
}

func ListStockSyncs(ctx context.Context, channelID, status string) ([]models.ChannelStockSync, error) {
	q := scopeTenant(ctx, tenantDB(ctx, nil))
	if channelID != "" {
		q = q.Where("channel_id = ?", channelID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var rows []models.ChannelStockSync
	err := q.Order("queued_at DESC").Find(&rows).Error
	return rows, err
}
