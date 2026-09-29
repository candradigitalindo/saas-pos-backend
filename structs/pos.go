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
	// CreditTermDays: tempo kasbon (hari); 0 = tanpa jatuh tempo.
	CreditTermDays int    `json:"credit_term_days" binding:"omitempty,gte=0,lte=365"`
	Note           string `json:"note" binding:"omitempty,max=500"`
}

type CustomerUpdateRequest struct {
	Name           *string `json:"name" binding:"omitempty,min=1,max=150"`
	Code           *string `json:"code" binding:"omitempty,max=40"`
	Phone          *string `json:"phone" binding:"omitempty,max=30"`
	Email          *string `json:"email" binding:"omitempty"`
	Address        *string `json:"address" binding:"omitempty,max=255"`
	Type           *string `json:"type" binding:"omitempty,oneof=person company store"`
	PriceListID    *string `json:"price_list_id" binding:"omitempty"`
	CreditLimit    *int64  `json:"credit_limit" binding:"omitempty,gte=0"`
	CreditTermDays *int    `json:"credit_term_days" binding:"omitempty,gte=0,lte=365"`
	Note           *string `json:"note" binding:"omitempty,max=500"`
}

type CustomerResponse struct {
	ID             string `json:"id"`
	Code           string `json:"code,omitempty"`
	Name           string `json:"name"`
	Phone          string `json:"phone,omitempty"`
	Email          string `json:"email,omitempty"`
	Address        string `json:"address,omitempty"`
	Type           string `json:"type"`
	CreditLimit    int64  `json:"credit_limit"`
	CreditTermDays int    `json:"credit_term_days"`
	Note           string `json:"note,omitempty"`
	// PriceListID: daftar harga khusus pelanggan ini (kosong = harga umum).
	PriceListID string `json:"price_list_id,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	// Ringkasan belanja — diisi GET /customers (daftar) & GET /customers/:id.
	Stats *CustomerStats `json:"stats,omitempty"`
}

// CustomerStats merangkum hubungan pelanggan dengan toko: berapa kali datang,
// total belanja bersih (retur mengurangi, void tidak ikut), kapan terakhir
// datang, dan sisa kasbon. ReceivableOutstanding hanya dikirim kepada yang
// memegang izin receivable.manage — izin yang sama dengan halaman Kasbon;
// begitu pula ReceivableOverdue (bagian sisa itu yang sudah lewat jatuh tempo).
type CustomerStats struct {
	VisitCount            int64  `json:"visit_count"`
	TotalSpent            int64  `json:"total_spent"`
	LastVisitAt           string `json:"last_visit_at,omitempty"`
	ReceivableOutstanding *int64 `json:"receivable_outstanding,omitempty"`
	ReceivableOverdue     *int64 `json:"receivable_overdue,omitempty"`
}

// CustomerTopProduct: barang yang paling sering dibeli seorang pelanggan
// (GET /customers/:id/top-products).
type CustomerTopProduct struct {
	ProductID   string `json:"product_id"`
	ProductName string `json:"product_name"`
	VariantName string `json:"variant_name,omitempty"`
	UnitName    string `json:"unit_name"`
	Times       int64  `json:"times"` // berapa nota memuatnya
	Qty         string `json:"qty"`   // jumlah (satuan dasar)
	LastAt      string `json:"last_at"`
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

	// Juga hanya pada GET /shifts/:id: siapa kasirnya, dan ringkasan
	// penjualannya — layar serah terima menampilkan keduanya sebelum laci
	// dihitung, supaya dua orang yang berganti tahu apa yang diserahkan.
	OpenedByName string               `json:"opened_by_name,omitempty"`
	ClosedByName string               `json:"closed_by_name,omitempty"`
	Sales        *SaleSummaryResponse `json:"sales,omitempty"`
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
	// Hanya pada daftar (GET /cash-movements): siapa yang mencatat — layar
	// kas dipakai bergantian, dan "siapa yang mengambil Rp 50.000 tadi?"
	// adalah pertanyaan pertama saat laci selisih.
	CreatedByName string `json:"created_by_name,omitempty"`
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
	// Nota asal (kasbon dari kasir) — diisi GET /receivables.
	ReceiptNo    string `json:"receipt_no,omitempty"`
	BusinessDate string `json:"business_date,omitempty"`
	OutletName   string `json:"outlet_name,omitempty"`
}

// ReceivableDueDateRequest: PUT /receivables/:id — atur/hapus jatuh tempo
// satu kasbon ("janji bayar"). Kosong = tanpa jatuh tempo.
type ReceivableDueDateRequest struct {
	DueDate *string `json:"due_date" binding:"required"`
}

// CustomerPaymentRequest: POST /customers/:id/receivable-payments — setoran
// pelanggan atas SEMUA kasbonnya, dipakai melunasi yang terlama dulu.
type CustomerPaymentRequest struct {
	Amount int64  `json:"amount" binding:"required,gt=0"`
	Method string `json:"method" binding:"required,oneof=cash qris transfer card ewallet"`
	// Source: tunai saja — "drawer" (masuk laci shift yang buka di OutletID)
	// atau "other" (laci tidak berubah). Non-tunai selalu "other".
	Source   string `json:"source" binding:"omitempty,oneof=drawer other"`
	OutletID string `json:"outlet_id" binding:"omitempty,ulid"`
	Note     string `json:"note" binding:"omitempty,max=255"`
}

// CustomerPaymentResponse: hasil setoran — kasbon mana saja yang terbayar.
type CustomerPaymentResponse struct {
	Amount      int64                       `json:"amount"`
	Outstanding int64                       `json:"outstanding"` // sisa kasbon pelanggan setelahnya
	Allocations []CustomerPaymentAllocation `json:"allocations"`
	ToDrawer    bool                        `json:"to_drawer"`
}

type CustomerPaymentAllocation struct {
	ReceivableID string `json:"receivable_id"`
	ReceiptNo    string `json:"receipt_no,omitempty"`
	Amount       int64  `json:"amount"`
	Settled      bool   `json:"settled"` // kasbon ini lunas karenanya
}

// ReceivablePaymentRow: satu setoran di riwayat setoran pelanggan.
type ReceivablePaymentRow struct {
	ID            string `json:"id"`
	ReceivableID  string `json:"receivable_id"`
	ReceiptNo     string `json:"receipt_no,omitempty"`
	Amount        int64  `json:"amount"`
	Method        string `json:"method"`
	ToDrawer      bool   `json:"to_drawer"`
	PaidAt        string `json:"paid_at"`
	CollectedName string `json:"collected_name,omitempty"`
	Note          string `json:"note,omitempty"`
}

// ReceivablesSummaryResponse: GET /receivable-summary — kasbon belum lunas
// per pelanggan, yang perlu ditagih lebih dulu di atas.
type ReceivablesSummaryResponse struct {
	Today         string                       `json:"today"`
	DueSoonDays   int                          `json:"due_soon_days"`
	Outstanding   int64                        `json:"outstanding"`
	Count         int64                        `json:"count"`
	OverdueCount  int64                        `json:"overdue_count"` // pelanggan dengan kasbon lewat jatuh tempo
	OverdueAmount int64                        `json:"overdue_amount"`
	DueSoonCount  int64                        `json:"due_soon_count"` // pelanggan dengan kasbon segera jatuh tempo
	DueSoonAmount int64                        `json:"due_soon_amount"`
	Customers     []ReceivableCustomerResponse `json:"customers"`
}

type ReceivableCustomerResponse struct {
	CustomerID    string `json:"customer_id"`
	CustomerName  string `json:"customer_name"`
	Phone         string `json:"phone,omitempty"`
	Outstanding   int64  `json:"outstanding"`
	Count         int64  `json:"count"`
	OverdueCount  int64  `json:"overdue_count"`
	OverdueAmount int64  `json:"overdue_amount"`
	DueSoonCount  int64  `json:"due_soon_count"`
	DueSoonAmount int64  `json:"due_soon_amount"`
	NearestDue    string `json:"nearest_due,omitempty"`
	OldestAt      string `json:"oldest_at"`
	LastPaidAt    string `json:"last_paid_at,omitempty"`
	CreditLimit   int64  `json:"credit_limit"`
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

	SKU          string  `json:"sku,omitempty"`
	ImageURL     string  `json:"image_url,omitempty"`
	CategoryID   *string `json:"category_id"`
	CategoryName string  `json:"category_name,omitempty"`
	SupplierID   *string `json:"supplier_id"` // pemasok utama barang
	SupplierName string  `json:"supplier_name,omitempty"`
	CostPrice    int64   `json:"cost_price"`
	StockValue   int64   `json:"stock_value"`  // max(qty,0) × harga modal
	Sold30d      string  `json:"sold_30d"`     // keluar bersih 30 hari terakhir
	LastSoldAt   *string `json:"last_sold_at"` // null = belum pernah terjual
}

// StockSummaryResponse menjawab GET /stocks/summary — hitungan atas SELURUH
// barang (lihat repositories.StockSummary untuk batas tiap keadaan).
type StockSummaryResponse struct {
	Total      int64 `json:"total"`
	Safe       int64 `json:"safe"`        // qty > min_stock
	Low        int64 `json:"low"`         // 0 < qty ≤ min_stock
	Out        int64 `json:"out"`         // qty = 0
	Negative   int64 `json:"negative"`    // qty < 0 — perlu dicocokkan
	StockValue int64 `json:"stock_value"` // Σ max(qty,0) × harga modal
	Idle       int64 `json:"idle"`        // ada barangnya, tak terjual 30 hari
	IdleValue  int64 `json:"idle_value"`  // modal yang tertahan di barang idle
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

// AdjustmentResponse: satu koreksi stok untuk GET /stock-adjustments.
type AdjustmentResponse struct {
	ID            string `json:"id"`
	OutletID      string `json:"outlet_id"`
	ProductID     string `json:"product_id"`
	ProductName   string `json:"product_name"`
	UnitName      string `json:"unit_name"`
	Kind          string `json:"kind"` // adjustment | initial
	QtyDelta      string `json:"qty_delta"`
	BalanceAfter  string `json:"balance_after"`
	UnitCost      int64  `json:"unit_cost"`
	Reason        string `json:"reason,omitempty"`
	OccurredAt    string `json:"occurred_at"`
	CreatedByName string `json:"created_by_name,omitempty"`
}
