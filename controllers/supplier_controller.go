package controllers

import (
	"net/http"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ListSuppliers mengembalikan supplier tenant, berpaginasi.
func ListSuppliers(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	rows, total, err := repositories.ListSuppliers(c.Request.Context(), c.Query("search"), limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.SupplierResponse, len(rows))
	for i, r := range rows {
		items[i] = supplierToResponse(r)
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.SupplierResponse]]{
		Success: true, Message: "Berhasil mengambil data supplier",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

// GetSupplier mengembalikan satu supplier tenant.
func GetSupplier(c *gin.Context) {
	var row models.Supplier
	if err := repositories.FindSupplierInTenant(c.Request.Context(), nil, c.Param("id"), &row); err != nil {
		notFoundOr(c, err, repositories.ErrSupplierNotFound, "Supplier tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.SupplierResponse]{
		Success: true, Message: "Berhasil mengambil data supplier", Data: supplierToResponse(row),
	})
}

// CreateSupplier menambah supplier.
func CreateSupplier(c *gin.Context) {
	var req structs.SupplierCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	ctx := c.Request.Context()
	row := models.Supplier{
		TenantID: reqctx.TenantID(ctx),
		Name:     req.Name,
		Phone:    req.Phone,
		Address:  req.Address,
		Note:     req.Note,
	}
	if err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		return repositories.CreateSupplier(ctx, tx, &row)
	}); err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.SupplierResponse]{
		Success: true, Message: "Supplier berhasil dibuat", Data: supplierToResponse(row),
	})
}

// UpdateSupplier mengubah supplier.
func UpdateSupplier(c *gin.Context) {
	id := c.Param("id")
	var req structs.SupplierUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}

	ctx := c.Request.Context()
	var row models.Supplier
	if err := repositories.FindSupplierInTenant(ctx, nil, id, &row); err != nil {
		notFoundOr(c, err, repositories.ErrSupplierNotFound, "Supplier tidak ditemukan")
		return
	}
	if req.Name != nil {
		row.Name = *req.Name
	}
	if req.Phone != nil {
		row.Phone = *req.Phone
	}
	if req.Address != nil {
		row.Address = *req.Address
	}
	if req.Note != nil {
		row.Note = *req.Note
	}

	if err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		return repositories.UpdateSupplier(ctx, tx, &row)
	}); err != nil {
		notFoundOr(c, err, repositories.ErrSupplierNotFound, "Supplier tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.SupplierResponse]{
		Success: true, Message: "Supplier berhasil diperbarui", Data: supplierToResponse(row),
	})
}

// DeleteSupplier menghapus (soft delete) supplier.
func DeleteSupplier(c *gin.Context) {
	ctx := c.Request.Context()
	if err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		return repositories.DeleteSupplier(ctx, tx, c.Param("id"))
	}); err != nil {
		notFoundOr(c, err, repositories.ErrSupplierNotFound, "Supplier tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[any]{Success: true, Message: "Supplier berhasil dihapus", Data: nil})
}
