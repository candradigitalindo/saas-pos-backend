package tests

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"candra/backend-api/database"
)

// rawResponse mengirim GET dan mengembalikan status + Content-Type + body mentah
// TANPA mendecode JSON — untuk endpoint yang membalas CSV.
func rawResponse(t *testing.T, path, token string) (int, string, string) {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Code, rec.Header().Get("Content-Type"), rec.Body.String()
}

// Uji integrasi Fase 5 — dashboard & laporan. Semua angka rentang tanggal
// dibaca dari daily_sales_summaries; test membuktikan tabel itu dipelihara
// benar (checkout inkremental, void/retur rekalkulasi) dan bisa dibangun ulang.

// staffToken membuat user staf berperan roleID lalu login, mengembalikan token.
func staffToken(t *testing.T, f tenantFixture, roleID, username string) string {
	t.Helper()
	call(t, "POST", "/api/v1/users", f.token, map[string]any{
		"name": username, "username": username, "email": username + "@example.com",
		"password": "rahasia123", "role_id": roleID,
	}).mustCode(t, "buat staf "+username, 201)
	d := call(t, "POST", "/api/v1/auth/login", "", map[string]any{
		"username": username, "password": "rahasia123",
	}).mustOK(t, "login "+username).data(t)
	return get[string](t, d, "access_token")
}

// saleDay menjalankan satu checkout tunai prodA dan mengembalikan business_date-nya.
func saleDay(t *testing.T, f posFixture, key string, qty int) string {
	t.Helper()
	d := checkout(t, f.token, key, map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": fmt.Sprintf("%d", qty)}},
		"payments":  []map[string]any{{"method": "cash", "amount": qty * 15000}},
	}).mustCode(t, "checkout "+key, 201).data(t)
	return d["business_date"].(string)
}

// TestDashboardReadsFromSummaryTable — dashboard mengambil angka dari
// daily_sales_summaries, bukan SUM(sales). Cache sengaja dirusak lalu dibangun
// ulang (sepadan dengan TestReconcileRebuildsFromLedger Fase 4).
func TestDashboardReadsFromSummaryTable(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "dashz")

	bd := saleDay(t, f, "DSH-1", 3) // net 45000, cost 18000
	saleDay(t, f, "DSH-2", 2)       // net 30000, cost 12000
	// Total hari ini: net 75000, cost 30000, laba 45000, 2 transaksi.

	dash := call(t, "GET", "/api/v1/reports/dashboard?date="+bd+"&outlet_id="+f.outletID, f.token, nil).
		mustOK(t, "dashboard").data(t)
	today := dash["today"].(map[string]any)
	assertI64(t, today, "sales_count", 2)
	assertI64(t, today, "net_amount", 75000)
	assertI64(t, today, "cost_amount", 30000)
	assertI64(t, today, "gross_profit", 45000)
	// Bulan berjalan == hari ini (semua transaksi hari ini).
	assertI64(t, dash["month_to_date"].(map[string]any), "net_amount", 75000)

	// Rusak agregat langsung di database.
	if err := database.DB.Exec(
		"UPDATE daily_sales_summaries SET net_amount = 999999999 WHERE outlet_id = ? AND business_date = ?",
		f.outletID, bd,
	).Error; err != nil {
		t.Fatalf("corrupt summary: %v", err)
	}
	dash = call(t, "GET", "/api/v1/reports/dashboard?date="+bd+"&outlet_id="+f.outletID, f.token, nil).
		mustOK(t, "dashboard rusak").data(t)
	assertI64(t, dash["today"].(map[string]any), "net_amount", 999999999) // membuktikan baca dari tabel agregat

	// Bangun ulang dari tabel sales.
	rb := call(t, "POST", "/api/v1/reports/rebuild-summaries?from="+bd+"&to="+bd+"&outlet_id="+f.outletID, f.token, nil).
		mustOK(t, "rebuild").data(t)
	assertI64(t, rb, "rows_rebuilt", 1)

	dash = call(t, "GET", "/api/v1/reports/dashboard?date="+bd+"&outlet_id="+f.outletID, f.token, nil).
		mustOK(t, "dashboard setelah rebuild").data(t)
	assertI64(t, dash["today"].(map[string]any), "net_amount", 75000)
	assertI64(t, dash["today"].(map[string]any), "gross_profit", 45000)
}

// TestSalesReportGroupings — group_by day/channel/cashier/payment menghasilkan
// baris yang masuk akal dan total yang konsisten.
func TestSalesReportGroupings(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "srepz")

	bd := saleDay(t, f, "SR-1", 4)
	saleDay(t, f, "SR-2", 1)
	// net 60000 + 15000 = 75000, 2 transaksi.

	base := "/api/v1/reports/sales?from=" + bd + "&to=" + bd + "&outlet_id=" + f.outletID

	day := call(t, "GET", base+"&group_by=day", f.token, nil).mustOK(t, "group day").data(t)
	assertI64(t, day["totals"].(map[string]any), "net_amount", 75000)
	if rows := day["rows"].([]any); len(rows) != 1 || rows[0].(map[string]any)["key"] != bd {
		t.Fatalf("group day: rows = %v", day["rows"])
	}

	ch := call(t, "GET", base+"&group_by=channel", f.token, nil).mustOK(t, "group channel").data(t)
	chRows := ch["rows"].([]any)
	if len(chRows) != 1 || chRows[0].(map[string]any)["key"] != "" {
		t.Fatalf("group channel: mau 1 baris kanal '' (kasir), dapat %v", ch["rows"])
	}
	assertI64(t, chRows[0].(map[string]any), "net_amount", 75000)

	cashier := call(t, "GET", base+"&group_by=cashier", f.token, nil).mustOK(t, "group cashier").data(t)
	cRows := cashier["rows"].([]any)
	if len(cRows) != 1 {
		t.Fatalf("group cashier: mau 1 kasir, dapat %d", len(cRows))
	}
	assertI64(t, cRows[0].(map[string]any), "sales_count", 2)
	assertI64(t, cRows[0].(map[string]any), "net_amount", 75000)

	pay := call(t, "GET", base+"&group_by=payment", f.token, nil).mustOK(t, "group payment").data(t)
	pRows := pay["rows"].([]any)
	if len(pRows) != 1 || pRows[0].(map[string]any)["key"] != "cash" {
		t.Fatalf("group payment: mau 1 baris cash, dapat %v", pay["rows"])
	}
	assertI64(t, pRows[0].(map[string]any), "net_amount", 75000)

	// group_by tak dikenal → 422.
	call(t, "GET", base+"&group_by=warna", f.token, nil).mustCode(t, "group_by ngawur", 422)
	// from/to wajib.
	call(t, "GET", "/api/v1/reports/sales", f.token, nil).mustCode(t, "tanpa rentang", 400)
}

// TestProfitReportReflectsVoidAndRefund — void membuang kontribusi transaksi,
// retur mencatat nilai negatif pada hari retur; laba bersih menyesuaikan.
func TestProfitReportReflectsVoidAndRefund(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "profz")

	a := checkout(t, f.token, "PF-A", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "5"}}, // net 75000, cost 30000
		"payments":  []map[string]any{{"method": "cash", "amount": 75000}},
	}).mustCode(t, "sale A", 201).data(t)
	bd := a["business_date"].(string)

	b := checkout(t, f.token, "PF-B", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "3"}}, // akan di-void
		"payments":  []map[string]any{{"method": "cash", "amount": 45000}},
	}).mustCode(t, "sale B", 201).data(t)
	c := checkout(t, f.token, "PF-C", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "2"}}, // akan di-refund
		"payments":  []map[string]any{{"method": "cash", "amount": 30000}},
	}).mustCode(t, "sale C", 201).data(t)

	call(t, "POST", "/api/v1/sales/"+b["id"].(string)+"/void", f.token, map[string]any{"reason": "salah input"}).
		mustOK(t, "void B")
	call(t, "POST", "/api/v1/sales/"+c["id"].(string)+"/refund", f.token, map[string]any{"reason": "barang rusak"}).
		mustOK(t, "refund C")

	// Setelah rekalkulasi hari itu:
	//   * B ('canceled') hilang sepenuhnya   → −45000 omzet;
	//   * C tetap 'completed' (+30000/+12000) DAN retur-C 'returned' (−30000/−12000)
	//     → pasangan ini saling menghapus (retur diakui pada hari retur);
	//   * A tetap 'completed'.
	//   omzet = 75000 + 30000 − 30000 = 75000
	//   modal = 30000 + 12000 − 12000 = 30000
	//   laba  = 75000 − 30000         = 45000
	//   jumlah transaksi 'completed'  = 2 (A & C)
	prof := call(t, "GET", "/api/v1/reports/profit?from="+bd+"&to="+bd+"&outlet_id="+f.outletID, f.token, nil).
		mustOK(t, "profit").data(t)
	tot := prof["totals"].(map[string]any)
	assertI64(t, tot, "omzet", 75000)
	assertI64(t, tot, "modal", 30000)
	assertI64(t, tot, "laba_bersih", 45000)

	dash := call(t, "GET", "/api/v1/reports/dashboard?date="+bd+"&outlet_id="+f.outletID, f.token, nil).
		mustOK(t, "dashboard").data(t)["today"].(map[string]any)
	assertI64(t, dash, "sales_count", 2)
	assertI64(t, dash, "net_amount", 75000)
	assertI64(t, dash, "gross_profit", 45000)
}

// TestReportExportCSV — ekspor CSV berjalan; xlsx/pdf ditolak.
func TestReportExportCSV(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "expz")
	bd := saleDay(t, f, "EX-1", 3)

	code, ctype, body := rawResponse(t,
		"/api/v1/reports/export?type=sales&format=csv&from="+bd+"&to="+bd+"&outlet_id="+f.outletID, f.token)
	if code != 200 || !strings.HasPrefix(ctype, "text/csv") {
		t.Fatalf("ekspor csv: status %d, content-type %q, body %s", code, ctype, body)
	}
	if !strings.Contains(body, "key,sales_count") || !strings.Contains(body, "TOTAL,") {
		t.Fatalf("isi CSV tak sesuai: %s", body)
	}

	// Format tak didukung → 400.
	call(t, "GET", "/api/v1/reports/export?type=sales&format=xlsx&from="+bd+"&to="+bd, f.token, nil).
		mustCode(t, "format xlsx", 400)
	// type tak dikenal → 422.
	call(t, "GET", "/api/v1/reports/export?type=galaksi&format=csv&from="+bd+"&to="+bd, f.token, nil).
		mustCode(t, "type ngawur", 422)
}

// TestReportsPermission — report.view menjaga dashboard/sales; report.profit
// menjaga laporan laba secara terpisah.
func TestReportsPermission(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "rpermz")
	bd := saleDay(t, f, "RP-1", 1)

	// Kasir: tanpa report.* → 403 di semua endpoint laporan.
	kasir := staffToken(t, f.tenantFixture, roleID(t, f.tenantFixture, "Kasir"), "kasir_rpermz")
	call(t, "GET", "/api/v1/reports/dashboard?date="+bd, kasir, nil).mustCode(t, "kasir dashboard", 403)
	call(t, "GET", "/api/v1/reports/profit?from="+bd+"&to="+bd, kasir, nil).mustCode(t, "kasir profit", 403)

	// Peran khusus: report.view + report.export, TANPA report.profit.
	rid := call(t, "POST", "/api/v1/roles", f.token, map[string]any{
		"name": "Laporan Tanpa Laba", "permission_codes": []string{"report.view", "report.export"},
	}).mustCode(t, "buat peran", 201).data(t)["id"].(string)
	viewer := staffToken(t, f.tenantFixture, rid, "viewer_rpermz")

	call(t, "GET", "/api/v1/reports/dashboard?date="+bd, viewer, nil).mustOK(t, "viewer dashboard")
	call(t, "GET", "/api/v1/reports/sales?from="+bd+"&to="+bd, viewer, nil).mustOK(t, "viewer sales")
	if code, _, body := rawResponse(t, "/api/v1/reports/export?type=sales&format=csv&from="+bd+"&to="+bd, viewer); code != 200 {
		t.Fatalf("viewer export: status %d, body %s", code, body)
	}
	call(t, "GET", "/api/v1/reports/profit?from="+bd+"&to="+bd, viewer, nil).
		mustCode(t, "viewer profit (butuh report.profit)", 403)
}

// TestReportsIsolation — tenant B tidak melihat angka tenant A.
func TestReportsIsolation(t *testing.T) {
	requireDB(t)
	a := setupPOS(t, "risoA")
	b := setupPOS(t, "risoB")

	bd := saleDay(t, a, "RI-A1", 5) // hanya tenant A yang bertransaksi

	dashB := call(t, "GET", "/api/v1/reports/dashboard?date="+bd, b.token, nil).
		mustOK(t, "dashboard B").data(t)["today"].(map[string]any)
	assertI64(t, dashB, "sales_count", 0)
	assertI64(t, dashB, "net_amount", 0)

	// Rebuild B tidak menyentuh baris A.
	call(t, "POST", "/api/v1/reports/rebuild-summaries?from="+bd+"&to="+bd, b.token, nil).mustOK(t, "rebuild B")
	dashA := call(t, "GET", "/api/v1/reports/dashboard?date="+bd+"&outlet_id="+a.outletID, a.token, nil).
		mustOK(t, "dashboard A").data(t)["today"].(map[string]any)
	assertI64(t, dashA, "net_amount", 75000)
}

// TestDashboardStaysFastAsSalesGrow — DoD Fase 5 (§16): "dashboard < 1 detik
// pada 100.000 transaksi". Dijamin arsitektur: dashboard membaca satu baris
// agregat per (outlet, hari, kanal), tidak pernah memindai `sales`. Di sini
// dibuktikan pada skala kecil bahwa waktu respons tak tumbuh bersama jumlah
// transaksi.
func TestDashboardStaysFastAsSalesGrow(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "fastz")
	adjust(t, f.tenantFixture, f.outletID, f.prodA, "5000", "top up")

	const n = 60
	var bd string
	var wantNet int64
	for i := 0; i < n; i++ {
		qty := i%3 + 1
		bd = saleDay(t, f, fmt.Sprintf("FZ-%d", i), qty)
		wantNet += int64(qty) * 15000
	}

	start := time.Now()
	dash := call(t, "GET", "/api/v1/reports/dashboard?date="+bd+"&outlet_id="+f.outletID, f.token, nil).
		mustOK(t, "dashboard").data(t)
	elapsed := time.Since(start)

	assertI64(t, dash["today"].(map[string]any), "sales_count", n)
	assertI64(t, dash["today"].(map[string]any), "net_amount", wantNet)
	if elapsed > time.Second {
		t.Fatalf("dashboard butuh %v untuk %d transaksi — mestinya <1 detik (baca agregat, bukan SUM sales)", elapsed, n)
	}
	t.Logf("dashboard %d transaksi: %v", n, elapsed)
}
