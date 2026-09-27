package controllers

import (
	"io"
	"net/http"

	"candra/backend-api/helpers"
	"candra/backend-api/repositories"
	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// Handler langganan & tagihan platform (Fase 7, §5.13). Semua di bawah
// /api/v1, dijaga permission `billing.manage` — kecuali GET /plans (katalog,
// cukup terautentikasi).

// ListPlans: GET /api/v1/plans — katalog paket + tabel harga per masa langganan.
func ListPlans(c *gin.Context) {
	plans, err := services.ListPlans(c.Request.Context())
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.PlanResponse]{
		Success: true, Message: "Katalog paket", Data: plans,
	})
}

// GetSubscription: GET /api/v1/subscription — status langganan + tagihan terbuka.
func GetSubscription(c *gin.Context) {
	res, err := services.GetSubscriptionOverview(c.Request.Context())
	if err != nil {
		notFoundOr(c, err, repositories.ErrSubscriptionNotFound, "Tenant belum berlangganan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.SubscriptionOverviewResponse]{
		Success: true, Message: "Status langganan", Data: res,
	})
}

// StartSubscription: POST /api/v1/subscription — pilih paket + masa, masuk trial.
func StartSubscription(c *gin.Context) {
	var req structs.SubscriptionStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.StartSubscription(c.Request.Context(), req.PlanCode, req.TermMonths)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.SubscriptionResponse]{
		Success: true, Message: "Langganan dimulai (trial)", Data: res,
	})
}

// GenerateSubInvoice: POST /api/v1/subscription/invoices — terbitkan tagihan.
func GenerateSubInvoice(c *gin.Context) {
	res, err := services.GenerateInvoice(c.Request.Context())
	if err != nil {
		notFoundOr(c, err, repositories.ErrSubscriptionNotFound, "Tenant belum berlangganan")
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.SubInvoiceResponse]{
		Success: true, Message: "Tagihan diterbitkan", Data: res,
	})
}

// ListSubInvoices: GET /api/v1/subscription/invoices — daftar tagihan tenant.
func ListSubInvoices(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	rows, total, err := repositories.ListSubInvoicesForTenant(c.Request.Context(), limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.SubInvoiceResponse, len(rows))
	for i := range rows {
		items[i] = services.SubInvoiceToResponse(rows[i])
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.SubInvoiceResponse]]{
		Success: true, Message: "Daftar tagihan langganan",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

// SubmitPaymentClaim: POST /api/v1/subscription-payment-claims — tenant
// mengonfirmasi pembayaran tagihan (idempoten). Paket aktif setelah staf
// keuangan platform menyetujuinya; tenant tidak lagi bisa menandai tagihannya
// sendiri lunas.
func SubmitPaymentClaim(c *gin.Context) {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		badRequest(c, "body", "Gagal membaca body")
		return
	}
	var req structs.PaymentClaimRequest
	if err := bindJSONBytes(raw, &req); err != nil {
		validationFailed(c, err)
		return
	}
	status, body, err := services.SubmitPaymentClaim(c.Request.Context(), services.SubmitClaimInput{
		InvoiceID:      req.InvoiceID,
		Amount:         req.Amount,
		Method:         req.Method,
		Reference:      req.Reference,
		Note:           req.Note,
		IdempotencyKey: c.GetHeader("Idempotency-Key"),
		RequestHash:    helpers.SHA256Hex(raw),
	})
	if err != nil {
		notFoundOr(c, err, repositories.ErrSubInvoiceNotFound, "Tagihan tidak ditemukan")
		return
	}
	c.Data(status, "application/json; charset=utf-8", body)
}

// ListPaymentClaims: GET /api/v1/subscription-payment-claims — riwayat
// konfirmasi pembayaran milik tenant.
func ListPaymentClaims(c *gin.Context) {
	rows, err := services.ListPaymentClaims(c.Request.Context())
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.PaymentClaimResponse]{
		Success: true, Message: "Daftar konfirmasi pembayaran", Data: rows,
	})
}

// CancelSubscription: POST /api/v1/subscription/cancel — batal + hitung refund.
func CancelSubscription(c *gin.Context) {
	var req structs.SubscriptionCancelRequest
	_ = c.ShouldBindJSON(&req) // body opsional
	res, err := services.CancelSubscription(c.Request.Context(), req.Reason)
	if err != nil {
		notFoundOr(c, err, repositories.ErrSubscriptionNotFound, "Tenant belum berlangganan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.SubscriptionCancelResponse]{
		Success: true, Message: "Langganan dibatalkan", Data: res,
	})
}

// ChangeSubscriptionPlan: POST /api/v1/subscription/change-plan — prorata.
func ChangeSubscriptionPlan(c *gin.Context) {
	var req structs.SubscriptionChangePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.ChangePlan(c.Request.Context(), req.PlanCode, req.TermMonths)
	if err != nil {
		notFoundOr(c, err, repositories.ErrSubscriptionNotFound, "Tenant belum berlangganan")
		return
	}
	msg := "Tagihan ganti paket diterbitkan — paket berpindah setelah dibayar"
	if res.Status == "paid" {
		msg = "Paket berpindah — biayanya tertutup sisa masa paket lama"
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.SubInvoiceResponse]{
		Success: true, Message: msg, Data: res,
	})
}

// VoidPlanChangeInvoice: POST /api/v1/subscription/invoices/:id/void —
// membatalkan tagihan ganti paket yang belum dibayar (paket tidak berubah).
func VoidPlanChangeInvoice(c *gin.Context) {
	res, err := services.VoidPlanChangeInvoice(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrSubInvoiceNotFound, "Tagihan tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.SubInvoiceResponse]{
		Success: true, Message: "Ganti paket dibatalkan", Data: res,
	})
}
