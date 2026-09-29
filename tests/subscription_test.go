package tests

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"candra/backend-api/database"
	"candra/backend-api/internal/ulid"
	"candra/backend-api/services"
)

// Uji integrasi Fase 7 — langganan & tagihan platform (§5.13, §13.4).

// startBasic12 memulai langganan Basic 12 bulan lalu menerbitkan tagihannya.
func startBasic12(t *testing.T, f tenantFixture) map[string]any {
	t.Helper()
	call(t, "POST", "/api/v1/subscription", f.token, map[string]any{
		"plan_code": "basic", "term_months": 12,
	}).mustCode(t, "start subscription", 201)
	return call(t, "POST", "/api/v1/subscription/invoices", f.token, nil).
		mustCode(t, "generate invoice", 201).data(t)
}

// kirimKonfirmasi: tenant mengonfirmasi pembayaran tagihan (Idempotency-Key).
func kirimKonfirmasi(t *testing.T, token, key, invoiceID string, amount int64) apiResp {
	t.Helper()
	req := jsonRequest(t, "POST", "/api/v1/subscription-payment-claims", token, map[string]any{
		"invoice_id": invoiceID, "amount": amount, "method": "transfer", "reference": "Transfer uji",
	})
	req.Header.Set("Idempotency-Key", key)
	return serve(t, req)
}

var (
	sekaliKeuangan sync.Once
	tokenKeuangan  string
)

// berhentiDengan: body POST /subscription/cancel lengkap dengan rekening
// tujuan pengembalian dana (wajib bila ada uang yang dikembalikan).
func berhentiDengan(alasan string) map[string]any {
	return map[string]any{
		"reason": alasan, "refund_bank": "BCA", "refund_account": "1234567890", "refund_holder": "Pemilik Uji",
	}
}

// tokenKeuanganPlatform: token staf keuangan panel (billing.verify), dibuat
// sekali per jalannya tes.
func tokenKeuanganPlatform(t *testing.T) string {
	t.Helper()
	sekaliKeuangan.Do(func() {
		_, email, pass := makePlatformAdmin(t, "keuangan-uji-"+strings.ToLower(ulid.New()), "finance")
		tokenKeuangan = platformToken(t, email, pass)
	})
	return tokenKeuangan
}

// paySub membayar sebuah tagihan lewat JALUR SUNGGUHAN: tenant mengirim
// konfirmasi, staf keuangan platform menyetujuinya. Mengembalikan balasan
// persetujuan (tagihan sesudahnya, 201) — atau balasan konfirmasi bila
// konfirmasinya sendiri sudah ditolak (mis. 422 melebihi sisa, 404 tagihan
// tenant lain).
func paySub(t *testing.T, token, key, invoiceID string, amount int64) apiResp {
	t.Helper()
	klaim := kirimKonfirmasi(t, token, key, invoiceID, amount)
	if klaim.Code != 201 {
		return klaim
	}
	id := klaim.data(t)["id"].(string)
	return call(t, "POST", "/api/v1/platform/subscription-payment-claims/"+id+"/approve", tokenKeuanganPlatform(t), nil)
}

// TestSubscriptionPrepaidCancelRefund — DoD Fase 7: daftar → trial → bayar 12
// bulan → batal setelah 5 bulan; pengembalian dana = hitungan tangan pada harga
// bulanan NORMAL.
func TestSubscriptionPrepaidCancelRefund(t *testing.T) {
	requireDB(t)
	f := registerTenantPolos(t, "sub12")

	inv := startBasic12(t, f)
	// Basic Rp79.000/bln, 12 bln, diskon 16,7%.
	//   gross    = 79000 * 12                 = 948000
	//   discount = round(948000 * 0.167)      = 158316
	//   total    = 948000 - 158316            = 789684
	assertI64(t, inv, "gross_amount", 948000)
	assertI64(t, inv, "discount_amount", 158316)
	assertI64(t, inv, "total_amount", 789684)
	invID := inv["id"].(string)

	paid := paySub(t, f.token, "PAY-"+ulid.New(), invID, 789684).
		mustCode(t, "pay invoice", 201).data(t)
	if paid["status"] != "paid" {
		t.Fatalf("status tagihan = %v, mau paid", paid["status"])
	}

	// Langganan aktif, 12 baris pengakuan berjumlah = total.
	ov := call(t, "GET", "/api/v1/subscription", f.token, nil).mustOK(t, "get sub").data(t)
	if ov["subscription"].(map[string]any)["status"] != "active" {
		t.Fatalf("status langganan = %v, mau active", ov["subscription"].(map[string]any)["status"])
	}
	var cnt int
	var sum int64
	database.DB.Raw(`SELECT count(*), COALESCE(SUM(amount),0) FROM deferred_revenue_entries WHERE subscription_invoice_id = ?`, invID).
		Row().Scan(&cnt, &sum)
	if cnt != 12 || sum != 789684 {
		t.Fatalf("deferred = %d baris jumlah %d, mau 12 baris jumlah 789684", cnt, sum)
	}

	// Mundurkan periode: mulai 5 bulan lalu (tanggal 1) → 5 bulan penuh terpakai.
	t0 := time.Now().UTC()
	periodStart := time.Date(t0.Year(), t0.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -5, 0)
	if err := database.DB.Exec(
		`UPDATE subscription_invoices SET period_start = ? WHERE id = ?`, periodStart, invID,
	).Error; err != nil {
		t.Fatalf("backdate invoice: %v", err)
	}

	res := call(t, "POST", "/api/v1/subscription/cancel", f.token, berhentiDengan("tutup usaha")).
		mustOK(t, "cancel").data(t)
	// refund = 789684 - 5 * 79000 = 394684 ; earned = 395000
	assertI64(t, res, "months_used", 5)
	assertI64(t, res, "refund_amount", 394684)
	assertI64(t, res, "earned_amount", 395000)
	if res["subscription"].(map[string]any)["status"] != "canceled" {
		t.Fatalf("status setelah batal = %v, mau canceled", res["subscription"].(map[string]any)["status"])
	}

	// Jejak audit + tagihan + status tenant.
	var refundAmt int64
	database.DB.Raw(`SELECT amount FROM subscription_refunds WHERE subscription_invoice_id = ?`, invID).Row().Scan(&refundAmt)
	if refundAmt != 394684 {
		t.Fatalf("subscription_refunds.amount = %d, mau 394684", refundAmt)
	}
	var invStatus string
	var invPaid int64
	database.DB.Raw(`SELECT status, paid_amount FROM subscription_invoices WHERE id = ?`, invID).Row().Scan(&invStatus, &invPaid)
	if invStatus != "refunded" || invPaid != 395000 {
		t.Fatalf("tagihan setelah batal: status=%s paid=%d, mau refunded/395000", invStatus, invPaid)
	}
	var tenantStatus string
	database.DB.Raw(`SELECT status FROM tenants WHERE id = ?`, f.tenantID).Row().Scan(&tenantStatus)
	if tenantStatus != "closed" {
		t.Fatalf("status tenant = %s, mau closed", tenantStatus)
	}

	// Total yang diakui = uang yang jadi hak platform (dibayar - refund).
	var recognized int64
	database.DB.Raw(`SELECT COALESCE(SUM(amount),0) FROM deferred_revenue_entries WHERE subscription_invoice_id = ? AND recognized_at IS NOT NULL`, invID).
		Row().Scan(&recognized)
	if recognized != 395000 {
		t.Fatalf("pendapatan diakui setelah batal = %d, mau 395000 (= dibayar - refund)", recognized)
	}
}

// TestSubscriptionPaymentIdempotent — kunci sama → replay, satu baris pembayaran.
func TestSubscriptionPaymentIdempotent(t *testing.T) {
	requireDB(t)
	f := registerTenantPolos(t, "subidem")
	invID := startBasic12(t, f)["id"].(string)

	// Konfirmasi yang dikirim ulang dengan kunci sama → konfirmasi yang SAMA.
	key := "K-" + ulid.New()
	a := kirimKonfirmasi(t, f.token, key, invID, 789684).mustCode(t, "konfirmasi #1", 201).data(t)
	b := kirimKonfirmasi(t, f.token, key, invID, 789684).mustCode(t, "konfirmasi #2 (replay)", 201).data(t)
	if a["id"] != b["id"] {
		t.Fatalf("replay membuat konfirmasi baru: %v vs %v", a["id"], b["id"])
	}
	// Kunci sama, body beda → 409.
	req := jsonRequest(t, "POST", "/api/v1/subscription-payment-claims", f.token, map[string]any{
		"invoice_id": invID, "amount": 1000, "method": "cash", "reference": "lain",
	})
	req.Header.Set("Idempotency-Key", key)
	serve(t, req).mustCode(t, "kunci sama body beda", 409)
	// Konfirmasi kedua (kunci lain) selagi yang pertama menunggu → 409.
	kirimKonfirmasi(t, f.token, "K2-"+ulid.New(), invID, 789684).mustCode(t, "konfirmasi ganda", 409)

	// Disetujui sekali → lunas; disetujui lagi → 409, pembayaran TIDAK dobel.
	setujui := func() apiResp {
		return call(t, "POST", "/api/v1/platform/subscription-payment-claims/"+a["id"].(string)+"/approve",
			tokenKeuanganPlatform(t), nil)
	}
	setujui().mustCode(t, "setujui", 201)
	setujui().mustCode(t, "setujui lagi", 409)

	var payCount int
	var invPaid int64
	database.DB.Raw(`SELECT count(*) FROM subscription_payments WHERE subscription_invoice_id = ?`, invID).Row().Scan(&payCount)
	database.DB.Raw(`SELECT paid_amount FROM subscription_invoices WHERE id = ?`, invID).Row().Scan(&invPaid)
	if payCount != 1 || invPaid != 789684 {
		t.Fatalf("setelah dua persetujuan: %d pembayaran, paid_amount %d; mau 1 & 789684", payCount, invPaid)
	}
}

// TestSubscriptionPartialPayment — pembayaran sebagian lalu pelunasan.
func TestSubscriptionPartialPayment(t *testing.T) {
	requireDB(t)
	f := registerTenantPolos(t, "subpartial")
	invID := startBasic12(t, f)["id"].(string)

	half := paySub(t, f.token, "H1-"+ulid.New(), invID, 400000).mustCode(t, "bayar sebagian", 201).data(t)
	if half["status"] != "open" {
		t.Fatalf("status setelah bayar sebagian = %v, mau open", half["status"])
	}
	assertI64(t, half, "paid_amount", 400000)

	// Belum ada pengakuan sebelum lunas.
	var early int
	database.DB.Raw(`SELECT count(*) FROM deferred_revenue_entries WHERE subscription_invoice_id = ?`, invID).Row().Scan(&early)
	if early != 0 {
		t.Fatalf("ada %d baris deferred sebelum lunas, mau 0", early)
	}

	// Melebihi sisa → 422.
	paySub(t, f.token, "OV-"+ulid.New(), invID, 500000).mustCode(t, "melebihi sisa", 422)

	full := paySub(t, f.token, "H2-"+ulid.New(), invID, 389684).mustCode(t, "pelunasan", 201).data(t)
	if full["status"] != "paid" {
		t.Fatalf("status setelah pelunasan = %v, mau paid", full["status"])
	}
	var n int
	database.DB.Raw(`SELECT count(*) FROM deferred_revenue_entries WHERE subscription_invoice_id = ?`, invID).Row().Scan(&n)
	if n != 12 {
		t.Fatalf("deferred setelah lunas = %d, mau 12", n)
	}
}

// TestDeferredRemainderInLastMonth — sisa pembulatan pembagian jatuh di bulan
// terakhir agar totalnya persis.
func TestDeferredRemainderInLastMonth(t *testing.T) {
	requireDB(t)
	f := registerTenantPolos(t, "subremain")

	// Harga ganjil supaya total tak habis dibagi 12.
	if err := database.DB.Exec(`UPDATE plans SET monthly_price = 79001 WHERE code = 'basic'`).Error; err != nil {
		t.Fatalf("set harga: %v", err)
	}
	defer database.DB.Exec(`UPDATE plans SET monthly_price = 79000 WHERE code = 'basic'`)

	inv := startBasic12(t, f)
	invID := inv["id"].(string)
	total := int64(inv["total_amount"].(float64))
	paySub(t, f.token, "R-"+ulid.New(), invID, total).mustCode(t, "bayar", 201)

	type row struct {
		Amount int64
	}
	var amounts []int64
	database.DB.Raw(`SELECT amount FROM deferred_revenue_entries WHERE subscription_invoice_id = ? ORDER BY recognition_month`, invID).Scan(&amounts)
	if len(amounts) != 12 {
		t.Fatalf("deferred = %d baris, mau 12", len(amounts))
	}
	var sum int64
	for _, a := range amounts {
		sum += a
	}
	if sum != total {
		t.Fatalf("jumlah deferred %d != total tagihan %d", sum, total)
	}
	if amounts[11] <= amounts[0] {
		t.Fatalf("bulan terakhir (%d) mestinya menampung sisa, > bulan lain (%d)", amounts[11], amounts[0])
	}
}

// TestRevenueRecognitionJob — pengakuan hanya menyentuh bulan yang sudah tiba.
func TestRevenueRecognitionJob(t *testing.T) {
	requireDB(t)
	f := registerTenantPolos(t, "subrecog")

	// Basic 3 bulan.
	call(t, "POST", "/api/v1/subscription", f.token, map[string]any{"plan_code": "basic", "term_months": 3}).
		mustCode(t, "start", 201)
	// Masa coba dihabiskan dulu: dibayar di tengah masa coba, masa berbayarnya
	// mulai saat masa coba berakhir — bisa jatuh di bulan depan, dan uji ini
	// butuh bulan pertama = bulan berjalan.
	lewat := time.Now().UTC().Add(-time.Hour)
	aturLangganan(t, f.tenantID, map[string]any{"trial_ends_at": lewat, "current_period_end": lewat})
	invID := call(t, "POST", "/api/v1/subscription/invoices", f.token, nil).mustCode(t, "invoice", 201).data(t)["id"].(string)
	total := int64(call(t, "GET", "/api/v1/subscription", f.token, nil).mustOK(t, "ov").data(t)["open_invoice"].(map[string]any)["total_amount"].(float64))
	paySub(t, f.token, "RJ-"+ulid.New(), invID, total).mustCode(t, "bayar", 201)

	// Bulan berjalan diakui; dua bulan berikutnya belum.
	if _, err := services.RecognizeDueRevenue(context.Background()); err != nil {
		t.Fatalf("recognize: %v", err)
	}
	var recognized, pending int
	database.DB.Raw(`SELECT count(*) FROM deferred_revenue_entries WHERE subscription_invoice_id = ? AND recognized_at IS NOT NULL`, invID).Row().Scan(&recognized)
	database.DB.Raw(`SELECT count(*) FROM deferred_revenue_entries WHERE subscription_invoice_id = ? AND recognized_at IS NULL`, invID).Row().Scan(&pending)
	if recognized != 1 || pending != 2 {
		t.Fatalf("setelah job pertama: %d diakui / %d menunggu, mau 1/2", recognized, pending)
	}

	// Mundurkan satu baris menunggu ke bulan lalu → job berikutnya mengakuinya.
	database.DB.Exec(`
		UPDATE deferred_revenue_entries SET recognition_month = date_trunc('month', now())::date - interval '1 month'
		WHERE id = (SELECT id FROM deferred_revenue_entries
		            WHERE subscription_invoice_id = ? AND recognized_at IS NULL
		            ORDER BY recognition_month DESC LIMIT 1)`, invID)
	if _, err := services.RecognizeDueRevenue(context.Background()); err != nil {
		t.Fatalf("recognize #2: %v", err)
	}
	database.DB.Raw(`SELECT count(*) FROM deferred_revenue_entries WHERE subscription_invoice_id = ? AND recognized_at IS NOT NULL`, invID).Row().Scan(&recognized)
	if recognized != 2 {
		t.Fatalf("setelah job kedua: %d diakui, mau 2", recognized)
	}
}

// TestSubscriptionChangePlanProration — ganti paket memberi kredit prorata atas
// sisa masa paket lama (harga bulanan normal).
func TestSubscriptionChangePlanProration(t *testing.T) {
	requireDB(t)
	f := registerTenantPolos(t, "subchange")

	// Basic 3 bulan, bayar lunas.
	call(t, "POST", "/api/v1/subscription", f.token, map[string]any{"plan_code": "basic", "term_months": 3}).
		mustCode(t, "start", 201)
	inv := call(t, "POST", "/api/v1/subscription/invoices", f.token, nil).mustCode(t, "invoice", 201).data(t)
	invID := inv["id"].(string)
	paySub(t, f.token, "C-"+ulid.New(), invID, int64(inv["total_amount"].(float64))).mustCode(t, "bayar", 201)

	// 1 bulan penuh terpakai → sisa 2 bulan → kredit 2 * 79000 = 158000.
	t0 := time.Now().UTC()
	ps := time.Date(t0.Year(), t0.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
	database.DB.Exec(`UPDATE subscription_invoices SET period_start = ? WHERE id = ?`, ps, invID)
	database.DB.Exec(`UPDATE subscriptions SET current_period_start = ? WHERE tenant_id = ?`, ps, f.tenantID)

	newInv := call(t, "POST", "/api/v1/subscription/change-plan", f.token, map[string]any{
		"plan_code": "pro", "term_months": 3,
	}).mustCode(t, "change plan", 201).data(t)

	// Pro 3 bln: gross 199000*3 = 597000 ; diskon masa = round(597000*0.05) = 29850.
	// discount_amount = 29850 + kredit 158000 = 187850.
	assertI64(t, newInv, "gross_amount", 597000)
	assertI64(t, newInv, "discount_amount", 187850)
	assertI64(t, newInv, "total_amount", 409150)
}

// TestSubscriptionIsolation — tenant B tak melihat / tak bisa membayar tagihan A.
func TestSubscriptionIsolation(t *testing.T) {
	requireDB(t)
	a := registerTenantPolos(t, "subisoA")
	b := registerTenantPolos(t, "subisoB")

	invA := startBasic12(t, a)["id"].(string)

	lb := call(t, "GET", "/api/v1/subscription/invoices", b.token, nil).mustOK(t, "list B").data(t)
	if len(lb["data"].([]any)) != 0 {
		t.Fatalf("tenant B melihat %d tagihan, mau 0", len(lb["data"].([]any)))
	}
	paySub(t, b.token, "X-"+ulid.New(), invA, 789684).mustCode(t, "B bayar tagihan A", 404)
}

// TestSubscriptionPermission — endpoint langganan butuh billing.manage; katalog
// paket cukup terautentikasi.
func TestSubscriptionPermission(t *testing.T) {
	requireDB(t)
	f := registerTenantPolos(t, "subperm")
	kasir := staffToken(t, f, roleID(t, f, "Kasir"), "kasir_subperm")

	call(t, "GET", "/api/v1/plans", kasir, nil).mustOK(t, "kasir lihat paket")
	call(t, "GET", "/api/v1/subscription", kasir, nil).mustCode(t, "kasir lihat langganan", 403)
	call(t, "POST", "/api/v1/subscription", kasir, map[string]any{"plan_code": "basic", "term_months": 1}).
		mustCode(t, "kasir mulai langganan", 403)
}

// TestPlansCatalog — katalog paket ter-seed dengan tabel harga per masa.
func TestPlansCatalog(t *testing.T) {
	requireDB(t)
	f := registerTenantPolos(t, "subplans")

	resp := call(t, "GET", "/api/v1/plans", f.token, nil).mustOK(t, "plans")
	plans, ok := resp.Body["data"].([]any)
	if !ok {
		t.Fatalf("data paket bukan array: %s", resp.Raw)
	}
	byCode := map[string]map[string]any{}
	for _, p := range plans {
		m := p.(map[string]any)
		byCode[m["code"].(string)] = m
	}
	if byCode["basic"] == nil || int64(byCode["basic"]["monthly_price"].(float64)) != 79000 {
		t.Fatalf("paket basic tidak ter-seed benar: %v", byCode["basic"])
	}
	if byCode["pro"] == nil || int64(byCode["pro"]["monthly_price"].(float64)) != 199000 {
		t.Fatalf("paket pro tidak ter-seed benar: %v", byCode["pro"])
	}
	tp := byCode["basic"]["term_prices"].([]any)
	if len(tp) != 5 {
		t.Fatalf("term_prices basic = %d, mau 5", len(tp))
	}
	last := tp[4].(map[string]any)
	if int64(last["term_months"].(float64)) != 12 || int64(last["total_amount"].(float64)) != 789684 {
		t.Fatalf("harga 12 bulan basic salah: %v", last)
	}
}
