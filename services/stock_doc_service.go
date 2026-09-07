package services

import (
	"context"
	"fmt"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ── Stok opname ────────────────────────────────────────────────────────────

// CreateOpname membuka sesi hitung fisik untuk sebuah outlet.
func CreateOpname(ctx context.Context, outletID, note string) (*models.StockOpname, error) {
	var outlet models.Outlet
	if err := repositories.FindOutletByID(ctx, nil, outletID, &outlet); err != nil {
		return nil, fmt.Errorf("%w: outlet tidak ditemukan", helpers.ErrValidation)
	}
	bizDate, err := saleBusinessDate(ctx, nil, outletID)
	if err != nil {
		return nil, err
	}
	o := models.StockOpname{
		OutletID: outletID, Status: "draft", Note: note,
		BusinessDate: bizDate, CreatedBy: reqctx.UserID(ctx),
	}
	if err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		return repositories.CreateOpname(ctx, tx, &o)
	}); err != nil {
		return nil, err
	}
	return &o, nil
}

// OpnameCountInput satu hasil hitung fisik.
type OpnameCountInput struct {
	ProductID  string
	VariantID  string
	CountedQty decimal.Decimal
}

// SetOpnameItems menyimpan hasil hitung; system_qty di-snapshot dari saldo cache
// SEKARANG, diff = counted - system. Hanya untuk opname berstatus 'draft'.
func SetOpnameItems(ctx context.Context, opnameID string, counts []OpnameCountInput) (*models.StockOpname, error) {
	var result models.StockOpname
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		var o models.StockOpname
		if err := repositories.FindOpnameInTenant(ctx, tx, opnameID, &o); err != nil {
			return err
		}
		if o.Status != "draft" {
			return fmt.Errorf("%w: opname sudah diposting", helpers.ErrConflict)
		}
		for _, c := range counts {
			sys, err := repositories.CurrentStockQty(ctx, tx, o.OutletID, c.ProductID, c.VariantID)
			if err != nil {
				return err
			}
			item := &models.StockOpnameItem{
				OpnameID: o.ID, ProductID: c.ProductID,
				SystemQty: sys, CountedQty: c.CountedQty, DiffQty: c.CountedQty.Sub(sys),
			}
			if c.VariantID != "" {
				vid := c.VariantID
				item.VariantID = &vid
			}
			if err := repositories.UpsertOpnameItem(ctx, tx, item); err != nil {
				return err
			}
		}
		return repositories.FindOpnameInTenant(ctx, tx, opnameID, &result)
	})
	return &result, err
}

// PostOpname mengunci opname dan menulis selisih tiap item sebagai gerakan
// kind='opname'; saldo cache disamakan (§13.2 disiplin: pembalikan/penyesuaian
// selalu baris baru).
func PostOpname(ctx context.Context, opnameID string) (*models.StockOpname, error) {
	var result models.StockOpname
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		var o models.StockOpname
		if err := repositories.FindOpnameInTenant(ctx, tx, opnameID, &o); err != nil {
			return err
		}
		if o.Status != "draft" {
			return fmt.Errorf("%w: opname sudah diposting", helpers.ErrConflict)
		}
		if len(o.Items) == 0 {
			return fmt.Errorf("%w: belum ada item hitungan", helpers.ErrValidation)
		}

		deltas := make([]repositories.StockDelta, 0, len(o.Items))
		for _, it := range o.Items {
			if it.DiffQty.IsZero() {
				continue
			}
			vid := ""
			if it.VariantID != nil {
				vid = *it.VariantID
			}
			deltas = append(deltas, repositories.StockDelta{
				ProductID: it.ProductID, VariantID: vid, Delta: it.DiffQty, Kind: "opname",
			})
		}
		now := time.Now().UTC()
		if _, err := repositories.ApplyStockDeltas(ctx, tx, o.OutletID, deltas, repositories.MovementMeta{
			Kind: "opname", RefTable: "stock_opnames", RefID: o.ID,
			Reason: "stok opname", OccurredAt: now, BusinessDate: o.BusinessDate,
		}); err != nil {
			return err
		}
		if err := repositories.SetOpnameStatus(ctx, tx, o.ID, "posted", now); err != nil {
			return err
		}
		return repositories.FindOpnameInTenant(ctx, tx, opnameID, &result)
	})
	return &result, err
}

// ── Transfer stok ─────────────────────────────────────────────────────────

// TransferItemInput satu baris transfer.
type TransferItemInput struct {
	ProductID string
	VariantID string
	Qty       decimal.Decimal
}

// CreateTransfer membuat draft transfer antar outlet.
func CreateTransfer(ctx context.Context, fromOutlet, toOutlet, note string, items []TransferItemInput) (*models.StockTransfer, error) {
	if fromOutlet == toOutlet {
		return nil, fmt.Errorf("%w: outlet asal dan tujuan tidak boleh sama", helpers.ErrValidation)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("%w: tidak ada item", helpers.ErrValidation)
	}
	var result models.StockTransfer
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		for _, oid := range []string{fromOutlet, toOutlet} {
			var o models.Outlet
			if err := repositories.FindOutletByID(ctx, tx, oid, &o); err != nil {
				return fmt.Errorf("%w: outlet tidak ditemukan", helpers.ErrValidation)
			}
		}
		bizDate, err := saleBusinessDate(ctx, tx, fromOutlet)
		if err != nil {
			return err
		}
		tr := models.StockTransfer{
			FromOutletID: fromOutlet, ToOutletID: toOutlet, Status: "draft", Note: note,
			BusinessDate: bizDate, CreatedBy: reqctx.UserID(ctx),
		}
		for _, it := range items {
			if it.Qty.LessThanOrEqual(decimal.Zero) {
				return fmt.Errorf("%w: qty harus > 0", helpers.ErrValidation)
			}
			ti := models.StockTransferItem{ProductID: it.ProductID, Qty: it.Qty}
			if it.VariantID != "" {
				vid := it.VariantID
				ti.VariantID = &vid
			}
			tr.Items = append(tr.Items, ti)
		}
		if err := repositories.CreateTransfer(ctx, tx, &tr); err != nil {
			return err
		}
		return repositories.FindTransferInTenant(ctx, tx, tr.ID, &result)
	})
	return &result, err
}

// SendTransfer memindahkan status draft→sent dan menulis gerakan
// kind='transfer_out' di outlet asal.
func SendTransfer(ctx context.Context, id string) (*models.StockTransfer, error) {
	return advanceTransfer(ctx, id, "draft", "sent", "transfer_out", func(qty decimal.Decimal) decimal.Decimal { return qty.Neg() }, "sent_at")
}

// ReceiveTransfer memindahkan status sent→received dan menulis gerakan
// kind='transfer_in' di outlet tujuan.
func ReceiveTransfer(ctx context.Context, id string) (*models.StockTransfer, error) {
	return advanceTransfer(ctx, id, "sent", "received", "transfer_in", func(qty decimal.Decimal) decimal.Decimal { return qty }, "received_at")
}

func advanceTransfer(ctx context.Context, id, fromStatus, toStatus, kind string, sign func(decimal.Decimal) decimal.Decimal, tsField string) (*models.StockTransfer, error) {
	var result models.StockTransfer
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		var tr models.StockTransfer
		if err := repositories.FindTransferInTenant(ctx, tx, id, &tr); err != nil {
			return err
		}
		if tr.Status != fromStatus {
			return fmt.Errorf("%w: status transfer harus '%s'", helpers.ErrConflict, fromStatus)
		}

		outletID := tr.FromOutletID
		if kind == "transfer_in" {
			outletID = tr.ToOutletID
		}
		deltas := make([]repositories.StockDelta, 0, len(tr.Items))
		for _, it := range tr.Items {
			vid := ""
			if it.VariantID != nil {
				vid = *it.VariantID
			}
			deltas = append(deltas, repositories.StockDelta{
				ProductID: it.ProductID, VariantID: vid, Delta: sign(it.Qty), Kind: kind,
			})
		}
		now := time.Now().UTC()
		if _, err := repositories.ApplyStockDeltas(ctx, tx, outletID, deltas, repositories.MovementMeta{
			Kind: kind, RefTable: "stock_transfers", RefID: tr.ID, OccurredAt: now, BusinessDate: tr.BusinessDate,
		}); err != nil {
			return err
		}
		if err := repositories.SetTransferStatus(ctx, tx, tr.ID, toStatus, map[string]any{tsField: now}); err != nil {
			return err
		}
		return repositories.FindTransferInTenant(ctx, tx, id, &result)
	})
	return &result, err
}
