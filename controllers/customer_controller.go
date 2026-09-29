package controllers

import (
	"context"
	"net/http"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/services"
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
	if err := lampirkanStatistikPelanggan(c.Request.Context(), items, hariUsahaOutlet(c, c.Query("outlet_id"))); err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.CustomerResponse]]{
		Success: true, Message: "Berhasil mengambil data pelanggan",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

// lampirkanStatistikPelanggan mengisi Stats setiap pelanggan di satu halaman
// daftar — dua query untuk seluruh halaman, bukan dua per pelanggan.
//
// Pelanggan yang belum pernah belanja tetap mendapat Stats bernilai nol:
// "belum pernah belanja" adalah informasi, bukan data yang hilang. Sisa kasbon
// hanya dihitung & dikirim bila pengguna memegang receivable.manage; yang
// lewat jatuh tempo dibanding `hariIni` (tanggal usaha toko).
func lampirkanStatistikPelanggan(ctx context.Context, items []structs.CustomerResponse, hariIni time.Time) error {
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	belanja, err := repositories.CustomerSpending(ctx, ids)
	if err != nil {
		return err
	}
	var kasbon map[string]repositories.CustomerOutstanding
	bolehKasbon := reqctx.HasPermission(ctx, "receivable.manage")
	if bolehKasbon {
		if kasbon, err = repositories.CustomerReceivableOutstanding(ctx, ids, hariIni); err != nil {
			return err
		}
	}
	for i := range items {
		b := belanja[items[i].ID]
		st := &structs.CustomerStats{VisitCount: b.VisitCount, TotalSpent: b.TotalSpent}
		if b.LastVisit != nil {
			st.LastVisitAt = b.LastVisit.UTC().Format(timeLayout)
		}
		if bolehKasbon {
			k := kasbon[items[i].ID]
			st.ReceivableOutstanding, st.ReceivableOverdue = &k.Outstanding, &k.Overdue
		}
		items[i].Stats = st
	}
	return nil
}

// GetCustomer mengembalikan satu pelanggan tenant, dengan ringkasan belanja
// & kasbonnya (sama dengan satu baris daftar).
func GetCustomer(c *gin.Context) {
	var row models.Customer
	if err := repositories.FindCustomerVisible(c.Request.Context(), c.Param("id"), &row); err != nil {
		notFoundOr(c, err, repositories.ErrCustomerNotFound, "Pelanggan tidak ditemukan")
		return
	}
	items := []structs.CustomerResponse{customerToResponse(row)}
	if err := lampirkanStatistikPelanggan(c.Request.Context(), items, hariUsahaOutlet(c, c.Query("outlet_id"))); err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.CustomerResponse]{
		Success: true, Message: "Berhasil mengambil data pelanggan", Data: items[0],
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
	// Daftar harga tak dikenal ditolak dengan pesan, bukan galat FK (500).
	if err := services.ValidateCustomerPriceList(ctx, req.PriceListID); err != nil {
		respondServiceError(c, err)
		return
	}
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

		CreditTermDays: req.CreditTermDays,
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
		if err := services.ValidateCustomerPriceList(ctx, *req.PriceListID); err != nil {
			respondServiceError(c, err)
			return
		}
		row.PriceListID = nilIfEmpty(*req.PriceListID)
	}
	if req.CreditLimit != nil {
		row.CreditLimit = *req.CreditLimit
	}
	if req.CreditTermDays != nil {
		row.CreditTermDays = *req.CreditTermDays
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

// CustomerTopProducts: 5 barang yang paling sering dibeli pelanggan.
func CustomerTopProducts(c *gin.Context) {
	ctx := c.Request.Context()
	var row models.Customer
	if err := repositories.FindCustomerVisible(ctx, c.Param("id"), &row); err != nil {
		notFoundOr(c, err, repositories.ErrCustomerNotFound, "Pelanggan tidak ditemukan")
		return
	}
	rows, err := repositories.CustomerTopProducts(ctx, row.ID, 5)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	out := make([]structs.CustomerTopProduct, len(rows))
	for i, r := range rows {
		out[i] = structs.CustomerTopProduct{
			ProductID: r.ProductID, ProductName: r.ProductName, UnitName: r.UnitName, Times: r.Times,
			Qty: r.Qty.String(), LastAt: r.LastAt.UTC().Format(timeLayout),
		}
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.CustomerTopProduct]{
		Success: true, Message: "Barang yang sering dibeli", Data: out,
	})
}
