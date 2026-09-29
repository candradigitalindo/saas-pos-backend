package controllers

import (
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"
)

func customerToResponse(c models.Customer) structs.CustomerResponse {
	return structs.CustomerResponse{
		ID:          c.ID,
		Code:        deref(c.Code),
		Name:        c.Name,
		Phone:       deref(c.Phone),
		Email:       c.Email,
		Address:     c.Address,
		Type:        c.Type,
		CreditLimit: c.CreditLimit,
		Note:        c.Note,
		PriceListID: deref(c.PriceListID),
		CreatedAt:   c.CreatedAt.UTC().Format(timeLayout),
		UpdatedAt:   c.UpdatedAt.UTC().Format(timeLayout),
	}
}

func shiftToResponse(s models.Shift) structs.ShiftResponse {
	r := structs.ShiftResponse{
		ID:           s.ID,
		OutletID:     s.OutletID,
		Status:       s.Status,
		OpenedBy:     s.OpenedBy,
		OpenedAt:     s.OpenedAt.UTC().Format(timeLayout),
		BusinessDate: s.BusinessDate.Format("2006-01-02"),
		OpeningCash:  s.OpeningCash,
		ExpectedCash: s.ExpectedCash,
		CountedCash:  s.CountedCash,
		Difference:   s.Difference,
		Note:         s.Note,
	}
	if s.ClosedBy != nil {
		r.ClosedBy = *s.ClosedBy
	}
	if s.ClosedAt != nil {
		r.ClosedAt = s.ClosedAt.UTC().Format(timeLayout)
	}
	return r
}

func cashMovementToResponse(m models.CashMovement) structs.CashMovementResponse {
	return structs.CashMovementResponse{
		ID:           m.ID,
		OutletID:     m.OutletID,
		ShiftID:      m.ShiftID,
		Direction:    m.Direction,
		Amount:       m.Amount,
		Reason:       m.Reason,
		OccurredAt:   m.OccurredAt.UTC().Format(timeLayout),
		BusinessDate: m.BusinessDate.Format("2006-01-02"),
	}
}

func stockRowToResponse(s repositories.StockRow) structs.StockResponse {
	var lastSold *string
	if s.LastSoldAt != nil {
		t := s.LastSoldAt.UTC().Format(timeLayout)
		lastSold = &t
	}
	return structs.StockResponse{
		OutletID:    s.OutletID,
		ProductID:   s.ProductID,
		VariantID:   s.VariantID,
		ProductName: s.ProductName,
		UnitName:    s.UnitName,
		Qty:         s.Qty.String(),
		ReservedQty: s.ReservedQty.String(),
		MinStock:    s.MinStock.String(),
		Low:         s.Qty.LessThanOrEqual(s.MinStock),

		SKU:          s.SKU,
		ImageURL:     s.ImageURL,
		CategoryID:   s.CategoryID,
		CategoryName: s.CategoryName,
		SupplierID:   s.SupplierID,
		SupplierName: s.SupplierName,
		CostPrice:    s.CostPrice,
		StockValue:   s.StockValue,
		Sold30d:      s.Sold30d.String(),
		LastSoldAt:   lastSold,
	}
}
