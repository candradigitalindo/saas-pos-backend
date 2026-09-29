package middlewares

import (
	"net/http"

	"candra/backend-api/internal/reqctx"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// Require menolak permintaan (403) kecuali user memiliki SETIDAKNYA SATU dari
// `codes`. Dipasang setelah TenantScope (yang mengisi himpunan permission).
//
// Contoh: middlewares.Require("user.manage")
func Require(codes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !reqctx.HasAnyPermission(c.Request.Context(), codes...) {
			c.AbortWithStatusJSON(http.StatusForbidden, structs.ErrorResponse{
				Success: false,
				Message: "Anda tidak memiliki izin untuk tindakan ini",
				Errors:  map[string]string{"permission": "akses ditolak"},
			})
			return
		}
		c.Next()
	}
}

// RequireOutletAccess memastikan user boleh mengakses outlet yang dirujuk
// permintaan. Sumber id outlet, berurutan: path param `outlet_id`, lalu query
// `outlet_id`. Pemegang `outlet.manage` dilewatkan (mereka mengelola semua
// outlet tenant).
//
// Dipakai untuk sumber daya per-outlet (shift, kas, stok per outlet) — mulai
// relevan di Fase 3.
func RequireOutletAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if reqctx.HasPermission(ctx, "outlet.manage") {
			c.Next()
			return
		}

		outletID := c.Param("outlet_id")
		if outletID == "" {
			outletID = c.Query("outlet_id")
		}
		if outletID == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, structs.ErrorResponse{
				Success: false,
				Message: "outlet_id wajib disertakan",
				Errors:  map[string]string{"outlet_id": "wajib diisi"},
			})
			return
		}

		ok, err := repositories.UserCanAccessOutlet(ctx, reqctx.UserID(ctx), outletID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, structs.ErrorResponse{
				Success: false,
				Message: "Terjadi kesalahan internal",
				Errors:  map[string]string{"server": "Terjadi kesalahan internal"},
			})
			return
		}
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, structs.ErrorResponse{
				Success: false,
				Message: "Anda tidak punya akses ke outlet ini",
				Errors:  map[string]string{"outlet_id": "akses ditolak"},
			})
			return
		}
		c.Next()
	}
}
