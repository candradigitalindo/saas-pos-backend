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

	"gorm.io/gorm"
)

// Sumber uang pembayaran pemasok (000047).
const (
	SumberLaci = "drawer" // laci kasir → uang keluar shift yang sedang buka
	SumberLain = "other"  // dompet/rekening → laci tidak berubah
)

const idempotencyScopePurchasePay = "purchase.pay"

// SumberBayarSah melaporkan apakah sumber pembayaran dikenali.
func SumberBayarSah(s string) bool { return s == SumberLaci || s == SumberLain }

// catatBayarTx mencatat SATU pembayaran pembelian di dalam tx pemanggil:
// baris purchase_payments, dan — bila dari laci — uang keluar pada shift yang
// sedang buka di outlet pembelian. paid_amount TIDAK diubah di sini (pemanggil
// yang tahu angka akhirnya).
func catatBayarTx(ctx context.Context, tx *gorm.DB, p *models.Purchase, amount int64, source, note, namaPemasok string, now, bizDate time.Time) (*models.PurchasePayment, error) {
	if !SumberBayarSah(source) {
		return nil, fmt.Errorf("%w: sumber pembayaran harus drawer atau other", helpers.ErrValidation)
	}
	pay := models.PurchasePayment{
		PurchaseID: p.ID, OutletID: p.OutletID, Amount: amount, Source: source,
		Note: strings.TrimSpace(note), PaidAt: now, BusinessDate: bizDate, CreatedBy: reqctx.UserID(ctx),
	}
	if source == SumberLaci {
		// Mengambil dari laci = uang keluar shift; butuh izin yang sama dengan
		// layar Uang Masuk & Keluar, bukan hanya izin barang masuk.
		if !reqctx.HasPermission(ctx, "cash.movement") {
			return nil, fmt.Errorf("%w: membayar dari laci kasir butuh izin uang masuk & keluar", helpers.ErrForbidden)
		}
		alasan := "Bayar pemasok"
		if namaPemasok != "" {
			alasan += " " + namaPemasok
		}
		if p.InvoiceNo != "" {
			alasan += " (nota " + p.InvoiceNo + ")"
		}
		mv, err := catatKasTx(ctx, tx, p.OutletID, "", "out", amount, alasan)
		if err != nil {
			if strings.Contains(err.Error(), "belum ada shift terbuka") {
				return nil, fmt.Errorf("%w: kasir di toko ini belum dibuka — buka kasir dulu, atau pilih bayar dengan uang lain", helpers.ErrValidation)
			}
			return nil, err
		}
		pay.CashMovementID = &mv.ID
	}
	if err := repositories.CreatePurchasePayment(ctx, tx, &pay); err != nil {
		return nil, err
	}
	return &pay, nil
}

// namaPemasokTx: nama pemasok pembelian ("" bila tanpa pemasok).
func namaPemasokTx(ctx context.Context, tx *gorm.DB, p *models.Purchase) string {
	if p.SupplierID == nil {
		return ""
	}
	var s models.Supplier
	if err := repositories.FindSupplierInTenant(ctx, tx, *p.SupplierID, &s); err != nil {
		return ""
	}
	return s.Name
}

// PayPurchaseInput: pelunasan (sebagian/penuh) utang satu pembelian.
type PayPurchaseInput struct {
	PurchaseID     string
	Amount         int64
	Source         string
	Note           string
	IdempotencyKey string
	RequestHash    string
}

// PayPurchase mencatat pembayaran utang satu pembelian. Baris pembelian
// DIKUNCI: dua pembayaran bersamaan tidak boleh sama-sama melihat sisa lama
// dan melunasinya dua kali. Wajib Idempotency-Key: ini memindahkan uang.
func PayPurchase(ctx context.Context, in PayPurchaseInput) (int, []byte, error) {
	if in.IdempotencyKey == "" {
		return 0, nil, fmt.Errorf("%w: header Idempotency-Key wajib", helpers.ErrValidation)
	}
	if in.Amount <= 0 {
		return 0, nil, fmt.Errorf("%w: nominal pembayaran harus lebih dari 0", helpers.ErrValidation)
	}
	if !SumberBayarSah(in.Source) {
		return 0, nil, fmt.Errorf("%w: sumber pembayaran harus drawer atau other", helpers.ErrValidation)
	}

	var (
		outStatus int
		outBody   []byte
	)
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		m, err := repositories.LookupIdempotency(ctx, tx, idempotencyScopePurchasePay, in.IdempotencyKey, in.RequestHash)
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
		sisa := p.Total - p.PaidAmount
		if sisa <= 0 {
			return fmt.Errorf("%w: pembelian ini sudah lunas", helpers.ErrConflict)
		}
		if in.Amount > sisa {
			return fmt.Errorf("%w: pembayaran Rp %d melebihi sisa utang Rp %d", helpers.ErrValidation, in.Amount, sisa)
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
		if _, err := catatBayarTx(ctx, tx, &p, in.Amount, in.Source, in.Note, namaPemasokTx(ctx, tx, &p), now, bizDate); err != nil {
			return err
		}
		p.PaidAmount += in.Amount
		if err := repositories.SetPurchasePaid(ctx, tx, p.ID, p.PaidAmount); err != nil {
			return err
		}

		body, err := json.Marshal(structs.SuccessResponse[structs.PurchaseResponse]{
			Success: true, Message: "Pembayaran dicatat", Data: PurchaseToResponse(&p),
		})
		if err != nil {
			return err
		}
		if err := repositories.SaveIdempotency(ctx, tx, idempotencyScopePurchasePay, in.IdempotencyKey, in.RequestHash,
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
