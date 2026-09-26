package controllers

import (
	"io"
	"net/http"

	"candra/backend-api/helpers"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

const checkoutMaxBodyBytes = 1 << 20 // 1 MB

// Checkout menjalankan transaksi kasir. Wajib header Idempotency-Key. Response
// (dan replay-nya) dibentuk oleh service agar identik antar pengulangan.
func Checkout(c *gin.Context) {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, checkoutMaxBodyBytes))
	if err != nil {
		badRequest(c, "body", "Gagal membaca body")
		return
	}

	var req structs.CheckoutRequest
	if err := bindJSONBytes(raw, &req); err != nil {
		validationFailed(c, err)
		return
	}

	in, err := services.BuildCheckoutInput(req, c.GetHeader("Idempotency-Key"), helpers.SHA256Hex(raw))
	if err != nil {
		respondServiceError(c, err)
		return
	}

	status, body, _, err := services.Checkout(c.Request.Context(), in)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.Data(status, "application/json; charset=utf-8", body)
}

// ListSales mengembalikan daftar transaksi (tanpa item/pembayaran).
func ListSales(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	f := repositories.SaleFilter{
		OutletID:     c.Query("outlet_id"),
		BusinessDate: c.Query("business_date"),
		Status:       c.Query("status"),
		ShiftID:      c.Query("shift_id"),
	}
	rows, total, err := repositories.ListSales(c.Request.Context(), f, limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.SaleResponse, len(rows))
	for i := range rows {
		items[i] = services.SaleToResponse(&rows[i])
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.SaleResponse]]{
		Success: true, Message: "Berhasil mengambil data transaksi",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

// GetSale mengembalikan satu transaksi lengkap dengan item & pembayaran.
func GetSale(c *gin.Context) {
	var sale models.Sale
	if err := repositories.FindSaleInTenant(c.Request.Context(), nil, c.Param("id"), &sale); err != nil {
		notFoundOr(c, err, repositories.ErrSaleNotFound, "Transaksi tidak ditemukan")
		return
	}
	// Transaksi cabang lain diperlakukan seperti tidak ada (batas cabang,
	// sama seperti lapis 3 CRM) — keberadaannya pun tidak dibocorkan.
	if !repositories.OutletVisible(c.Request.Context(), sale.OutletID) {
		notFound(c, "Transaksi tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.SaleResponse]{
		Success: true, Message: "Berhasil mengambil data transaksi", Data: services.SaleToResponse(&sale),
	})
}

// VoidSale membatalkan transaksi.
func VoidSale(c *gin.Context) {
	var req structs.VoidSaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.VoidSale(c.Request.Context(), c.Param("id"), req.Reason)
	if err != nil {
		notFoundOr(c, err, repositories.ErrSaleNotFound, "Transaksi tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.SaleResponse]{
		Success: true, Message: "Transaksi dibatalkan", Data: *res,
	})
}

// RefundSale membuat retur penuh atas sebuah transaksi.
func RefundSale(c *gin.Context) {
	var req structs.RefundSaleRequest
	_ = c.ShouldBindJSON(&req) // body opsional
	res, err := services.RefundSale(c.Request.Context(), c.Param("id"), req.Reason)
	if err != nil {
		notFoundOr(c, err, repositories.ErrSaleNotFound, "Transaksi tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.SaleResponse]{
		Success: true, Message: "Retur berhasil", Data: *res,
	})
}

// SalesSummary mengembalikan agregat transaksi pada rentang business_date.
func SalesSummary(c *gin.Context) {
	from := c.Query("from")
	to := c.Query("to")
	if from == "" || to == "" {
		badRequest(c, "from", "Parameter from & to (YYYY-MM-DD) wajib")
		return
	}
	s, err := repositories.SummarizeSales(c.Request.Context(), c.Query("outlet_id"), from, to)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[repositories.SalesSummary]{
		Success: true, Message: "Ringkasan transaksi", Data: s,
	})
}
