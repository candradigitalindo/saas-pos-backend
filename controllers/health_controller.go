package controllers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"candra/backend-api/database"
	"candra/backend-api/helpers"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// healthPayload adalah bentuk data untuk endpoint kesehatan.
type healthPayload struct {
	Status   string `json:"status"`             // "ok" atau "degraded"
	Database string `json:"database,omitempty"` // "up" / "down" — hanya pada readiness
}

// Health adalah probe LIVENESS: apakah proses hidup dan bisa melayani HTTP.
// Selalu 200 selama server berjalan. TIDAK menyentuh database — dipakai load
// balancer / Kubernetes untuk memutuskan restart pod, bukan untuk lalu lintas.
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, structs.SuccessResponse[healthPayload]{
		Success: true,
		Message: "Service hidup",
		Data:    healthPayload{Status: "ok"},
	})
}

// Readiness adalah probe READINESS: apakah service siap menerima lalu lintas,
// termasuk database terjangkau. 200 bila siap, 503 bila tidak — load balancer
// berhenti mengirim request saat 503, tanpa me-restart pod.
func Readiness(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if err := database.Ping(ctx); err != nil {
		helpers.LoggerFromContext(c.Request.Context()).Warn("readiness gagal: database tidak menjawab", slog.Any("error", err))
		c.JSON(http.StatusServiceUnavailable, structs.ErrorResponse{
			Success: false,
			Message: "Service belum siap",
			Errors:  map[string]string{"database": "tidak terjangkau"},
		})
		return
	}

	c.JSON(http.StatusOK, structs.SuccessResponse[healthPayload]{
		Success: true,
		Message: "Service siap",
		Data:    healthPayload{Status: "ok", Database: "up"},
	})
}
