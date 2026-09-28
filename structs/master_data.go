package structs

// ── Category ────────────────────────────────────────────────────────────────

type CategoryCreateRequest struct {
	Name      string `json:"name" binding:"required,min=1,max=100"`
	ParentID  string `json:"parent_id" binding:"omitempty,ulid"`
	SortOrder int    `json:"sort_order" binding:"omitempty"`
}

type CategoryUpdateRequest struct {
	Name      *string `json:"name" binding:"omitempty,min=1,max=100"`
	ParentID  *string `json:"parent_id" binding:"omitempty"` // "" mengosongkan induk; ulid dicek di controller
	SortOrder *int    `json:"sort_order" binding:"omitempty"`
}

type CategoryResponse struct {
	ID        string `json:"id"`
	ParentID  string `json:"parent_id,omitempty"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ── Unit ────────────────────────────────────────────────────────────────────

type UnitCreateRequest struct {
	Name         string `json:"name" binding:"required,min=1,max=50"`
	BaseUnitID   string `json:"base_unit_id" binding:"omitempty,ulid"`
	Conversion   string `json:"conversion" binding:"omitempty"` // desimal string, default "1"
	AllowDecimal *bool  `json:"allow_decimal" binding:"omitempty"`
}

type UnitUpdateRequest struct {
	Name         *string `json:"name" binding:"omitempty,min=1,max=50"`
	BaseUnitID   *string `json:"base_unit_id" binding:"omitempty"`
	Conversion   *string `json:"conversion" binding:"omitempty"`
	AllowDecimal *bool   `json:"allow_decimal" binding:"omitempty"`
}

type UnitResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	BaseUnitID   string `json:"base_unit_id,omitempty"`
	Conversion   string `json:"conversion"`
	AllowDecimal bool   `json:"allow_decimal"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// ── Supplier ────────────────────────────────────────────────────────────────

type SupplierCreateRequest struct {
	Name    string `json:"name" binding:"required,min=1,max=150"`
	Phone   string `json:"phone" binding:"omitempty,max=30"`
	Address string `json:"address" binding:"omitempty,max=255"`
	Note    string `json:"note" binding:"omitempty,max=500"`
}

type SupplierUpdateRequest struct {
	Name    *string `json:"name" binding:"omitempty,min=1,max=150"`
	Phone   *string `json:"phone" binding:"omitempty,max=30"`
	Address *string `json:"address" binding:"omitempty,max=255"`
	Note    *string `json:"note" binding:"omitempty,max=500"`
}

type SupplierResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Phone     string `json:"phone,omitempty"`
	Address   string `json:"address,omitempty"`
	Note      string `json:"note,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ── Product ─────────────────────────────────────────────────────────────────

type ProductCreateRequest struct {
	Name       string `json:"name" binding:"required,min=1,max=200"`
	CategoryID string `json:"category_id" binding:"omitempty,ulid"`
	UnitID     string `json:"unit_id" binding:"required,ulid"`
	SKU        string `json:"sku" binding:"omitempty,max=60"`
	Barcode    string `json:"barcode" binding:"omitempty,max=60"`
	SellPrice  int64  `json:"sell_price" binding:"omitempty,gte=0"`
	CostPrice  int64  `json:"cost_price" binding:"omitempty,gte=0"`
	TrackStock *bool  `json:"track_stock" binding:"omitempty"`
	MinStock   string `json:"min_stock" binding:"omitempty"` // desimal string
	IsActive   *bool  `json:"is_active" binding:"omitempty"`
	ImageURL   string `json:"image_url" binding:"omitempty,max=500"`
	// Description: untuk menu aplikasi antar (GoFood menampilkan 250 karakter).
	Description string `json:"description" binding:"omitempty,max=500"`
	// WholesalePrices: harga grosir per jumlah (maks. 5 tingkat).
	WholesalePrices []WholesalePriceRequest `json:"wholesale_prices" binding:"omitempty,max=5,dive"`
}

// WholesalePriceRequest: satu tingkat harga grosir — beli minimal MinQty
// (total barang itu dalam satu transaksi), harga satuannya Price.
type WholesalePriceRequest struct {
	MinQty string `json:"min_qty" binding:"required"` // desimal string, > 1
	Price  int64  `json:"price" binding:"required,gt=0"`
}

type WholesalePriceResponse struct {
	MinQty string `json:"min_qty"`
	Price  int64  `json:"price"`
}

type ProductUpdateRequest struct {
	Name        *string `json:"name" binding:"omitempty,min=1,max=200"`
	CategoryID  *string `json:"category_id" binding:"omitempty"`
	UnitID      *string `json:"unit_id" binding:"omitempty,ulid"`
	SKU         *string `json:"sku" binding:"omitempty,max=60"`
	Barcode     *string `json:"barcode" binding:"omitempty,max=60"`
	SellPrice   *int64  `json:"sell_price" binding:"omitempty,gte=0"`
	CostPrice   *int64  `json:"cost_price" binding:"omitempty,gte=0"`
	TrackStock  *bool   `json:"track_stock" binding:"omitempty"`
	MinStock    *string `json:"min_stock" binding:"omitempty"`
	IsActive    *bool   `json:"is_active" binding:"omitempty"`
	ImageURL    *string `json:"image_url" binding:"omitempty,max=500"`
	Description *string `json:"description" binding:"omitempty,max=500"`
	// WholesalePrices: nil = tidak diubah; [] = semua tingkat dihapus.
	WholesalePrices *[]WholesalePriceRequest `json:"wholesale_prices" binding:"omitempty,dive"`
}

type ProductResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	CategoryID   string `json:"category_id,omitempty"`
	CategoryName string `json:"category_name,omitempty"`
	UnitID       string `json:"unit_id"`
	UnitName     string `json:"unit_name,omitempty"`
	SKU          string `json:"sku,omitempty"`
	Barcode      string `json:"barcode,omitempty"`
	SellPrice    int64  `json:"sell_price"`
	CostPrice    int64  `json:"cost_price"`
	TrackStock   bool   `json:"track_stock"`
	MinStock     string `json:"min_stock"`
	IsActive     bool   `json:"is_active"`
	ImageURL     string `json:"image_url,omitempty"`
	Description  string `json:"description,omitempty"`
	// WholesalePrices hanya di GET satu barang & balasan buat/ubah.
	WholesalePrices []WholesalePriceResponse `json:"wholesale_prices,omitempty"`
	CreatedAt       string                   `json:"created_at"`
	UpdatedAt       string                   `json:"updated_at"`
}

// ProductVariantRequest: satu varian (PUT = ganti utuh).
type ProductVariantRequest struct {
	Name       string `json:"name" binding:"required,min=1,max=60"`
	PriceDelta int64  `json:"price_delta"` // selisih terhadap harga jual barang; boleh negatif
	SKU        string `json:"sku" binding:"omitempty,max=60"`
	Barcode    string `json:"barcode" binding:"omitempty,max=60"`
	IsActive   *bool  `json:"is_active"`
}

type ProductVariantResponse struct {
	ID         string `json:"id"`
	ProductID  string `json:"product_id"`
	Name       string `json:"name"`
	PriceDelta int64  `json:"price_delta"`
	Price      int64  `json:"price"` // harga jual barang + selisih
	SKU        string `json:"sku,omitempty"`
	Barcode    string `json:"barcode,omitempty"`
	IsActive   bool   `json:"is_active"`
}

// ── Impor produk ────────────────────────────────────────────────────────────

// ProductImportRowError menjelaskan satu baris CSV yang gagal divalidasi.
type ProductImportRowError struct {
	Row     int    `json:"row"`   // nomor baris data (1 = baris pertama setelah header)
	Field   string `json:"field"` // kolom yang bermasalah, "" bila umum
	Message string `json:"message"`
}

// ProductImportResult adalah ringkasan hasil impor.
type ProductImportResult struct {
	DryRun   bool                    `json:"dry_run"`
	Total    int                     `json:"total"`    // baris data terbaca
	Imported int                     `json:"imported"` // baris yang berhasil (atau akan berhasil bila dry-run)
	Failed   int                     `json:"failed"`
	Errors   []ProductImportRowError `json:"errors"`
}
