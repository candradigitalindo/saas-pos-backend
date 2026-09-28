package structs

// ── Purchase ───────────────────────────────────────────────────────────────

type PurchaseItemRequest struct {
	ProductID string `json:"product_id" binding:"required,ulid"`
	VariantID string `json:"variant_id" binding:"omitempty,ulid"`
	// ProductUnitID: kemasan yang dibeli (dus); qty & unit_cost per kemasan.
	ProductUnitID string `json:"product_unit_id" binding:"omitempty,ulid"`
	Qty           string `json:"qty" binding:"required"`
	UnitCost      int64  `json:"unit_cost" binding:"required,gte=0"`
}

type PurchaseRequest struct {
	OutletID       string                `json:"outlet_id" binding:"required,ulid"`
	SupplierID     string                `json:"supplier_id" binding:"omitempty,ulid"`
	InvoiceNo      string                `json:"invoice_no" binding:"omitempty,max=100"`
	DiscountAmount int64                 `json:"discount_amount" binding:"omitempty,gte=0"`
	TaxAmount      int64                 `json:"tax_amount" binding:"omitempty,gte=0"`
	PaidAmount     int64                 `json:"paid_amount" binding:"omitempty,gte=0"`
	DueDate        string                `json:"due_date" binding:"omitempty"` // YYYY-MM-DD
	Items          []PurchaseItemRequest `json:"items" binding:"required,min=1,dive"`
}

type PurchaseItemResponse struct {
	ID        string `json:"id"`
	ProductID string `json:"product_id"`
	VariantID string `json:"variant_id,omitempty"`
	Qty       string `json:"qty"`
	UnitCost  int64  `json:"unit_cost"`
	LineTotal int64  `json:"line_total"`
	// Kemasan: qty & unit_cost per kemasan; stok masuk qty × unit_conversion.
	UnitName       string `json:"unit_name,omitempty"`
	UnitConversion string `json:"unit_conversion,omitempty"`
	// Diisi GET /purchases/:id — baris hanya menyimpan product_id.
	ProductName  string `json:"product_name,omitempty"`
	VariantName  string `json:"variant_name,omitempty"`
	BaseUnitName string `json:"base_unit_name,omitempty"`
}

type PurchaseResponse struct {
	ID             string                 `json:"id"`
	OutletID       string                 `json:"outlet_id"`
	SupplierID     string                 `json:"supplier_id,omitempty"`
	InvoiceNo      string                 `json:"invoice_no,omitempty"`
	Status         string                 `json:"status"`
	Subtotal       int64                  `json:"subtotal"`
	DiscountAmount int64                  `json:"discount_amount"`
	TaxAmount      int64                  `json:"tax_amount"`
	Total          int64                  `json:"total"`
	PaidAmount     int64                  `json:"paid_amount"`
	OccurredAt     string                 `json:"occurred_at"`
	BusinessDate   string                 `json:"business_date"`
	Items          []PurchaseItemResponse `json:"items,omitempty"`
	CreatedAt      string                 `json:"created_at"`

	// Keterangan tampilan (GET /purchases & /purchases/:id).
	SupplierName  string   `json:"supplier_name,omitempty"`
	CreatedByName string   `json:"created_by_name,omitempty"`
	ItemCount     int64    `json:"item_count"`
	ItemNames     []string `json:"item_names,omitempty"` // maks 3 nama pertama
}

// ── Stock opname ───────────────────────────────────────────────────────────

type OpnameCreateRequest struct {
	OutletID string `json:"outlet_id" binding:"required,ulid"`
	Note     string `json:"note" binding:"omitempty,max=255"`
}

type OpnameItemInputRequest struct {
	ProductID  string `json:"product_id" binding:"required,ulid"`
	VariantID  string `json:"variant_id" binding:"omitempty,ulid"`
	CountedQty string `json:"counted_qty" binding:"required"`
}

type OpnameSetItemsRequest struct {
	Items []OpnameItemInputRequest `json:"items" binding:"required,min=1,dive"`
}

type OpnameItemResponse struct {
	ID         string `json:"id"`
	ProductID  string `json:"product_id"`
	VariantID  string `json:"variant_id,omitempty"`
	SystemQty  string `json:"system_qty"`
	CountedQty string `json:"counted_qty"`
	DiffQty    string `json:"diff_qty"`
	// Diisi GET /stock-opnames/:id — baris hanya menyimpan product_id.
	ProductName string `json:"product_name,omitempty"`
	UnitName    string `json:"unit_name,omitempty"`
	CostPrice   int64  `json:"cost_price"` // harga modal SEKARANG
}

type OpnameResponse struct {
	ID           string               `json:"id"`
	OutletID     string               `json:"outlet_id"`
	Status       string               `json:"status"`
	Note         string               `json:"note,omitempty"`
	CountedAt    string               `json:"counted_at,omitempty"`
	BusinessDate string               `json:"business_date"`
	Items        []OpnameItemResponse `json:"items,omitempty"`
	CreatedAt    string               `json:"created_at"`

	// Ringkasan untuk riwayat (GET /stock-opnames & /:id).
	CreatedByName string `json:"created_by_name,omitempty"`
	ItemCount     int64  `json:"item_count"`
	ChangedCount  int64  `json:"changed_count"`
	ValueDiff     int64  `json:"value_diff"` // ≈ perubahan nilai stok (catatan minus = 0)
}

// ── Stock transfer ─────────────────────────────────────────────────────────

type TransferItemRequest struct {
	ProductID string `json:"product_id" binding:"required,ulid"`
	VariantID string `json:"variant_id" binding:"omitempty,ulid"`
	Qty       string `json:"qty" binding:"required"`
}

type TransferCreateRequest struct {
	FromOutletID string                `json:"from_outlet_id" binding:"required,ulid"`
	ToOutletID   string                `json:"to_outlet_id" binding:"required,ulid"`
	Note         string                `json:"note" binding:"omitempty,max=255"`
	Items        []TransferItemRequest `json:"items" binding:"required,min=1,dive"`
}

type TransferItemResponse struct {
	ID        string `json:"id"`
	ProductID string `json:"product_id"`
	VariantID string `json:"variant_id,omitempty"`
	Qty       string `json:"qty"`
}

type TransferResponse struct {
	ID           string                 `json:"id"`
	FromOutletID string                 `json:"from_outlet_id"`
	ToOutletID   string                 `json:"to_outlet_id"`
	Status       string                 `json:"status"`
	Note         string                 `json:"note,omitempty"`
	SentAt       string                 `json:"sent_at,omitempty"`
	ReceivedAt   string                 `json:"received_at,omitempty"`
	BusinessDate string                 `json:"business_date"`
	Items        []TransferItemResponse `json:"items,omitempty"`
	CreatedAt    string                 `json:"created_at"`
}

// ── Recipe ─────────────────────────────────────────────────────────────────

type RecipeItemRequest struct {
	IngredientProductID string `json:"ingredient_product_id" binding:"required,ulid"`
	Qty                 string `json:"qty" binding:"required"`
}

type RecipeUpsertRequest struct {
	YieldQty string              `json:"yield_qty" binding:"omitempty"` // default "1"
	Items    []RecipeItemRequest `json:"items" binding:"required,dive"`
}

type RecipeItemResponse struct {
	ID                  string `json:"id"`
	IngredientProductID string `json:"ingredient_product_id"`
	Qty                 string `json:"qty"`
}

type RecipeResponse struct {
	ID        string               `json:"id"`
	ProductID string               `json:"product_id"`
	YieldQty  string               `json:"yield_qty"`
	Items     []RecipeItemResponse `json:"items"`
}
