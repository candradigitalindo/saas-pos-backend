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

// ListCustomers mengembalikan pelanggan tenant, berpaginasi.
func ListCustomers(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	rows, total, err := repositories.ListCustomersVisible(c.Request.Context(), c.Query("search"), limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.CustomerResponse, len(rows))
	for i, r := range rows {
		items[i] = customerToResponse(r)
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.CustomerResponse]]{
		Success: true, Message: "Berhasil mengambil data pelanggan",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

// GetCustomer mengembalikan satu pelanggan tenant.
func GetCustomer(c *gin.Context) {
	var row models.Customer
	if err := repositories.FindCustomerVisible(c.Request.Context(), c.Param("id"), &row); err != nil {
		notFoundOr(c, err, repositories.ErrCustomerNotFound, "Pelanggan tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.CustomerResponse]{
		Success: true, Message: "Berhasil mengambil data pelanggan", Data: customerToResponse(row),
	})
}

// CreateCustomer menambah pelanggan.
func CreateCustomer(c *gin.Context) {
	var req structs.CustomerCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	ctx := c.Request.Context()
	ownerID := reqctx.UserID(ctx)
	row := models.Customer{
		TenantID:    reqctx.TenantID(ctx),
		Code:        nilIfEmpty(req.Code),
		Name:        req.Name,
		Phone:       nilIfEmpty(req.Phone),
		Email:       req.Email,
		Address:     req.Address,
		Type:        orDefault(req.Type, "person"),
		PriceListID: nilIfEmpty(req.PriceListID),
		OwnerID:     nilIfEmpty(ownerID), // lapis 3: pemilik data CRM (§6, blueprint E.5)
		CreditLimit: req.CreditLimit,
		Note:        req.Note,
	}
	if err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		return repositories.CreateCustomer(ctx, tx, &row)
	}); err != nil {
		if helpers.IsDuplicateEntryError(err) {
			conflict(c, "phone", "Nomor telepon pelanggan sudah terdaftar")
			return
		}
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.CustomerResponse]{
		Success: true, Message: "Pelanggan berhasil dibuat", Data: customerToResponse(row),
	})
}

// UpdateCustomer mengubah pelanggan.
func UpdateCustomer(c *gin.Context) {
	var req structs.CustomerUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	ctx := c.Request.Context()
	var row models.Customer
	if err := repositories.FindCustomerInTenant(ctx, nil, c.Param("id"), &row); err != nil {
		notFoundOr(c, err, repositories.ErrCustomerNotFound, "Pelanggan tidak ditemukan")
		return
	}
	if req.Name != nil {
		row.Name = *req.Name
	}
	if req.Code != nil {
		row.Code = nilIfEmpty(*req.Code)
	}
	if req.Phone != nil {
		row.Phone = nilIfEmpty(*req.Phone)
	}
	if req.Email != nil {
		row.Email = *req.Email
	}
	if req.Address != nil {
		row.Address = *req.Address
	}
	if req.Type != nil {
		row.Type = *req.Type
	}
	if req.PriceListID != nil {
		row.PriceListID = nilIfEmpty(*req.PriceListID)
	}
	if req.CreditLimit != nil {
		row.CreditLimit = *req.CreditLimit
	}
	if req.Note != nil {
		row.Note = *req.Note
	}
	if err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		return repositories.UpdateCustomer(ctx, tx, &row)
	}); err != nil {
		if helpers.IsDuplicateEntryError(err) {
			conflict(c, "phone", "Nomor telepon pelanggan sudah terdaftar")
			return
		}
		notFoundOr(c, err, repositories.ErrCustomerNotFound, "Pelanggan tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.CustomerResponse]{
		Success: true, Message: "Pelanggan berhasil diperbarui", Data: customerToResponse(row),
	})
}

// DeleteCustomer menghapus (soft delete) pelanggan.
func DeleteCustomer(c *gin.Context) {
	ctx := c.Request.Context()
	if err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		return repositories.DeleteCustomer(ctx, tx, c.Param("id"))
	}); err != nil {
		notFoundOr(c, err, repositories.ErrCustomerNotFound, "Pelanggan tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[any]{Success: true, Message: "Pelanggan berhasil dihapus", Data: nil})
}
