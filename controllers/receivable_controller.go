package controllers

import (
	"errors"
	"net/http"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
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
		items[i] = receivableToResponse(r)
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
		Success: true, Message: "Berhasil mengambil data piutang", Data: receivableToResponse(row),
	})
}

// AddReceivablePayment mencatat pembayaran cicilan/pelunasan piutang.
func AddReceivablePayment(c *gin.Context) {
	var req structs.ReceivablePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	ctx := c.Request.Context()
	collectedBy := reqctx.UserID(ctx)
	pay := &models.ReceivablePayment{
		ReceivableID: req.ReceivableID,
		Amount:       req.Amount,
		Method:       req.Method,
		PaidAt:       time.Now().UTC(),
		BusinessDate: time.Now().UTC(),
		CollectedBy:  &collectedBy,
		ProofURL:     req.ProofURL,
	}
	rec, err := repositories.AddReceivablePayment(ctx, pay)
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
		c.JSON(http.StatusCreated, structs.SuccessResponse[structs.ReceivableResponse]{
			Success: true, Message: "Pembayaran piutang dicatat", Data: receivableToResponse(rec),
		})
	}
}
