package middlewares

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/ulid"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

const (
	// RequestIDKey adalah kunci gin.Context tempat request ID disimpan.
	RequestIDKey = "requestID"
	// RequestIDHeader adalah nama header response yang membawa request ID balik
	// ke klien, agar bisa dicantumkan saat melapor bug.
	RequestIDHeader = "X-Request-Id"
)

// RequestID mengambil request ID dari gin.Context. Mengembalikan "" bila
// RequestObservability belum dipasang (mis. di test).
func RequestID(c *gin.Context) string {
	return c.GetString(RequestIDKey)
}

// RequestObservability memasang tiga hal sekaligus untuk setiap permintaan:
//
//  1. Request ID (ULID) baru — SELALU dibuat server, tidak pernah diambil dari
//     header klien (§4 "jangan percaya header dari klien"). Ditaruh di
//     gin.Context, di context.Context request, dan di header response.
//  2. Logger request-scoped yang sudah membawa request_id — dilekatkan ke
//     context.Context sehingga repository/service di bawah bisa memakai
//     helpers.LoggerFromContext tanpa parameter tambahan.
//  3. Satu baris log terstruktur per permintaan setelah selesai, berisi
//     request_id, method, route, status, latency_ms, client_ip, dan — bila ada —
//     user_id & tenant_id (§15).
func RequestObservability() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		requestID := ulid.New()
		c.Set(RequestIDKey, requestID)
		c.Header(RequestIDHeader, requestID)

		reqLogger := slog.Default().With(slog.String("request_id", requestID))
		ctx := helpers.ContextWithLogger(c.Request.Context(), reqLogger)
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		// route memakai pola terdaftar ("/api/v1/users/:id"), bukan path mentah,
		// supaya metrik tidak pecah per-ID. Kosong berarti 404 tak terdaftar.
		route := c.FullPath()
		if route == "" {
			route = "(unmatched)"
		}

		attrs := []any{
			slog.String("method", c.Request.Method),
			slog.String("route", route),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", c.Writer.Status()),
			slog.Int64("latency_ms", time.Since(start).Milliseconds()),
			slog.String("client_ip", c.ClientIP()),
			slog.Int("bytes", c.Writer.Size()),
		}
		if uid := c.GetString(authorizationPayloadKey); uid != "" {
			attrs = append(attrs, slog.String("user_id", uid))
		}
		if tid := c.GetString(tenantIDKey); tid != "" {
			attrs = append(attrs, slog.String("tenant_id", tid))
		}

		msg := "request selesai"
		switch {
		case c.Writer.Status() >= 500:
			reqLogger.Error(msg, attrs...)
		case c.Writer.Status() >= 400:
			reqLogger.Warn(msg, attrs...)
		default:
			reqLogger.Info(msg, attrs...)
		}
	}
}

// Recovery menangkap panic di handler mana pun, mencatatnya lengkap dengan
// stack trace dan request_id ke log, lalu membalas 500 dengan pesan generik —
// detail internal tidak pernah bocor ke klien (§4).
//
// Menggantikan gin.Recovery() bawaan supaya keluarannya ikut format terstruktur
// dan memakai bentuk response baku aplikasi.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				helpers.LoggerFromContext(c.Request.Context()).Error(
					"panic dipulihkan",
					slog.Any("error", rec),
					slog.String("route", c.FullPath()),
					slog.String("stack", string(debug.Stack())),
				)
				if !c.Writer.Written() {
					c.AbortWithStatusJSON(http.StatusInternalServerError, structs.ErrorResponse{
						Success: false,
						Message: "Terjadi kesalahan internal",
						Errors:  map[string]string{"server": "Terjadi kesalahan internal"},
					})
				} else {
					c.Abort()
				}
			}
		}()
		c.Next()
	}
}
