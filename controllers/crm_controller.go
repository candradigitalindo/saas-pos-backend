package controllers

import (
	"net/http"

	"candra/backend-api/helpers"
	"candra/backend-api/repositories"
	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// Handler CRM inti — sumber prospek, pipeline, deal, aktivitas (Fase 9, §5.9).
// Visibilitas kepemilikan (lapis 3) dijaga di repository.

// ── Lead source ───────────────────────────────────────────────────────────

func ListLeadSources(c *gin.Context) {
	rows, err := services.ListLeadSources(c.Request.Context())
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.LeadSourceResponse]{
		Success: true, Message: "Sumber prospek", Data: rows,
	})
}

func CreateLeadSource(c *gin.Context) {
	var req structs.LeadSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.CreateLeadSource(c.Request.Context(), req.Name)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.LeadSourceResponse]{
		Success: true, Message: "Sumber prospek dibuat", Data: res,
	})
}

// ── Pipeline ──────────────────────────────────────────────────────────────

func ListPipelines(c *gin.Context) {
	rows, err := services.ListPipelines(c.Request.Context())
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.PipelineResponse]{
		Success: true, Message: "Pipeline", Data: rows,
	})
}

func CreatePipeline(c *gin.Context) {
	var req structs.PipelineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.CreatePipeline(c.Request.Context(), req)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.PipelineResponse]{
		Success: true, Message: "Pipeline dibuat", Data: res,
	})
}

// ── Deal ──────────────────────────────────────────────────────────────────

func ListDeals(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	f := repositories.DealFilter{
		PipelineID: c.Query("pipeline_id"), StageID: c.Query("stage_id"),
		Status: c.Query("status"), CustomerID: c.Query("customer_id"),
	}
	rows, total, err := repositories.ListDeals(c.Request.Context(), f, limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.DealResponse, len(rows))
	for i := range rows {
		items[i] = services.DealToResponse(rows[i])
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.DealResponse]]{
		Success: true, Message: "Daftar deal",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

func GetDeal(c *gin.Context) {
	d, err := repositories.FindDeal(c.Request.Context(), nil, c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrDealNotFound, "Deal tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.DealResponse]{
		Success: true, Message: "Detail deal", Data: services.DealToResponse(d),
	})
}

func CreateDeal(c *gin.Context) {
	var req structs.DealCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.CreateDeal(c.Request.Context(), req)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.DealResponse]{
		Success: true, Message: "Deal dibuat", Data: res,
	})
}

func UpdateDeal(c *gin.Context) {
	var req structs.DealUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.UpdateDeal(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		notFoundOr(c, err, repositories.ErrDealNotFound, "Deal tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.DealResponse]{
		Success: true, Message: "Deal diperbarui", Data: res,
	})
}

func WinDeal(c *gin.Context) {
	res, err := services.WinDeal(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrDealNotFound, "Deal tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.DealResponse]{
		Success: true, Message: "Deal ditandai menang", Data: res,
	})
}

func LoseDeal(c *gin.Context) {
	var req structs.DealLoseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.LoseDeal(c.Request.Context(), c.Param("id"), req.LostReason)
	if err != nil {
		notFoundOr(c, err, repositories.ErrDealNotFound, "Deal tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.DealResponse]{
		Success: true, Message: "Deal ditandai kalah", Data: res,
	})
}

// ── Activity ──────────────────────────────────────────────────────────────

func ListActivities(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	f := repositories.ActivityFilter{
		DealID: c.Query("deal_id"), CustomerID: c.Query("customer_id"),
		Status: c.Query("status"), Kind: c.Query("kind"),
	}
	rows, total, err := repositories.ListActivities(c.Request.Context(), f, limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.ActivityResponse, len(rows))
	for i := range rows {
		items[i] = services.ActivityToResponse(rows[i])
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.ActivityResponse]]{
		Success: true, Message: "Daftar aktivitas",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

func CreateActivity(c *gin.Context) {
	var req structs.ActivityCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.CreateActivity(c.Request.Context(), req)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.ActivityResponse]{
		Success: true, Message: "Aktivitas dijadwalkan", Data: res,
	})
}

func CompleteActivity(c *gin.Context) {
	res, err := services.CompleteActivity(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrActivityNotFound, "Aktivitas tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.ActivityResponse]{
		Success: true, Message: "Aktivitas selesai", Data: res,
	})
}

func CancelActivity(c *gin.Context) {
	res, err := services.CancelActivity(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrActivityNotFound, "Aktivitas tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.ActivityResponse]{
		Success: true, Message: "Aktivitas dibatalkan", Data: res,
	})
}
