package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/internal/timez"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const idempotencyScopePurchaseReturn = "purchase.return"

// PurchaseReturnLine: satu baris nota yang diretur (jumlah dalam satuan beli
// baris itu — dus bila dibeli per dus).
type PurchaseReturnLine struct {
	PurchaseItemID string
	Qty            decimal.Decimal
}

// PurchaseReturnInput: retur barang ke pemasok atas satu nota.
type PurchaseReturnInput struct {
	PurchaseID string
	Items      []PurchaseReturnLine
	Reason     string
	// RefundSource: ke mana uang dari pemasok masuk BILA ada yang harus
	// dikembalikan (nota sudah dibayar melebihi total barunya) — SumberLaci
	// (uang masuk shift) atau SumberLain. Wajib hanya bila ada pengembalian.
	RefundSource   string
	IdempotencyKey string
	RequestHash    string
}

// ReturnPurchase mencatat retur ke pemasok (000049): stok berkurang, total
// nota berkurang sebesar nilai retur, dan bila yang sudah dibayar melebihi
// total baru, pemasok mengembalikan selisihnya (paid_amount turun sebesar
// itu; ke laci = uang masuk shift). Baris pembelian DIKUNCI: dua retur
// bersamaan tidak boleh melampaui jumlah yang dibeli. Wajib Idempotency-Key.
func ReturnPurchase(ctx context.Context, in PurchaseReturnInput) (int, []byte, error) {
	if in.IdempotencyKey == "" {
		return 0, nil, fmt.Errorf("%w: header Idempotency-Key wajib", helpers.ErrValidation)
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if in.Reason == "" {
		return 0, nil, fmt.Errorf("%w: alasan retur wajib diisi", helpers.ErrValidation)
	}
	if len(in.Items) == 0 {
		return 0, nil, fmt.Errorf("%w: pilih barang yang diretur", helpers.ErrValidation)
	}

	var (
		outStatus int
		outBody   []byte
	)
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		m, err := repositories.LookupIdempotency(ctx, tx, idempotencyScopePurchaseReturn, in.IdempotencyKey, in.RequestHash)
		if err != nil {
			return err
		}
		if m.Found {
			if !m.SameRequest {
				return fmt.Errorf("%w: Idempotency-Key sudah dipakai untuk permintaan berbeda", helpers.ErrConflict)
			}
			outStatus, outBody = m.ResponseStatus, m.ResponseBody
			return nil
		}

		var p models.Purchase
		if err := repositories.LockPurchaseInTenant(ctx, tx, in.PurchaseID, &p); err != nil {
			return err
		}
		if err := ensureOutletAccess(ctx, p.OutletID); err != nil {
			return err
		}
		if p.Status != "received" {
			return fmt.Errorf("%w: nota ini tidak bisa diretur", helpers.ErrConflict)
		}
		sudah, err := repositories.ReturnedQtyByItem(ctx, tx, p.ID)
		if err != nil {
			return err
		}
		baris := make(map[string]models.PurchaseItem, len(p.Items))
		for _, it := range p.Items {
			baris[it.ID] = it
		}

		var (
			nilai   int64
			items   []models.PurchaseReturnItem
			deltas  []repositories.StockDelta
			dipakai = map[string]bool{}
		)
		for _, l := range in.Items {
			it, ok := baris[l.PurchaseItemID]
			if !ok {
				return fmt.Errorf("%w: baris %s bukan bagian nota ini", helpers.ErrValidation, l.PurchaseItemID)
			}
			if dipakai[it.ID] {
				return fmt.Errorf("%w: baris nota yang sama disebut dua kali", helpers.ErrValidation)
			}
			dipakai[it.ID] = true
			if l.Qty.LessThanOrEqual(decimal.Zero) {
				return fmt.Errorf("%w: jumlah retur harus lebih dari 0", helpers.ErrValidation)
			}
			sisa := it.Qty.Sub(sudah[it.ID])
			if l.Qty.GreaterThan(sisa) {
				return fmt.Errorf("%w: retur %s melebihi yang tersisa di nota (%s)", helpers.ErrValidation, l.Qty, sisa)
			}
			konversi := it.UnitConversion
			if konversi.LessThanOrEqual(decimal.Zero) {
				konversi = decimal.NewFromInt(1)
			}
			total := helpers.LineAmount(l.Qty, it.UnitCost)
			nilai += total
			items = append(items, models.PurchaseReturnItem{
				PurchaseItemID: it.ID, ProductID: it.ProductID, Qty: l.Qty,
				UnitConversion: konversi, UnitCost: it.UnitCost, LineTotal: total,
			})
			vid := ""
			if it.VariantID != nil {
				vid = *it.VariantID
			}
			deltas = append(deltas, repositories.StockDelta{
				ProductID: it.ProductID, VariantID: vid, Delta: l.Qty.Mul(konversi).Neg(),
				UnitCost: decimal.NewFromInt(it.UnitCost).Div(konversi).Round(0).IntPart(),
			})
		}
		if nilai <= 0 {
			return fmt.Errorf("%w: nilai retur Rp 0", helpers.ErrValidation)
		}
		if nilai > p.Total {
			return fmt.Errorf("%w: nilai retur melebihi total nota", helpers.ErrValidation)
		}

		totalBaru := p.Total - nilai
		kembali := p.PaidAmount - totalBaru
		if kembali < 0 {
			kembali = 0
		}

		var outlet models.Outlet
		if err := repositories.FindOutletByID(ctx, tx, p.OutletID, &outlet); err != nil {
			return err
		}
		now := time.Now().UTC()
		bizDate, err := timez.BusinessDate(now, outlet.Timezone, outlet.DayStartOffset())
		if err != nil {
			return err
		}

		ret := models.PurchaseReturn{
			OutletID: p.OutletID, PurchaseID: p.ID, Reason: in.Reason, Total: nilai,
			RefundAmount: kembali, OccurredAt: now, BusinessDate: bizDate, CreatedBy: reqctx.UserID(ctx),
			Items: items,
		}
		if kembali > 0 {
			if !SumberBayarSah(in.RefundSource) {
				return fmt.Errorf("%w: nota sudah dibayar — pilih ke mana uang Rp %d dari pemasok masuk (drawer/other)",
					helpers.ErrValidation, kembali)
			}
			src := in.RefundSource
			ret.RefundSource = &src
			if src == SumberLaci {
				if !reqctx.HasPermission(ctx, "cash.movement") {
					return fmt.Errorf("%w: memasukkan uang ke laci kasir butuh izin uang masuk & keluar", helpers.ErrForbidden)
				}
				alasan := "Retur ke pemasok"
				if n := namaPemasokTx(ctx, tx, &p); n != "" {
					alasan += " " + n
				}
				if p.InvoiceNo != "" {
					alasan += " (nota " + p.InvoiceNo + ")"
				}
				mv, err := catatKasTx(ctx, tx, p.OutletID, "", "in", kembali, alasan)
				if err != nil {
					if strings.Contains(err.Error(), "belum ada shift terbuka") {
						return fmt.Errorf("%w: kasir di toko ini belum dibuka — buka kasir dulu, atau pilih uang lain", helpers.ErrValidation)
					}
					return err
				}
				ret.CashMovementID = &mv.ID
			}
		}

		if err := repositories.CreatePurchaseReturn(ctx, tx, &ret); err != nil {
			return err
		}
		if _, err := repositories.ApplyStockDeltas(ctx, tx, p.OutletID, deltas, repositories.MovementMeta{
			Kind: "purchase_return", RefTable: "purchase_returns", RefID: ret.ID,
			Reason: in.Reason, OccurredAt: now, BusinessDate: bizDate,
		}); err != nil {
			return err
		}
		p.Total, p.ReturnedAmount, p.PaidAmount = totalBaru, p.ReturnedAmount+nilai, p.PaidAmount-kembali
		if err := repositories.SetPurchaseAfterReturn(ctx, tx, p.ID, p.Total, p.ReturnedAmount, p.PaidAmount); err != nil {
			return err
		}

		body, err := json.Marshal(structs.SuccessResponse[structs.PurchaseReturnResponse]{
			Success: true, Message: "Retur dicatat",
			Data: structs.PurchaseReturnResponse{
				ID: ret.ID, PurchaseID: p.ID, Total: nilai, RefundAmount: kembali, RefundSource: ptrStr(ret.RefundSource),
				PurchaseTotal: p.Total, PurchaseOutstanding: p.Total - p.PaidAmount,
			},
		})
		if err != nil {
			return err
		}
		if err := repositories.SaveIdempotency(ctx, tx, idempotencyScopePurchaseReturn, in.IdempotencyKey, in.RequestHash,
			http.StatusCreated, body, idempotencyTTL()); err != nil {
			return err
		}
		outStatus, outBody = http.StatusCreated, body
		return nil
	})
	if err != nil {
		return 0, nil, err
	}
	return outStatus, outBody, nil
}
