package middlewares

import (
	"errors"
	"net/http"
	"strings"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// PlatformAuth menjaga panel internal (/api/v1/platform/*) — realm KETIGA,
// terpisah penuh dari user tenant dan akun mitra (blueprint G.5).
//
// Hanya token ber-realm "platform" yang diterima, dan token itu ditolak di
// seluruh rute tenant maupun portal mitra.
func PlatformAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		fields := strings.Fields(c.GetHeader(authorizationHeaderKey))
		if len(fields) < 2 || strings.ToLower(fields[0]) != authorizationTypeBearer {
			abortPlatform(c, http.StatusUnauthorized, "Format token otorisasi tidak valid")
			return
		}

		claims, err := helpers.ParseAccessToken(fields[1])
		if err != nil {
			msg := "Token tidak valid"
			if errors.Is(err, jwt.ErrTokenExpired) {
				msg = "Token sudah kedaluwarsa"
			}
			abortPlatform(c, http.StatusUnauthorized, msg)
			return
		}
		if claims.Rlm != helpers.RealmPlatform {
			abortPlatform(c, http.StatusUnauthorized, "Token bukan untuk panel internal")
			return
		}

		admin, err := repositories.FindPlatformAdminByID(c.Request.Context(), claims.Subject)
		if err != nil || !admin.IsActive {
			abortPlatform(c, http.StatusUnauthorized, "Akun admin tidak aktif atau tidak ditemukan")
			return
		}

		c.Request = c.Request.WithContext(
			reqctx.WithPlatformAdmin(c.Request.Context(), admin.ID, admin.Role),
		)
		c.Next()
	}
}

// RequirePlatform menolak permintaan bila peran admin tidak punya kemampuan
// yang diminta. Dipasang SETELAH PlatformAuth.
//
// Peran di sini tetap (superadmin/operator/finance/support), bukan dinamis
// seperti peran tenant: aturan siapa boleh mencairkan uang mitra tidak boleh
// bisa diubah lewat panel oleh salah satu penggunanya sendiri.
func RequirePlatform(capability string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !models.PlatformCan(reqctx.PlatformRole(c.Request.Context()), capability) {
			abortPlatform(c, http.StatusForbidden, "Peran Anda tidak berwenang melakukan tindakan ini")
			return
		}
		c.Next()
	}
}

func abortPlatform(c *gin.Context, status int, msg string) {
	field := "token"
	if status == http.StatusForbidden {
		field = "role"
	}
	c.AbortWithStatusJSON(status, structs.ErrorResponse{
		Success: false,
		Message: msg,
		Errors:  map[string]string{field: msg},
	})
}
