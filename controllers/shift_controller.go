package controllers

import (
	"net/http"

	"candra/backend-api/helpers"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// OpenShift membuka shift kas untuk sebuah outlet.
func OpenShift(c *gin.Context) {
	var req structs.ShiftOpenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	sh, err := services.OpenShift(c.Request.Context(), req.OutletID, req.OpeningCash, req.Note)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.ShiftResponse]{
		Success: true, Message: "Shift dibuka", Data: shiftToResponse(*sh),
	})
}

// CloseShift menutup shift dan mencatat selisih kas.
// HandoverShift: POST /api/v1/shifts/:id/handover
//
// Menutup shift berjalan dan membuka shift baru dalam SATU transaksi — lihat
// services.HandoverShift soal kenapa ini tidak boleh jadi dua panggilan.
func HandoverShift(c *gin.Context) {
	var req structs.ShiftHandoverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	lama, baru, err := services.HandoverShift(
		c.Request.Context(), c.Param("id"), req.CountedCash, req.OpeningCash, req.Note)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.ShiftHandoverResponse]{
		Success: true, Message: "Shift diserahterimakan",
		Data: structs.ShiftHandoverResponse{
			Ditutup: shiftToResponse(*lama), Dibuka: shiftToResponse(*baru),
		},
	})
}

func CloseShift(c *gin.Context) {
	var req structs.ShiftCloseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	sh, err := services.CloseShift(c.Request.Context(), c.Param("id"), req.CountedCash, req.Note)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.ShiftResponse]{
		Success: true, Message: "Shift ditutup", Data: shiftToResponse(*sh),
	})
}

// ListShifts mengembalikan riwayat shift (opsional per outlet).
func ListShifts(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	rows, total, err := repositories.ListShifts(c.Request.Context(), c.Query("outlet_id"), limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.ShiftResponse, len(rows))
	for i, r := range rows {
		items[i] = shiftToResponse(r)
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.ShiftResponse]]{
		Success: true, Message: "Berhasil mengambil data shift",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

// GetShift mengembalikan satu shift.
func GetShift(c *gin.Context) {
	ctx := c.Request.Context()

	var sh models.Shift
	if err := repositories.FindShiftInTenant(ctx, nil, c.Param("id"), &sh); err != nil {
		notFoundOr(c, err, repositories.ErrShiftNotFound, "Shift tidak ditemukan")
		return
	}
	if !repositories.OutletVisible(ctx, sh.OutletID) {
		notFound(c, "Shift tidak ditemukan")
		return
	}

	res := shiftToResponse(sh)

	// Rincian kas. Untuk shift terbuka, expected_cash di baris belum terisi
	// (baru ditulis saat penutupan), jadi dihitung di sini memakai fungsi yang
	// SAMA dengan CloseShift. Layar "Tutup Shift" perlu menampilkan rinciannya
	// sebelum kasir menghitung laci; pratinjau yang berbeda dari angka akhir
	// akan lebih merugikan daripada tidak ada pratinjau sama sekali.
	cashSales, cashIn, cashOut, err := repositories.ShiftCashTotals(ctx, nil, sh.ID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	res.CashSales, res.CashIn, res.CashOut = &cashSales, &cashIn, &cashOut
	if sh.Status == "open" {
		res.ExpectedCash = sh.OpeningCash + cashSales + cashIn - cashOut
	}

	ids := []string{sh.OpenedBy}
	if sh.ClosedBy != nil {
		ids = append(ids, *sh.ClosedBy)
	}
	nama, err := repositories.UserNames(ctx, ids)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	res.OpenedByName = nama[sh.OpenedBy]
	if sh.ClosedBy != nil {
		res.ClosedByName = nama[*sh.ClosedBy]
	}
	penjualan, err := services.ShiftSalesSummary(ctx, sh.ID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	res.Sales = &penjualan

	c.JSON(http.StatusOK, structs.SuccessResponse[structs.ShiftResponse]{
		Success: true, Message: "Berhasil mengambil data shift", Data: res,
	})
}

// CreateCashMovement mencatat kas masuk/keluar non-penjualan.
func CreateCashMovement(c *gin.Context) {
	var req structs.CashMovementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	mv, err := services.CreateCashMovement(c.Request.Context(), req.OutletID, req.ShiftID, req.Direction, req.Amount, req.Reason)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.CashMovementResponse]{
		Success: true, Message: "Gerakan kas dicatat", Data: cashMovementToResponse(*mv),
	})
}

// ListCashMovements mengembalikan gerakan kas satu shift.
func ListCashMovements(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	rows, total, err := repositories.ListCashMovements(c.Request.Context(), c.Query("shift_id"), limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.CreatedBy)
	}
	nama, err := repositories.UserNames(c.Request.Context(), ids)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.CashMovementResponse, len(rows))
	for i, r := range rows {
		items[i] = cashMovementToResponse(r)
		items[i].CreatedByName = nama[r.CreatedBy]
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.CashMovementResponse]]{
		Success: true, Message: "Berhasil mengambil gerakan kas",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}
