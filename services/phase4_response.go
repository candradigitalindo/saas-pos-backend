package services

import (
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"
)

func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// PurchaseToResponse memetakan Purchase (+Items) ke DTO.
func PurchaseToResponse(p *models.Purchase) structs.PurchaseResponse {
	r := structs.PurchaseResponse{
		ID: p.ID, OutletID: p.OutletID, SupplierID: ptrStr(p.SupplierID),
		InvoiceNo: p.InvoiceNo, Status: p.Status,
		Subtotal: p.Subtotal, DiscountAmount: p.DiscountAmount, TaxAmount: p.TaxAmount,
		Total: p.Total, PaidAmount: p.PaidAmount,
		OccurredAt:   p.OccurredAt.UTC().Format(saleTimeLayout),
		BusinessDate: p.BusinessDate.Format("2006-01-02"),
		CreatedAt:    p.CreatedAt.UTC().Format(saleTimeLayout),
	}
	for _, it := range p.Items {
		r.Items = append(r.Items, structs.PurchaseItemResponse{
			ID: it.ID, ProductID: it.ProductID, VariantID: ptrStr(it.VariantID),
			Qty: it.Qty.String(), UnitCost: it.UnitCost, LineTotal: it.LineTotal,
			UnitName: it.UnitName, UnitConversion: it.UnitConversion.String(),
		})
	}
	return r
}

// OpnameToResponse memetakan StockOpname (+Items) ke DTO.
func OpnameToResponse(o *models.StockOpname) structs.OpnameResponse {
	r := structs.OpnameResponse{
		ID: o.ID, OutletID: o.OutletID, Status: o.Status, Note: o.Note,
		BusinessDate: o.BusinessDate.Format("2006-01-02"),
		CreatedAt:    o.CreatedAt.UTC().Format(saleTimeLayout),
	}
	if o.CountedAt != nil {
		r.CountedAt = o.CountedAt.UTC().Format(saleTimeLayout)
	}
	for _, it := range o.Items {
		r.Items = append(r.Items, structs.OpnameItemResponse{
			ID: it.ID, ProductID: it.ProductID, VariantID: ptrStr(it.VariantID),
			SystemQty: it.SystemQty.String(), CountedQty: it.CountedQty.String(), DiffQty: it.DiffQty.String(),
		})
	}
	return r
}

// TransferToResponse memetakan StockTransfer (+Items) ke DTO.
func TransferToResponse(t *models.StockTransfer) structs.TransferResponse {
	r := structs.TransferResponse{
		ID: t.ID, FromOutletID: t.FromOutletID, ToOutletID: t.ToOutletID,
		Status: t.Status, Note: t.Note,
		BusinessDate: t.BusinessDate.Format("2006-01-02"),
		CreatedAt:    t.CreatedAt.UTC().Format(saleTimeLayout),
	}
	if t.SentAt != nil {
		r.SentAt = t.SentAt.UTC().Format(saleTimeLayout)
	}
	if t.ReceivedAt != nil {
		r.ReceivedAt = t.ReceivedAt.UTC().Format(saleTimeLayout)
	}
	for _, it := range t.Items {
		r.Items = append(r.Items, structs.TransferItemResponse{
			ID: it.ID, ProductID: it.ProductID, VariantID: ptrStr(it.VariantID), Qty: it.Qty.String(),
		})
	}
	return r
}

// RecipeToResponse memetakan resep+bahan ke DTO.
func RecipeToResponse(rw repositories.RecipeWithItems) structs.RecipeResponse {
	r := structs.RecipeResponse{
		ID: rw.Recipe.ID, ProductID: rw.Recipe.ProductID, YieldQty: rw.Recipe.YieldQty.String(),
		Items: []structs.RecipeItemResponse{},
	}
	for _, it := range rw.Items {
		r.Items = append(r.Items, structs.RecipeItemResponse{
			ID: it.ID, IngredientProductID: it.IngredientProductID, Qty: it.Qty.String(),
		})
	}
	return r
}

// ReceivableToResponse memetakan Receivable ke DTO. Diekspor agar controller
// daftar/detail piutang dan balasan setoran (yang disimpan untuk replay
// idempotensi) memakai bentuk yang sama.
func ReceivableToResponse(r models.Receivable) structs.ReceivableResponse {
	out := structs.ReceivableResponse{
		ID:          r.ID,
		CustomerID:  r.CustomerID,
		SourceTable: r.SourceTable,
		SourceID:    r.SourceID,
		Amount:      r.Amount,
		PaidAmount:  r.PaidAmount,
		Outstanding: r.Outstanding(),
		Status:      r.Status,
		CreatedAt:   r.CreatedAt.UTC().Format(saleTimeLayout),
	}
	if r.DueDate != nil {
		out.DueDate = r.DueDate.Format("2006-01-02")
	}
	return out
}

// StockMovementToResponse memetakan satu gerakan stok ke DTO (kartu stok &
// balasan penyesuaian stok).
func StockMovementToResponse(m models.StockMovement) structs.StockMovementResponse {
	out := structs.StockMovementResponse{
		ID:           m.ID,
		OutletID:     m.OutletID,
		ProductID:    m.ProductID,
		Kind:         m.Kind,
		QtyDelta:     m.QtyDelta.String(),
		BalanceAfter: m.BalanceAfter.String(),
		UnitCost:     m.UnitCost,
		RefTable:     m.RefTable,
		Reason:       m.Reason,
		OccurredAt:   m.OccurredAt.UTC().Format(saleTimeLayout),
		BusinessDate: m.BusinessDate.Format("2006-01-02"),
	}
	if m.VariantID != nil {
		out.VariantID = *m.VariantID
	}
	if m.RefID != nil {
		out.RefID = *m.RefID
	}
	return out
}
