package controllers

import (
	"io"
	"net/http"
	"strings"

	"candra/backend-api/helpers"
	"candra/backend-api/repositories"
	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// StockAdjustRequest: isi new_qty (set absolut) ATAU delta (geser), tidak
// keduanya. Nilai desimal string.
type stockAdjustRequest struct {
	OutletID  string `json:"outlet_id" binding:"required,ulid"`
	ProductID string `json:"product_id" binding:"required,ulid"`
	VariantID string `json:"variant_id" binding:"omitempty,ulid"`
	NewQty    string `json:"new_qty" binding:"omitempty"`
	Delta     string `json:"delta" binding:"omitempty"`
	Reason    string `json:"reason" binding:"required,min=1,max=200"`
}

// AdjustStock menyetel/menggeser saldo stok dan mencatat gerakannya. Wajib
// header Idempotency-Key (§8).
func AdjustStock(c *gin.Context) {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, checkoutMaxBodyBytes))
	if err != nil {
		badRequest(c, "body", "Gagal membaca body")
		return
	}
	var req stockAdjustRequest
	if err := bindJSONBytes(raw, &req); err != nil {
		validationFailed(c, err)
		return
	}
	in := services.StockAdjustInput{
		OutletID: req.OutletID, ProductID: req.ProductID, VariantID: req.VariantID, Reason: req.Reason,
		IdempotencyKey: c.GetHeader("Idempotency-Key"), RequestHash: helpers.SHA256Hex(raw),
	}
	if req.NewQty != "" {
		d, err := decimal.NewFromString(req.NewQty)
		if err != nil {
			badRequest(c, "new_qty", "new_qty bukan angka yang valid")
			return
		}
		in.NewQty = &d
	}
	if req.Delta != "" {
		d, err := decimal.NewFromString(req.Delta)
		if err != nil {
			badRequest(c, "delta", "delta bukan angka yang valid")
			return
		}
		in.Delta = &d
	}

	status, body, err := services.AdjustStock(c.Request.Context(), in)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.Data(status, "application/json; charset=utf-8", body)
}

// ListStocks mengembalikan saldo stok (opsional per outlet, opsional hanya yang
// di bawah min_stock via ?low=true, atau satu keadaan via ?status=), berurut
// ?sort= (bawaan nama), lengkap dengan nilai stok & laku 30 hari per baris.
func ListStocks(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	f := repositories.StockFilter{
		OutletID: c.Query("outlet_id"),
		LowOnly:  c.Query("low") == "true" || c.Query("low") == "1",
		Search:   strings.TrimSpace(c.Query("q")),
		Status:   c.Query("status"),
		Sort:     c.Query("sort"),
	}
	if f.Status != "" && !repositories.StockStatuses[f.Status] {
		badRequest(c, "status", "status harus salah satu: safe, low, out, negative, idle, restock")
		return
	}
	if f.Sort != "" && !repositories.StockSorts[f.Sort] {
		badRequest(c, "sort", "sort harus salah satu: name, urgent, value, sold")
		return
	}
	for _, id := range strings.Split(c.Query("product_ids"), ",") {
		if id = strings.TrimSpace(id); id != "" {
			f.ProductIDs = append(f.ProductIDs, id)
		}
	}

	rows, total, err := repositories.ListStocks(c.Request.Context(), f, limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.StockResponse, len(rows))
	for i, r := range rows {
		items[i] = stockRowToResponse(r)
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.StockResponse]]{
		Success: true, Message: "Berhasil mengambil saldo stok",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

// StockSummary merangkum saldo stok seluruh barang satu outlet (atau semua
// outlet yang boleh dilihat): aman / hampir habis / habis / minus + nilai stok.
func StockSummary(c *gin.Context) {
	r, err := repositories.StockSummary(c.Request.Context(), c.Query("outlet_id"))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.StockSummaryResponse]{
		Success: true, Message: "Berhasil mengambil ringkasan stok",
		Data: structs.StockSummaryResponse{
			Total: r.Total, Safe: r.Safe, Low: r.Low, Out: r.Out,
			Negative: r.Negative, StockValue: r.StockValue,
			Idle: r.Idle, IdleValue: r.IdleValue,
		},
	})
}

// ListStockMovements mengembalikan kartu stok satu produk (wajib product_id).
func ListStockMovements(c *gin.Context) {
	productID := c.Query("product_id")
	if productID == "" {
		badRequest(c, "product_id", "Parameter product_id wajib")
		return
	}
	page, limit, offset := helpers.ParsePaginationParams(c)
	rows, total, err := repositories.ListStockMovements(c.Request.Context(), productID, c.Query("outlet_id"), limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.StockMovementResponse, len(rows))
	for i, r := range rows {
		items[i] = services.StockMovementToResponse(r)
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.StockMovementResponse]]{
		Success: true, Message: "Kartu stok",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}
