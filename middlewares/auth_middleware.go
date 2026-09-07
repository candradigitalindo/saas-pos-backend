package middlewares

import (
	"errors"
	"net/http"
	"strings"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	authorizationHeaderKey  = "Authorization"
	authorizationTypeBearer = "bearer"
)

// authorizationPayloadKey dan CurrentUserKey dideklarasikan di context.go
// (dipakai lintas middleware).

// Auth memverifikasi access token Bearer dan menaruh USER ID (subject token) ke
// gin.Context serta ke context.Context request. Ini satu-satunya yang dilakukan
// di sini: tenant_id dan permission dibaca dari database oleh middleware
// TenantScope setiap permintaan (§9), bukan dari isi token.
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader(authorizationHeaderKey)
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, structs.ErrorResponse{
				Success: false,
				Message: "Header otorisasi tidak ditemukan",
				Errors:  map[string]string{"authorization": "Header otorisasi 'Bearer [token]' dibutuhkan"},
			})
			return
		}

		fields := strings.Fields(authHeader)
		if len(fields) < 2 || strings.ToLower(fields[0]) != authorizationTypeBearer {
			c.AbortWithStatusJSON(http.StatusUnauthorized, structs.ErrorResponse{
				Success: false,
				Message: "Format token otorisasi tidak valid",
				Errors:  map[string]string{"authorization": "Format harus 'Bearer [token]'"},
			})
			return
		}

		claims, err := helpers.ParseAccessToken(fields[1])
		if err != nil {
			// Pesan generik ke klien; detail hanya untuk membedakan kedaluwarsa
			// (agar klien tahu harus me-refresh, bukan login ulang).
			errMsg := "Token tidak valid"
			if errors.Is(err, jwt.ErrTokenExpired) {
				errMsg = "Token sudah kedaluwarsa"
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, structs.ErrorResponse{
				Success: false,
				Message: errMsg,
				Errors:  map[string]string{"token": errMsg},
			})
			return
		}

		c.Set(authorizationPayloadKey, claims.Subject)
		c.Request = c.Request.WithContext(reqctx.WithUserID(c.Request.Context(), claims.Subject))
		c.Next()
	}
}
