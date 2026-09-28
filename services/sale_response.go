package services

import (
	"encoding/json"
	"time"

	"candra/backend-api/models"
	"candra/backend-api/structs"
)

// saleTimeLayout: waktu di response SELALU UTC berformat RFC 3339 dengan penanda
// zona ("2026-09-26T00:22:10Z"). Format lama tanpa zona ("2006-01-02
// 15:04:05") ditafsirkan browser sebagai jam LOKAL perangkat, sehingga jam
// UTC dari server produksi tampil mundur 7 jam di layar WIB.
const saleTimeLayout = time.RFC3339

// SaleToResponse memetakan model Sale (dengan Items & Payments ter-load) ke DTO.
// Diekspor agar controller list/detail memakainya juga.
func SaleToResponse(s *models.Sale) structs.SaleResponse {
	r := structs.SaleResponse{
		ID:             s.ID,
		OutletID:       s.OutletID,
		ReceiptNo:      s.ReceiptNo,
		OrderType:      s.OrderType,
		Status:         s.Status,
		Subtotal:       s.Subtotal,
		DiscountAmount: s.DiscountAmount,
		TaxAmount:      s.TaxAmount,
		ServiceAmount:  s.ServiceAmount,
		RoundingAmount: s.RoundingAmount,
		Total:          s.Total,
		PaidAmount:     s.PaidAmount,
		ChangeAmount:   s.ChangeAmount,
		CostTotal:      s.CostTotal,
		GrossProfit:    s.Total - s.CostTotal,
		Note:           s.Note,
		OccurredAt:     s.OccurredAt.UTC().Format(saleTimeLayout),
		BusinessDate:   s.BusinessDate.Format("2006-01-02"),
		VoidReason:     s.VoidReason,
		CreatedAt:      s.CreatedAt.UTC().Format(saleTimeLayout),
	}
	if s.ShiftID != nil {
		r.ShiftID = *s.ShiftID
	}
	if s.CustomerID != nil {
		r.CustomerID = *s.CustomerID
	}
	if s.ReturnOfSaleID != nil {
		r.ReturnOfSaleID = *s.ReturnOfSaleID
	}
	if s.VoidedAt != nil {
		r.VoidedAt = s.VoidedAt.UTC().Format(saleTimeLayout)
	}

	for _, it := range s.Items {
		ir := structs.SaleItemResponse{
			ID: it.ID, ProductID: it.ProductID, ProductName: it.ProductName,
			UnitName: it.UnitName, Qty: it.Qty.String(),
			UnitPrice: it.UnitPrice, UnitCost: it.UnitCost,
			DiscountAmount: it.DiscountAmount, TaxAmount: it.TaxAmount,
			LineTotal: it.LineTotal, Note: it.Note,
		}
		if it.VariantID != nil {
			ir.VariantID = *it.VariantID
		}
		if it.ProductUnitID != nil {
			ir.ProductUnitID = *it.ProductUnitID
			ir.UnitConversion = it.UnitConversion.String()
		}
		r.Items = append(r.Items, ir)
	}
	for _, p := range s.Payments {
		r.Payments = append(r.Payments, structs.SalePaymentResponse{
			ID: p.ID, Method: p.Method, Amount: p.Amount, Reference: p.Reference,
			FeeAmount: p.FeeAmount, PaidAt: p.PaidAt.UTC().Format(saleTimeLayout),
		})
	}
	return r
}

// marshalSaleResponse membungkus SaleToResponse dalam SuccessResponse dan
// men-serialisasi ke JSON — bentuk yang disimpan ke idempotency_keys dan dikirim
// apa adanya ke klien.
func marshalSaleResponse(s *models.Sale, message string) ([]byte, error) {
	return json.Marshal(structs.SuccessResponse[structs.SaleResponse]{
		Success: true,
		Message: message,
		Data:    SaleToResponse(s),
	})
}
