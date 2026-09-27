package controllers

import (
	"errors"
	"log/slog"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/models"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// bindJSONBytes meng-unmarshal raw ke obj DAN menjalankan validator binding
// (termasuk tag kustom seperti `ulid` & `dive`). Dipakai handler yang sudah
// membaca body sendiri, mis. checkout yang perlu meng-hash body mentah.
func bindJSONBytes(raw []byte, obj any) error {
	return binding.JSON.BindBody(raw, obj)
}

// timeLayout adalah format waktu seragam untuk semua response (CONVENTIONS §3):
// waktu di response SELALU UTC berformat RFC 3339 dengan penanda
// zona ("2026-09-26T00:22:10Z"). Format lama tanpa zona ("2006-01-02
// 15:04:05") ditafsirkan browser sebagai jam LOKAL perangkat, sehingga jam
// UTC dari server produksi tampil mundur 7 jam di layar WIB.
const timeLayout = time.RFC3339

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

// conflict membalas 409 dengan satu pesan per field.
func conflict(c *gin.Context, field, msg string) {
	c.JSON(409, structs.ErrorResponse{
		Success: false,
		Message: msg,
		Errors:  map[string]string{field: msg},
	})
}

// notFoundOr membalas 404 bila err adalah sentinel "tidak ditemukan" resource
// ini, atau menyerahkan ke respondServiceError untuk error lain.
func notFoundOr(c *gin.Context, err, notFoundSentinel error, msg string) {
	if errors.Is(err, notFoundSentinel) {
		notFound(c, msg)
		return
	}
	respondServiceError(c, err)
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
	msg := helpers.PesanUntukPengguna(err)
	c.JSON(status, structs.ErrorResponse{
		Success: false,
		Message: msg,
		Errors:  map[string]string{"request": msg},
	})
}

// deref mengembalikan isi *string, atau "" bila nil.
func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// nilIfEmpty mengembalikan nil untuk string kosong — dipakai mengisi kolom FK /
// unik yang nullable agar tersimpan NULL, bukan "".
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// categoryToResponse memetakan model Category ke DTO.
func categoryToResponse(c models.Category) structs.CategoryResponse {
	return structs.CategoryResponse{
		ID:        c.ID,
		ParentID:  deref(c.ParentID),
		Name:      c.Name,
		SortOrder: c.SortOrder,
		CreatedAt: c.CreatedAt.UTC().Format(timeLayout),
		UpdatedAt: c.UpdatedAt.UTC().Format(timeLayout),
	}
}

// unitToResponse memetakan model Unit ke DTO.
func unitToResponse(u models.Unit) structs.UnitResponse {
	return structs.UnitResponse{
		ID:           u.ID,
		Name:         u.Name,
		BaseUnitID:   deref(u.BaseUnitID),
		Conversion:   u.Conversion.String(),
		AllowDecimal: u.AllowDecimal,
		CreatedAt:    u.CreatedAt.UTC().Format(timeLayout),
		UpdatedAt:    u.UpdatedAt.UTC().Format(timeLayout),
	}
}

// supplierToResponse memetakan model Supplier ke DTO.
func supplierToResponse(s models.Supplier) structs.SupplierResponse {
	return structs.SupplierResponse{
		ID:        s.ID,
		Name:      s.Name,
		Phone:     s.Phone,
		Address:   s.Address,
		Note:      s.Note,
		CreatedAt: s.CreatedAt.UTC().Format(timeLayout),
		UpdatedAt: s.UpdatedAt.UTC().Format(timeLayout),
	}
}

// productToResponse memetakan model Product (dengan Unit/Category opsional) ke DTO.
func productToResponse(p models.Product) structs.ProductResponse {
	r := structs.ProductResponse{
		ID:         p.ID,
		Name:       p.Name,
		CategoryID: deref(p.CategoryID),
		UnitID:     p.UnitID,
		SKU:        deref(p.SKU),
		Barcode:    deref(p.Barcode),
		SellPrice:  p.SellPrice,
		CostPrice:  p.CostPrice,
		TrackStock: p.TrackStock,
		MinStock:   p.MinStock.String(),
		IsActive:   p.IsActive,
		ImageURL:   p.ImageURL,
		CreatedAt:  p.CreatedAt.UTC().Format(timeLayout),
		UpdatedAt:  p.UpdatedAt.UTC().Format(timeLayout),
	}
	if p.Unit != nil {
		r.UnitName = p.Unit.Name
	}
	if p.Category != nil {
		r.CategoryName = p.Category.Name
	}
	return r
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
		CreatedAt:    t.CreatedAt.UTC().Format(timeLayout),
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
		CreatedAt:         o.CreatedAt.UTC().Format(timeLayout),
		UpdatedAt:         o.UpdatedAt.UTC().Format(timeLayout),
	}
}
