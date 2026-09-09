package controllers

import (
	"net/http"

	"candra/backend-api/helpers"
	"candra/backend-api/repositories"
	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// Handler SDM — karyawan, jadwal, libur, absensi, cuti, kasbon (Fase 13, §5.11).

// ── Employee ──────────────────────────────────────────────────────────────

func ListEmployees(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	rows, total, err := repositories.ListEmployees(c.Request.Context(), c.Query("outlet_id"), c.Query("active") == "true", limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.EmployeeResponse, len(rows))
	for i := range rows {
		items[i] = services.EmployeeToResponse(rows[i])
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.EmployeeResponse]]{
		Success: true, Message: "Daftar karyawan",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

func GetEmployee(c *gin.Context) {
	e, err := repositories.FindEmployee(c.Request.Context(), nil, c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrEmployeeNotFound, "Karyawan tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.EmployeeResponse]{
		Success: true, Message: "Detail karyawan", Data: services.EmployeeToResponse(e),
	})
}

func CreateEmployee(c *gin.Context) {
	var req structs.EmployeeCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.CreateEmployee(c.Request.Context(), req)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.EmployeeResponse]{
		Success: true, Message: "Karyawan dibuat", Data: res,
	})
}

func UpdateEmployee(c *gin.Context) {
	var req structs.EmployeeUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.UpdateEmployee(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		notFoundOr(c, err, repositories.ErrEmployeeNotFound, "Karyawan tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.EmployeeResponse]{
		Success: true, Message: "Karyawan diperbarui", Data: res,
	})
}

func SetWorkSchedule(c *gin.Context) {
	var req structs.WorkScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.SetWorkSchedule(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		notFoundOr(c, err, repositories.ErrEmployeeNotFound, "Karyawan tidak ditemukan")
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.WorkScheduleResponse]{
		Success: true, Message: "Jadwal kerja tersimpan", Data: res,
	})
}

// ── Holiday ───────────────────────────────────────────────────────────────

func ListHolidays(c *gin.Context) {
	rows, err := repositories.ListHolidays(c.Request.Context(), c.Query("from"), c.Query("to"))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.HolidayResponse, len(rows))
	for i, h := range rows {
		items[i] = structs.HolidayResponse{ID: h.ID, HolidayDate: h.HolidayDate.Format("2006-01-02"), Name: h.Name, IsPaid: h.IsPaid}
		if h.OutletID != nil {
			items[i].OutletID = *h.OutletID
		}
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.HolidayResponse]{
		Success: true, Message: "Hari libur", Data: items,
	})
}

func CreateHoliday(c *gin.Context) {
	var req structs.HolidayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.CreateHoliday(c.Request.Context(), req)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.HolidayResponse]{
		Success: true, Message: "Hari libur dibuat", Data: res,
	})
}

// ── Attendance ────────────────────────────────────────────────────────────

func ListAttendances(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	rows, total, err := repositories.ListAttendances(c.Request.Context(), c.Query("employee_id"), c.Query("business_date"), limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.AttendanceResponse, len(rows))
	for i := range rows {
		items[i] = services.AttendanceToResponse(rows[i])
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.AttendanceResponse]]{
		Success: true, Message: "Daftar absensi",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

func RecordAttendance(c *gin.Context) {
	var req structs.AttendanceRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.RecordAttendance(c.Request.Context(), req)
	if err != nil {
		notFoundOr(c, err, repositories.ErrEmployeeNotFound, "Karyawan tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.AttendanceResponse]{
		Success: true, Message: "Absensi tercatat", Data: res,
	})
}

func RequestCorrection(c *gin.Context) {
	var req structs.CorrectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.RequestCorrection(c.Request.Context(), req)
	if err != nil {
		notFoundOr(c, err, repositories.ErrAttendanceNotFound, "Absensi tidak ditemukan")
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.CorrectionResponse]{
		Success: true, Message: "Koreksi diajukan", Data: res,
	})
}

func ApproveCorrection(c *gin.Context) {
	var req structs.CorrectionApproveRequest
	_ = c.ShouldBindJSON(&req) // body opsional
	res, err := services.ApproveCorrection(c.Request.Context(), c.Param("id"), req.Adjustment)
	if err != nil {
		notFoundOr(c, err, repositories.ErrCorrectionNotFound, "Koreksi tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.CorrectionResponse]{
		Success: true, Message: "Koreksi disetujui", Data: res,
	})
}

// ── Leave ─────────────────────────────────────────────────────────────────

func CreateLeaveRequest(c *gin.Context) {
	var req structs.LeaveRequestCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.CreateLeaveRequest(c.Request.Context(), req)
	if err != nil {
		notFoundOr(c, err, repositories.ErrEmployeeNotFound, "Karyawan tidak ditemukan")
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.LeaveRequestResponse]{
		Success: true, Message: "Pengajuan cuti dibuat", Data: res,
	})
}

func ApproveLeaveRequest(c *gin.Context) {
	var req structs.LeaveApproveRequest
	_ = c.ShouldBindJSON(&req)
	res, err := services.ApproveLeaveRequest(c.Request.Context(), c.Param("id"), req, true)
	if err != nil {
		notFoundOr(c, err, repositories.ErrLeaveNotFound, "Pengajuan cuti tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.LeaveRequestResponse]{
		Success: true, Message: "Cuti disetujui", Data: res,
	})
}

func RejectLeaveRequest(c *gin.Context) {
	var req structs.LeaveApproveRequest
	_ = c.ShouldBindJSON(&req)
	res, err := services.ApproveLeaveRequest(c.Request.Context(), c.Param("id"), req, false)
	if err != nil {
		notFoundOr(c, err, repositories.ErrLeaveNotFound, "Pengajuan cuti tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.LeaveRequestResponse]{
		Success: true, Message: "Cuti ditolak", Data: res,
	})
}

// ── Employee advance ──────────────────────────────────────────────────────

func ListAdvances(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	rows, total, err := repositories.ListAdvances(c.Request.Context(), c.Query("employee_id"), c.Query("status"), limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.AdvanceResponse, len(rows))
	for i := range rows {
		items[i] = services.AdvanceToResponse(rows[i])
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.AdvanceResponse]]{
		Success: true, Message: "Daftar kasbon",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

func CreateAdvance(c *gin.Context) {
	var req structs.AdvanceCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.CreateAdvance(c.Request.Context(), req)
	if err != nil {
		notFoundOr(c, err, repositories.ErrEmployeeNotFound, "Karyawan tidak ditemukan")
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.AdvanceResponse]{
		Success: true, Message: "Kasbon diajukan", Data: res,
	})
}

func DisburseAdvance(c *gin.Context) {
	res, err := services.DisburseAdvance(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrAdvanceNotFound, "Kasbon tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.AdvanceResponse]{
		Success: true, Message: "Kasbon dicairkan", Data: res,
	})
}
