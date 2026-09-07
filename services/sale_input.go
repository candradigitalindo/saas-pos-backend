package services

import (
	"fmt"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/structs"

	"github.com/shopspring/decimal"
)

// BuildCheckoutInput memetakan body checkout (structs.CheckoutRequest) ke
// CheckoutInput layanan. Dipakai bersama oleh handler HTTP POST /api/v1/sales
// dan oleh sinkronisasi offline (operasi "sale.create" di /sync/push) supaya
// kedua jalur menuju logika checkout yang sama persis.
//
// qty diparse sebagai desimal string di sini; qty yang bukan angka →
// helpers.ErrValidation.
func BuildCheckoutInput(req structs.CheckoutRequest, idempotencyKey, requestHash string) (CheckoutInput, error) {
	in := CheckoutInput{
		SaleID:         req.ID,
		OutletID:       req.OutletID,
		ShiftID:        req.ShiftID,
		CustomerID:     req.CustomerID,
		OrderType:      req.OrderType,
		OrderDiscount:  req.OrderDiscount,
		Note:           req.Note,
		IdempotencyKey: idempotencyKey,
		RequestHash:    requestHash,
	}
	if req.ClientCreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, req.ClientCreatedAt); err == nil {
			in.ClientCreatedAt = &t
		}
	}
	for _, it := range req.Items {
		qty, err := decimal.NewFromString(it.Qty)
		if err != nil {
			return in, fmt.Errorf("%w: qty %q bukan angka desimal yang valid", helpers.ErrValidation, it.Qty)
		}
		in.Items = append(in.Items, CheckoutItem{
			ProductID:      it.ProductID,
			VariantID:      it.VariantID,
			Qty:            qty,
			DiscountAmount: it.DiscountAmount,
			Note:           it.Note,
		})
	}
	for _, p := range req.Payments {
		in.Payments = append(in.Payments, CheckoutPayment{
			Method:    p.Method,
			Amount:    p.Amount,
			Reference: p.Reference,
		})
	}
	return in, nil
}
