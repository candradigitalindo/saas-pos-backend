package controllers

import (
	"io"
	"net/http"

	"candra/backend-api/helpers"
	"candra/backend-api/repositories"
	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// Handler pipeline peristiwa kanal — Fase 11b (§5.10, blueprint F.6/F.8).

// maxWebhookBody membatasi ukuran payload webhook (payload pesanan kanal wajar
// di bawah 256 KiB; batas ini mencegah body raksasa membebani inbox).
const maxWebhookBody = 256 << 10

// IngestChannelWebhook menerima webhook kanal. TANPA AUTH — kanal tidak membawa
// token kita. Kanal ditautkan lewat (provider, merchant_ref). Payload disimpan
// MENTAH ke channel_events lalu balas 200 secepatnya; pemrosesan dilakukan
// pekerja terpisah (blueprint F.6 aturan 1 & 2).
//
//	POST /webhooks/channels/:provider        (merchant_ref: header X-Merchant-Ref atau query ?merchant_ref=)
func IngestChannelWebhook(c *gin.Context) {
	provider := c.Param("provider")
	merchantRef := c.GetHeader("X-Merchant-Ref")
	if merchantRef == "" {
		merchantRef = c.Query("merchant_ref")
	}

	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, maxWebhookBody))
	if err != nil {
		// Gagal baca body = kemungkinan koneksi putus; minta kanal mengulang.
		c.JSON(http.StatusServiceUnavailable, structs.ErrorResponse{
			Success: false, Message: "Gagal membaca payload",
			Errors: map[string]string{"body": "tidak terbaca"},
		})
		return
	}

	res, serr := services.IngestChannelEvent(c.Request.Context(), provider, merchantRef, raw)
	if serr != nil {
		// Kegagalan infrastruktur → 503 agar kanal mengirim ulang.
		helpers.LoggerFromContext(c.Request.Context()).Error("ingest webhook kanal gagal", "error", serr)
		c.JSON(http.StatusServiceUnavailable, structs.ErrorResponse{
			Success: false, Message: "Terjadi kesalahan, silakan kirim ulang",
			Errors: map[string]string{"server": "sementara"},
		})
		return
	}
	// Selalu 200 untuk hasil "bisnis" (diterima / duplikat / ditolak lunak) —
	// kanal tidak boleh menonaktifkan webhook karena non-2xx berulang.
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.WebhookIngestResult]{
		Success: true, Message: "Diterima", Data: res,
	})
}

// ListChannelEvents menampilkan inbox peristiwa sebuah kanal (untuk memantau
// antrean & antrean mati). Butuh channel.manage.
//
//	GET /channels/:id/events?status=pending|processing|done|failed|dead
func ListChannelEvents(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	rows, total, err := services.ListChannelEvents(c.Request.Context(), c.Param("id"), c.Query("status"), limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.ChannelEventResponse]]{
		Success: true, Message: "Inbox peristiwa kanal",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, rows),
	})
}

// ProcessChannelEventsNow memicu pekerja pemroses secara manual (selain cron
// cmd/process-channel-events). Butuh channel.manage.
//
//	POST /channel-events/process
func ProcessChannelEventsNow(c *gin.Context) {
	res, err := services.ProcessChannelEvents(c.Request.Context())
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.ChannelWorkerResult]{
		Success: true, Message: "Pemrosesan peristiwa kanal selesai", Data: res,
	})
}

// ListChannelSettlements menampilkan rekonsiliasi pencairan sebuah kanal.
// Butuh channel.settlement.view atau channel.manage.
//
//	GET /channels/:id/settlements
func ListChannelSettlements(c *gin.Context) {
	rows, err := services.ListChannelSettlements(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.ChannelSettlementResponse]{
		Success: true, Message: "Rekonsiliasi pencairan kanal", Data: rows,
	})
}

// RecomputeChannelSettlement menghitung ulang nilai kotor & potongan sebuah
// periode dari penjualan + channel_fees.
//
//	POST /channels/:id/settlements   {period_start, period_end, note?}
func RecomputeChannelSettlement(c *gin.Context) {
	var req structs.SettlementRecomputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.RecomputeSettlement(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		notFoundOr(c, err, repositories.ErrChannelNotFound, "Kanal tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.ChannelSettlementResponse]{
		Success: true, Message: "Settlement dihitung ulang", Data: res,
	})
}

// RecordChannelSettlementReceipt mencatat uang yang benar-benar masuk untuk
// sebuah periode; status jadi 'matched'/'mismatch'.
//
//	POST /channels/:id/settlements/receipt   {period_start, received_amount, received_at?, note?}
func RecordChannelSettlementReceipt(c *gin.Context) {
	var req structs.SettlementReceiptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.RecordSettlementReceipt(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		notFoundOr(c, err, repositories.ErrChannelSettlementNotFound, "Settlement periode itu belum ada — hitung dulu")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.ChannelSettlementResponse]{
		Success: true, Message: "Penerimaan pencairan dicatat", Data: res,
	})
}

// ListChannelStockSyncs menampilkan antrean sinkron stok sebuah kanal + umur
// keterlambatannya. Butuh channel.manage.
//
//	GET /channels/:id/stock-syncs?status=pending|sent|failed
func ListChannelStockSyncs(c *gin.Context) {
	rows, err := services.ListChannelStockSyncs(c.Request.Context(), c.Param("id"), c.Query("status"))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.ChannelStockSyncResponse]{
		Success: true, Message: "Antrean sinkron stok kanal", Data: rows,
	})
}
