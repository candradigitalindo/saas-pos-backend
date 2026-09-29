package middlewares

import (
	"errors"
	"net/http"
	"strings"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// PartnerAuth menjaga rute portal mitra (/api/v1/partner/*). Ini jalur
// autentikasi TERPISAH dari user tenant (blueprint G.8): hanya token ber-realm
// "partner" yang diterima, dan token tersebut ditolak di semua rute tenant.
//
// Setelah lolos: id mitra + id akun-login mitra ditaruh di context.Context.
// Mitra yang mitranya tidak berstatus 'active', atau akunnya nonaktif, ditolak.
func PartnerAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		fields := strings.Fields(c.GetHeader(authorizationHeaderKey))
		if len(fields) < 2 || strings.ToLower(fields[0]) != authorizationTypeBearer {
			abortPartner(c, "Format token otorisasi tidak valid")
			return
		}

		claims, err := helpers.ParseAccessToken(fields[1])
		if err != nil {
			msg := "Token tidak valid"
			if errors.Is(err, jwt.ErrTokenExpired) {
				msg = "Token sudah kedaluwarsa"
			}
			abortPartner(c, msg)
			return
		}
		if claims.Rlm != helpers.RealmPartner {
			abortPartner(c, "Token bukan untuk portal mitra")
			return
		}

		pu, err := repositories.FindPartnerUserByID(c.Request.Context(), claims.Subject)
		if err != nil || !pu.IsActive {
			abortPartner(c, "Akun mitra tidak aktif atau tidak ditemukan")
			return
		}
		partner, err := repositories.FindPartnerByID(c.Request.Context(), pu.PartnerID)
		if err != nil || partner.Status != "active" {
			abortPartner(c, "Mitra belum aktif")
			return
		}

		ctx := reqctx.WithPartnerUserID(
			reqctx.WithPartnerID(c.Request.Context(), partner.ID),
			pu.ID,
		)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func abortPartner(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, structs.ErrorResponse{
		Success: false,
		Message: msg,
		Errors:  map[string]string{"token": msg},
	})
}
