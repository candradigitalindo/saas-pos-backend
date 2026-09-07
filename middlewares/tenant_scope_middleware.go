package middlewares

import (
	"log/slog"
	"net/http"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// TenantScope memuat user permintaan dari database (§9: tenant_id & permission
// SELALU dari DB, tidak pernah dari token) lalu menaruh ke context:
//
//   - tenant_id efektif   → dipakai scopeTenant & WithTenant di repositories
//   - objek User          → CurrentUserKey, agar handler tidak query ulang
//   - himpunan permission  → dipakai Require & HasPermission
//
// WAJIB dipasang SETELAH Auth(). Menolak permintaan bila user tidak ada,
// dinonaktifkan, atau bukan anggota tenant mana pun.
func TenantScope() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString(authorizationPayloadKey)
		if userID == "" {
			abort(c, http.StatusUnauthorized, "auth", "Sesi tidak dikenali")
			return
		}

		ctx := c.Request.Context()
		log := helpers.LoggerFromContext(ctx)

		var user models.User
		if err := repositories.FindUserByID(ctx, userID, &user); err != nil {
			// Token sah tapi user-nya hilang → perlakukan seperti tidak berwenang.
			abort(c, http.StatusUnauthorized, "auth", "Sesi tidak valid")
			return
		}
		if !user.IsActive {
			abort(c, http.StatusForbidden, "auth", "Akun dinonaktifkan")
			return
		}
		if user.TenantID == "" {
			abort(c, http.StatusForbidden, "auth", "Akun tidak terhubung ke usaha mana pun")
			return
		}

		codes, err := repositories.EffectivePermissionCodes(ctx, user.RoleID)
		if err != nil {
			log.Error("gagal memuat permission efektif", slog.Any("error", err), slog.String("user_id", userID))
			abort(c, http.StatusInternalServerError, "server", "Terjadi kesalahan internal")
			return
		}

		// gin.Context — untuk middleware & handler.
		c.Set(tenantIDKey, user.TenantID)
		c.Set(permissionsKey, codes)
		c.Set(CurrentUserKey, user)

		// context.Context — untuk repository & service di bawah.
		ctx = reqctx.WithTenantID(ctx, user.TenantID)
		ctx = reqctx.WithUserID(ctx, user.ID)
		ctx = reqctx.WithPermissions(ctx, codes)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// abort menulis ErrorResponse baku lalu menghentikan chain.
func abort(c *gin.Context, status int, field, msg string) {
	c.AbortWithStatusJSON(status, structs.ErrorResponse{
		Success: false,
		Message: msg,
		Errors:  map[string]string{field: msg},
	})
}
