package controllers

import (
	"errors"
	"io"
	"net/http"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// ListReceivables mengembalikan piutang tenant (opsional per pelanggan /
// status; status=unpaid = open + partial, terlama dulu), lengkap dengan nomor
// nota asalnya.
func ListReceivables(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	ctx := c.Request.Context()
	rows, total, err := repositories.ListReceivables(ctx, c.Query("customer_id"), c.Query("status"), limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	saleIDs := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.SourceTable == "sales" {
			saleIDs = append(saleIDs, r.SourceID)
		}
	}
	nota, err := repositories.ReceivableSaleRefs(ctx, saleIDs)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.ReceivableResponse, len(rows))
	for i, r := range rows {
		items[i] = services.ReceivableToResponse(r)
		if n, ok := nota[r.SourceID]; ok && r.SourceTable == "sales" {
			items[i].ReceiptNo, items[i].OutletName = n.ReceiptNo, n.OutletName
			items[i].BusinessDate = n.BusinessDate.Format("2006-01-02")
		}
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
		Success: true, Message: "Berhasil mengambil data piutang", Data: services.ReceivableToResponse(row),
	})
}

// AddReceivablePayment mencatat pembayaran cicilan/pelunasan piutang.
//
// Wajib header Idempotency-Key — setoran adalah uang masuk (§8).
func AddReceivablePayment(c *gin.Context) {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, checkoutMaxBodyBytes))
	if err != nil {
		badRequest(c, "body", "Gagal membaca body")
		return
	}
	var req structs.ReceivablePaymentRequest
	if err := bindJSONBytes(raw, &req); err != nil {
		validationFailed(c, err)
		return
	}
	ctx := c.Request.Context()
	if req.Method == "qris" {
		if err := services.RequireFeature(ctx, services.FeatureQRIS); err != nil {
			respondServiceError(c, err)
			return
		}
	}
	collectedBy := reqctx.UserID(ctx)
	// PaidAt dan BusinessDate sengaja TIDAK diisi di sini: business_date harus
	// dihitung dari zona waktu outlet, dan itu urusan service.
	pay := &models.ReceivablePayment{
		ReceivableID: req.ReceivableID,
		Amount:       req.Amount,
		Method:       req.Method,
		CollectedBy:  &collectedBy,
		ProofURL:     req.ProofURL,
	}
	status, body, err := services.AddReceivablePayment(ctx, pay,
		c.GetHeader("Idempotency-Key"), helpers.SHA256Hex(raw))
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
		c.Data(status, "application/json; charset=utf-8", body)
	}
}

// ReceivablesSummary merangkum kasbon belum lunas per pelanggan — yang perlu
// ditagih lebih dulu di atas. outlet_id hanya menentukan "hari ini" (tanggal
// usaha toko) untuk jatuh tempo; kasbon tidak berlapis toko.
func ReceivablesSummary(c *gin.Context) {
	hariIni := hariUsahaOutlet(c, c.Query("outlet_id"))
	segera := services.ReceivableDueSoonDays()
	rows, err := repositories.ReceivablesSummary(c.Request.Context(), hariIni, segera)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	out := structs.ReceivablesSummaryResponse{
		Today: hariIni.Format("2006-01-02"), DueSoonDays: segera, Customers: []structs.ReceivableCustomerResponse{},
	}
	for _, r := range rows {
		k := structs.ReceivableCustomerResponse{
			CustomerID: r.CustomerID, CustomerName: r.CustomerName, Phone: r.Phone, CreditLimit: r.CreditLimit,
			Outstanding: r.Outstanding, Count: r.Count, OverdueCount: r.OverdueCount, OverdueAmount: r.OverdueAmount,
			DueSoonCount: r.DueSoonCount, DueSoonAmount: r.DueSoonAmount, OldestAt: r.OldestAt.UTC().Format(timeLayout),
		}
		if r.NearestDue != nil {
			k.NearestDue = r.NearestDue.Format("2006-01-02")
		}
		if r.LastPaidAt != nil {
			k.LastPaidAt = r.LastPaidAt.UTC().Format(timeLayout)
		}
		out.Outstanding += r.Outstanding
		out.Count += r.Count
		if r.OverdueCount > 0 {
			out.OverdueCount++
			out.OverdueAmount += r.OverdueAmount
		}
		if r.DueSoonCount > 0 {
			out.DueSoonCount++
			out.DueSoonAmount += r.DueSoonAmount
		}
		out.Customers = append(out.Customers, k)
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.ReceivablesSummaryResponse]{
		Success: true, Message: "Ringkasan kasbon", Data: out,
	})
}

// SetReceivableDueDate mengatur / menghapus jatuh tempo satu kasbon.
func SetReceivableDueDate(c *gin.Context) {
	var req structs.ReceivableDueDateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	rec, err := services.SetReceivableDueDate(c.Request.Context(), c.Param("id"), *req.DueDate)
	switch {
	case errors.Is(err, repositories.ErrReceivableNotFound):
		notFound(c, "Kasbon tidak ditemukan")
	case errors.Is(err, repositories.ErrReceivableSettled):
		conflict(c, "receivable", "Kasbon ini sudah lunas")
	case err != nil:
		respondServiceError(c, err)
	default:
		c.JSON(http.StatusOK, structs.SuccessResponse[structs.ReceivableResponse]{
			Success: true, Message: "Jatuh tempo diperbarui", Data: services.ReceivableToResponse(rec),
		})
	}
}

// PayCustomerReceivables mencatat setoran pelanggan atas kasbonnya (terlama
// dulu). Wajib header Idempotency-Key — setoran adalah uang masuk.
func PayCustomerReceivables(c *gin.Context) {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, checkoutMaxBodyBytes))
	if err != nil {
		badRequest(c, "body", "Gagal membaca body")
		return
	}
	var req structs.CustomerPaymentRequest
	if err := bindJSONBytes(raw, &req); err != nil {
		validationFailed(c, err)
		return
	}
	ctx := c.Request.Context()
	if req.Method == "qris" {
		if err := services.RequireFeature(ctx, services.FeatureQRIS); err != nil {
			respondServiceError(c, err)
			return
		}
	}
	status, body, err := services.PayCustomerReceivables(ctx, services.CustomerPaymentInput{
		CustomerID: c.Param("id"), Amount: req.Amount, Method: req.Method, Source: req.Source,
		OutletID: req.OutletID, Note: req.Note,
		IdempotencyKey: c.GetHeader("Idempotency-Key"), RequestHash: helpers.SHA256Hex(raw),
	})
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.Data(status, "application/json; charset=utf-8", body)
}

// ListCustomerReceivablePayments: 50 setoran kasbon terakhir seorang pelanggan.
func ListCustomerReceivablePayments(c *gin.Context) {
	rows, err := repositories.ListCustomerReceivablePayments(c.Request.Context(), c.Param("id"), 50)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	out := make([]structs.ReceivablePaymentRow, len(rows))
	for i, r := range rows {
		out[i] = structs.ReceivablePaymentRow{
			ID: r.ID, ReceivableID: r.ReceivableID, ReceiptNo: r.ReceiptNo, Amount: r.Amount, Method: r.Method,
			ToDrawer: r.CashMovementID != nil, PaidAt: r.PaidAt.UTC().Format(timeLayout),
			CollectedName: r.CollectedName, Note: r.Note,
		}
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.ReceivablePaymentRow]{
		Success: true, Message: "Riwayat setoran kasbon", Data: out,
	})
}
