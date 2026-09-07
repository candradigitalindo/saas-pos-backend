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
		CreatedAt:   c.CreatedAt.Format(timeLayout),
		UpdatedAt:   c.UpdatedAt.Format(timeLayout),
	}
}

func shiftToResponse(s models.Shift) structs.ShiftResponse {
	r := structs.ShiftResponse{
		ID:           s.ID,
		OutletID:     s.OutletID,
		Status:       s.Status,
		OpenedBy:     s.OpenedBy,
		OpenedAt:     s.OpenedAt.Format(timeLayout),
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
		r.ClosedAt = s.ClosedAt.Format(timeLayout)
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
		OccurredAt:   m.OccurredAt.Format(timeLayout),
		BusinessDate: m.BusinessDate.Format("2006-01-02"),
	}
}

func receivableToResponse(r models.Receivable) structs.ReceivableResponse {
	out := structs.ReceivableResponse{
		ID:          r.ID,
		CustomerID:  r.CustomerID,
		SourceTable: r.SourceTable,
		SourceID:    r.SourceID,
		Amount:      r.Amount,
		PaidAmount:  r.PaidAmount,
		Outstanding: r.Outstanding(),
		Status:      r.Status,
		CreatedAt:   r.CreatedAt.Format(timeLayout),
	}
	if r.DueDate != nil {
		out.DueDate = r.DueDate.Format("2006-01-02")
	}
	return out
}

func stockRowToResponse(s repositories.StockRow) structs.StockResponse {
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
	}
}

func stockMovementToResponse(m models.StockMovement) structs.StockMovementResponse {
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
		OccurredAt:   m.OccurredAt.Format(timeLayout),
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
