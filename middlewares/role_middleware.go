package middlewares

import (
	"net/http"

	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// RoleMiddleware membuat sebuah middleware yang memeriksa apakah pengguna yang terautentikasi
// memiliki salah satu peran yang dibutuhkan.
func RoleMiddleware(requiredRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Ambil user ID dari konteks (yang sudah di-set oleh AuthMiddleware)
		userID, exists := c.Get(authorizationPayloadKey)
		if !exists {
			// Ini seharusnya tidak terjadi jika AuthMiddleware digunakan sebelum middleware ini
			c.AbortWithStatusJSON(http.StatusForbidden, structs.ErrorResponse{
				Success: false,
				Message: "Akses ditolak. Tidak ada informasi pengguna.",
			})
			return
		}

		// 2. Ambil detail user dari database untuk mendapatkan perannya
		var user models.User
		if err := repositories.FindUserByID(c.Request.Context(), userID.(string), &user); err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, structs.ErrorResponse{
				Success: false,
				Message: "Akses ditolak. Pengguna tidak valid.",
			})
			return
		}

		// 3. Periksa apakah peran user ada di dalam daftar peran yang diizinkan
		hasPermission := false
		for _, requiredRole := range requiredRoles {
			if user.Role.Name == requiredRole {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			c.AbortWithStatusJSON(http.StatusForbidden, structs.ErrorResponse{Success: false, Message: "Akses ditolak. Anda tidak memiliki izin yang diperlukan."})
			return
		}

		// 4. Simpan user yang sudah di-load ke context agar handler berikutnya
		// bisa memakainya kembali tanpa query ulang ke database.
		c.Set(CurrentUserKey, user)

		// 5. Pengguna memiliki peran yang sesuai, lanjutkan ke handler berikutnya
		c.Next()
	}
}
