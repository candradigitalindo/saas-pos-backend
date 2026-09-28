package controllers

import (
	"net/http"

	"candra/backend-api/helpers"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// ── Stok opname ────────────────────────────────────────────────────────────

func CreateOpname(c *gin.Context) {
	var req structs.OpnameCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	o, err := services.CreateOpname(c.Request.Context(), req.OutletID, req.Note)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.OpnameResponse]{
		Success: true, Message: "Opname dibuat", Data: services.OpnameToResponse(o),
	})
}

func SetOpnameItems(c *gin.Context) {
	var req structs.OpnameSetItemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	counts := make([]services.OpnameCountInput, 0, len(req.Items))
	for _, it := range req.Items {
		q, e := decimal.NewFromString(it.CountedQty)
		if e != nil {
			badRequest(c, "items.counted_qty", "counted_qty bukan angka yang valid")
			return
		}
		counts = append(counts, services.OpnameCountInput{ProductID: it.ProductID, VariantID: it.VariantID, CountedQty: q})
	}
	o, err := services.SetOpnameItems(c.Request.Context(), c.Param("id"), counts)
	if err != nil {
		notFoundOr(c, err, repositories.ErrOpnameNotFound, "Opname tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.OpnameResponse]{
		Success: true, Message: "Hitungan disimpan", Data: services.OpnameToResponse(o),
	})
}

func PostOpname(c *gin.Context) {
	o, err := services.PostOpname(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrOpnameNotFound, "Opname tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.OpnameResponse]{
		Success: true, Message: "Opname diposting; stok disesuaikan", Data: services.OpnameToResponse(o),
	})
}

func ListOpnames(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	rows, total, err := repositories.ListOpnames(c.Request.Context(), c.Query("outlet_id"), limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	ids := make([]string, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	extras, err := repositories.OpnameExtras(c.Request.Context(), ids)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.OpnameResponse, len(rows))
	for i := range rows {
		items[i] = services.OpnameToResponse(&rows[i])
		isiOpnameExtra(&items[i], extras[rows[i].ID])
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.OpnameResponse]]{
		Success: true, Message: "Daftar opname",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

func GetOpname(c *gin.Context) {
	var o models.StockOpname
	if err := repositories.FindOpnameInTenant(c.Request.Context(), nil, c.Param("id"), &o); err != nil {
		notFoundOr(c, err, repositories.ErrOpnameNotFound, "Opname tidak ditemukan")
		return
	}
	if !repositories.OutletVisible(c.Request.Context(), o.OutletID) {
		notFound(c, "Opname tidak ditemukan")
		return
	}
	ctx := c.Request.Context()
	extras, err := repositories.OpnameExtras(ctx, []string{o.ID})
	if err != nil {
		respondServiceError(c, err)
		return
	}
	info, err := repositories.OpnameItemInfos(ctx, o.ID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	r := services.OpnameToResponse(&o)
	isiOpnameExtra(&r, extras[o.ID])
	for i := range r.Items {
		n := info[r.Items[i].ID]
		r.Items[i].ProductName, r.Items[i].UnitName, r.Items[i].CostPrice = n.ProductName, n.UnitName, n.CostPrice
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.OpnameResponse]{
		Success: true, Message: "Detail opname", Data: r,
	})
}

// isiOpnameExtra menempelkan ringkasan riwayat ke satu sesi hitung fisik.
func isiOpnameExtra(r *structs.OpnameResponse, e repositories.OpnameExtra) {
	r.CreatedByName = e.CreatedByName
	r.ItemCount, r.ChangedCount, r.ValueDiff = e.ItemCount, e.ChangedCount, e.ValueDiff
}

// ── Transfer stok ─────────────────────────────────────────────────────────

func CreateTransfer(c *gin.Context) {
	var req structs.TransferCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	items := make([]services.TransferItemInput, 0, len(req.Items))
	for _, it := range req.Items {
		q, e := decimal.NewFromString(it.Qty)
		if e != nil {
			badRequest(c, "items.qty", "qty bukan angka yang valid")
			return
		}
		items = append(items, services.TransferItemInput{ProductID: it.ProductID, VariantID: it.VariantID, Qty: q})
	}
	tr, err := services.CreateTransfer(c.Request.Context(), req.FromOutletID, req.ToOutletID, req.Note, items)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.TransferResponse]{
		Success: true, Message: "Transfer dibuat", Data: services.TransferToResponse(tr),
	})
}

func SendTransfer(c *gin.Context) {
	tr, err := services.SendTransfer(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrTransferNotFound, "Transfer tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.TransferResponse]{
		Success: true, Message: "Transfer dikirim; stok outlet asal berkurang", Data: services.TransferToResponse(tr),
	})
}

func ReceiveTransfer(c *gin.Context) {
	tr, err := services.ReceiveTransfer(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrTransferNotFound, "Transfer tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.TransferResponse]{
		Success: true, Message: "Transfer diterima; stok outlet tujuan bertambah", Data: services.TransferToResponse(tr),
	})
}

func ListTransfers(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	rows, total, err := repositories.ListTransfers(c.Request.Context(), c.Query("outlet_id"), limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.TransferResponse, len(rows))
	for i := range rows {
		items[i] = services.TransferToResponse(&rows[i])
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.TransferResponse]]{
		Success: true, Message: "Daftar transfer",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

func GetTransfer(c *gin.Context) {
	var tr models.StockTransfer
	if err := repositories.FindTransferInTenant(c.Request.Context(), nil, c.Param("id"), &tr); err != nil {
		notFoundOr(c, err, repositories.ErrTransferNotFound, "Transfer tidak ditemukan")
		return
	}
	// Transfer terlihat oleh cabang asal MAUPUN tujuan.
	if !repositories.OutletVisible(c.Request.Context(), tr.FromOutletID, tr.ToOutletID) {
		notFound(c, "Transfer tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.TransferResponse]{
		Success: true, Message: "Detail transfer", Data: services.TransferToResponse(&tr),
	})
}

// ── Rekonsiliasi ──────────────────────────────────────────────────────────

// ReconcileStocks membangun ulang cache stok satu outlet dari buku besar (§5.6).
func ReconcileStocks(c *gin.Context) {
	outletID := c.Query("outlet_id")
	if outletID == "" {
		badRequest(c, "outlet_id", "Parameter outlet_id wajib")
		return
	}
	n, err := repositories.ReconcileStocks(c.Request.Context(), outletID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[gin.H]{
		Success: true, Message: "Rekonsiliasi selesai", Data: gin.H{"rows_reconciled": n, "outlet_id": outletID},
	})
}
