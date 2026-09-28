package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Tagihan terbuka (open bill / tahan transaksi).
//
// Aturan yang dijaga di sini:
//   - Tagihan disimpan UTUH (dokumen), dengan pemeriksaan versi: perubahan yang
//     dibuat dari versi basi ditolak 409, bukan menimpa pesanan perangkat lain.
//   - Operasi dari antrean offline membawa id operasi; dikirim ulang (jaringan
//     putus setelah server menyimpan) → dikenali lewat last_op_id = duplikat.
//   - Tagihan ditutup DI DALAM transaksi checkout (closeOpenBillForSale), jadi
//     dua kasir yang menagih meja yang sama tidak bisa sama-sama berhasil.

// errTagihanBentrok: versi yang dibaca klien sudah basi.
var errTagihanBentrok = fmt.Errorf("%w: tagihan ini sudah diubah di perangkat lain — muat ulang daftar tagihan", helpers.ErrConflict)

// ListOpenBills: tagihan terbuka di satu cabang, dengan nama pencatat.
func ListOpenBills(ctx context.Context, outletID string) ([]structs.OpenBillResponse, error) {
	if err := ensureOutletAccess(ctx, outletID); err != nil {
		return nil, err
	}
	rows, err := repositories.ListOpenBills(ctx, outletID)
	if err != nil {
		return nil, err
	}
	return openBillsToResponse(ctx, rows)
}

// UpsertOpenBill membuat (base_version 0) atau mengganti isi satu tagihan.
// opID diisi untuk operasi antrean offline (idempotensi); "" untuk HTTP.
// duplicate = operasi ini sudah pernah diterapkan.
func UpsertOpenBill(ctx context.Context, id, opID string, in structs.OpenBillUpsertRequest) (out structs.OpenBillResponse, duplicate bool, err error) {
	if err = ensureOutletAccess(ctx, in.OutletID); err != nil {
		return out, false, err
	}
	label := strings.TrimSpace(in.Label)
	if label == "" {
		return out, false, fmt.Errorf("%w: nama tagihan wajib diisi (mis. \"Meja 5\")", helpers.ErrValidation)
	}
	items, err := normalOpenBillItems(in.Items)
	if err != nil {
		return out, false, err
	}
	var diskon json.RawMessage
	if d := in.OrderDiscount; d != nil && d.Value > 0 {
		if d.Kind != "nominal" && d.Kind != "percent" {
			return out, false, fmt.Errorf("%w: jenis diskon tagihan harus nominal atau percent", helpers.ErrValidation)
		}
		if d.Kind == "percent" && d.Value > 100 {
			return out, false, fmt.Errorf("%w: diskon tagihan paling banyak 100%%", helpers.ErrValidation)
		}
		diskon, _ = json.Marshal(d)
	}
	uid := reqctx.UserID(ctx)
	now := time.Now().UTC()

	var bill models.OpenBill
	err = repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		lama, ferr := repositories.FindOpenBillForUpdate(ctx, tx, id)
		switch {
		case errors.Is(ferr, repositories.ErrOpenBillNotFound):
			if in.BaseVersion != 0 {
				return fmt.Errorf("%w: tagihan tidak ditemukan", helpers.ErrNotFound)
			}
			bill = models.OpenBill{
				ID: id, OutletID: in.OutletID, Label: label, CustomerID: nilIfEmpty(in.CustomerID),
				OrderType: orDefault(in.OrderType, "dine_in"), Items: items, OrderDiscount: diskon,
				Note: strings.TrimSpace(in.Note), Status: "open", Version: 1, LastOpID: nilIfEmpty(opID),
				CreatedBy: uid, UpdatedBy: uid, CreatedAt: now, UpdatedAt: now,
			}
			if cerr := repositories.CreateOpenBill(ctx, tx, &bill); helpers.IsDuplicateEntryError(cerr) {
				// id ULID global; bentrok hanya bila id yang sama dipakai
				// tenant lain (tidak terlihat oleh scopeTenant di atas).
				return fmt.Errorf("%w: id tagihan sudah dipakai", helpers.ErrConflict)
			} else if cerr != nil {
				return cerr
			}
			return nil
		case ferr != nil:
			return ferr
		}
		if opID != "" && lama.LastOpID != nil && *lama.LastOpID == opID {
			bill, duplicate = lama, true
			return nil
		}
		if lama.OutletID != in.OutletID {
			return fmt.Errorf("%w: tagihan milik cabang lain", helpers.ErrValidation)
		}
		if lama.Status != "open" {
			return tagihanTertutup(lama)
		}
		if in.BaseVersion != lama.Version {
			return errTagihanBentrok
		}
		bill = lama
		bill.Label, bill.CustomerID = label, nilIfEmpty(in.CustomerID)
		bill.OrderType = orDefault(in.OrderType, lama.OrderType)
		bill.Items, bill.OrderDiscount, bill.Note = items, diskon, strings.TrimSpace(in.Note)
		bill.Version++
		bill.LastOpID, bill.UpdatedBy, bill.UpdatedAt = nilIfEmpty(opID), uid, now
		return repositories.SaveOpenBill(ctx, tx, &bill)
	})
	if err != nil {
		return out, false, err
	}
	res, err := openBillsToResponse(ctx, []models.OpenBill{bill})
	if err != nil {
		return out, false, err
	}
	return res[0], duplicate, nil
}

// CancelOpenBill membatalkan tagihan terbuka. Tidak dihapus: status
// 'canceled' + siapa & kapan tetap tercatat.
func CancelOpenBill(ctx context.Context, id, opID string, baseVersion int) (duplicate bool, err error) {
	err = repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		b, ferr := repositories.FindOpenBillForUpdate(ctx, tx, id)
		if errors.Is(ferr, repositories.ErrOpenBillNotFound) {
			return fmt.Errorf("%w: tagihan tidak ditemukan", helpers.ErrNotFound)
		}
		if ferr != nil {
			return ferr
		}
		if err := ensureOutletAccess(ctx, b.OutletID); err != nil {
			return err
		}
		if opID != "" && b.LastOpID != nil && *b.LastOpID == opID {
			duplicate = true
			return nil
		}
		if b.Status != "open" {
			return tagihanTertutup(b)
		}
		if baseVersion != b.Version {
			return errTagihanBentrok
		}
		uid, now := reqctx.UserID(ctx), time.Now().UTC()
		b.Status, b.Version = "canceled", b.Version+1
		b.LastOpID, b.UpdatedBy, b.UpdatedAt = nilIfEmpty(opID), uid, now
		b.ClosedBy, b.ClosedAt = &uid, &now
		return repositories.SaveOpenBill(ctx, tx, &b)
	})
	return duplicate, err
}

// lockOpenBillForCheckout mengunci tagihan yang akan dibayar, di dalam tx
// checkout. Mengembalikan nil bila tagihan tidak ada (mis. dibuat offline lalu
// ditolak) — penjualannya tetap dicatat: uangnya sudah diterima. Tagihan yang
// SUDAH DIBAYAR menolak checkout (409): itu tanda meja yang sama ditagih dua
// kali. Tagihan yang dibatalkan tetap boleh dibayar — pembayaran yang nyata
// mengalahkan pembatalan dari perangkat lain.
func lockOpenBillForCheckout(ctx context.Context, tx *gorm.DB, id, outletID string) (*models.OpenBill, error) {
	b, err := repositories.FindOpenBillForUpdate(ctx, tx, id)
	if errors.Is(err, repositories.ErrOpenBillNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if b.OutletID != outletID {
		return nil, fmt.Errorf("%w: tagihan milik cabang lain", helpers.ErrValidation)
	}
	if b.Status == "paid" {
		return nil, fmt.Errorf("%w: tagihan %q sudah dibayar", helpers.ErrConflict, b.Label)
	}
	return &b, nil
}

// closeOpenBillForSale menandai tagihan dibayar oleh penjualan saleID. Dipanggil
// setelah baris sales tertulis (FK sale_id), di tx yang sama.
func closeOpenBillForSale(ctx context.Context, tx *gorm.DB, b *models.OpenBill, saleID string) error {
	uid, now := reqctx.UserID(ctx), time.Now().UTC()
	b.Status, b.SaleID, b.Version = "paid", &saleID, b.Version+1
	b.UpdatedBy, b.UpdatedAt, b.ClosedBy, b.ClosedAt = uid, now, &uid, &now
	return repositories.SaveOpenBill(ctx, tx, b)
}

func tagihanTertutup(b models.OpenBill) error {
	if b.Status == "paid" {
		return fmt.Errorf("%w: tagihan %q sudah dibayar", helpers.ErrConflict, b.Label)
	}
	return fmt.Errorf("%w: tagihan %q sudah dibatalkan", helpers.ErrConflict, b.Label)
}

// normalOpenBillItems memeriksa & merapikan baris tagihan lalu menyandikannya
// sebagai JSON. Barang/varian TIDAK diperiksa ke basis data di sini: checkout
// yang menegakkannya (dan barang yang dihapus di antaranya ketahuan di sana).
func normalOpenBillItems(in []structs.OpenBillItemRequest) (json.RawMessage, error) {
	items := make([]models.OpenBillItem, 0, len(in))
	for _, it := range in {
		q, err := decimal.NewFromString(it.Qty)
		if err != nil || !q.IsPositive() {
			return nil, fmt.Errorf("%w: jumlah %q tidak valid", helpers.ErrValidation, it.Qty)
		}
		items = append(items, models.OpenBillItem{
			ProductID: it.ProductID, VariantID: it.VariantID, ProductUnitID: it.ProductUnitID, Qty: q.String(),
			DiscountAmount: it.DiscountAmount, DiscountPercent: it.DiscountPercent,
			Note: strings.TrimSpace(it.Note),
		})
	}
	return json.Marshal(items)
}

func openBillsToResponse(ctx context.Context, rows []models.OpenBill) ([]structs.OpenBillResponse, error) {
	ids := make([]string, 0, len(rows)*2)
	for _, b := range rows {
		ids = append(ids, b.CreatedBy, b.UpdatedBy)
	}
	nama, err := repositories.UserNames(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]structs.OpenBillResponse, 0, len(rows))
	for _, b := range rows {
		r := structs.OpenBillResponse{
			ID: b.ID, OutletID: b.OutletID, Label: b.Label, OrderType: b.OrderType,
			Items: []models.OpenBillItem{}, Note: b.Note, Status: b.Status, Version: b.Version,
			CreatedBy: b.CreatedBy, CreatedByName: nama[b.CreatedBy], UpdatedByName: nama[b.UpdatedBy],
			CreatedAt: b.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: b.UpdatedAt.UTC().Format(time.RFC3339),
		}
		if b.CustomerID != nil {
			r.CustomerID = *b.CustomerID
		}
		if b.SaleID != nil {
			r.SaleID = *b.SaleID
		}
		if len(b.Items) > 0 {
			if err := json.Unmarshal(b.Items, &r.Items); err != nil {
				return nil, err
			}
		}
		if len(b.OrderDiscount) > 0 && string(b.OrderDiscount) != "null" {
			var d models.OpenBillDiscount
			if err := json.Unmarshal(b.OrderDiscount, &d); err != nil {
				return nil, err
			}
			r.OrderDiscount = &d
		}
		out = append(out, r)
	}
	return out, nil
}
