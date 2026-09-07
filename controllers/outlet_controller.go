package controllers

import (
	"errors"
	"net/http"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/internal/timez"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ListOutlets mengembalikan outlet milik tenant, berpaginasi.
func ListOutlets(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	search := c.Query("search")

	outlets, total, err := repositories.ListOutlets(c.Request.Context(), search, limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	items := make([]structs.OutletResponse, len(outlets))
	for i, o := range outlets {
		items[i] = outletToResponse(o)
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.OutletResponse]]{
		Success: true,
		Message: "Berhasil mengambil data outlet",
		Data:    helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

// GetOutlet mengembalikan satu outlet milik tenant.
func GetOutlet(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		badRequest(c, "id", "ID outlet tidak boleh kosong")
		return
	}

	var outlet models.Outlet
	if err := repositories.FindOutletByID(c.Request.Context(), nil, id, &outlet); err != nil {
		if errors.Is(err, repositories.ErrOutletNotFound) {
			notFound(c, "Outlet tidak ditemukan")
			return
		}
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, structs.SuccessResponse[structs.OutletResponse]{
		Success: true,
		Message: "Berhasil mengambil data outlet",
		Data:    outletToResponse(outlet),
	})
}

// CreateOutlet menambah outlet baru untuk tenant.
func CreateOutlet(c *gin.Context) {
	var req structs.OutletCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}

	ctx := c.Request.Context()

	dayStart, err := timez.ParseClock(req.BusinessDayStart)
	if err != nil {
		badRequest(c, "business_day_start", "Format jam mulai hari usaha tidak valid")
		return
	}
	taxRate, err := parseRate(req.TaxRate)
	if err != nil {
		badRequest(c, "tax_rate", "Nilai tax_rate tidak valid")
		return
	}
	serviceRate, err := parseRate(req.ServiceChargeRate)
	if err != nil {
		badRequest(c, "service_charge_rate", "Nilai service_charge_rate tidak valid")
		return
	}

	outlet := models.Outlet{
		TenantID:          reqctx.TenantID(ctx),
		Name:              req.Name,
		Type:              orDefault(req.Type, "store"),
		Address:           req.Address,
		Phone:             req.Phone,
		Timezone:          orDefault(req.Timezone, timez.WIB),
		BusinessDayStart:  dayStart,
		Currency:          orDefault(req.Currency, "IDR"),
		TaxEnabled:        boolOr(req.TaxEnabled, false),
		TaxRate:           taxRate,
		TaxInclusive:      boolOr(req.TaxInclusive, true),
		ServiceChargeRate: serviceRate,
		ReceiptHeader:     req.ReceiptHeader,
		ReceiptFooter:     req.ReceiptFooter,
		IsActive:          true,
	}

	if err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		return repositories.CreateOutlet(ctx, tx, &outlet)
	}); err != nil {
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.OutletResponse]{
		Success: true,
		Message: "Outlet berhasil dibuat",
		Data:    outletToResponse(outlet),
	})
}

// UpdateOutlet mengubah outlet. Hanya field yang dikirim yang diubah.
func UpdateOutlet(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		badRequest(c, "id", "ID outlet tidak boleh kosong")
		return
	}

	var req structs.OutletUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}

	ctx := c.Request.Context()

	var outlet models.Outlet
	if err := repositories.FindOutletByID(ctx, nil, id, &outlet); err != nil {
		if errors.Is(err, repositories.ErrOutletNotFound) {
			notFound(c, "Outlet tidak ditemukan")
			return
		}
		respondServiceError(c, err)
		return
	}

	if req.Name != nil {
		outlet.Name = *req.Name
	}
	if req.Type != nil {
		outlet.Type = *req.Type
	}
	if req.Address != nil {
		outlet.Address = *req.Address
	}
	if req.Phone != nil {
		outlet.Phone = *req.Phone
	}
	if req.Timezone != nil {
		outlet.Timezone = *req.Timezone
	}
	if req.BusinessDayStart != nil {
		d, err := timez.ParseClock(*req.BusinessDayStart)
		if err != nil {
			badRequest(c, "business_day_start", "Format jam mulai hari usaha tidak valid")
			return
		}
		outlet.BusinessDayStart = d
	}
	if req.Currency != nil {
		outlet.Currency = *req.Currency
	}
	if req.TaxEnabled != nil {
		outlet.TaxEnabled = *req.TaxEnabled
	}
	if req.TaxRate != nil {
		r, err := parseRate(*req.TaxRate)
		if err != nil {
			badRequest(c, "tax_rate", "Nilai tax_rate tidak valid")
			return
		}
		outlet.TaxRate = r
	}
	if req.TaxInclusive != nil {
		outlet.TaxInclusive = *req.TaxInclusive
	}
	if req.ServiceChargeRate != nil {
		r, err := parseRate(*req.ServiceChargeRate)
		if err != nil {
			badRequest(c, "service_charge_rate", "Nilai service_charge_rate tidak valid")
			return
		}
		outlet.ServiceChargeRate = r
	}
	if req.ReceiptHeader != nil {
		outlet.ReceiptHeader = *req.ReceiptHeader
	}
	if req.ReceiptFooter != nil {
		outlet.ReceiptFooter = *req.ReceiptFooter
	}
	if req.IsActive != nil {
		outlet.IsActive = *req.IsActive
	}

	if err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		return repositories.UpdateOutlet(ctx, tx, &outlet)
	}); err != nil {
		if errors.Is(err, repositories.ErrOutletNotFound) {
			notFound(c, "Outlet tidak ditemukan")
			return
		}
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, structs.SuccessResponse[structs.OutletResponse]{
		Success: true,
		Message: "Outlet berhasil diperbarui",
		Data:    outletToResponse(outlet),
	})
}

// DeleteOutlet menghapus (soft delete) outlet.
func DeleteOutlet(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		badRequest(c, "id", "ID outlet tidak boleh kosong")
		return
	}

	ctx := c.Request.Context()
	if err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		return repositories.SoftDeleteOutlet(ctx, tx, id)
	}); err != nil {
		if errors.Is(err, repositories.ErrOutletNotFound) {
			notFound(c, "Outlet tidak ditemukan")
			return
		}
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, structs.SuccessResponse[any]{
		Success: true,
		Message: "Outlet berhasil dihapus",
		Data:    nil,
	})
}

// parseRate mengurai string tarif ("0.11") menjadi decimal. Kosong = 0.
func parseRate(s string) (decimal.Decimal, error) {
	if s == "" {
		return decimal.Zero, nil
	}
	return decimal.NewFromString(s)
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func boolOr(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}
