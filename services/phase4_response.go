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
		OccurredAt:   p.OccurredAt.Format(saleTimeLayout),
		BusinessDate: p.BusinessDate.Format("2006-01-02"),
		CreatedAt:    p.CreatedAt.Format(saleTimeLayout),
	}
	for _, it := range p.Items {
		r.Items = append(r.Items, structs.PurchaseItemResponse{
			ID: it.ID, ProductID: it.ProductID, VariantID: ptrStr(it.VariantID),
			Qty: it.Qty.String(), UnitCost: it.UnitCost, LineTotal: it.LineTotal,
		})
	}
	return r
}

// OpnameToResponse memetakan StockOpname (+Items) ke DTO.
func OpnameToResponse(o *models.StockOpname) structs.OpnameResponse {
	r := structs.OpnameResponse{
		ID: o.ID, OutletID: o.OutletID, Status: o.Status, Note: o.Note,
		BusinessDate: o.BusinessDate.Format("2006-01-02"),
		CreatedAt:    o.CreatedAt.Format(saleTimeLayout),
	}
	if o.CountedAt != nil {
		r.CountedAt = o.CountedAt.Format(saleTimeLayout)
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
		CreatedAt:    t.CreatedAt.Format(saleTimeLayout),
	}
	if t.SentAt != nil {
		r.SentAt = t.SentAt.Format(saleTimeLayout)
	}
	if t.ReceivedAt != nil {
		r.ReceivedAt = t.ReceivedAt.Format(saleTimeLayout)
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
