package structs

// ── Customer ───────────────────────────────────────────────────────────────

type CustomerCreateRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=150"`
	Code        string `json:"code" binding:"omitempty,max=40"`
	Phone       string `json:"phone" binding:"omitempty,max=30"`
	Email       string `json:"email" binding:"omitempty,email"`
	Address     string `json:"address" binding:"omitempty,max=255"`
	Type        string `json:"type" binding:"omitempty,oneof=person company store"`
	PriceListID string `json:"price_list_id" binding:"omitempty,ulid"`
	CreditLimit int64  `json:"credit_limit" binding:"omitempty,gte=0"`
	Note        string `json:"note" binding:"omitempty,max=500"`
}

type CustomerUpdateRequest struct {
	Name        *string `json:"name" binding:"omitempty,min=1,max=150"`
	Code        *string `json:"code" binding:"omitempty,max=40"`
	Phone       *string `json:"phone" binding:"omitempty,max=30"`
	Email       *string `json:"email" binding:"omitempty"`
	Address     *string `json:"address" binding:"omitempty,max=255"`
	Type        *string `json:"type" binding:"omitempty,oneof=person company store"`
	PriceListID *string `json:"price_list_id" binding:"omitempty"`
	CreditLimit *int64  `json:"credit_limit" binding:"omitempty,gte=0"`
	Note        *string `json:"note" binding:"omitempty,max=500"`
}

type CustomerResponse struct {
	ID          string `json:"id"`
	Code        string `json:"code,omitempty"`
	Name        string `json:"name"`
	Phone       string `json:"phone,omitempty"`
	Email       string `json:"email,omitempty"`
	Address     string `json:"address,omitempty"`
	Type        string `json:"type"`
	CreditLimit int64  `json:"credit_limit"`
	Note        string `json:"note,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// ── Shift ──────────────────────────────────────────────────────────────────

type ShiftOpenRequest struct {
	OutletID    string `json:"outlet_id" binding:"required,ulid"`
	OpeningCash int64  `json:"opening_cash" binding:"omitempty,gte=0"`
	Note        string `json:"note" binding:"omitempty,max=255"`
}

type ShiftCloseRequest struct {
	CountedCash int64  `json:"counted_cash" binding:"gte=0"`
	Note        string `json:"note" binding:"omitempty,max=255"`
}

// ShiftHandoverRequest: serah terima kasir ke orang berikutnya.
type ShiftHandoverRequest struct {
	CountedCash int64 `json:"counted_cash" binding:"gte=0"`
	// OpeningCash: uang yang DITINGGAL di laci untuk shift berikutnya. Kosong
	// berarti seluruh uang yang dihitung diteruskan. Pointer, bukan int64
	// biasa: tanpa itu, "tidak diisi" dan "sengaja diisi nol" (seluruh uang
	// disetor ke brankas) tidak bisa dibedakan.
	OpeningCash *int64 `json:"opening_cash" binding:"omitempty,gte=0"`
	Note        string `json:"note" binding:"omitempty,max=255"`
}

// ShiftHandoverResponse memuat KEDUA shift: yang ditutup dan yang dibuka.
// Layar serah terima menampilkan selisih laci shift lama sekaligus modal awal
// shift baru, dan keduanya lahir dari satu transaksi yang sama.
type ShiftHandoverResponse struct {
	Ditutup ShiftResponse `json:"ditutup"`
	Dibuka  ShiftResponse `json:"dibuka"`
}

type ShiftResponse struct {
	ID           string `json:"id"`
	OutletID     string `json:"outlet_id"`
	Status       string `json:"status"`
	OpenedBy     string `json:"opened_by"`
	ClosedBy     string `json:"closed_by,omitempty"`
	OpenedAt     string `json:"opened_at"`
	ClosedAt     string `json:"closed_at,omitempty"`
	BusinessDate string `json:"business_date"`
	OpeningCash  int64  `json:"opening_cash"`
	ExpectedCash int64  `json:"expected_cash"`
	CountedCash  *int64 `json:"counted_cash,omitempty"`
	Difference   *int64 `json:"difference,omitempty"`
	Note         string `json:"note,omitempty"`

	// Rincian pembentuk expected_cash:
	//
	//	expected = opening_cash + cash_sales + cash_in - cash_out
	//
	// Diisi pada GET /shifts/:id. Untuk shift yang MASIH TERBUKA angkanya
	// dihitung langsung saat diminta, memakai fungsi yang sama dengan penutupan
	// shift — layar "Tutup Shift" menampilkan rincian ini sebelum kasir
	// memasukkan hasil hitung fisik, dan angkanya harus tidak mungkin berbeda
	// dari yang nanti tersimpan.
	CashSales *int64 `json:"cash_sales,omitempty"`
	CashIn    *int64 `json:"cash_in,omitempty"`
	CashOut   *int64 `json:"cash_out,omitempty"`
}

// ── Cash movement ──────────────────────────────────────────────────────────

type CashMovementRequest struct {
	OutletID  string `json:"outlet_id" binding:"required,ulid"`
	ShiftID   string `json:"shift_id" binding:"omitempty,ulid"`
	Direction string `json:"direction" binding:"required,oneof=in out"`
	Amount    int64  `json:"amount" binding:"required,gt=0"`
	Reason    string `json:"reason" binding:"required,min=1,max=200"`
}

type CashMovementResponse struct {
	ID           string `json:"id"`
	OutletID     string `json:"outlet_id"`
	ShiftID      string `json:"shift_id"`
	Direction    string `json:"direction"`
	Amount       int64  `json:"amount"`
	Reason       string `json:"reason"`
	OccurredAt   string `json:"occurred_at"`
	BusinessDate string `json:"business_date"`
}

// ── Receivable ─────────────────────────────────────────────────────────────

type ReceivablePaymentRequest struct {
	ReceivableID string `json:"receivable_id" binding:"required,ulid"`
	Amount       int64  `json:"amount" binding:"required,gt=0"`
	Method       string `json:"method" binding:"required,oneof=cash qris transfer card ewallet"`
	ProofURL     string `json:"proof_url" binding:"omitempty,max=500"`
}

type ReceivableResponse struct {
	ID          string `json:"id"`
	CustomerID  string `json:"customer_id"`
	SourceTable string `json:"source_table"`
	SourceID    string `json:"source_id"`
	Amount      int64  `json:"amount"`
	PaidAmount  int64  `json:"paid_amount"`
	Outstanding int64  `json:"outstanding"`
	DueDate     string `json:"due_date,omitempty"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}

// ── Stock ──────────────────────────────────────────────────────────────────

type StockResponse struct {
	OutletID    string `json:"outlet_id"`
	ProductID   string `json:"product_id"`
	VariantID   string `json:"variant_id,omitempty"`
	ProductName string `json:"product_name"`
	UnitName    string `json:"unit_name"`
	Qty         string `json:"qty"`
	ReservedQty string `json:"reserved_qty"`
	MinStock    string `json:"min_stock"`
	Low         bool   `json:"low"`
}

type StockMovementResponse struct {
	ID           string `json:"id"`
	OutletID     string `json:"outlet_id"`
	ProductID    string `json:"product_id"`
	VariantID    string `json:"variant_id,omitempty"`
	Kind         string `json:"kind"`
	QtyDelta     string `json:"qty_delta"`
	BalanceAfter string `json:"balance_after"`
	UnitCost     int64  `json:"unit_cost"`
	RefTable     string `json:"ref_table,omitempty"`
	RefID        string `json:"ref_id,omitempty"`
	Reason       string `json:"reason,omitempty"`
	OccurredAt   string `json:"occurred_at"`
	BusinessDate string `json:"business_date"`
}
