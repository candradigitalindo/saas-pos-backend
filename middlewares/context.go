package middlewares

import "github.com/gin-gonic/gin"

// Kunci gin.Context yang dipakai lintas middleware & handler.
//
// Dikumpulkan di satu berkas supaya tidak tersebar dan tidak bentrok. Nilainya
// string (bukan tipe khusus) karena gin.Context memang memakai map[string]any;
// pembatas tabrakannya adalah konvensi penamaan berprefiks di paket ini.
const (
	// authorizationPayloadKey menyimpan USER ID dari subject access token.
	authorizationPayloadKey = "userID"

	// tenantIDKey menyimpan tenant_id efektif user untuk permintaan ini.
	// Diisi oleh middleware tenant di Fase 1; kosong sebelum itu.
	tenantIDKey = "tenantID"

	// CurrentUserKey menyimpan objek models.User yang sudah di-load middleware
	// otorisasi, agar handler tidak query ulang.
	CurrentUserKey = "currentUser"
)

// UserIDFromGin mengembalikan user ID terautentikasi, atau "" bila belum ada.
func UserIDFromGin(c *gin.Context) string {
	return c.GetString(authorizationPayloadKey)
}

// TenantIDFromGin mengembalikan tenant_id efektif, atau "" bila belum di-scope.
func TenantIDFromGin(c *gin.Context) string {
	return c.GetString(tenantIDKey)
}
