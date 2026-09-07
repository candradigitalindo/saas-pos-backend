package routes

import (
	"net/http"
	"reflect"
	"strings"

	"candra/backend-api/config"
	"candra/backend-api/controllers"
	"candra/backend-api/internal/ulid"
	"candra/backend-api/middlewares"
	"candra/backend-api/structs"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// SetupRouter merakit seluruh rute HTTP beserta middleware-nya.
//
// Urutan middleware global penting:
//  1. Recovery       — paling luar, agar panic di middleware lain pun tertangkap.
//  2. Observability   — request ID + logger request-scoped + satu baris log/permintaan.
//  3. CORS.
//
// gin.New() (bukan gin.Default()) dipakai supaya logger & recovery bawaan Gin
// yang tidak terstruktur tidak ikut terpasang.
func SetupRouter() *gin.Engine {
	if config.GetEnv("APP_ENV", "development") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	// Aktifkan 405 (Method Not Allowed) alih-alih 404 saat path cocok tapi
	// method-nya tidak — supaya NoMethod handler di bawah bisa bekerja.
	r.HandleMethodNotAllowed = true
	r.Use(middlewares.Recovery())
	r.Use(middlewares.RequestObservability())

	// Trusted proxies: secara default TIDAK mempercayai proxy mana pun, sehingga
	// c.ClientIP() memakai alamat koneksi asli dan X-Forwarded-For tidak bisa
	// dipalsukan untuk menembus rate limiter (§4 CONVENTIONS).
	if tp := config.GetStringSliceEnv("TRUSTED_PROXIES", nil); len(tp) > 0 {
		_ = r.SetTrustedProxies(tp)
	} else {
		_ = r.SetTrustedProxies(nil)
	}

	registerValidators()

	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = config.GetStringSliceEnv(
		"ALLOWED_ORIGINS",
		[]string{"http://localhost:5173", "http://localhost:3000"},
	)
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	r.Use(cors.New(corsConfig))

	// Rute & method tak dikenal tetap memakai bentuk response baku aplikasi,
	// bukan teks polos bawaan Gin (klien mengandalkan bentuk yang konsisten).
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, structs.ErrorResponse{
			Success: false,
			Message: "Endpoint tidak ditemukan",
			Errors:  map[string]string{"path": c.Request.URL.Path},
		})
	})
	r.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, structs.ErrorResponse{
			Success: false,
			Message: "Metode tidak diizinkan untuk endpoint ini",
			Errors:  map[string]string{"method": c.Request.Method},
		})
	})

	// Health check — tanpa auth, tanpa rate limit, tanpa prefiks versi.
	r.GET("/health", controllers.Health)
	r.GET("/health/ready", controllers.Readiness)

	v1 := r.Group("/api/v1")
	registerAuthRoutes(v1)
	registerTenantRoutes(v1)

	return r
}

// registerValidators memasang validator kustom dan membuat pesan error memakai
// nama field dari tag `json` (konsisten dengan payload API).
func registerValidators() {
	v, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		return
	}

	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	// Tag `binding:"...,ulid"` memvalidasi bahwa sebuah string berbentuk ULID
	// yang sah — dipakai untuk ID kiriman klien (§3.1).
	_ = v.RegisterValidation("ulid", func(fl validator.FieldLevel) bool {
		s, ok := fl.Field().Interface().(string)
		return ok && ulid.IsValid(s)
	})
}

// registerAuthRoutes memasang endpoint autentikasi di bawah /api/v1/auth.
// Endpoint publik di-rate-limit untuk meredam brute-force & pembuatan akun
// massal. Laju & burst dapat dikonfigurasi (AUTH_RATELIMIT_RPS / _BURST);
// default ~0.2 req/detik, burst 5. /register kini = pendaftaran USAHA baru.
func registerAuthRoutes(v1 *gin.RouterGroup) {
	authLimiter := middlewares.RateLimit(
		config.GetFloatEnv("AUTH_RATELIMIT_RPS", 0.2),
		float64(config.GetIntEnv("AUTH_RATELIMIT_BURST", 5)),
	)

	auth := v1.Group("/auth")
	auth.POST("/register", authLimiter, controllers.Register)
	auth.POST("/login", authLimiter, controllers.Login)
	auth.POST("/refresh", authLimiter, controllers.Refresh)
	auth.POST("/logout", middlewares.Auth(), controllers.Logout)
}

// registerTenantRoutes memasang seluruh endpoint yang beroperasi di dalam satu
// tenant. Rantai middleware: Auth (token → user id) → TenantScope (user id →
// tenant_id + permission ke context) → Require(...) per-endpoint.
func registerTenantRoutes(v1 *gin.RouterGroup) {
	t := v1.Group("", middlewares.Auth(), middlewares.TenantScope())

	// Profil pengguna — semua user tenant boleh.
	t.GET("/me", controllers.Me)

	// Outlet.
	outlet := t.Group("/outlets", middlewares.Require("outlet.manage"))
	outlet.GET("", controllers.ListOutlets)
	outlet.POST("", controllers.CreateOutlet)
	outlet.GET("/:id", controllers.GetOutlet)
	outlet.PUT("/:id", controllers.UpdateOutlet)
	outlet.DELETE("/:id", controllers.DeleteOutlet)

	// Manajemen user staf.
	user := t.Group("/users", middlewares.Require("user.manage"))
	user.GET("", controllers.GetAllUsers)
	user.POST("", controllers.CreateUser)
	user.GET("/:id", controllers.GetUserByID)
	user.PUT("/:id", controllers.UpdateUser)
	user.DELETE("/:id", controllers.DeleteUser)

	// Peran & hak akses.
	t.GET("/permissions", middlewares.Require("role.manage"), controllers.ListPermissions)
	role := t.Group("/roles", middlewares.Require("role.manage"))
	role.GET("", controllers.GetAllRoles)
	role.POST("", controllers.CreateRole)
	role.GET("/:id", controllers.GetRoleByID)
	role.PUT("/:id", controllers.UpdateRole)
	role.PUT("/:id/permissions", controllers.SetRolePermissions)
	role.DELETE("/:id", controllers.DeleteRole)
}
