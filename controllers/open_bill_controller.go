package controllers

import (
	"net/http"

	"candra/backend-api/internal/ulid"
	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// Tagihan terbuka (open bill). Semua butuh izin sale.create — pelayan dan
// kasir sama-sama mencatat pesanan.

// ListOpenBills: GET /api/v1/open-bills?outlet_id=
func ListOpenBills(c *gin.Context) {
	outlet := c.Query("outlet_id")
	if !ulid.IsValid(outlet) {
		badRequest(c, "outlet_id", "outlet_id wajib diisi")
		return
	}
	res, err := services.ListOpenBills(c.Request.Context(), outlet)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.OpenBillResponse]{Success: true, Message: "Tagihan terbuka", Data: res})
}

// UpsertOpenBill: PUT /api/v1/open-bills/:id — id ULID dibuat klien (sama
// dengan jalur offline), jadi membuat & mengubah memakai satu rute.
// base_version 0 = tagihan baru → 201; selebihnya 200.
func UpsertOpenBill(c *gin.Context) {
	id := c.Param("id")
	if !ulid.IsValid(id) {
		badRequest(c, "id", "id tagihan tidak valid")
		return
	}
	var req structs.OpenBillUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, _, err := services.UpsertOpenBill(c.Request.Context(), id, "", req)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	kode := http.StatusOK
	if req.BaseVersion == 0 {
		kode = http.StatusCreated
	}
	c.JSON(kode, structs.SuccessResponse[structs.OpenBillResponse]{Success: true, Message: "Tagihan disimpan", Data: res})
}

// CancelOpenBill: POST /api/v1/open-bills/:id/cancel.
func CancelOpenBill(c *gin.Context) {
	var req structs.OpenBillCancelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	if _, err := services.CancelOpenBill(c.Request.Context(), c.Param("id"), "", req.BaseVersion); err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[any]{Success: true, Message: "Tagihan dibatalkan"})
}
