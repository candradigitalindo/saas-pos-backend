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

	// Master data: kategori, satuan, supplier — baca butuh product.view, tulis
	// butuh product.edit.
	for _, res := range []struct {
		path                           string
		list, get, create, update, del gin.HandlerFunc
	}{
		{"/categories", controllers.ListCategories, controllers.GetCategory, controllers.CreateCategory, controllers.UpdateCategory, controllers.DeleteCategory},
		{"/units", controllers.ListUnits, controllers.GetUnit, controllers.CreateUnit, controllers.UpdateUnit, controllers.DeleteUnit},
		{"/suppliers", controllers.ListSuppliers, controllers.GetSupplier, controllers.CreateSupplier, controllers.UpdateSupplier, controllers.DeleteSupplier},
	} {
		g := t.Group(res.path)
		g.GET("", middlewares.Require("product.view"), res.list)
		g.GET("/:id", middlewares.Require("product.view"), res.get)
		g.POST("", middlewares.Require("product.edit"), res.create)
		g.PUT("/:id", middlewares.Require("product.edit"), res.update)
		g.DELETE("/:id", middlewares.Require("product.edit"), res.del)
	}

	// Produk — perizinan lebih rinci.
	prod := t.Group("/products")
	prod.GET("", middlewares.Require("product.view"), controllers.ListProducts)
	prod.GET("/:id", middlewares.Require("product.view"), controllers.GetProduct)
	prod.POST("", middlewares.Require("product.edit"), controllers.CreateProduct)
	prod.POST("/import", middlewares.Require("product.import"), controllers.ImportProducts)
	prod.PUT("/:id", middlewares.Require("product.edit"), controllers.UpdateProduct)
	prod.DELETE("/:id", middlewares.Require("product.delete"), controllers.DeleteProduct)

	// Pelanggan.
	cust := t.Group("/customers")
	cust.GET("", middlewares.Require("customer.view"), controllers.ListCustomers)
	cust.GET("/:id", middlewares.Require("customer.view"), controllers.GetCustomer)
	cust.POST("", middlewares.Require("customer.edit"), controllers.CreateCustomer)
	cust.PUT("/:id", middlewares.Require("customer.edit"), controllers.UpdateCustomer)
	cust.DELETE("/:id", middlewares.Require("customer.edit"), controllers.DeleteCustomer)

	// Shift & kas.
	t.POST("/shifts/open", middlewares.Require("shift.open"), controllers.OpenShift)
	t.POST("/shifts/:id/close", middlewares.Require("shift.close"), controllers.CloseShift)
	t.GET("/shifts", middlewares.Require("shift.open", "shift.close"), controllers.ListShifts)
	t.GET("/shifts/:id", middlewares.Require("shift.open", "shift.close"), controllers.GetShift)
	t.POST("/cash-movements", middlewares.Require("cash.movement"), controllers.CreateCashMovement)
	t.GET("/cash-movements", middlewares.Require("cash.movement"), controllers.ListCashMovements)

	// Transaksi kasir. (Ringkasan di path terpisah agar tidak bentrok dengan
	// wildcard :id di pohon rute GET.)
	t.POST("/sales", middlewares.Require("sale.create"), controllers.Checkout)
	t.GET("/sales", middlewares.Require("sale.create"), controllers.ListSales)
	t.GET("/sales-summary", middlewares.Require("report.view"), controllers.SalesSummary)
	t.GET("/sales/:id", middlewares.Require("sale.create"), controllers.GetSale)
	t.POST("/sales/:id/void", middlewares.Require("sale.void"), controllers.VoidSale)
	t.POST("/sales/:id/refund", middlewares.Require("sale.refund"), controllers.RefundSale)

	// Piutang.
	t.GET("/receivables", middlewares.Require("receivable.manage"), controllers.ListReceivables)
	t.GET("/receivables/:id", middlewares.Require("receivable.manage"), controllers.GetReceivable)
	t.POST("/receivable-payments", middlewares.Require("receivable.manage"), controllers.AddReceivablePayment)

	// Stok: baca + penyesuaian manual (saldo awal / koreksi).
	t.GET("/stocks", middlewares.Require("stock.view"), controllers.ListStocks)
	t.GET("/stock-movements", middlewares.Require("stock.view"), controllers.ListStockMovements)
	t.POST("/stock-adjustments", middlewares.Require("stock.adjust"), controllers.AdjustStock)
	t.POST("/stock-reconcile", middlewares.Require("stock.opname"), controllers.ReconcileStocks)

	// Pembelian (stok masuk). Wajib Idempotency-Key.
	t.POST("/purchases", middlewares.Require("stock.adjust"), controllers.ReceivePurchase)
	t.GET("/purchases", middlewares.Require("stock.view"), controllers.ListPurchases)
	t.GET("/purchases/:id", middlewares.Require("stock.view"), controllers.GetPurchase)

	// Stok opname.
	op := t.Group("/stock-opnames")
	op.POST("", middlewares.Require("stock.opname"), controllers.CreateOpname)
	op.GET("", middlewares.Require("stock.view"), controllers.ListOpnames)
	op.GET("/:id", middlewares.Require("stock.view"), controllers.GetOpname)
	op.POST("/:id/items", middlewares.Require("stock.opname"), controllers.SetOpnameItems)
	op.POST("/:id/post", middlewares.Require("stock.opname"), controllers.PostOpname)

	// Transfer stok antar outlet.
	tr := t.Group("/stock-transfers")
	tr.POST("", middlewares.Require("stock.transfer"), controllers.CreateTransfer)
	tr.GET("", middlewares.Require("stock.view"), controllers.ListTransfers)
	tr.GET("/:id", middlewares.Require("stock.view"), controllers.GetTransfer)
	tr.POST("/:id/send", middlewares.Require("stock.transfer"), controllers.SendTransfer)
	tr.POST("/:id/receive", middlewares.Require("stock.transfer"), controllers.ReceiveTransfer)

	// Resep (F&B) — bahan baku dipotong saat menu terjual.
	prod.GET("/:id/recipe", middlewares.Require("product.view"), controllers.GetProductRecipe)
	prod.PUT("/:id/recipe", middlewares.Require("product.edit"), controllers.UpsertProductRecipe)

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
