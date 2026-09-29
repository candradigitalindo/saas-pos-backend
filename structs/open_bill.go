package structs

import "candra/backend-api/models"

// OpenBillUpsertRequest adalah body PUT /api/v1/open-bills/:id — tagihan
// utuh (bukan tambalan). BaseVersion = versi yang dibaca klien; 0 = tagihan baru.
type OpenBillUpsertRequest struct {
	OutletID      string                   `json:"outlet_id" binding:"required,ulid"`
	Label         string                   `json:"label" binding:"required,max=60"`
	CustomerID    string                   `json:"customer_id" binding:"omitempty,ulid"`
	OrderType     string                   `json:"order_type" binding:"omitempty,oneof=dine_in takeaway delivery pickup"`
	Items         []OpenBillItemRequest    `json:"items" binding:"omitempty,max=200,dive"`
	OrderDiscount *models.OpenBillDiscount `json:"order_discount"`
	Note          string                   `json:"note" binding:"omitempty,max=500"`
	BaseVersion   int                      `json:"base_version" binding:"gte=0"`
}

type OpenBillItemRequest struct {
	ProductID       string `json:"product_id" binding:"required,ulid"`
	VariantID       string `json:"variant_id" binding:"omitempty,ulid"`
	ProductUnitID   string `json:"product_unit_id" binding:"omitempty,ulid"`
	Qty             string `json:"qty" binding:"required"`
	DiscountAmount  int64  `json:"discount_amount" binding:"omitempty,gte=0"`
	DiscountPercent *int   `json:"discount_percent" binding:"omitempty,gte=1,lte=100"`
	Note            string `json:"note" binding:"omitempty,max=200"`
}

// OpenBillCancelRequest adalah body POST /api/v1/open-bills/:id/cancel.
type OpenBillCancelRequest struct {
	BaseVersion int `json:"base_version" binding:"required,gte=1"`
}

// OpenBillSyncPayload adalah payload operasi /sync/push "open_bill.upsert" /
// "open_bill.cancel". `id` = tagihan; id OPERASI ada di SyncOperation.ID.
type OpenBillSyncPayload struct {
	ID string `json:"id" binding:"required,ulid"`
	OpenBillUpsertRequest
}

type OpenBillCancelSyncPayload struct {
	ID          string `json:"id" binding:"required,ulid"`
	BaseVersion int    `json:"base_version" binding:"required,gte=1"`
}

// OpenBillResponse: tagihan beserta nama pembuat & pengubah terakhir (untuk
// daftar di kasir: "dicatat Budi 10 menit lalu").
type OpenBillResponse struct {
	ID            string                   `json:"id"`
	OutletID      string                   `json:"outlet_id"`
	Label         string                   `json:"label"`
	CustomerID    string                   `json:"customer_id,omitempty"`
	OrderType     string                   `json:"order_type"`
	Items         []models.OpenBillItem    `json:"items"`
	OrderDiscount *models.OpenBillDiscount `json:"order_discount,omitempty"`
	Note          string                   `json:"note,omitempty"`
	Status        string                   `json:"status"`
	Version       int                      `json:"version"`
	SaleID        string                   `json:"sale_id,omitempty"`
	CreatedBy     string                   `json:"created_by"`
	CreatedByName string                   `json:"created_by_name,omitempty"`
	UpdatedByName string                   `json:"updated_by_name,omitempty"`
	CreatedAt     string                   `json:"created_at"`
	UpdatedAt     string                   `json:"updated_at"`
}
