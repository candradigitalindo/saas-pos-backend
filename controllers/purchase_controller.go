package controllers

import (
	"io"
	"net/http"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/timez"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// ReceivePurchase mencatat penerimaan barang. Wajib header Idempotency-Key.
func ReceivePurchase(c *gin.Context) {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, checkoutMaxBodyBytes))
	if err != nil {
		badRequest(c, "body", "Gagal membaca body")
		return
	}
	var req structs.PurchaseRequest
	if err := bindJSONBytes(raw, &req); err != nil {
		validationFailed(c, err)
		return
	}

	in := services.PurchaseInput{
		OutletID:       req.OutletID,
		SupplierID:     req.SupplierID,
		InvoiceNo:      req.InvoiceNo,
		DiscountAmount: req.DiscountAmount,
		TaxAmount:      req.TaxAmount,
		PaidAmount:     req.PaidAmount,
		PaymentSource:  req.PaymentSource,
		IdempotencyKey: c.GetHeader("Idempotency-Key"),
		RequestHash:    helpers.SHA256Hex(raw),
	}
	if req.DueDate != "" {
		d, e := time.Parse("2006-01-02", req.DueDate)
		if e != nil {
			badRequest(c, "due_date", "Tanggal jatuh tempo harus YYYY-MM-DD")
			return
		}
		in.DueDate = &d
	}
	for _, it := range req.Items {
		q, e := decimal.NewFromString(it.Qty)
		if e != nil {
			badRequest(c, "items.qty", "qty bukan angka yang valid")
			return
		}
		in.Items = append(in.Items, services.PurchaseItemInput{
			ProductID: it.ProductID, VariantID: it.VariantID, ProductUnitID: it.ProductUnitID, Qty: q, UnitCost: it.UnitCost,
		})
	}

	status, body, err := services.ReceivePurchase(c.Request.Context(), in)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.Data(status, "application/json; charset=utf-8", body)
}

// ListPurchases mengembalikan daftar pembelian.
func ListPurchases(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	rows, total, err := repositories.ListPurchases(c.Request.Context(), repositories.PurchaseFilter{
		OutletID:   c.Query("outlet_id"),
		Unpaid:     c.Query("unpaid") == "true" || c.Query("unpaid") == "1",
		SupplierID: c.Query("supplier_id"),
	}, limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	ids := make([]string, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	extras, err := repositories.PurchaseExtras(c.Request.Context(), ids)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.PurchaseResponse, len(rows))
	for i := range rows {
		items[i] = services.PurchaseToResponse(&rows[i])
		isiExtra(&items[i], extras[rows[i].ID])
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.PurchaseResponse]]{
		Success: true, Message: "Berhasil mengambil data pembelian",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

// GetPurchase mengembalikan satu pembelian lengkap.
func GetPurchase(c *gin.Context) {
	var p models.Purchase
	if err := repositories.FindPurchaseInTenant(c.Request.Context(), nil, c.Param("id"), &p); err != nil {
		notFoundOr(c, err, repositories.ErrPurchaseNotFound, "Pembelian tidak ditemukan")
		return
	}
	if !repositories.OutletVisible(c.Request.Context(), p.OutletID) {
		notFound(c, "Pembelian tidak ditemukan")
		return
	}
	ctx := c.Request.Context()
	extras, err := repositories.PurchaseExtras(ctx, []string{p.ID})
	if err != nil {
		respondServiceError(c, err)
		return
	}
	nama, err := repositories.PurchaseItemNames(ctx, p.ID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	bayar, err := repositories.ListPurchasePayments(ctx, p.ID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	r := services.PurchaseToResponse(&p)
	isiExtra(&r, extras[p.ID])
	for _, b := range bayar {
		r.Payments = append(r.Payments, structs.PurchasePaymentResponse{
			ID: b.ID, Amount: b.Amount, Source: b.Source, Note: b.Note,
			PaidAt: b.PaidAt.UTC().Format(timeLayout), CreatedByName: b.CreatedByName,
		})
	}
	for i := range r.Items {
		n := nama[r.Items[i].ID]
		r.Items[i].ProductName, r.Items[i].VariantName, r.Items[i].BaseUnitName = n.ProductName, n.VariantName, n.BaseUnitName
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PurchaseResponse]{
		Success: true, Message: "Berhasil mengambil data pembelian", Data: r,
	})
}

// isiExtra menempelkan nama pemasok, pencatat, dan ringkasan barang.
func isiExtra(r *structs.PurchaseResponse, e repositories.PurchaseExtra) {
	r.SupplierName, r.CreatedByName = e.SupplierName, e.CreatedByName
	r.ItemCount, r.ItemNames = e.ItemCount, e.ItemNames
}

// PayPurchase mencatat pelunasan (sebagian/penuh) utang satu pembelian. Wajib
// header Idempotency-Key: ini memindahkan uang.
func PayPurchase(c *gin.Context) {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, checkoutMaxBodyBytes))
	if err != nil {
		badRequest(c, "body", "Gagal membaca body")
		return
	}
	var req structs.PurchasePayRequest
	if err := bindJSONBytes(raw, &req); err != nil {
		validationFailed(c, err)
		return
	}
	status, body, err := services.PayPurchase(c.Request.Context(), services.PayPurchaseInput{
		PurchaseID: c.Param("id"), Amount: req.Amount, Source: req.Source, Note: req.Note,
		IdempotencyKey: c.GetHeader("Idempotency-Key"), RequestHash: helpers.SHA256Hex(raw),
	})
	if err != nil {
		notFoundOr(c, err, repositories.ErrPurchaseNotFound, "Pembelian tidak ditemukan")
		return
	}
	c.Data(status, "application/json; charset=utf-8", body)
}

// PayablesSummary merangkum utang pemasok satu outlet (atau semua yang
// boleh): total, jumlah nota, yang lewat jatuh tempo, dan per pemasok.
func PayablesSummary(c *gin.Context) {
	ctx := c.Request.Context()
	outletID := c.Query("outlet_id")
	hariIni := time.Now().UTC()
	if outletID != "" {
		var o models.Outlet
		if err := repositories.FindOutletByID(ctx, nil, outletID, &o); err == nil {
			if d, e := timez.BusinessDate(hariIni, o.Timezone, o.DayStartOffset()); e == nil {
				hariIni = d
			}
		}
	}
	hariIni = time.Date(hariIni.Year(), hariIni.Month(), hariIni.Day(), 0, 0, 0, 0, time.UTC)
	rows, err := repositories.PayablesSummary(ctx, outletID, hariIni)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	out := structs.PayablesSummaryResponse{Today: hariIni.Format("2006-01-02"), Suppliers: []structs.PayablesSupplierResponse{}}
	for _, r := range rows {
		s := structs.PayablesSupplierResponse{
			SupplierID: r.SupplierID, SupplierName: r.SupplierName, Outstanding: r.Outstanding,
			Count: r.Count, OverdueCount: r.OverdueCount, OldestAt: r.OldestAt.UTC().Format(timeLayout),
		}
		if r.NearestDue != nil {
			s.NearestDue = r.NearestDue.Format("2006-01-02")
		}
		out.Outstanding += r.Outstanding
		out.Count += r.Count
		out.OverdueCount += r.OverdueCount
		out.Suppliers = append(out.Suppliers, s)
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PayablesSummaryResponse]{
		Success: true, Message: "Ringkasan utang pemasok", Data: out,
	})
}
