package controllers

import (
	"errors"
	"io"
	"net/http"

	"candra/backend-api/helpers"
	"candra/backend-api/repositories"
	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// Sambungan API kanal milik tenant — lihat services/channel_provider.go.

// ListChannelProviders: GET /api/v1/channel-providers.
func ListChannelProviders(c *gin.Context) {
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.ChannelProviderInfo]{
		Success: true, Message: "Penyedia kanal", Data: services.ListChannelProviders(),
	})
}

func balasSambungan(c *gin.Context, pesan string, res structs.ChannelConnectionResponse, err error) {
	if err != nil {
		notFoundOr(c, err, repositories.ErrChannelNotFound, "Kanal tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.ChannelConnectionResponse]{
		Success: true, Message: pesan, Data: res,
	})
}

// GetChannelConnection: GET /api/v1/channels/:id/connection.
func GetChannelConnection(c *gin.Context) {
	res, err := services.GetChannelConnection(c.Request.Context(), c.Param("id"))
	balasSambungan(c, "Sambungan kanal", res, err)
}

// SaveChannelConnection: PUT /api/v1/channels/:id/connection.
func SaveChannelConnection(c *gin.Context) {
	var req structs.ChannelConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.SaveChannelConnection(c.Request.Context(), c.Param("id"), req)
	balasSambungan(c, "Kredensial disimpan", res, err)
}

// TestChannelConnection: POST /api/v1/channels/:id/connection/test.
func TestChannelConnection(c *gin.Context) {
	res, err := services.TestChannelConnection(c.Request.Context(), c.Param("id"))
	balasSambungan(c, "Tes koneksi selesai", res, err)
}

// DeleteChannelConnection: DELETE /api/v1/channels/:id/connection.
func DeleteChannelConnection(c *gin.Context) {
	if err := services.DisconnectChannel(c.Request.Context(), c.Param("id")); err != nil {
		notFoundOr(c, err, repositories.ErrChannelNotFound, "Kanal tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[any]{Success: true, Message: "Sambungan diputus", Data: nil})
}

// ProviderWebhookChallenge: GET /webhooks/channels/:provider/:token — penyedia
// (mis. Meta) memverifikasi alamat sebelum mau mengirim. Balasannya teks polos.
func ProviderWebhookChallenge(c *gin.Context) {
	body, ok := services.ProviderWebhookChallenge(c.Request.Context(), c.Param("provider"), c.Param("token"), c.Request.URL.Query())
	if !ok {
		c.String(http.StatusForbidden, "verifikasi ditolak")
		return
	}
	c.String(http.StatusOK, body)
}

// ProviderWebhook: POST /webhooks/channels/:provider/:token — TANPA sesi,
// diamankan tanda tangan milik kanal. Tanda tangan salah / alamat tak dikenal
// → 401 (bukan dari penyedia). Hasil "bisnis" (bukan pesanan, duplikat) → 200
// supaya penyedia tidak mematikan webhook-nya.
func ProviderWebhook(c *gin.Context) {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, maxWebhookBody))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, structs.ErrorResponse{
			Success: false, Message: "Gagal membaca payload", Errors: map[string]string{"body": "tidak terbaca"},
		})
		return
	}
	res, balasan, err := services.IngestProviderWebhook(c.Request.Context(), c.Param("provider"), c.Param("token"),
		c.Param("aksi"), c.Request.Header, raw)
	if balasan != nil {
		c.JSON(balasan.Status, balasan.Body)
		return
	}
	if errors.Is(err, services.ErrWebhookUnauthorized) {
		c.JSON(http.StatusUnauthorized, structs.ErrorResponse{
			Success: false, Message: "Webhook tidak sah", Errors: map[string]string{"signature": "tidak cocok"},
		})
		return
	}
	if err != nil {
		helpers.LoggerFromContext(c.Request.Context()).Error("webhook penyedia gagal", "error", err)
		c.JSON(http.StatusServiceUnavailable, structs.ErrorResponse{
			Success: false, Message: "Terjadi kesalahan, silakan kirim ulang", Errors: map[string]string{"server": "sementara"},
		})
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.ProviderWebhookResult]{
		Success: true, Message: "Diterima", Data: res,
	})
}
