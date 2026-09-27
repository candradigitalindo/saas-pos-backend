package controllers

import (
	"errors"
	"io"
	"net/http"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// ListReceivables mengembalikan piutang tenant (opsional per pelanggan / status).
func ListReceivables(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	rows, total, err := repositories.ListReceivables(c.Request.Context(),
		c.Query("customer_id"), c.Query("status"), limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.ReceivableResponse, len(rows))
	for i, r := range rows {
		items[i] = services.ReceivableToResponse(r)
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.ReceivableResponse]]{
		Success: true, Message: "Berhasil mengambil data piutang",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

// GetReceivable mengembalikan satu piutang.
func GetReceivable(c *gin.Context) {
	var row models.Receivable
	if err := repositories.FindReceivableInTenant(c.Request.Context(), nil, c.Param("id"), &row); err != nil {
		notFoundOr(c, err, repositories.ErrReceivableNotFound, "Piutang tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.ReceivableResponse]{
		Success: true, Message: "Berhasil mengambil data piutang", Data: services.ReceivableToResponse(row),
	})
}

// AddReceivablePayment mencatat pembayaran cicilan/pelunasan piutang.
//
// Wajib header Idempotency-Key — setoran adalah uang masuk (§8).
func AddReceivablePayment(c *gin.Context) {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, checkoutMaxBodyBytes))
	if err != nil {
		badRequest(c, "body", "Gagal membaca body")
		return
	}
	var req structs.ReceivablePaymentRequest
	if err := bindJSONBytes(raw, &req); err != nil {
		validationFailed(c, err)
		return
	}
	ctx := c.Request.Context()
	if req.Method == "qris" {
		if err := services.RequireFeature(ctx, services.FeatureQRIS); err != nil {
			respondServiceError(c, err)
			return
		}
	}
	collectedBy := reqctx.UserID(ctx)
	// PaidAt dan BusinessDate sengaja TIDAK diisi di sini: business_date harus
	// dihitung dari zona waktu outlet, dan itu urusan service.
	pay := &models.ReceivablePayment{
		ReceivableID: req.ReceivableID,
		Amount:       req.Amount,
		Method:       req.Method,
		CollectedBy:  &collectedBy,
		ProofURL:     req.ProofURL,
	}
	status, body, err := services.AddReceivablePayment(ctx, pay,
		c.GetHeader("Idempotency-Key"), helpers.SHA256Hex(raw))
	switch {
	case errors.Is(err, repositories.ErrReceivableNotFound):
		notFound(c, "Piutang tidak ditemukan")
	case errors.Is(err, repositories.ErrReceivableSettled):
		conflict(c, "receivable", "Piutang sudah lunas atau dihapusbukukan")
	case errors.Is(err, repositories.ErrOverpay):
		badRequest(c, "amount", "Nominal melebihi sisa piutang")
	case err != nil:
		respondServiceError(c, err)
	default:
		c.Data(status, "application/json; charset=utf-8", body)
	}
}
