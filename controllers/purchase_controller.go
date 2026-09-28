package controllers

import (
	"io"
	"net/http"
	"time"

	"candra/backend-api/helpers"
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
		IdempotencyKey: c.GetHeader("Idempotency-Key"),
		RequestHash:    helpers.SHA256Hex(raw),
	}
	if req.DueDate != "" {
		if d, e := time.Parse("2006-01-02", req.DueDate); e == nil {
			in.DueDate = &d
		}
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
	rows, total, err := repositories.ListPurchases(c.Request.Context(), c.Query("outlet_id"), limit, offset)
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
	r := services.PurchaseToResponse(&p)
	isiExtra(&r, extras[p.ID])
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
