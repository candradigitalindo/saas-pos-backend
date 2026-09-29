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

// SupplierStats: angka belanja per pemasok di satu outlet — belanja 30 hari,
// jumlah nota, belanja terakhir, utang & yang lewat jatuh tempo.
func SupplierStats(c *gin.Context) {
	outletID := c.Query("outlet_id")
	rows, err := repositories.SupplierStats(c.Request.Context(), outletID, hariUsahaOutlet(c, outletID))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	out := make([]structs.SupplierStatResponse, len(rows))
	for i, r := range rows {
		out[i] = structs.SupplierStatResponse{
			SupplierID: r.SupplierID, PurchaseCount: r.PurchaseCount, Spent30d: r.Spent30d,
			Outstanding: r.Outstanding, OverdueCount: r.OverdueCount,
		}
		if r.LastPurchaseAt != nil {
			out[i].LastPurchaseAt = r.LastPurchaseAt.UTC().Format(timeLayout)
		}
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.SupplierStatResponse]{
		Success: true, Message: "Angka belanja per pemasok", Data: out,
	})
}

// SupplierProducts: barang yang biasa dibeli dari satu pemasok — dasar
// "pesan lagi" lewat WhatsApp.
func SupplierProducts(c *gin.Context) {
	ctx := c.Request.Context()
	var sup models.Supplier
	if err := repositories.FindSupplierInTenant(ctx, nil, c.Param("id"), &sup); err != nil {
		notFoundOr(c, err, repositories.ErrSupplierNotFound, "Pemasok tidak ditemukan")
		return
	}
	rows, err := repositories.SupplierProducts(ctx, sup.ID, c.Query("outlet_id"))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	out := make([]structs.SupplierProductResponse, len(rows))
	for i, r := range rows {
		out[i] = structs.SupplierProductResponse{
			ProductID: r.ProductID, ProductName: r.ProductName, BaseUnitName: r.BaseUnitName,
			UnitCost: r.UnitCost, UnitName: r.UnitName, UnitConversion: r.UnitConversion.String(),
			ProductUnitID: deref(r.ProductUnitID), LastBoughtAt: r.LastBoughtAt.UTC().Format(timeLayout),
			Times: r.Times, Qty90d: r.Qty90d.String(),
		}
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.SupplierProductResponse]{
		Success: true, Message: "Barang dari pemasok", Data: out,
	})
}
