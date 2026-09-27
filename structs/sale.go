package structs

// ── Checkout request ───────────────────────────────────────────────────────

// CheckoutItemRequest: klien mengirim product_id + qty (+ opsi). HARGA TIDAK
// dikirim klien — server memakai harga dari master produk (§13.1).
type CheckoutItemRequest struct {
	ProductID      string `json:"product_id" binding:"required,ulid"`
	VariantID      string `json:"variant_id" binding:"omitempty,ulid"`
	Qty            string `json:"qty" binding:"required"` // desimal string ("2", "0.25")
	DiscountAmount int64  `json:"discount_amount" binding:"omitempty,gte=0"`
	Note           string `json:"note" binding:"omitempty,max=200"`
}

// CheckoutPaymentRequest: satu tender. method='credit' = kasbon.
type CheckoutPaymentRequest struct {
	Method    string `json:"method" binding:"required,oneof=cash qris transfer card ewallet credit"`
	Amount    int64  `json:"amount" binding:"required,gt=0"`
	Reference string `json:"reference" binding:"omitempty,max=100"`
}

// CheckoutRequest adalah body POST /api/v1/sales. Wajib menyertakan header
// Idempotency-Key.
type CheckoutRequest struct {
	ID              string                   `json:"id" binding:"omitempty,ulid"` // ULID dari klien (mode offline); server buat bila kosong
	OutletID        string                   `json:"outlet_id" binding:"required,ulid"`
	ShiftID         string                   `json:"shift_id" binding:"omitempty,ulid"` // kosong = pakai shift terbuka outlet
	CustomerID      string                   `json:"customer_id" binding:"omitempty,ulid"`
	OrderType       string                   `json:"order_type" binding:"omitempty,oneof=dine_in takeaway delivery pickup"`
	OrderDiscount   int64                    `json:"order_discount" binding:"omitempty,gte=0"`
	Note            string                   `json:"note" binding:"omitempty,max=500"`
	ClientCreatedAt string                   `json:"client_created_at" binding:"omitempty"` // RFC3339 dgn offset
	Items           []CheckoutItemRequest    `json:"items" binding:"required,min=1,dive"`
	Payments        []CheckoutPaymentRequest `json:"payments" binding:"required,min=1,dive"`
}

// ── Void / refund ──────────────────────────────────────────────────────────

type VoidSaleRequest struct {
	Reason string `json:"reason" binding:"required,min=3,max=200"`
}

type RefundSaleRequest struct {
	Reason string `json:"reason" binding:"omitempty,max=200"`
}

// ── Response ───────────────────────────────────────────────────────────────

type SaleItemResponse struct {
	ID             string `json:"id"`
	ProductID      string `json:"product_id"`
	VariantID      string `json:"variant_id,omitempty"`
	ProductName    string `json:"product_name"`
	UnitName       string `json:"unit_name"`
	Qty            string `json:"qty"`
	UnitPrice      int64  `json:"unit_price"`
	UnitCost       int64  `json:"unit_cost"`
	DiscountAmount int64  `json:"discount_amount"`
	TaxAmount      int64  `json:"tax_amount"`
	LineTotal      int64  `json:"line_total"`
	Note           string `json:"note,omitempty"`
}

type SalePaymentResponse struct {
	ID        string `json:"id"`
	Method    string `json:"method"`
	Amount    int64  `json:"amount"`
	Reference string `json:"reference,omitempty"`
	FeeAmount int64  `json:"fee_amount"`
	PaidAt    string `json:"paid_at"`
}

type SaleResponse struct {
	ID             string `json:"id"`
	OutletID       string `json:"outlet_id"`
	ShiftID        string `json:"shift_id,omitempty"`
	CustomerID     string `json:"customer_id,omitempty"`
	ReceiptNo      string `json:"receipt_no"`
	OrderType      string `json:"order_type"`
	Status         string `json:"status"`
	Subtotal       int64  `json:"subtotal"`
	DiscountAmount int64  `json:"discount_amount"`
	TaxAmount      int64  `json:"tax_amount"`
	ServiceAmount  int64  `json:"service_amount"`
	RoundingAmount int64  `json:"rounding_amount"`
	Total          int64  `json:"total"`
	PaidAmount     int64  `json:"paid_amount"`
	ChangeAmount   int64  `json:"change_amount"`
	CostTotal      int64  `json:"cost_total"`
	GrossProfit    int64  `json:"gross_profit"` // total - cost_total
	ReturnOfSaleID string `json:"return_of_sale_id,omitempty"`
	Note           string `json:"note,omitempty"`
	OccurredAt     string `json:"occurred_at"`
	BusinessDate   string `json:"business_date"`
	VoidedAt       string `json:"voided_at,omitempty"`
	VoidReason     string `json:"void_reason,omitempty"`
	// Hanya di daftar riwayat (GET /sales): nama pelanggan & kasir pembuatnya.
	CustomerName string `json:"customer_name,omitempty"`
	CashierName  string `json:"cashier_name,omitempty"`
	// Hanya di daftar riwayat: nota penjualan yang diretur (pada baris retur),
	// dan nota retur (pada penjualan yang sudah diretur).
	ReturnOfReceiptNo   string                `json:"return_of_receipt_no,omitempty"`
	ReturnedByReceiptNo string                `json:"returned_by_receipt_no,omitempty"`
	Items               []SaleItemResponse    `json:"items,omitempty"`
	Payments            []SalePaymentResponse `json:"payments,omitempty"`
	CreatedAt           string                `json:"created_at"`
}

// SaleSummaryResponse: ringkasan penjualan satu hari (layar riwayat) atau satu
// shift (layar ganti/tutup shift). Penjualan = transaksi selesai; retur & batal
// disebut terpisah (tidak disembunyikan di dalam angka bersih).
type SaleSummaryResponse struct {
	BusinessDate  string            `json:"business_date,omitempty"`
	SalesCount    int64             `json:"sales_count"`
	SalesTotal    int64             `json:"sales_total"`
	AverageSale   int64             `json:"average_sale"`
	ReturnsCount  int64             `json:"returns_count"`
	ReturnsTotal  int64             `json:"returns_total"` // positif: nilai yang dikembalikan
	CanceledCount int64             `json:"canceled_count"`
	CanceledTotal int64             `json:"canceled_total"`
	NetTotal      int64             `json:"net_total"` // penjualan − retur
	ByMethod      []SaleMethodTotal `json:"by_method"`
}

// SaleMethodTotal: uang masuk per cara bayar (tunai sudah dikurangi kembalian).
type SaleMethodTotal struct {
	Method string `json:"method"`
	Count  int64  `json:"count"`
	Amount int64  `json:"amount"`
}
