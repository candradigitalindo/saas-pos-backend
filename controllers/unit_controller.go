package controllers

import (
	"errors"
	"net/http"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// parseConversion mengurai string faktor konversi. Kosong → 1. Harus > 0.
func parseConversion(s string) (decimal.Decimal, error) {
	if s == "" {
		return decimal.NewFromInt(1), nil
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero, err
	}
	if d.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, errors.New("konversi harus lebih besar dari 0")
	}
	return d, nil
}

// checkBaseUnit memastikan base_unit_id (bila diisi) menunjuk satuan lain milik
// tenant. Mengembalikan pointer id siap simpan, atau (nil, true) bila sudah
// membalas error.
func checkBaseUnit(c *gin.Context, baseID, selfID string) (*string, bool) {
	if baseID == "" {
		return nil, false
	}
	if baseID == selfID {
		badRequest(c, "base_unit_id", "Satuan tidak bisa menjadi dasar dirinya sendiri")
		return nil, true
	}
	var base models.Unit
	if err := repositories.FindUnitInTenant(c.Request.Context(), nil, baseID, &base); err != nil {
		badRequest(c, "base_unit_id", "Satuan dasar tidak ditemukan")
		return nil, true
	}
	return &baseID, false
}

// ListUnits mengembalikan satuan tenant, berpaginasi.
func ListUnits(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	rows, total, err := repositories.ListUnits(c.Request.Context(), c.Query("search"), limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.UnitResponse, len(rows))
	for i, r := range rows {
		items[i] = unitToResponse(r)
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.UnitResponse]]{
		Success: true, Message: "Berhasil mengambil data satuan",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

// GetUnit mengembalikan satu satuan tenant.
func GetUnit(c *gin.Context) {
	var row models.Unit
	if err := repositories.FindUnitInTenant(c.Request.Context(), nil, c.Param("id"), &row); err != nil {
		notFoundOr(c, err, repositories.ErrUnitNotFound, "Satuan tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.UnitResponse]{
		Success: true, Message: "Berhasil mengambil data satuan", Data: unitToResponse(row),
	})
}

// CreateUnit menambah satuan.
func CreateUnit(c *gin.Context) {
	var req structs.UnitCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	conv, err := parseConversion(req.Conversion)
	if err != nil {
		badRequest(c, "conversion", "Faktor konversi tidak valid")
		return
	}
	base, handled := checkBaseUnit(c, req.BaseUnitID, "")
	if handled {
		return
	}

	ctx := c.Request.Context()
	row := models.Unit{
		TenantID:     reqctx.TenantID(ctx),
		Name:         req.Name,
		BaseUnitID:   base,
		Conversion:   conv,
		AllowDecimal: boolOr(req.AllowDecimal, false),
	}
	if err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		return repositories.CreateUnit(ctx, tx, &row)
	}); err != nil {
		if helpers.IsDuplicateEntryError(err) {
			conflict(c, "name", "Nama satuan sudah dipakai")
			return
		}
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.UnitResponse]{
		Success: true, Message: "Satuan berhasil dibuat", Data: unitToResponse(row),
	})
}

// UpdateUnit mengubah satuan.
func UpdateUnit(c *gin.Context) {
	id := c.Param("id")
	var req structs.UnitUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}

	ctx := c.Request.Context()
	var row models.Unit
	if err := repositories.FindUnitInTenant(ctx, nil, id, &row); err != nil {
		notFoundOr(c, err, repositories.ErrUnitNotFound, "Satuan tidak ditemukan")
		return
	}

	if req.Name != nil {
		row.Name = *req.Name
	}
	if req.AllowDecimal != nil {
		row.AllowDecimal = *req.AllowDecimal
	}
	if req.Conversion != nil {
		conv, err := parseConversion(*req.Conversion)
		if err != nil {
			badRequest(c, "conversion", "Faktor konversi tidak valid")
			return
		}
		row.Conversion = conv
	}
	if req.BaseUnitID != nil {
		base, handled := checkBaseUnit(c, *req.BaseUnitID, id)
		if handled {
			return
		}
		row.BaseUnitID = base
	}

	if err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		return repositories.UpdateUnit(ctx, tx, &row)
	}); err != nil {
		if helpers.IsDuplicateEntryError(err) {
			conflict(c, "name", "Nama satuan sudah dipakai")
			return
		}
		notFoundOr(c, err, repositories.ErrUnitNotFound, "Satuan tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.UnitResponse]{
		Success: true, Message: "Satuan berhasil diperbarui", Data: unitToResponse(row),
	})
}

// DeleteUnit menghapus (soft delete) satuan yang tidak dipakai.
func DeleteUnit(c *gin.Context) {
	err := repositories.DeleteUnit(c.Request.Context(), c.Param("id"))
	switch {
	case errors.Is(err, repositories.ErrUnitNotFound):
		notFound(c, "Satuan tidak ditemukan")
	case errors.Is(err, repositories.ErrUnitInUse):
		conflict(c, "unit", "Satuan masih dipakai produk atau satuan turunan")
	case err != nil:
		respondServiceError(c, err)
	default:
		c.JSON(http.StatusOK, structs.SuccessResponse[any]{Success: true, Message: "Satuan berhasil dihapus", Data: nil})
	}
}
