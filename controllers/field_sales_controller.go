package controllers

import (
	"net/http"

	"candra/backend-api/helpers"
	"candra/backend-api/repositories"
	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// Handler CRM sales lapangan — rencana kunjungan, kunjungan, target, komisi
// (Fase 10, §5.9, blueprint E.3).

// ── Visit plan ────────────────────────────────────────────────────────────

func CreateVisitPlan(c *gin.Context) {
	var req structs.VisitPlanCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.CreateVisitPlan(c.Request.Context(), req)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.VisitPlanResponse]{
		Success: true, Message: "Rencana kunjungan dibuat", Data: res,
	})
}

func ListVisitPlans(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	rows, total, err := repositories.ListVisitPlans(c.Request.Context(), c.Query("date"), limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.VisitPlanResponse, len(rows))
	for i := range rows {
		items[i] = services.VisitPlanToResponse(rows[i])
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.VisitPlanResponse]]{
		Success: true, Message: "Daftar rencana kunjungan",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

func GetVisitPlan(c *gin.Context) {
	p, err := repositories.FindVisitPlan(c.Request.Context(), nil, c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrVisitPlanNotFound, "Rencana kunjungan tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.VisitPlanResponse]{
		Success: true, Message: "Detail rencana kunjungan", Data: services.VisitPlanToResponse(p),
	})
}

// ── Visit ─────────────────────────────────────────────────────────────────

func ListVisits(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	f := repositories.VisitFilter{
		BusinessDate: c.Query("business_date"), Result: c.Query("result"),
		CustomerID: c.Query("customer_id"), VisitPlanID: c.Query("visit_plan_id"),
	}
	rows, total, err := repositories.ListVisits(c.Request.Context(), f, limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.VisitResponse, len(rows))
	for i := range rows {
		items[i] = services.VisitToResponse(rows[i])
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.VisitResponse]]{
		Success: true, Message: "Daftar kunjungan",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

func GetVisit(c *gin.Context) {
	v, err := repositories.FindVisit(c.Request.Context(), nil, c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrVisitNotFound, "Kunjungan tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.VisitResponse]{
		Success: true, Message: "Detail kunjungan", Data: services.VisitToResponse(v),
	})
}

// UpsertVisit menangani POST /visits (check-in atau kiriman offline). Idempoten
// per id klien.
func UpsertVisit(c *gin.Context) {
	var req structs.VisitUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.UpsertVisit(c.Request.Context(), services.VisitUpsertInput{
		ID: req.ID, VisitPlanID: req.VisitPlanID, CustomerID: req.CustomerID,
		CheckinAt: req.CheckinAt, CheckoutAt: req.CheckoutAt,
		CheckinLat: req.CheckinLat, CheckinLng: req.CheckinLng,
		PhotoURL: req.PhotoURL, Result: req.Result,
		NoOrderReason: req.NoOrderReason, SaleID: req.SaleID,
	})
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.VisitResponse]{
		Success: true, Message: "Kunjungan tersimpan", Data: res,
	})
}

func CheckoutVisit(c *gin.Context) {
	var req structs.VisitCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.CheckoutVisit(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		notFoundOr(c, err, repositories.ErrVisitNotFound, "Kunjungan tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.VisitResponse]{
		Success: true, Message: "Kunjungan selesai", Data: res,
	})
}

// ── Sales target ──────────────────────────────────────────────────────────

func SetSalesTarget(c *gin.Context) {
	var req structs.SalesTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.SetSalesTarget(c.Request.Context(), req)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.SalesTargetResponse]{
		Success: true, Message: "Target sales tersimpan", Data: res,
	})
}

func ListSalesTargets(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	rows, total, err := services.ListSalesTargets(c.Request.Context(), c.Query("user_id"), limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.SalesTargetResponse]]{
		Success: true, Message: "Daftar target sales",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, rows),
	})
}

// ── Commission ────────────────────────────────────────────────────────────

func ComputeCommission(c *gin.Context) {
	var req structs.CommissionComputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.ComputeCommission(c.Request.Context(), req)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.CommissionResponse]{
		Success: true, Message: "Komisi dihitung", Data: res,
	})
}

func ListCommissions(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	rows, total, err := repositories.ListCommissions(c.Request.Context(), c.Query("user_id"), c.Query("status"), limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.CommissionResponse, len(rows))
	for i := range rows {
		items[i] = services.CommissionToResponse(rows[i])
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.CommissionResponse]]{
		Success: true, Message: "Daftar komisi",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

func ApproveCommission(c *gin.Context) {
	res, err := services.ApproveCommission(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrCommissionNotFound, "Komisi tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.CommissionResponse]{
		Success: true, Message: "Komisi disetujui", Data: res,
	})
}

func PayCommission(c *gin.Context) {
	res, err := services.PayCommission(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrCommissionNotFound, "Komisi tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.CommissionResponse]{
		Success: true, Message: "Komisi dibayar", Data: res,
	})
}
