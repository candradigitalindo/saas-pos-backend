package repositories

import (
	"context"
	"errors"
	"strings"

	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"

	"gorm.io/gorm"
)

var ErrPurchaseNotFound = errors.New("pembelian tidak ditemukan")

// CreatePurchase menyimpan pembelian + itemnya (tenant_id di-stempel).
func CreatePurchase(ctx context.Context, tx *gorm.DB, p *models.Purchase) error {
	tid := currentTenantID(ctx)
	p.TenantID = tid
	for i := range p.Items {
		p.Items[i].TenantID = tid
	}
	return tx.WithContext(ctx).Create(p).Error
}

// FindPurchaseInTenant memuat satu pembelian beserta itemnya. tx opsional.
func FindPurchaseInTenant(ctx context.Context, tx *gorm.DB, id string, out *models.Purchase) error {
	err := scopeTenant(ctx, tenantDB(ctx, tx)).Preload("Items").First(out, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrPurchaseNotFound
	}
	return err
}

// ListPurchases mengembalikan satu halaman pembelian (opsional per outlet).
func ListPurchases(ctx context.Context, outletID string, limit, offset int) ([]models.Purchase, int64, error) {
	where, args := "", []any(nil)
	if outletID != "" {
		where, args = "outlet_id = ?", []any{outletID}
	}
	where, args = whereOutlet(ctx, where, args, "outlet_id")
	return paginateTenant[models.Purchase](ctx, where, args, "occurred_at DESC, id DESC", limit, offset)
}

// SetProductLastCost menyetel harga modal produk ke unit_cost pembelian terakhir
// (metode last-cost). Dipakai setelah menerima pembelian.
func SetProductLastCost(ctx context.Context, tx *gorm.DB, productID string, cost int64) error {
	return scopeTenant(ctx, tenantDB(ctx, tx)).
		Model(&models.Product{}).
		Where("id = ?", productID).
		UpdateColumn("cost_price", cost).Error
}

// PurchaseExtra adalah keterangan tampilan satu pembelian yang tidak disimpan
// di barisnya sendiri: nama pemasok, nama pencatat, dan ringkasan barangnya.
type PurchaseExtra struct {
	SupplierName  string
	CreatedByName string
	ItemCount     int64
	// ItemNames: paling banyak tiga nama barang pertama — cukup untuk baris
	// riwayat ("Gula, Minyak, Kopi, +2 lagi").
	ItemNames []string
}

// namaSep memisahkan nama barang dalam string_agg — karakter kendali yang
// tidak mungkin diketik di nama barang.
const namaSep = "\x1f"

// PurchaseExtras mengambil PurchaseExtra untuk banyak pembelian sekaligus
// (dua query, bukan satu per baris). Barang yang sudah dihapus tetap disebut
// namanya: riwayat harus tetap terbaca.
func PurchaseExtras(ctx context.Context, ids []string) (map[string]PurchaseExtra, error) {
	out := make(map[string]PurchaseExtra, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	tid := reqctx.TenantID(ctx)

	var kepala []struct {
		ID            string
		SupplierName  string
		CreatedByName string
	}
	if err := tenantDB(ctx, nil).Raw(`
		SELECT pu.id, COALESCE(s.name, '') AS supplier_name, COALESCE(u.name, '') AS created_by_name
		FROM purchases pu
		LEFT JOIN suppliers s ON s.tenant_id = pu.tenant_id AND s.id = pu.supplier_id
		LEFT JOIN users u ON u.tenant_id = pu.tenant_id AND u.id = pu.created_by
		WHERE pu.tenant_id = ? AND pu.id IN ?`, tid, ids).Scan(&kepala).Error; err != nil {
		return nil, err
	}
	for _, k := range kepala {
		out[k.ID] = PurchaseExtra{SupplierName: k.SupplierName, CreatedByName: k.CreatedByName}
	}

	var barang []struct {
		PurchaseID string
		ItemCount  int64
		Names      string
	}
	if err := tenantDB(ctx, nil).Raw(`
		SELECT pi.purchase_id, COUNT(*) AS item_count,
			array_to_string((array_agg(p.name ORDER BY pi.id))[1:3], ?) AS names
		FROM purchase_items pi
		JOIN products p ON p.tenant_id = pi.tenant_id AND p.id = pi.product_id
		WHERE pi.tenant_id = ? AND pi.purchase_id IN ?
		GROUP BY pi.purchase_id`, namaSep, tid, ids).Scan(&barang).Error; err != nil {
		return nil, err
	}
	for _, b := range barang {
		e := out[b.PurchaseID]
		e.ItemCount = b.ItemCount
		if b.Names != "" {
			e.ItemNames = strings.Split(b.Names, namaSep)
		}
		out[b.PurchaseID] = e
	}
	return out, nil
}

// PurchaseItemName: nama barang (dan varian) serta satuan dasar satu baris
// pembelian — baris pembelian hanya menyimpan product_id.
type PurchaseItemName struct {
	ProductName  string
	VariantName  string
	BaseUnitName string
}

// PurchaseItemNames mengembalikan PurchaseItemName per id baris pembelian.
func PurchaseItemNames(ctx context.Context, purchaseID string) (map[string]PurchaseItemName, error) {
	var rows []struct {
		ID string
		PurchaseItemName
	}
	err := tenantDB(ctx, nil).Raw(`
		SELECT pi.id, p.name AS product_name, COALESCE(v.name, '') AS variant_name,
			COALESCE(un.name, '') AS base_unit_name
		FROM purchase_items pi
		JOIN products p ON p.tenant_id = pi.tenant_id AND p.id = pi.product_id
		LEFT JOIN product_variants v ON v.tenant_id = pi.tenant_id AND v.id = pi.variant_id
		LEFT JOIN units un ON un.tenant_id = p.tenant_id AND un.id = p.unit_id
		WHERE pi.tenant_id = ? AND pi.purchase_id = ?`, reqctx.TenantID(ctx), purchaseID).Scan(&rows).Error
	out := make(map[string]PurchaseItemName, len(rows))
	for _, r := range rows {
		out[r.ID] = r.PurchaseItemName
	}
	return out, err
}
