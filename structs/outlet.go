package structs

// OutletCreateRequest adalah body POST /api/v1/outlets.
//
// tax_rate & service_charge_rate berupa STRING desimal ("0.11" = 11%) agar tidak
// rusak oleh pembulatan float JavaScript (§8). timezone divalidasi lebih lanjut
// di service terhadap daftar zona yang didukung.
type OutletCreateRequest struct {
	Name              string `json:"name" binding:"required,min=1,max=100"`
	Type              string `json:"type" binding:"omitempty,oneof=store kitchen warehouse vehicle"`
	Address           string `json:"address" binding:"omitempty,max=255"`
	Phone             string `json:"phone" binding:"omitempty,max=30"`
	Timezone          string `json:"timezone" binding:"omitempty,oneof=Asia/Jakarta Asia/Makassar Asia/Jayapura"`
	BusinessDayStart  string `json:"business_day_start" binding:"omitempty"`
	Currency          string `json:"currency" binding:"omitempty,len=3"`
	TaxEnabled        *bool  `json:"tax_enabled" binding:"omitempty"`
	TaxRate           string `json:"tax_rate" binding:"omitempty"`
	TaxInclusive      *bool  `json:"tax_inclusive" binding:"omitempty"`
	ServiceChargeRate string `json:"service_charge_rate" binding:"omitempty"`
	ReceiptHeader     string `json:"receipt_header" binding:"omitempty,max=500"`
	ReceiptFooter     string `json:"receipt_footer" binding:"omitempty,max=500"`
}

// OutletUpdateRequest adalah body PUT /api/v1/outlets/:id. Semua opsional; hanya
// field yang dikirim yang diubah (pointer membedakan "tidak dikirim" dari "nol").
type OutletUpdateRequest struct {
	Name              *string `json:"name" binding:"omitempty,min=1,max=100"`
	Type              *string `json:"type" binding:"omitempty,oneof=store kitchen warehouse vehicle"`
	Address           *string `json:"address" binding:"omitempty,max=255"`
	Phone             *string `json:"phone" binding:"omitempty,max=30"`
	Timezone          *string `json:"timezone" binding:"omitempty,oneof=Asia/Jakarta Asia/Makassar Asia/Jayapura"`
	BusinessDayStart  *string `json:"business_day_start" binding:"omitempty"`
	Currency          *string `json:"currency" binding:"omitempty,len=3"`
	TaxEnabled        *bool   `json:"tax_enabled" binding:"omitempty"`
	TaxRate           *string `json:"tax_rate" binding:"omitempty"`
	TaxInclusive      *bool   `json:"tax_inclusive" binding:"omitempty"`
	ServiceChargeRate *string `json:"service_charge_rate" binding:"omitempty"`
	ReceiptHeader     *string `json:"receipt_header" binding:"omitempty,max=500"`
	ReceiptFooter     *string `json:"receipt_footer" binding:"omitempty,max=500"`
	IsActive          *bool   `json:"is_active" binding:"omitempty"`
}

// OutletResponse adalah bentuk publik data outlet.
type OutletResponse struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Type              string `json:"type"`
	Address           string `json:"address,omitempty"`
	Phone             string `json:"phone,omitempty"`
	Timezone          string `json:"timezone"`
	BusinessDayStart  string `json:"business_day_start"` // "HH:MM"
	Currency          string `json:"currency"`
	TaxEnabled        bool   `json:"tax_enabled"`
	TaxRate           string `json:"tax_rate"` // desimal string
	TaxInclusive      bool   `json:"tax_inclusive"`
	ServiceChargeRate string `json:"service_charge_rate"`
	ReceiptHeader     string `json:"receipt_header,omitempty"`
	ReceiptFooter     string `json:"receipt_footer,omitempty"`
	IsActive          bool   `json:"is_active"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}
