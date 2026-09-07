package controllers

import (
	"log/slog"

	"candra/backend-api/helpers"
	"candra/backend-api/models"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// timeLayout adalah format waktu seragam untuk semua response (CONVENTIONS §3).
const timeLayout = "2006-01-02 15:04:05"

// badRequest membalas 400 dengan satu pesan per field.
func badRequest(c *gin.Context, field, msg string) {
	c.JSON(400, structs.ErrorResponse{
		Success: false,
		Message: msg,
		Errors:  map[string]string{field: msg},
	})
}

// notFound membalas 404 dengan pesan generik.
func notFound(c *gin.Context, msg string) {
	c.JSON(404, structs.ErrorResponse{
		Success: false,
		Message: msg,
		Errors:  map[string]string{"resource": msg},
	})
}

// validationFailed membalas 422 dengan detail per field dari binding/validator.
func validationFailed(c *gin.Context, err error) {
	c.JSON(422, structs.ErrorResponse{
		Success: false,
		Message: "Validasi gagal",
		Errors:  helpers.TranslateErrorMessage(err),
	})
}

// respondServiceError memetakan error sentinel service ke response HTTP:
//   - < 500 → tampilkan pesan error apa adanya (pesan sentinel sudah ramah user).
//   - ≥ 500 → catat detail ke log, kirim pesan generik (§4: jangan bocorkan
//     detail internal).
func respondServiceError(c *gin.Context, err error) {
	status := helpers.StatusForError(err)
	if status >= 500 {
		helpers.LoggerFromContext(c.Request.Context()).Error("kesalahan service", slog.Any("error", err))
		c.JSON(status, structs.ErrorResponse{
			Success: false,
			Message: "Terjadi kesalahan internal",
			Errors:  map[string]string{"server": "Terjadi kesalahan internal"},
		})
		return
	}
	msg := err.Error()
	c.JSON(status, structs.ErrorResponse{
		Success: false,
		Message: msg,
		Errors:  map[string]string{"request": msg},
	})
}

// tenantToResponse memetakan model Tenant ke DTO.
func tenantToResponse(t models.Tenant) structs.TenantResponse {
	return structs.TenantResponse{
		ID:           t.ID,
		BusinessName: t.BusinessName,
		BusinessType: t.BusinessType,
		OwnerName:    t.OwnerName,
		Phone:        t.Phone,
		Email:        t.Email,
		Status:       t.Status,
		CreatedAt:    t.CreatedAt.Format(timeLayout),
	}
}

// outletToResponse memetakan model Outlet ke DTO. Nilai desimal & waktu-hari
// dikembalikan sebagai string.
func outletToResponse(o models.Outlet) structs.OutletResponse {
	return structs.OutletResponse{
		ID:                o.ID,
		Name:              o.Name,
		Type:              o.Type,
		Address:           o.Address,
		Phone:             o.Phone,
		Timezone:          o.Timezone,
		BusinessDayStart:  o.BusinessDayStart.String(),
		Currency:          o.Currency,
		TaxEnabled:        o.TaxEnabled,
		TaxRate:           o.TaxRate.String(),
		TaxInclusive:      o.TaxInclusive,
		ServiceChargeRate: o.ServiceChargeRate.String(),
		ReceiptHeader:     o.ReceiptHeader,
		ReceiptFooter:     o.ReceiptFooter,
		IsActive:          o.IsActive,
		CreatedAt:         o.CreatedAt.Format(timeLayout),
		UpdatedAt:         o.UpdatedAt.Format(timeLayout),
	}
}
