package routes

import (
	"net/http"
	"reflect"
	"strings"

	"candra/backend-api/config"
	"candra/backend-api/controllers"
	"candra/backend-api/internal/ulid"
	"candra/backend-api/middlewares"
	"candra/backend-api/models"
	"candra/backend-api/services"
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

	// Foto barang, disajikan apa adanya dari disk.
	//
	// TANPA autentikasi, dan itu disengaja: <img> tidak mengirim header
	// Authorization, jadi foto berpagar token tidak akan pernah tampil di
	// kasir. Pagarnya adalah nama berkas ULID yang dibuat server dan tidak
	// bisa ditebak — bukan rahasia besar, tapi foto barang dagangan memang
	// bukan rahasia. Yang TIDAK boleh terjadi adalah folder ini memuat apa
	// pun selain gambar; itu dijaga di sisi unggahnya.
	r.Static("/uploads", services.FolderUnggah())

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
	// Idempotency-Key WAJIB ada di daftar ini. Tanpanya, preflight browser
	// menolak POST /sales, /purchases, /invoice-payments, dan
	// /subscription-payments — yaitu seluruh aksi yang menciptakan uang atau
	// stok — sehingga klien web tidak bisa checkout sama sekali.
	corsConfig.AllowHeaders = []string{
		"Origin", "Content-Type", "Accept", "Authorization", "Idempotency-Key",
	}
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

	// Webhook kanal (Fase 11b, §5.10, blueprint F.6) — TANPA auth, tanpa
	// prefiks versi: kanal tidak membawa token kita, ditautkan lewat
	// (provider, merchant_ref). Payload disimpan MENTAH ke channel_events lalu
	// balas 200 cepat; pekerja cmd/process-channel-events yang memprosesnya.
	//
	// Rate limit per-IP: endpoint publik yang menulis ke channel_events, jadi
	// dibatasi agar banjir permintaan tidak membanjiri inbox. Longgar (kanal
	// wajar membebankan puluhan pesanan/detik saat jam sibuk) & dapat diatur
	// (CHANNEL_WEBHOOK_RATELIMIT_RPS / _BURST).
	webhookLimiter := middlewares.RateLimit("channel-webhook",
		config.GetFloatEnv("CHANNEL_WEBHOOK_RATELIMIT_RPS", 20),
		float64(config.GetIntEnv("CHANNEL_WEBHOOK_RATELIMIT_BURST", 40)),
	)
	r.POST("/webhooks/channels/:provider", webhookLimiter, controllers.IngestChannelWebhook)
	// Alamat PER KANAL untuk sambungan API milik tenant: tanda tangan diperiksa
	// dengan rahasia kanal itu. GET = verifikasi alamat oleh penyedia (Meta).
	r.GET("/webhooks/channels/:provider/:token", webhookLimiter, controllers.ProviderWebhookChallenge)
	r.POST("/webhooks/channels/:provider/:token", webhookLimiter, controllers.ProviderWebhook)
	// Sub-jalur (mis. GrabFood: /oauth/token, /order) — penyedia yang
	// memanggil beberapa endpoint "server partner".
	r.GET("/webhooks/channels/:provider/:token/*aksi", webhookLimiter, controllers.ProviderWebhookChallenge)
	r.POST("/webhooks/channels/:provider/:token/*aksi", webhookLimiter, controllers.ProviderWebhook)

	v1 := r.Group("/api/v1")
	// Struk digital publik — dibuka pembeli dari tautan WhatsApp, tanpa akun.
	// Token 128 bit tidak bisa ditebak; rate limit per-IP hanya meredam
	// penyisiran & beban (STRUK_PUBLIK_RATELIMIT_RPS / _BURST).
	strukLimiter := middlewares.RateLimit("struk-publik",
		config.GetFloatEnv("STRUK_PUBLIK_RATELIMIT_RPS", 1),
		float64(config.GetIntEnv("STRUK_PUBLIK_RATELIMIT_BURST", 20)),
	)
	v1.GET("/public/receipts/:token", strukLimiter, controllers.PublicReceipt)
	registerAuthRoutes(v1)
	registerPartnerRoutes(v1)
	registerPlatformRoutes(v1)
	registerTenantRoutes(v1)

	return r
}

// registerPartnerRoutes memasang portal Program Mitra di bawah
// /api/v1/partner/* (Fase 12, blueprint G.4/G.8). Jalur autentikasi TERPISAH:
// PartnerAuth hanya menerima token ber-realm "partner", dan token itu ditolak
// di seluruh rute tenant. Mitra TIDAK PERNAH bisa menyentuh data operasional
// tenant — yang boleh dilihat hanya status langganan merchant binaannya.
func registerPartnerRoutes(v1 *gin.RouterGroup) {
	loginLimiter := middlewares.RateLimit("partner-login",
		config.GetFloatEnv("AUTH_RATELIMIT_RPS", 0.2),
		float64(config.GetIntEnv("AUTH_RATELIMIT_BURST", 5)),
	)
	v1.POST("/partner/auth/login", loginLimiter, controllers.PartnerLogin)

	p := v1.Group("/partner", middlewares.PartnerAuth())
	p.GET("/me", controllers.PartnerMe)
	p.GET("/dashboard", controllers.PartnerDashboard)
	p.GET("/leads", controllers.PartnerListLeads)
	p.POST("/leads", controllers.PartnerCreateLead)
	p.GET("/merchants", controllers.PartnerListMerchants)
	p.GET("/commissions", controllers.PartnerListCommissions)
	p.GET("/payouts", controllers.PartnerListPayouts)

	// Pelengkap (blueprint G.4): materi jualan disaring per tingkat, pelatihan
	// beserta status penyelesaiannya, target berbasis MERCHANT AKTIF, dan
	// pengajuan sengketa atribusi.
	p.GET("/materials", controllers.PartnerListMaterials)
	p.GET("/trainings", controllers.PartnerListTrainings)
	p.POST("/trainings/:id/complete", controllers.PartnerCompleteTraining)
	p.GET("/targets", controllers.PartnerListTargets)
	p.GET("/disputes", controllers.PartnerListDisputes)
	p.POST("/disputes", controllers.PartnerCreateDispute)
}

// registerPlatformRoutes memasang panel internal penyedia SaaS di bawah
// /api/v1/platform/* (blueprint G.5, migrasi 000035). Realm KETIGA: hanya token
// ber-realm "platform" yang diterima, dan token itu ditolak di rute tenant
// maupun portal mitra.
//
// Wewenang dipisah per peran supaya orang yang memverifikasi mitra bukan orang
// yang sama dengan yang mencairkan uangnya:
//
//	operator → verifikasi mitra & tingkat      finance → komisi & pencairan
//	support  → hanya membaca                   superadmin → semuanya
func registerPlatformRoutes(v1 *gin.RouterGroup) {
	loginLimiter := middlewares.RateLimit("platform-login",
		config.GetFloatEnv("AUTH_RATELIMIT_RPS", 0.2),
		float64(config.GetIntEnv("AUTH_RATELIMIT_BURST", 5)),
	)
	v1.POST("/platform/auth/login", loginLimiter, controllers.PlatformLogin)

	pf := v1.Group("/platform", middlewares.PlatformAuth())
	pf.GET("/me", controllers.PlatformMe)

	baca := middlewares.RequirePlatform(models.CapPlatformRead)
	verif := middlewares.RequirePlatform(models.CapPartnerVerify)
	uang := middlewares.RequirePlatform(models.CapPartnerFinance)
	kelola := middlewares.RequirePlatform(models.CapPlatformAdmin)

	// Akun staf internal — hanya superadmin.
	pf.GET("/admins", kelola, controllers.PlatformListAdmins)
	pf.POST("/admins", kelola, controllers.PlatformCreateAdmin)
	pf.PUT("/admins/:id/active", kelola, controllers.PlatformSetAdminActive)

	// Mitra: verifikasi & tingkat.
	pf.GET("/partners", baca, controllers.PlatformListPartners)
	pf.POST("/partners", verif, controllers.PlatformCreatePartner)
	pf.POST("/partners/:id/approve", verif, controllers.PlatformApprovePartner)
	pf.POST("/partners/:id/suspend", verif, controllers.PlatformSuspendPartner)
	pf.GET("/partner-tiers", baca, controllers.PlatformListTiers)
	pf.POST("/partner-tiers", verif, controllers.PlatformCreateTier)

	// Komisi & pencairan — hanya finance/superadmin.
	pf.POST("/partner-commissions/run", uang, controllers.PlatformRunCommissions)
	pf.POST("/partner-commissions/:id/approve", uang, controllers.PlatformApproveCommission)
	pf.POST("/partner-payouts", uang, controllers.PlatformCreatePayout)
	pf.POST("/partner-payouts/:id/paid", uang, controllers.PlatformMarkPayoutPaid)

	// Target, materi jualan, pelatihan.
	pf.GET("/partners/:id/targets", baca, controllers.PlatformListTargets)
	pf.POST("/partner-targets", verif, controllers.PlatformSetTarget)
	pf.GET("/partner-materials", baca, controllers.PlatformListMaterials)
	pf.POST("/partner-materials", verif, controllers.PlatformCreateMaterial)
	pf.GET("/partner-trainings", baca, controllers.PlatformListTrainings)
	pf.POST("/partner-trainings", verif, controllers.PlatformCreateTraining)

	// Sengketa atribusi — diputus admin dan keputusannya dicatat (G.2 #5).
	pf.GET("/partner-disputes", baca, controllers.PlatformListDisputes)
	pf.POST("/partner-disputes/:id/resolve", middlewares.RequirePlatform(models.CapPartnerDispute), controllers.PlatformResolveDispute)

	// Konfirmasi pembayaran langganan — verifikasi uang masuk (billing.verify:
	// finance & superadmin). Menyetujui = mencatat pembayaran & mengaktifkan paket.
	tagihan := middlewares.RequirePlatform(models.CapBillingVerify)
	pf.GET("/subscription-payment-claims", tagihan, controllers.PlatformListPaymentClaims)
	pf.POST("/subscription-payment-claims/:id/approve", tagihan, controllers.PlatformApprovePaymentClaim)
	pf.POST("/subscription-payment-claims/:id/reject", tagihan, controllers.PlatformRejectPaymentClaim)
	// Pengembalian dana langganan yang dihentikan — uang KELUAR, wewenang keuangan.
	refund := middlewares.RequirePlatform(models.CapBillingRefund)
	pf.GET("/subscription-refunds", refund, controllers.PlatformListRefunds)
	pf.POST("/subscription-refunds/:id/paid", refund, controllers.PlatformMarkRefundPaid)

	// Outbox notifikasi (§5.14): antrean, antrean mati, dan template pesan.
	pf.GET("/outbox", baca, controllers.PlatformListOutbox)
	pf.POST("/outbox/:id/retry", kelola, controllers.PlatformRetryOutbox)
	pf.POST("/notification-templates", kelola, controllers.PlatformUpsertTemplate)
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
	authLimiter := middlewares.RateLimit("auth",
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

	// Outlet. Daftarnya juga dibuka untuk pemegang stock.transfer: petugas
	// gudang harus bisa MEMILIH cabang tujuan kiriman, dan dulu layar transfer
	// menerima 403 sehingga pilihan tujuannya kosong bagi peran Gudang bawaan.
	// Mengubah/menambah cabang tetap hanya outlet.manage.
	t.GET("/outlets", middlewares.Require("outlet.manage", "stock.transfer"), controllers.ListOutlets)
	outlet := t.Group("/outlets", middlewares.Require("outlet.manage"))
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

	// Daftar harga khusus (member, reseller) — seperti master data lain.
	pl := t.Group("/price-lists")
	pl.GET("", middlewares.Require("product.view"), controllers.ListPriceLists)
	pl.POST("", middlewares.Require("product.edit"), controllers.CreatePriceList)
	pl.DELETE("/:id", middlewares.Require("product.edit"), controllers.DeletePriceList)

	// Produk — perizinan lebih rinci.
	prod := t.Group("/products")
	prod.GET("", middlewares.Require("product.view"), controllers.ListProducts)
	prod.GET("/:id", middlewares.Require("product.view"), controllers.GetProduct)
	prod.POST("", middlewares.Require("product.edit"), controllers.CreateProduct)
	prod.POST("/import", middlewares.Require("product.import"), controllers.ImportProducts)
	prod.PUT("/:id", middlewares.Require("product.edit"), controllers.UpdateProduct)
	prod.DELETE("/:id", middlewares.Require("product.delete"), controllers.DeleteProduct)
	prod.POST("/:id/image", middlewares.Require("product.edit"), controllers.UploadProductImage)
	prod.DELETE("/:id/image", middlewares.Require("product.edit"), controllers.DeleteProductImage)
	prod.GET("/:id/variants", middlewares.Require("product.view"), controllers.ListProductVariants)
	prod.POST("/:id/variants", middlewares.Require("product.edit"), controllers.CreateProductVariant)
	prod.PUT("/:id/variants/:vid", middlewares.Require("product.edit"), controllers.UpdateProductVariant)
	prod.DELETE("/:id/variants/:vid", middlewares.Require("product.edit"), controllers.DeleteProductVariant)

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
	// Serah terima: tutup + buka dalam satu transaksi. Izinnya shift.close DAN
	// shift.open — orang yang boleh menyerahterimakan harus boleh melakukan
	// keduanya, bukan hanya salah satunya.
	t.POST("/shifts/:id/handover",
		middlewares.Require("shift.close"), middlewares.Require("shift.open"),
		controllers.HandoverShift)
	t.GET("/shifts", middlewares.Require("shift.open", "shift.close"), controllers.ListShifts)
	t.GET("/shifts/:id", middlewares.Require("shift.open", "shift.close"), controllers.GetShift)
	t.POST("/cash-movements", middlewares.Require("cash.movement"), controllers.CreateCashMovement)
	t.GET("/cash-movements", middlewares.Require("cash.movement"), controllers.ListCashMovements)

	// Transaksi kasir. (Ringkasan di path terpisah agar tidak bentrok dengan
	// wildcard :id di pohon rute GET.)
	t.POST("/sales", middlewares.Require("sale.create"), controllers.Checkout)
	t.GET("/sales", middlewares.Require("sale.create"), controllers.ListSales)
	t.GET("/sales/day-summary", middlewares.Require("sale.create"), controllers.SalesDaySummary)
	t.GET("/sales-summary", middlewares.Require("report.view"), controllers.SalesSummary)
	t.GET("/sales/:id", middlewares.Require("sale.create"), controllers.GetSale)
	t.POST("/sales/:id/void", middlewares.Require("sale.void"), controllers.VoidSale)
	t.POST("/sales/:id/refund", middlewares.Require("sale.refund"), controllers.RefundSale)
	t.POST("/sales/:id/receipt-link", middlewares.Require("sale.create"), controllers.CreateReceiptLink)

	// Tagihan terbuka (open bill / tahan transaksi) — dibayar lewat POST /sales
	// dengan open_bill_id.
	t.GET("/open-bills", middlewares.Require("sale.create"), controllers.ListOpenBills)
	t.PUT("/open-bills/:id", middlewares.Require("sale.create"), controllers.UpsertOpenBill)
	t.POST("/open-bills/:id/cancel", middlewares.Require("sale.create"), controllers.CancelOpenBill)

	// Piutang.
	t.GET("/receivables", middlewares.Require("receivable.manage"), controllers.ListReceivables)
	t.GET("/receivables/:id", middlewares.Require("receivable.manage"), controllers.GetReceivable)
	t.POST("/receivable-payments", middlewares.Require("receivable.manage"), controllers.AddReceivablePayment)

	// Laporan & dashboard — agregat dari daily_sales_summaries (Fase 5, §8).
	// Semua path statis di bawah /reports; tak ada :id → tak bentrok wildcard.
	rep := t.Group("/reports")
	rep.GET("/dashboard", middlewares.Require("report.view"), controllers.ReportDashboard)
	rep.GET("/sales", middlewares.Require("report.view"), controllers.ReportSales)
	rep.GET("/profit", middlewares.Require("report.profit"), controllers.ReportProfit)
	rep.GET("/export", middlewares.Require("report.export"), controllers.ReportExport)
	// Bangun ulang ringkasan: pekerjaan pemeliharaan BERAT (sampai 366 hari ×
	// semua cabang) yang menulis ulang tabel laporan. Izin baca laporan saja
	// tidak cukup — butuh juga outlet.manage (pengelola seluruh cabang), sama
	// seperti kasir tidak boleh memicu rekonsiliasi stok.
	rep.POST("/rebuild-summaries",
		middlewares.Require("report.view"), middlewares.Require("outlet.manage"),
		controllers.RebuildReportSummaries)

	// Sinkronisasi offline (Fase 6, §10). Perangkat kasir mendorong penjualan
	// yang dibuat offline dan perangkat sales lapangan mendorong kunjungan;
	// keduanya menarik master data + stok. Pintu masuknya cukup salah satu
	// izin, dan izin tiap operasi push diperiksa lagi di services.SyncPush.
	sync := t.Group("/sync", middlewares.Require("sale.create", "crm.visit.checkin"))
	sync.POST("/push", controllers.SyncPush)
	sync.GET("/pull", controllers.SyncPull)

	// Langganan & tagihan platform (Fase 7, §5.13). Tabel platform (tanpa RLS);
	// isolasi dijaga filter tenant_id di repo. Katalog paket cukup terautentikasi;
	// selebihnya butuh billing.manage.
	t.GET("/plans", controllers.ListPlans)
	sub := t.Group("/subscription", middlewares.Require("billing.manage"))
	sub.GET("", controllers.GetSubscription)
	sub.POST("", controllers.StartSubscription)
	sub.POST("/invoices", controllers.GenerateSubInvoice)
	sub.GET("/invoices", controllers.ListSubInvoices)
	sub.GET("/cancel-preview", controllers.CancelPreview)
	sub.POST("/cancel", controllers.CancelSubscription)
	sub.POST("/change-plan", controllers.ChangeSubscriptionPlan)
	sub.POST("/invoices/:id/void", controllers.VoidPlanChangeInvoice)
	// Tenant hanya MENGONFIRMASI pembayaran; yang mencatat pembayaran & mengaktifkan
	// paket adalah staf keuangan platform (panel). Dulu di sini ada
	// POST /subscription-payments — tenant menandai tagihannya sendiri lunas.
	t.POST("/subscription-payment-claims", middlewares.Require("billing.manage"), controllers.SubmitPaymentClaim)
	t.GET("/subscription-payment-claims", middlewares.Require("billing.manage"), controllers.ListPaymentClaims)

	// CRM tenant (Fase 9, §5.9). Baca butuh salah satu izin lihat prospek;
	// tulis butuh crm.deal.edit. Visibilitas kepemilikan (lapis 3) dijaga repo.
	crmView := middlewares.Require("crm.lead.view.own", "crm.lead.view.all")
	crmEdit := middlewares.Require("crm.deal.edit")
	// Kunci paket: MEMULAI hal baru di CRM butuh fitur crm_freelance. Yang
	// sudah berjalan (ubah, menangkan, kirim, terima bayaran invoice lama)
	// sengaja tidak dikunci — turun paket tidak boleh membuat pekerjaan yang
	// sudah setengah jalan tidak bisa diselesaikan atau ditagih.
	crmBaru := middlewares.RequireFeatureForWrites(services.FeatureCRMFreelance)

	t.GET("/lead-sources", crmView, controllers.ListLeadSources)
	t.POST("/lead-sources", crmEdit, crmBaru, controllers.CreateLeadSource)

	t.GET("/pipelines", crmView, controllers.ListPipelines)
	t.POST("/pipelines", crmEdit, crmBaru, controllers.CreatePipeline)

	deal := t.Group("/deals")
	deal.GET("", crmView, controllers.ListDeals)
	deal.POST("", crmEdit, crmBaru, controllers.CreateDeal)
	deal.GET("/:id", crmView, controllers.GetDeal)
	deal.PUT("/:id", crmEdit, controllers.UpdateDeal)
	deal.POST("/:id/win", crmEdit, controllers.WinDeal)
	deal.POST("/:id/lose", crmEdit, controllers.LoseDeal)

	act := t.Group("/activities")
	act.GET("", crmView, controllers.ListActivities)
	act.POST("", crmEdit, crmBaru, controllers.CreateActivity)
	act.POST("/:id/complete", crmEdit, controllers.CompleteActivity)
	act.POST("/:id/cancel", crmEdit, controllers.CancelActivity)

	quo := t.Group("/quotations")
	quo.GET("", crmView, controllers.ListQuotations)
	quo.POST("", crmEdit, crmBaru, controllers.CreateQuotation)
	quo.GET("/:id", crmView, controllers.GetQuotation)
	quo.POST("/:id/send", crmEdit, controllers.SendQuotation)
	quo.POST("/:id/accept", middlewares.Require("quotation.approve"), controllers.AcceptQuotation)
	quo.POST("/:id/reject", crmEdit, controllers.RejectQuotation)

	proj := t.Group("/projects")
	proj.GET("", crmView, controllers.ListProjects)
	proj.POST("", crmEdit, crmBaru, controllers.CreateProject)
	proj.GET("/:id", crmView, controllers.GetProject)
	proj.PUT("/:id", crmEdit, controllers.UpdateProject)
	proj.POST("/:id/tasks", crmEdit, controllers.AddProjectTask)
	proj.POST("/:id/expenses", crmEdit, controllers.AddProjectExpense)

	inv := t.Group("/invoices")
	inv.GET("", crmView, controllers.ListInvoices)
	inv.POST("", middlewares.Require("invoice.issue"), crmBaru, controllers.CreateInvoice)
	inv.GET("/:id", crmView, controllers.GetInvoice)
	inv.POST("/:id/send", middlewares.Require("invoice.issue"), controllers.SendInvoice)
	inv.POST("/:id/void", middlewares.Require("invoice.void"), controllers.VoidInvoice)
	t.POST("/invoice-payments", middlewares.Require("invoice.issue"), controllers.PayInvoice)

	// CRM sales lapangan (Fase 10, §5.9). Kunjungan & rencana dijaga
	// crm.visit.checkin; target & komisi butuh crm.commission.view (pengawas).
	visitView := middlewares.Require("crm.visit.checkin", "crm.lead.view.all")
	visitEdit := middlewares.Require("crm.visit.checkin")
	commView := middlewares.Require("crm.commission.view")
	// Kunci paket crm_sales: hanya MEMULAI yang dikunci — rencana kunjungan,
	// check-in langsung, target baru. Menyelesaikan kunjungan yang berjalan,
	// membaca riwayat, dan menghitung/menyetujui/membayar komisi periode yang
	// sudah lewat tetap boleh (sales yang sudah bekerja tetap dibayar).
	// Kunjungan offline lewat /sync/push juga tidak dikunci: sudah terjadi.
	salesBaru := middlewares.RequireFeatureForWrites(services.FeatureCRMSales)

	vp := t.Group("/visit-plans")
	vp.GET("", visitView, controllers.ListVisitPlans)
	vp.POST("", visitEdit, salesBaru, controllers.CreateVisitPlan)
	vp.GET("/:id", visitView, controllers.GetVisitPlan)

	vis := t.Group("/visits")
	vis.GET("", visitView, controllers.ListVisits)
	vis.POST("", visitEdit, salesBaru, controllers.UpsertVisit)
	vis.GET("/:id", visitView, controllers.GetVisit)
	vis.POST("/:id/checkout", visitEdit, controllers.CheckoutVisit)

	st := t.Group("/sales-targets")
	st.GET("", commView, controllers.ListSalesTargets)
	st.POST("", commView, salesBaru, controllers.SetSalesTarget)

	// POST /commissions = hitung (bukan /commissions/compute — path statis di
	// posisi yang sama dengan :id membuat gin panik).
	com := t.Group("/commissions", commView)
	com.GET("", controllers.ListCommissions)
	com.POST("", controllers.ComputeCommission)
	com.POST("/:id/approve", controllers.ApproveCommission)
	com.POST("/:id/pay", controllers.PayCommission)

	// Kanal pesanan online — fondasi (Fase 11a, §5.10). Definisi kanal &
	// pemetaan SKU butuh channel.manage; entri/impor pesanan butuh
	// channel.order.accept. Laba bersih per kanal keluar lewat /reports/profit.
	chMgr := middlewares.Require("channel.manage")
	chOrd := middlewares.Require("channel.order.accept", "channel.manage")
	// Kunci paket: membuat/mengubah kanal & pemetaan barang serta MEMASUKKAN
	// pesanan baru butuh fitur online_channel. Pesanan yang sudah masuk tetap
	// bisa diproses/dibatalkan dan pencairan tetap bisa dicatat — itu
	// pembukuan penjualan yang sudah terjadi.
	chFitur := middlewares.RequireFeatureForWrites(services.FeatureOnlineChannel)

	ch := t.Group("/channels", chMgr, chFitur)
	ch.GET("", controllers.ListChannels)
	ch.POST("", controllers.CreateChannel)
	ch.GET("/:id", controllers.GetChannel)
	ch.PUT("/:id", controllers.UpdateChannel)
	ch.DELETE("/:id", controllers.DeleteChannel)
	ch.GET("/:id/products", controllers.ListChannelProducts)
	ch.POST("/:id/products", controllers.UpsertChannelProduct)
	ch.DELETE("/:id/products/:pid", controllers.DeleteChannelProduct)
	ch.POST("/:id/orders/import", middlewares.Require("channel.order.accept", "channel.manage"), controllers.ImportChannelOrders)
	// Sambungan API milik tenant (kredensial tenant sendiri, disimpan terenkripsi).
	ch.GET("/:id/connection", controllers.GetChannelConnection)
	ch.PUT("/:id/connection", controllers.SaveChannelConnection)
	ch.POST("/:id/connection/test", controllers.TestChannelConnection)
	ch.POST("/:id/connection/authorize", controllers.AuthorizeChannelConnection)
	ch.DELETE("/:id/connection", controllers.DeleteChannelConnection)
	ch.POST("/:id/products/match", controllers.MatchChannelProducts)
	ch.GET("/:id/stock-status", controllers.ChannelStockStatus)
	ch.GET("/:id/menu", controllers.GetChannelMenu)
	ch.PUT("/:id/menu", controllers.SaveChannelMenu)
	ch.POST("/:id/menu/publish", controllers.PublishChannelMenu)
	t.GET("/channel-providers", chMgr, controllers.ListChannelProviders)

	co := t.Group("/channel-orders", chOrd)
	co.GET("", controllers.ListChannelOrders)
	co.POST("", chFitur, controllers.CreateChannelOrder)
	co.GET("/:id", controllers.GetChannelOrder)
	co.POST("/:id/status", controllers.UpdateChannelOrderStatus)
	co.POST("/:id/cancel", controllers.CancelChannelOrder)
	co.POST("/:id/ready", controllers.MarkChannelOrderReady)

	// Pipeline peristiwa kanal (Fase 11b, §5.10, blueprint F.6/F.8). Inbox
	// peristiwa & antrean sinkron stok dijaga channel.manage; rekonsiliasi
	// pencairan cukup channel.settlement.view (peran keuangan, tanpa akses
	// kelola kanal). Pemicu pemroses manual global.
	ch.GET("/:id/events", controllers.ListChannelEvents)
	ch.GET("/:id/stock-syncs", controllers.ListChannelStockSyncs)
	t.POST("/channel-events/process", chMgr, controllers.ProcessChannelEventsNow)

	chSet := t.Group("/channels/:id/settlements", middlewares.Require("channel.settlement.view", "channel.manage"))
	chSet.GET("", controllers.ListChannelSettlements)
	chSet.POST("", controllers.RecomputeChannelSettlement)
	chSet.POST("/receipt", controllers.RecordChannelSettlementReceipt)

	// SDM & penggajian (Fase 13, §5.11). hr.salary.view adalah izin paling
	// sensitif — slip & kasbon.
	hrView := middlewares.Require("hr.employee.view", "hr.employee.edit")
	hrEdit := middlewares.Require("hr.employee.edit")
	attView := middlewares.Require("hr.attendance.view", "hr.attendance.correct")

	emp := t.Group("/employees")
	emp.GET("", hrView, controllers.ListEmployees)
	emp.POST("", hrEdit, controllers.CreateEmployee)
	emp.GET("/:id", hrView, controllers.GetEmployee)
	emp.PUT("/:id", hrEdit, controllers.UpdateEmployee)
	emp.POST("/:id/schedule", hrEdit, controllers.SetWorkSchedule)

	t.GET("/holidays", attView, controllers.ListHolidays)
	t.POST("/holidays", hrEdit, controllers.CreateHoliday)

	att := t.Group("/attendances")
	att.GET("", attView, controllers.ListAttendances)
	att.POST("", middlewares.Require("hr.attendance.view"), controllers.RecordAttendance)

	ac := t.Group("/attendance-corrections")
	ac.POST("", middlewares.Require("hr.attendance.view"), controllers.RequestCorrection)
	ac.POST("/:id/approve", middlewares.Require("hr.attendance.correct"), controllers.ApproveCorrection)

	lv := t.Group("/leave-requests")
	lv.POST("", middlewares.Require("hr.leave.request", "hr.leave.approve"), controllers.CreateLeaveRequest)
	lv.POST("/:id/approve", middlewares.Require("hr.leave.approve"), controllers.ApproveLeaveRequest)
	lv.POST("/:id/reject", middlewares.Require("hr.leave.approve"), controllers.RejectLeaveRequest)

	pr := t.Group("/payroll-rules", middlewares.Require("hr.payroll.run"))
	pr.GET("", controllers.ListPayrollRules)
	pr.POST("", controllers.CreatePayrollRule)

	pp := t.Group("/payroll-periods")
	pp.GET("", middlewares.Require("hr.payroll.run", "hr.salary.view"), controllers.ListPayrollPeriods)
	pp.POST("", middlewares.Require("hr.payroll.run"), controllers.CreatePayrollPeriod)
	pp.POST("/:id/calculate", middlewares.Require("hr.payroll.run"), controllers.CalculatePayroll)
	pp.POST("/:id/lock", middlewares.Require("hr.payroll.lock"), controllers.LockPayroll)
	pp.POST("/:id/pay", middlewares.Require("hr.payroll.pay"), controllers.PayPayroll)
	pp.GET("/:id/payslips", middlewares.Require("hr.salary.view"), controllers.ListPayslips)

	t.GET("/payslips/:id", middlewares.Require("hr.salary.view"), controllers.GetPayslip)

	adv := t.Group("/employee-advances")
	adv.GET("", middlewares.Require("hr.salary.view", "hr.advance.approve"), controllers.ListAdvances)
	adv.POST("", middlewares.Require("hr.advance.approve"), controllers.CreateAdvance)
	adv.POST("/:id/disburse", middlewares.Require("hr.advance.approve"), controllers.DisburseAdvance)

	// Stok: baca + penyesuaian manual (saldo awal / koreksi).
	t.GET("/stocks", middlewares.Require("stock.view"), controllers.ListStocks)
	t.GET("/stocks/summary", middlewares.Require("stock.view"), controllers.StockSummary)
	t.GET("/stock-movements", middlewares.Require("stock.view"), controllers.ListStockMovements)
	t.POST("/stock-adjustments", middlewares.Require("stock.adjust"), controllers.AdjustStock)
	t.GET("/stock-adjustments", middlewares.Require("stock.view"), controllers.ListStockAdjustments)
	t.POST("/stock-reconcile", middlewares.Require("stock.opname"), controllers.ReconcileStocks)

	// Pembelian (stok masuk). Wajib Idempotency-Key.
	t.POST("/purchases", middlewares.Require("stock.adjust"), controllers.ReceivePurchase)
	t.GET("/purchases", middlewares.Require("stock.view"), controllers.ListPurchases)
	t.GET("/purchases/:id", middlewares.Require("stock.view"), controllers.GetPurchase)
	// Utang pemasok (000047): bayar = izin barang masuk; dari laci juga butuh
	// cash.movement (diperiksa service).
	t.POST("/purchases/:id/payments", middlewares.Require("stock.adjust"), controllers.PayPurchase)
	t.GET("/payables/summary", middlewares.Require("stock.view"), controllers.PayablesSummary)

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
