package structs

// DTO kanal pesanan online (Fase 11a, §5.10). Nilai uang int64 rupiah bulat;
// tarif komisi & qty string desimal.

// ── Channel ───────────────────────────────────────────────────────────────

type ChannelCreateRequest struct {
	OutletID        string `json:"outlet_id" binding:"required,ulid"`
	Kind            string `json:"kind" binding:"required,oneof=pos marketplace delivery_app conversation"`
	Provider        string `json:"provider" binding:"required,min=1,max=40"`
	Name            string `json:"name" binding:"required,min=1,max=80"`
	MerchantRef     string `json:"merchant_ref" binding:"omitempty,max=100"`
	CommissionRate  string `json:"commission_rate" binding:"omitempty"` // "0.20" = 20%
	PriceListID     string `json:"price_list_id" binding:"omitempty,ulid"`
	IntegrationMode string `json:"integration_mode" binding:"omitempty,oneof=manual csv api"`
}

type ChannelUpdateRequest struct {
	Name            *string `json:"name" binding:"omitempty,min=1,max=80"`
	MerchantRef     *string `json:"merchant_ref" binding:"omitempty,max=100"`
	CommissionRate  *string `json:"commission_rate" binding:"omitempty"`
	PriceListID     *string `json:"price_list_id" binding:"omitempty"`
	IntegrationMode *string `json:"integration_mode" binding:"omitempty,oneof=manual csv api"`
	IsActive        *bool   `json:"is_active"`
}

type ChannelResponse struct {
	ID              string `json:"id"`
	OutletID        string `json:"outlet_id"`
	Kind            string `json:"kind"`
	Provider        string `json:"provider"`
	Name            string `json:"name"`
	MerchantRef     string `json:"merchant_ref,omitempty"`
	CommissionRate  string `json:"commission_rate"`
	PriceListID     string `json:"price_list_id,omitempty"`
	IntegrationMode string `json:"integration_mode"`
	IsActive        bool   `json:"is_active"`
	CreatedAt       string `json:"created_at"`
}

// ── Channel product (pemetaan SKU) ────────────────────────────────────────

type ChannelProductRequest struct {
	ProductID    string `json:"product_id" binding:"required,ulid"`
	VariantID    string `json:"variant_id" binding:"omitempty,ulid"`
	ExternalSKU  string `json:"external_sku" binding:"required,min=1,max=120"`
	ChannelPrice *int64 `json:"channel_price" binding:"omitempty,gte=0"`
	IsAvailable  *bool  `json:"is_available"`
	StockBuffer  string `json:"stock_buffer" binding:"omitempty"`
}

type ChannelProductResponse struct {
	ID           string `json:"id"`
	ChannelID    string `json:"channel_id"`
	ProductID    string `json:"product_id"`
	VariantID    string `json:"variant_id,omitempty"`
	ExternalSKU  string `json:"external_sku"`
	ChannelPrice *int64 `json:"channel_price"`
	IsAvailable  bool   `json:"is_available"`
	StockBuffer  string `json:"stock_buffer"`
}

// ── Channel order (entri manual) ──────────────────────────────────────────

type ChannelOrderItemRequest struct {
	ProductID string `json:"product_id" binding:"required,ulid"`
	VariantID string `json:"variant_id" binding:"omitempty,ulid"`
	Qty       string `json:"qty" binding:"required"`
	UnitPrice *int64 `json:"unit_price" binding:"omitempty,gte=0"` // override harga; kosong → harga kanal/master
}

type ChannelOrderCreateRequest struct {
	ChannelID       string                    `json:"channel_id" binding:"required,ulid"`
	ExternalOrderID string                    `json:"external_order_id" binding:"required,min=1,max=120"`
	BuyerName       string                    `json:"buyer_name" binding:"omitempty,max=120"`
	BuyerPhone      string                    `json:"buyer_phone" binding:"omitempty,max=30"`
	ShippingAddress string                    `json:"shipping_address" binding:"omitempty,max=500"`
	Courier         string                    `json:"courier" binding:"omitempty,max=60"`
	OrderDiscount   int64                     `json:"order_discount" binding:"omitempty,gte=0"`
	FeeAmount       *int64                    `json:"fee_amount" binding:"omitempty,gte=0"` // kosong → dihitung dari commission_rate
	OccurredAt      string                    `json:"occurred_at" binding:"omitempty"`      // RFC3339; untuk entri historis
	Items           []ChannelOrderItemRequest `json:"items" binding:"required,min=1,dive"`
}

type ChannelOrderStatusRequest struct {
	ExternalStatus string `json:"external_status" binding:"required,max=40"`
	Courier        string `json:"courier" binding:"omitempty,max=60"`
	TrackingNo     string `json:"tracking_no" binding:"omitempty,max=80"`
	DriverName     string `json:"driver_name" binding:"omitempty,max=80"`
}

type ChannelOrderCancelRequest struct {
	Reason string `json:"reason" binding:"required,min=3,max=200"`
}

type ChannelOrderResponse struct {
	ID              string `json:"id"`
	ChannelID       string `json:"channel_id"`
	SaleID          string `json:"sale_id"`
	ExternalOrderID string `json:"external_order_id"`
	ExternalStatus  string `json:"external_status,omitempty"`
	BuyerName       string `json:"buyer_name,omitempty"`
	BuyerPhone      string `json:"buyer_phone,omitempty"`
	ShippingAddress string `json:"shipping_address,omitempty"`
	Courier         string `json:"courier,omitempty"`
	TrackingNo      string `json:"tracking_no,omitempty"`
	GrossAmount     int64  `json:"gross_amount"`
	FeeAmount       int64  `json:"fee_amount"`
	NetAmount       int64  `json:"net_amount"`
	CreatedAt       string `json:"created_at"`
}
