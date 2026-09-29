package repositories

import (
	"context"
	"errors"
	"strings"

	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrOpnameNotFound   = errors.New("stok opname tidak ditemukan")
	ErrTransferNotFound = errors.New("transfer stok tidak ditemukan")
)

// ── stock_opnames ──────────────────────────────────────────────────────────

func CreateOpname(ctx context.Context, tx *gorm.DB, o *models.StockOpname) error {
	return createTenant(ctx, tx, o)
}

func FindOpnameInTenant(ctx context.Context, tx *gorm.DB, id string, out *models.StockOpname) error {
	err := scopeTenant(ctx, tenantDB(ctx, tx)).Preload("Items").First(out, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrOpnameNotFound
	}
	return err
}

// LockOpnameInTenant = FindOpnameInTenant, tetapi baris opname dikunci
// (FOR UPDATE) lebih dulu. Wajib dipakai di dalam tx untuk mengubah status.
func LockOpnameInTenant(ctx context.Context, tx *gorm.DB, id string, out *models.StockOpname) error {
	if err := lockTenantRow[models.StockOpname](ctx, tx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrOpnameNotFound
		}
		return err
	}
	return FindOpnameInTenant(ctx, tx, id, out)
}

func ListOpnames(ctx context.Context, outletID string, limit, offset int) ([]models.StockOpname, int64, error) {
	where, args := "", []any(nil)
	if outletID != "" {
		where, args = "outlet_id = ?", []any{outletID}
	}
	where, args = whereOutlet(ctx, where, args, "outlet_id")
	return paginateTenant[models.StockOpname](ctx, where, args, "created_at DESC, id DESC", limit, offset)
}

// UpsertOpnameItems menyisipkan/memperbarui banyak baris hitungan opname dalam
// SATU pernyataan (ON CONFLICT pada (tenant_id, opname_id, product_id,
// variant_id) — UNIQUE NULLS NOT DISTINCT sejak migrasi 000037, jadi barang
// tanpa varian pun menimpa hitungannya sendiri, bukan menambah baris).
//
// Pemanggil WAJIB sudah membuang key ganda di dalam `items`: satu pernyataan
// ON CONFLICT tidak boleh memperbarui baris yang sama dua kali.
func UpsertOpnameItems(ctx context.Context, tx *gorm.DB, items []models.StockOpnameItem) error {
	if len(items) == 0 {
		return nil
	}
	tid := currentTenantID(ctx)
	for i := range items {
		items[i].TenantID = tid
	}
	return tx.WithContext(ctx). // BeforeCreate meng-generate ID tiap baris bila kosong
					Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "tenant_id"}, {Name: "opname_id"}, {Name: "product_id"}, {Name: "variant_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"system_qty", "counted_qty", "diff_qty"}),
		}).
		Create(&items).Error
}

func SetOpnameStatus(ctx context.Context, tx *gorm.DB, id, status string, countedAt any) error {
	upd := map[string]any{"status": status, "updated_at": gorm.Expr("now()")}
	if countedAt != nil {
		upd["counted_at"] = countedAt
	}
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.StockOpname{}).Where("id = ?", id).Updates(upd)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrOpnameNotFound
	}
	return nil
}

// ── stock_transfers ────────────────────────────────────────────────────────

func CreateTransfer(ctx context.Context, tx *gorm.DB, tr *models.StockTransfer) error {
	tid := currentTenantID(ctx)
	tr.TenantID = tid
	for i := range tr.Items {
		tr.Items[i].TenantID = tid
	}
	return tx.WithContext(ctx).Create(tr).Error
}

func FindTransferInTenant(ctx context.Context, tx *gorm.DB, id string, out *models.StockTransfer) error {
	err := scopeTenant(ctx, tenantDB(ctx, tx)).Preload("Items").First(out, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrTransferNotFound
	}
	return err
}

// LockTransferInTenant = FindTransferInTenant, tetapi baris transfer dikunci
// (FOR UPDATE) lebih dulu. Wajib dipakai di dalam tx untuk mengubah status.
func LockTransferInTenant(ctx context.Context, tx *gorm.DB, id string, out *models.StockTransfer) error {
	if err := lockTenantRow[models.StockTransfer](ctx, tx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTransferNotFound
		}
		return err
	}
	return FindTransferInTenant(ctx, tx, id, out)
}

func ListTransfers(ctx context.Context, outletID string, limit, offset int) ([]models.StockTransfer, int64, error) {
	where, args := "", []any(nil)
	if outletID != "" {
		where = "from_outlet_id = ? OR to_outlet_id = ?"
		args = []any{outletID, outletID}
	}
	where, args = whereOutlet(ctx, where, args, "from_outlet_id", "to_outlet_id")
	return paginateTenant[models.StockTransfer](ctx, where, args, "created_at DESC, id DESC", limit, offset)
}

func SetTransferStatus(ctx context.Context, tx *gorm.DB, id, status string, extra map[string]any) error {
	upd := map[string]any{"status": status, "updated_at": gorm.Expr("now()")}
	for k, v := range extra {
		upd[k] = v
	}
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.StockTransfer{}).Where("id = ?", id).Updates(upd)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrTransferNotFound
	}
	return nil
}

// OpnameExtra: ringkasan satu sesi hitung fisik untuk riwayat.
type OpnameExtra struct {
	CreatedByName string
	ItemCount     int64 // barang yang dihitung
	ChangedCount  int64 // yang hitungannya beda dari catatan
	// ValueDiff: perubahan NILAI STOK, Σ (dihitung − max(catatan, 0)) × harga
	// modal SEKARANG. Catatan minus dihitung nol — sama dengan nilai stok di
	// ringkasan — sehingga mencocokkan catatan minus tidak tampil sebagai
	// "untung". Gerakan opname tidak menyimpan harga modal saat itu, jadi ini
	// perkiraan — layar menyebutnya "≈".
	ValueDiff int64
}

// OpnameExtras mengambil OpnameExtra untuk banyak sesi sekaligus.
func OpnameExtras(ctx context.Context, ids []string) (map[string]OpnameExtra, error) {
	out := make(map[string]OpnameExtra, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		ID string
		OpnameExtra
	}
	err := tenantDB(ctx, nil).Raw(`
		SELECT o.id, COALESCE(u.name, '') AS created_by_name,
			COUNT(i.id) AS item_count,
			COUNT(i.id) FILTER (WHERE i.diff_qty <> 0) AS changed_count,
			COALESCE(ROUND(SUM((i.counted_qty - GREATEST(i.system_qty, 0)) * p.cost_price)), 0)::bigint AS value_diff
		FROM stock_opnames o
		LEFT JOIN users u ON u.tenant_id = o.tenant_id AND u.id = o.created_by
		LEFT JOIN stock_opname_items i ON i.tenant_id = o.tenant_id AND i.opname_id = o.id
		LEFT JOIN products p ON p.tenant_id = i.tenant_id AND p.id = i.product_id
		WHERE o.tenant_id = ? AND o.id IN ?
		GROUP BY o.id, u.name`, reqctx.TenantID(ctx), ids).Scan(&rows).Error
	for _, r := range rows {
		out[r.ID] = r.OpnameExtra
	}
	return out, err
}

// OpnameItemInfo: nama, satuan dasar, dan harga modal sekarang satu baris
// hitungan — baris opname hanya menyimpan product_id.
type OpnameItemInfo struct {
	ProductName string
	UnitName    string
	CostPrice   int64
}

// OpnameItemInfos mengembalikan OpnameItemInfo per id baris hitungan.
func OpnameItemInfos(ctx context.Context, opnameID string) (map[string]OpnameItemInfo, error) {
	var rows []struct {
		ID string
		OpnameItemInfo
	}
	err := tenantDB(ctx, nil).Raw(`
		SELECT i.id, p.name AS product_name, COALESCE(un.name, '') AS unit_name, p.cost_price
		FROM stock_opname_items i
		JOIN products p ON p.tenant_id = i.tenant_id AND p.id = i.product_id
		LEFT JOIN units un ON un.tenant_id = p.tenant_id AND un.id = p.unit_id
		WHERE i.tenant_id = ? AND i.opname_id = ?`, reqctx.TenantID(ctx), opnameID).Scan(&rows).Error
	out := make(map[string]OpnameItemInfo, len(rows))
	for _, r := range rows {
		out[r.ID] = r.OpnameItemInfo
	}
	return out, err
}

// TransferExtra: keterangan tampilan satu transfer untuk daftar pengiriman.
type TransferExtra struct {
	FromOutletName string
	ToOutletName   string
	CreatedByName  string
	ItemCount      int64
	ItemNames      []string // maks 3 nama pertama
}

// TransferExtras mengambil TransferExtra untuk banyak transfer sekaligus.
func TransferExtras(ctx context.Context, ids []string) (map[string]TransferExtra, error) {
	out := make(map[string]TransferExtra, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	tid := reqctx.TenantID(ctx)
	var kepala []struct {
		ID             string
		FromOutletName string
		ToOutletName   string
		CreatedByName  string
	}
	if err := tenantDB(ctx, nil).Raw(`
		SELECT t.id, COALESCE(fo.name, '') AS from_outlet_name, COALESCE(tou.name, '') AS to_outlet_name,
			COALESCE(u.name, '') AS created_by_name
		FROM stock_transfers t
		LEFT JOIN outlets fo ON fo.tenant_id = t.tenant_id AND fo.id = t.from_outlet_id
		LEFT JOIN outlets tou ON tou.tenant_id = t.tenant_id AND tou.id = t.to_outlet_id
		LEFT JOIN users u ON u.tenant_id = t.tenant_id AND u.id = t.created_by
		WHERE t.tenant_id = ? AND t.id IN ?`, tid, ids).Scan(&kepala).Error; err != nil {
		return nil, err
	}
	for _, k := range kepala {
		out[k.ID] = TransferExtra{FromOutletName: k.FromOutletName, ToOutletName: k.ToOutletName, CreatedByName: k.CreatedByName}
	}
	var barang []struct {
		TransferID string
		ItemCount  int64
		Names      string
	}
	if err := tenantDB(ctx, nil).Raw(`
		SELECT i.transfer_id, COUNT(*) AS item_count,
			array_to_string((array_agg(p.name ORDER BY i.id))[1:3], ?) AS names
		FROM stock_transfer_items i
		JOIN products p ON p.tenant_id = i.tenant_id AND p.id = i.product_id
		WHERE i.tenant_id = ? AND i.transfer_id IN ?
		GROUP BY i.transfer_id`, namaSep, tid, ids).Scan(&barang).Error; err != nil {
		return nil, err
	}
	for _, b := range barang {
		e := out[b.TransferID]
		e.ItemCount = b.ItemCount
		if b.Names != "" {
			e.ItemNames = strings.Split(b.Names, namaSep)
		}
		out[b.TransferID] = e
	}
	return out, nil
}

// TransferItemNames: nama barang & satuan dasar per id baris transfer.
func TransferItemNames(ctx context.Context, transferID string) (map[string]OpnameItemInfo, error) {
	var rows []struct {
		ID string
		OpnameItemInfo
	}
	err := tenantDB(ctx, nil).Raw(`
		SELECT i.id, p.name AS product_name, COALESCE(un.name, '') AS unit_name, p.cost_price
		FROM stock_transfer_items i
		JOIN products p ON p.tenant_id = i.tenant_id AND p.id = i.product_id
		LEFT JOIN units un ON un.tenant_id = p.tenant_id AND un.id = p.unit_id
		WHERE i.tenant_id = ? AND i.transfer_id = ?`, reqctx.TenantID(ctx), transferID).Scan(&rows).Error
	out := make(map[string]OpnameItemInfo, len(rows))
	for _, r := range rows {
		out[r.ID] = r.OpnameItemInfo
	}
	return out, err
}
