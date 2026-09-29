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

// Handler dokumen CRM — penawaran, proyek, invoice pelanggan bertermin
// (Fase 9, §5.9, blueprint E.2).

// ── Quotation ─────────────────────────────────────────────────────────────

func ListQuotations(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	f := repositories.QuotationFilter{CustomerID: c.Query("customer_id"), Status: c.Query("status")}
	rows, total, err := repositories.ListQuotations(c.Request.Context(), f, limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.QuotationResponse, len(rows))
	for i := range rows {
		items[i] = services.QuotationToResponse(rows[i])
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.QuotationResponse]]{
		Success: true, Message: "Daftar penawaran",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

func GetQuotation(c *gin.Context) {
	q, err := repositories.FindQuotation(c.Request.Context(), nil, c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrQuotationNotFound, "Penawaran tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.QuotationResponse]{
		Success: true, Message: "Detail penawaran", Data: services.QuotationToResponse(q),
	})
}

func CreateQuotation(c *gin.Context) {
	var req structs.QuotationCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.CreateQuotation(c.Request.Context(), req)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.QuotationResponse]{
		Success: true, Message: "Penawaran dibuat", Data: res,
	})
}

func SendQuotation(c *gin.Context) {
	res, err := services.SendQuotation(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrQuotationNotFound, "Penawaran tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.QuotationResponse]{
		Success: true, Message: "Penawaran dikirim", Data: res,
	})
}

func AcceptQuotation(c *gin.Context) {
	res, err := services.AcceptQuotation(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrQuotationNotFound, "Penawaran tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.QuotationResponse]{
		Success: true, Message: "Penawaran disetujui — proyek dibuat", Data: res,
	})
}

func RejectQuotation(c *gin.Context) {
	res, err := services.RejectQuotation(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrQuotationNotFound, "Penawaran tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.QuotationResponse]{
		Success: true, Message: "Penawaran ditolak", Data: res,
	})
}

// ── Project ───────────────────────────────────────────────────────────────

func ListProjects(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	f := repositories.ProjectFilter{CustomerID: c.Query("customer_id"), Status: c.Query("status")}
	rows, total, err := repositories.ListProjects(c.Request.Context(), f, limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.ProjectResponse, len(rows))
	for i := range rows {
		items[i] = services.ProjectToResponse(rows[i])
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.ProjectResponse]]{
		Success: true, Message: "Daftar proyek",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

func GetProject(c *gin.Context) {
	p, err := repositories.FindProject(c.Request.Context(), nil, c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrProjectNotFound, "Proyek tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.ProjectResponse]{
		Success: true, Message: "Detail proyek", Data: services.ProjectToResponse(p),
	})
}

func CreateProject(c *gin.Context) {
	var req structs.ProjectCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.CreateProject(c.Request.Context(), req)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.ProjectResponse]{
		Success: true, Message: "Proyek dibuat", Data: res,
	})
}

func UpdateProject(c *gin.Context) {
	var req structs.ProjectUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.UpdateProject(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		notFoundOr(c, err, repositories.ErrProjectNotFound, "Proyek tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.ProjectResponse]{
		Success: true, Message: "Proyek diperbarui", Data: res,
	})
}

func AddProjectTask(c *gin.Context) {
	var req structs.ProjectTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.AddProjectTask(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		notFoundOr(c, err, repositories.ErrProjectNotFound, "Proyek tidak ditemukan")
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.ProjectResponse]{
		Success: true, Message: "Tugas ditambahkan", Data: res,
	})
}

func AddProjectExpense(c *gin.Context) {
	var req structs.ProjectExpenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.AddProjectExpense(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		notFoundOr(c, err, repositories.ErrProjectNotFound, "Proyek tidak ditemukan")
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.ProjectResponse]{
		Success: true, Message: "Biaya proyek dicatat", Data: res,
	})
}

// ── Invoice ───────────────────────────────────────────────────────────────

func ListInvoices(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	f := repositories.InvoiceFilter{
		CustomerID: c.Query("customer_id"), ProjectID: c.Query("project_id"), Status: c.Query("status"),
	}
	rows, total, err := repositories.ListInvoices(c.Request.Context(), f, limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.InvoiceResponse, len(rows))
	for i := range rows {
		items[i] = services.InvoiceToResponse(rows[i])
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.InvoiceResponse]]{
		Success: true, Message: "Daftar invoice",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

func GetInvoice(c *gin.Context) {
	in, err := repositories.FindInvoice(c.Request.Context(), nil, c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrInvoiceNotFound, "Invoice tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.InvoiceResponse]{
		Success: true, Message: "Detail invoice", Data: services.InvoiceToResponse(in),
	})
}

func CreateInvoice(c *gin.Context) {
	var req structs.InvoiceCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.CreateInvoice(c.Request.Context(), req)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.InvoiceResponse]{
		Success: true, Message: "Invoice dibuat", Data: res,
	})
}

func SendInvoice(c *gin.Context) {
	res, err := services.SendInvoice(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrInvoiceNotFound, "Invoice tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.InvoiceResponse]{
		Success: true, Message: "Invoice dikirim", Data: res,
	})
}

func VoidInvoice(c *gin.Context) {
	res, err := services.VoidInvoice(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrInvoiceNotFound, "Invoice tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.InvoiceResponse]{
		Success: true, Message: "Invoice dibatalkan", Data: res,
	})
}

// PayInvoice mencatat pembayaran bertahap. Wajib header Idempotency-Key.
func PayInvoice(c *gin.Context) {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		badRequest(c, "body", "Gagal membaca body")
		return
	}
	var req structs.InvoicePaymentRequest
	if err := bindJSONBytes(raw, &req); err != nil {
		validationFailed(c, err)
		return
	}
	status, body, err := services.PayInvoice(c.Request.Context(), services.InvoicePayInput{
		InvoiceID:      req.InvoiceID,
		Amount:         req.Amount,
		Method:         req.Method,
		ProofURL:       req.ProofURL,
		IdempotencyKey: c.GetHeader("Idempotency-Key"),
		RequestHash:    helpers.SHA256Hex(raw),
	})
	if err != nil {
		notFoundOr(c, err, repositories.ErrInvoiceNotFound, "Invoice tidak ditemukan")
		return
	}
	c.Data(status, "application/json; charset=utf-8", body)
}
