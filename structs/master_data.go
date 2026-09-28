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
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
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
