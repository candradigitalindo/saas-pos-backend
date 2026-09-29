package controllers

import (
	"net/http"

	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// Handler laporan & dashboard (Fase 5, §8). Semua di bawah /api/v1/reports,
// dijaga permission report.view / report.profit / report.export.

// ReportDashboard: GET /api/v1/reports/dashboard?outlet_id=&date=
// Ringkasan satu hari + bulan berjalan + per kanal, dari daily_sales_summaries.
func ReportDashboard(c *gin.Context) {
	res, err := services.DashboardReport(c.Request.Context(), c.Query("outlet_id"), c.Query("date"))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.DashboardResponse]{
		Success: true, Message: "Dashboard", Data: res,
	})
}

// ReportSales: GET /api/v1/reports/sales?from=&to=&outlet_id=&group_by=
// group_by: day (default) | hour | channel | cashier | payment.
// hour mengelompokkan per jam dinding DI ZONA OUTLET (key "00".."23").
func ReportSales(c *gin.Context) {
	from, to := c.Query("from"), c.Query("to")
	if from == "" || to == "" {
		badRequest(c, "from", "Parameter from & to (YYYY-MM-DD) wajib")
		return
	}
	res, err := services.SalesReport(c.Request.Context(), c.Query("outlet_id"), from, to, c.Query("group_by"))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.SalesReportResponse]{
		Success: true, Message: "Laporan penjualan", Data: res,
	})
}

// ReportProfit: GET /api/v1/reports/profit?from=&to=&outlet_id=
// Laba bersih per kanal (§13.5). Butuh permission report.profit (harga modal).
func ReportProfit(c *gin.Context) {
	from, to := c.Query("from"), c.Query("to")
	if from == "" || to == "" {
		badRequest(c, "from", "Parameter from & to (YYYY-MM-DD) wajib")
		return
	}
	res, err := services.ProfitReport(c.Request.Context(), c.Query("outlet_id"), from, to)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.ProfitReportResponse]{
		Success: true, Message: "Laporan laba", Data: res,
	})
}

// ReportExport: GET /api/v1/reports/export?type=&format=csv&from=&to=&outlet_id=&group_by=&date=
// Fase 5 hanya mendukung format csv; xlsx/pdf ditolak 422.
func ReportExport(c *gin.Context) {
	reportType := c.Query("type")
	format := c.DefaultQuery("format", "csv")
	if format != "csv" {
		badRequest(c, "format", "Format "+format+" belum didukung pada fase ini — gunakan csv")
		return
	}
	if reportType != "dashboard" {
		if c.Query("from") == "" || c.Query("to") == "" {
			badRequest(c, "from", "Parameter from & to (YYYY-MM-DD) wajib")
			return
		}
	}

	filename, body, err := services.ExportReportCSV(
		c.Request.Context(), reportType,
		c.Query("outlet_id"), c.Query("from"), c.Query("to"), c.Query("group_by"), c.Query("date"),
	)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", body)
}

// RebuildReportSummaries: POST /api/v1/reports/rebuild-summaries?from=&to=&outlet_id=
// Menghitung ulang daily_sales_summaries dari tabel sales — koreksi drift,
// sepadan dengan /stock-reconcile.
func RebuildReportSummaries(c *gin.Context) {
	from, to := c.Query("from"), c.Query("to")
	if from == "" || to == "" {
		badRequest(c, "from", "Parameter from & to (YYYY-MM-DD) wajib")
		return
	}
	res, err := services.RebuildDailySummaries(c.Request.Context(), c.Query("outlet_id"), from, to)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.RebuildSummariesResponse]{
		Success: true, Message: "Rebuild ringkasan selesai", Data: res,
	})
}

// ReportPurchases: GET /api/v1/reports/purchases?from=&to=&outlet_id=
// Laporan belanja & utang pemasok. Butuh report.view DAN stock.view (nilai
// pembelian).
func ReportPurchases(c *gin.Context) {
	from, to := c.Query("from"), c.Query("to")
	if from == "" || to == "" {
		badRequest(c, "from", "Parameter from & to (YYYY-MM-DD) wajib")
		return
	}
	res, err := services.PurchaseReport(c.Request.Context(), c.Query("outlet_id"), from, to)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PurchaseReportResponse]{
		Success: true, Message: "Laporan belanja", Data: res,
	})
}
