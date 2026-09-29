package middlewares

import (
	"errors"
	"log/slog"
	"net/http"

	"candra/backend-api/helpers"
	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// RequireFeatureForWrites menolak permintaan yang MENGUBAH data (selain GET,
// HEAD, OPTIONS) bila fitur `kode` tidak termasuk paket langganan yang
// berlaku → 402 dengan pesan yang menyebut paket termurah yang memuatnya.
//
// Permintaan BACA sengaja dilewatkan: tenant yang turun paket tetap bisa
// membuka prospek, invoice, dan pesanan kanal lamanya — data tidak disandera
// (lihat services/plan_entitlement_service.go). Yang ditutup hanya tindakan
// baru.
//
// Dipasang SETELAH TenantScope (butuh tenant di context) dan setelah Require
// (izin peran diperiksa dulu: orang tanpa izin mendapat 403, bukan ajakan
// naik paket yang tidak bisa ia putuskan).
func RequireFeatureForWrites(kode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		if err := services.RequireFeature(c.Request.Context(), kode); err != nil {
			if !errors.Is(err, helpers.ErrPlanRequired) {
				helpers.LoggerFromContext(c.Request.Context()).Error("memeriksa paket", slog.Any("error", err))
				c.AbortWithStatusJSON(http.StatusInternalServerError, structs.ErrorResponse{
					Success: false,
					Message: "Terjadi kesalahan internal",
					Errors:  map[string]string{"server": "Terjadi kesalahan internal"},
				})
				return
			}
			pesan := helpers.PesanUntukPengguna(err)
			c.AbortWithStatusJSON(http.StatusPaymentRequired, structs.ErrorResponse{
				Success: false,
				Message: pesan,
				Errors:  map[string]string{"plan": pesan},
			})
			return
		}
		c.Next()
	}
}
