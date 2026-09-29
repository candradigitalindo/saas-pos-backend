package controllers

import (
	"net/http"

	"candra/backend-api/helpers"
	"candra/backend-api/repositories"
	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// Handler penggajian (Fase 13, §5.11, §13.6).

// ── Payroll rule ──────────────────────────────────────────────────────────

func ListPayrollRules(c *gin.Context) {
	rows, err := repositories.ListPayrollRules(c.Request.Context())
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.PayrollRuleResponse, len(rows))
	for i, r := range rows {
		items[i] = structs.PayrollRuleResponse{
			ID: r.ID, Code: r.Code, Name: r.Name, Type: r.Type, Category: r.Category,
			Params: r.Params, TargetType: r.TargetType, Priority: r.Priority,
			EffectiveFrom: r.EffectiveFrom.Format("2006-01-02"), IsActive: r.IsActive,
		}
		if r.TargetID != nil {
			items[i].TargetID = *r.TargetID
		}
		if r.EffectiveTo != nil {
			items[i].EffectiveTo = r.EffectiveTo.Format("2006-01-02")
		}
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.PayrollRuleResponse]{
		Success: true, Message: "Aturan gaji", Data: items,
	})
}

func CreatePayrollRule(c *gin.Context) {
	var req structs.PayrollRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.CreatePayrollRule(c.Request.Context(), req)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.PayrollRuleResponse]{
		Success: true, Message: "Aturan gaji dibuat", Data: res,
	})
}

// ── Payroll period ────────────────────────────────────────────────────────

func ListPayrollPeriods(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	rows, total, err := repositories.ListPayrollPeriods(c.Request.Context(), limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.PayrollPeriodResponse, len(rows))
	for i, p := range rows {
		items[i] = structs.PayrollPeriodResponse{
			ID: p.ID, PeriodType: p.PeriodType,
			StartDate: p.StartDate.Format("2006-01-02"), EndDate: p.EndDate.Format("2006-01-02"),
			Status: p.Status, TotalGross: p.TotalGross, TotalDeduction: p.TotalDeduction, TotalNet: p.TotalNet,
		}
		if p.OutletID != nil {
			items[i].OutletID = *p.OutletID
		}
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.PayrollPeriodResponse]]{
		Success: true, Message: "Daftar periode gaji",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

func CreatePayrollPeriod(c *gin.Context) {
	var req structs.PayrollPeriodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.CreatePayrollPeriod(c.Request.Context(), req)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.PayrollPeriodResponse]{
		Success: true, Message: "Periode gaji dibuat", Data: res,
	})
}

func CalculatePayroll(c *gin.Context) {
	res, err := services.CalculatePayroll(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrPayrollPeriodNotFound, "Periode gaji tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PayrollPeriodResponse]{
		Success: true, Message: "Perhitungan gaji selesai", Data: res,
	})
}

func LockPayroll(c *gin.Context) {
	res, err := services.LockPayroll(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrPayrollPeriodNotFound, "Periode gaji tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PayrollPeriodResponse]{
		Success: true, Message: "Periode gaji dikunci", Data: res,
	})
}

func PayPayroll(c *gin.Context) {
	res, err := services.PayPayroll(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrPayrollPeriodNotFound, "Periode gaji tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PayrollPeriodResponse]{
		Success: true, Message: "Gaji dibayar", Data: res,
	})
}

// ── Payslip ───────────────────────────────────────────────────────────────

func ListPayslips(c *gin.Context) {
	rows, err := repositories.PayslipsForPeriod(c.Request.Context(), nil, c.Param("id"))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.PayslipResponse, len(rows))
	for i := range rows {
		items[i] = services.PayslipToResponse(rows[i])
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.PayslipResponse]{
		Success: true, Message: "Slip gaji periode", Data: items,
	})
}

func GetPayslip(c *gin.Context) {
	s, err := repositories.FindPayslip(c.Request.Context(), nil, c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrPayslipNotFound, "Slip gaji tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PayslipResponse]{
		Success: true, Message: "Detail slip gaji", Data: services.PayslipToResponse(s),
	})
}
