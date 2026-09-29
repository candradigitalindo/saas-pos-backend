package structs

// DTO dokumen CRM — penawaran, proyek, invoice pelanggan (Fase 9, §5.9).

// ── Quotation ─────────────────────────────────────────────────────────────

type QuotationItemRequest struct {
	ProductID      string `json:"product_id" binding:"omitempty,ulid"`
	Description    string `json:"description" binding:"required,min=1,max=255"`
	Qty            string `json:"qty" binding:"required"`
	UnitPrice      int64  `json:"unit_price" binding:"required,gte=0"`
	DiscountAmount int64  `json:"discount_amount" binding:"omitempty,gte=0"`
}

type QuotationCreateRequest struct {
	CustomerID string                 `json:"customer_id" binding:"required,ulid"`
	DealID     string                 `json:"deal_id" binding:"omitempty,ulid"`
	ValidUntil string                 `json:"valid_until" binding:"omitempty"` // YYYY-MM-DD
	TaxAmount  int64                  `json:"tax_amount" binding:"omitempty,gte=0"`
	Note       string                 `json:"note" binding:"omitempty,max=1000"`
	Items      []QuotationItemRequest `json:"items" binding:"required,min=1,dive"`
}

type QuotationItemResponse struct {
	ID             string `json:"id"`
	ProductID      string `json:"product_id,omitempty"`
	Description    string `json:"description"`
	Qty            string `json:"qty"`
	UnitPrice      int64  `json:"unit_price"`
	DiscountAmount int64  `json:"discount_amount"`
	LineTotal      int64  `json:"line_total"`
}

type QuotationResponse struct {
	ID             string                  `json:"id"`
	Number         string                  `json:"number"`
	CustomerID     string                  `json:"customer_id"`
	DealID         string                  `json:"deal_id,omitempty"`
	OwnerID        string                  `json:"owner_id"`
	ValidUntil     string                  `json:"valid_until,omitempty"`
	Status         string                  `json:"status"`
	Subtotal       int64                   `json:"subtotal"`
	DiscountAmount int64                   `json:"discount_amount"`
	TaxAmount      int64                   `json:"tax_amount"`
	Total          int64                   `json:"total"`
	Note           string                  `json:"note,omitempty"`
	AcceptedAt     string                  `json:"accepted_at,omitempty"`
	ProjectID      string                  `json:"project_id,omitempty"` // diisi setelah accept
	Items          []QuotationItemResponse `json:"items,omitempty"`
	CreatedAt      string                  `json:"created_at"`
}

// ── Project ───────────────────────────────────────────────────────────────

type ProjectCreateRequest struct {
	CustomerID    string `json:"customer_id" binding:"required,ulid"`
	QuotationID   string `json:"quotation_id" binding:"omitempty,ulid"`
	Name          string `json:"name" binding:"required,min=1,max=200"`
	StartDate     string `json:"start_date" binding:"omitempty"`
	DueDate       string `json:"due_date" binding:"omitempty"`
	ContractValue int64  `json:"contract_value" binding:"omitempty,gte=0"`
}

type ProjectUpdateRequest struct {
	Name          *string `json:"name" binding:"omitempty,min=1,max=200"`
	StartDate     *string `json:"start_date" binding:"omitempty"`
	DueDate       *string `json:"due_date" binding:"omitempty"`
	Status        *string `json:"status" binding:"omitempty,oneof=active on_hold completed canceled"`
	ContractValue *int64  `json:"contract_value" binding:"omitempty,gte=0"`
}

type ProjectTaskRequest struct {
	Title   string `json:"title" binding:"required,min=1,max=200"`
	DueDate string `json:"due_date" binding:"omitempty"`
}

type ProjectExpenseRequest struct {
	Description string `json:"description" binding:"required,min=1,max=255"`
	Amount      int64  `json:"amount" binding:"required,gt=0"`
	SpentAt     string `json:"spent_at" binding:"required"` // YYYY-MM-DD
	ReceiptURL  string `json:"receipt_url" binding:"omitempty,max=500"`
}

type ProjectTaskResponse struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	DueDate   string `json:"due_date,omitempty"`
	DoneAt    string `json:"done_at,omitempty"`
	SortOrder int    `json:"sort_order"`
}

type ProjectExpenseResponse struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Amount      int64  `json:"amount"`
	SpentAt     string `json:"spent_at"`
	ReceiptURL  string `json:"receipt_url,omitempty"`
}

type ProjectResponse struct {
	ID            string                   `json:"id"`
	CustomerID    string                   `json:"customer_id"`
	QuotationID   string                   `json:"quotation_id,omitempty"`
	OwnerID       string                   `json:"owner_id"`
	Name          string                   `json:"name"`
	StartDate     string                   `json:"start_date,omitempty"`
	DueDate       string                   `json:"due_date,omitempty"`
	Status        string                   `json:"status"`
	ContractValue int64                    `json:"contract_value"`
	TotalExpense  int64                    `json:"total_expense"`
	Tasks         []ProjectTaskResponse    `json:"tasks,omitempty"`
	Expenses      []ProjectExpenseResponse `json:"expenses,omitempty"`
	CreatedAt     string                   `json:"created_at"`
}

// ── Invoice ───────────────────────────────────────────────────────────────

type InvoiceItemRequest struct {
	ProductID   string `json:"product_id" binding:"omitempty,ulid"`
	Description string `json:"description" binding:"required,min=1,max=255"`
	Qty         string `json:"qty" binding:"required"`
	UnitPrice   int64  `json:"unit_price" binding:"required,gte=0"`
}

type InvoiceCreateRequest struct {
	CustomerID     string               `json:"customer_id" binding:"required,ulid"`
	ProjectID      string               `json:"project_id" binding:"omitempty,ulid"`
	QuotationID    string               `json:"quotation_id" binding:"omitempty,ulid"`
	DueDate        string               `json:"due_date" binding:"required"` // YYYY-MM-DD
	DiscountAmount int64                `json:"discount_amount" binding:"omitempty,gte=0"`
	TaxAmount      int64                `json:"tax_amount" binding:"omitempty,gte=0"`
	TermLabel      string               `json:"term_label" binding:"omitempty,max=60"`
	Items          []InvoiceItemRequest `json:"items" binding:"required,min=1,dive"`
}

type InvoicePaymentRequest struct {
	InvoiceID string `json:"invoice_id" binding:"required,ulid"`
	Amount    int64  `json:"amount" binding:"required,gt=0"`
	Method    string `json:"method" binding:"required,oneof=cash qris transfer card ewallet"`
	ProofURL  string `json:"proof_url" binding:"omitempty,max=500"`
}

type InvoiceItemResponse struct {
	ID          string `json:"id"`
	ProductID   string `json:"product_id,omitempty"`
	Description string `json:"description"`
	Qty         string `json:"qty"`
	UnitPrice   int64  `json:"unit_price"`
	LineTotal   int64  `json:"line_total"`
}

type InvoicePaymentResponse struct {
	ID           string `json:"id"`
	Amount       int64  `json:"amount"`
	Method       string `json:"method"`
	PaidAt       string `json:"paid_at"`
	BusinessDate string `json:"business_date"`
	ProofURL     string `json:"proof_url,omitempty"`
}

type InvoiceResponse struct {
	ID             string                   `json:"id"`
	Number         string                   `json:"number"`
	CustomerID     string                   `json:"customer_id"`
	ProjectID      string                   `json:"project_id,omitempty"`
	QuotationID    string                   `json:"quotation_id,omitempty"`
	OwnerID        string                   `json:"owner_id"`
	IssueDate      string                   `json:"issue_date"`
	DueDate        string                   `json:"due_date"`
	Status         string                   `json:"status"`
	Subtotal       int64                    `json:"subtotal"`
	DiscountAmount int64                    `json:"discount_amount"`
	TaxAmount      int64                    `json:"tax_amount"`
	Total          int64                    `json:"total"`
	PaidAmount     int64                    `json:"paid_amount"`
	Outstanding    int64                    `json:"outstanding"`
	TermLabel      string                   `json:"term_label,omitempty"`
	SaleID         string                   `json:"sale_id,omitempty"`
	Items          []InvoiceItemResponse    `json:"items,omitempty"`
	Payments       []InvoicePaymentResponse `json:"payments,omitempty"`
	CreatedAt      string                   `json:"created_at"`
}
