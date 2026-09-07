package middlewares

import (
	"context"

	"candra/backend-api/internal/reqctx"

	"github.com/gin-gonic/gin"
)

// Kunci gin.Context yang dipakai lintas middleware & handler.
//
// Nilainya string (bukan tipe khusus) karena gin.Context memang memakai
// map[string]any; pembatas tabrakannya adalah konvensi penamaan berprefiks.
// Untuk context.Context (yang mengalir ke repository/service), sumber
// kebenarannya adalah paket internal/reqctx, bukan kunci-kunci ini.
const (
	// authorizationPayloadKey menyimpan USER ID dari subject access token.
	authorizationPayloadKey = "userID"

	// tenantIDKey menyimpan tenant_id efektif user untuk permintaan ini.
	tenantIDKey = "tenantID"

	// permissionsKey menyimpan []string kode permission efektif user.
	permissionsKey = "permissions"

	// CurrentUserKey menyimpan objek models.User yang sudah di-load middleware
	// TenantScope, agar handler tidak query ulang.
	CurrentUserKey = "currentUser"
)

// UserIDFromGin mengembalikan user ID terautentikasi, atau "".
func UserIDFromGin(c *gin.Context) string { return c.GetString(authorizationPayloadKey) }

// TenantIDFromGin mengembalikan tenant_id efektif, atau "".
func TenantIDFromGin(c *gin.Context) string { return c.GetString(tenantIDKey) }

// Re-export dengan nama yang dipakai docs/TECHNICAL-BACKEND.md §6, mendelegasikan
// ke internal/reqctx. Memakai context.Context sehingga bisa dipanggil dari
// repository/service.

// TenantIDFromContext mengembalikan tenant_id efektif dari context.
func TenantIDFromContext(ctx context.Context) string { return reqctx.TenantID(ctx) }

// UserIDFromContext mengembalikan user_id terautentikasi dari context.
func UserIDFromContext(ctx context.Context) string { return reqctx.UserID(ctx) }

// HasPermission melaporkan apakah user permintaan ini memiliki `code`.
func HasPermission(ctx context.Context, code string) bool { return reqctx.HasPermission(ctx, code) }
