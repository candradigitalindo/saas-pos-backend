package controllers

import (
	"io"
	"net/http"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/repositories"
	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// Handler kanal pesanan online — fondasi (Fase 11a, §5.10).

// ── Channel ───────────────────────────────────────────────────────────────

func ListChannels(c *gin.Context) {
	rows, err := services.ListChannels(c.Request.Context())
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.ChannelResponse]{
		Success: true, Message: "Daftar kanal", Data: rows,
	})
}

func GetChannel(c *gin.Context) {
	ch, err := repositories.FindChannel(c.Request.Context(), nil, c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrChannelNotFound, "Kanal tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.ChannelResponse]{
		Success: true, Message: "Detail kanal", Data: services.ChannelToResponse(ch),
	})
}

func CreateChannel(c *gin.Context) {
	var req structs.ChannelCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.CreateChannel(c.Request.Context(), req)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.ChannelResponse]{
		Success: true, Message: "Kanal dibuat", Data: res,
	})
}

func UpdateChannel(c *gin.Context) {
	var req structs.ChannelUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.UpdateChannel(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		notFoundOr(c, err, repositories.ErrChannelNotFound, "Kanal tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.ChannelResponse]{
		Success: true, Message: "Kanal diperbarui", Data: res,
	})
}

func DeleteChannel(c *gin.Context) {
	if err := services.DeactivateChannel(c.Request.Context(), c.Param("id")); err != nil {
		notFoundOr(c, err, repositories.ErrChannelNotFound, "Kanal tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[any]{Success: true, Message: "Kanal dinonaktifkan", Data: nil})
}

// ── Channel product (pemetaan SKU) ────────────────────────────────────────

func ListChannelProducts(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	rows, total, err := repositories.ListChannelProducts(c.Request.Context(), c.Param("id"), limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.ChannelProductResponse, len(rows))
	for i := range rows {
		items[i] = services.ChannelProductToResponse(rows[i])
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.ChannelProductResponse]]{
		Success: true, Message: "Pemetaan produk kanal",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

func UpsertChannelProduct(c *gin.Context) {
	var req structs.ChannelProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.UpsertChannelProduct(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		notFoundOr(c, err, repositories.ErrChannelNotFound, "Kanal tidak ditemukan")
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.ChannelProductResponse]{
		Success: true, Message: "Pemetaan produk kanal tersimpan", Data: res,
	})
}

func DeleteChannelProduct(c *gin.Context) {
	if err := services.DeleteChannelProduct(c.Request.Context(), c.Param("pid")); err != nil {
		notFoundOr(c, err, repositories.ErrChannelProductNotFound, "Pemetaan tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[any]{Success: true, Message: "Pemetaan dihapus", Data: nil})
}

// ── Channel order ─────────────────────────────────────────────────────────

func ListChannelOrders(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	f := repositories.ChannelOrderFilter{ChannelID: c.Query("channel_id"), ExternalStatus: c.Query("status")}
	items, total, err := services.ListChannelOrdersDetailed(c.Request.Context(), f, limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.ChannelOrderResponse]]{
		Success: true, Message: "Daftar pesanan kanal",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

func GetChannelOrder(c *gin.Context) {
	o, err := repositories.FindChannelOrder(c.Request.Context(), nil, c.Param("id"))
	if err != nil {
		notFoundOr(c, err, repositories.ErrChannelOrderNotFound, "Pesanan kanal tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.ChannelOrderResponse]{
		Success: true, Message: "Detail pesanan kanal", Data: services.ChannelOrderToResponse(o),
	})
}

// CreateChannelOrder mencatat satu pesanan kanal (entri manual).
func CreateChannelOrder(c *gin.Context) {
	var req structs.ChannelOrderCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	in := services.ChannelOrderInput{
		ChannelID: req.ChannelID, ExternalOrderID: req.ExternalOrderID,
		BuyerName: req.BuyerName, BuyerPhone: req.BuyerPhone,
		ShippingAddress: req.ShippingAddress, Courier: req.Courier,
		OrderDiscount: req.OrderDiscount, FeeAmount: req.FeeAmount,
	}
	if req.OccurredAt != "" {
		if t, e := time.Parse(time.RFC3339, req.OccurredAt); e == nil {
			in.OccurredAt = &t
		}
	}
	for _, it := range req.Items {
		qty, e := decimal.NewFromString(it.Qty)
		if e != nil {
			badRequest(c, "items.qty", "qty bukan angka yang valid")
			return
		}
		in.Items = append(in.Items, services.ChannelOrderItemInput{
			ProductID: it.ProductID, VariantID: it.VariantID, Qty: qty, UnitPrice: it.UnitPrice,
		})
	}

	res, created, err := services.RecordChannelOrder(c.Request.Context(), in)
	if err != nil {
		notFoundOr(c, err, repositories.ErrChannelNotFound, "Kanal tidak ditemukan")
		return
	}
	status := http.StatusCreated
	msg := "Pesanan kanal dicatat"
	if !created {
		status = http.StatusOK
		msg = "Pesanan kanal sudah tercatat sebelumnya"
	}
	c.JSON(status, structs.SuccessResponse[structs.ChannelOrderResponse]{Success: true, Message: msg, Data: res})
}

func UpdateChannelOrderStatus(c *gin.Context) {
	var req structs.ChannelOrderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.UpdateChannelOrderStatus(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		notFoundOr(c, err, repositories.ErrChannelOrderNotFound, "Pesanan kanal tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.ChannelOrderResponse]{
		Success: true, Message: "Status pesanan kanal diperbarui", Data: res,
	})
}

func CancelChannelOrder(c *gin.Context) {
	var req structs.ChannelOrderCancelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.CancelChannelOrder(c.Request.Context(), c.Param("id"), req.Reason)
	if err != nil {
		notFoundOr(c, err, repositories.ErrChannelOrderNotFound, "Pesanan kanal tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.ChannelOrderResponse]{
		Success: true, Message: "Pesanan kanal dibatalkan", Data: res,
	})
}

// ImportChannelOrders mengimpor laporan harian kanal dari CSV (text/csv atau
// multipart field `file`).
func ImportChannelOrders(c *gin.Context) {
	raw, err := readCSVBody(c)
	if err != nil {
		badRequest(c, "file", err.Error())
		return
	}
	res, serr := services.ImportChannelOrdersCSV(c.Request.Context(), c.Param("id"), raw)
	if serr != nil {
		respondServiceError(c, serr)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[services.ChannelOrderImportResult]{
		Success: true, Message: "Impor laporan kanal selesai", Data: res,
	})
}

// readCSVBody mengambil isi CSV dari body mentah atau multipart `file`.
func readCSVBody(c *gin.Context) ([]byte, error) {
	if fh, ferr := c.FormFile("file"); ferr == nil {
		f, oerr := fh.Open()
		if oerr != nil {
			return nil, oerr
		}
		defer f.Close()
		return io.ReadAll(io.LimitReader(f, 5<<20))
	}
	return io.ReadAll(io.LimitReader(c.Request.Body, 5<<20))
}
