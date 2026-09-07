package tests

import (
	"fmt"
	"testing"
	"time"

	"candra/backend-api/internal/ulid"
)

// Uji integrasi Fase 9 — CRM tenant (§5.9, blueprint Bagian E).

// makeCustomer membuat pelanggan dan mengembalikan id-nya.
func makeCustomer(t *testing.T, token, name string) string {
	t.Helper()
	return call(t, "POST", "/api/v1/customers", token, map[string]any{"name": name}).
		mustCode(t, "buat pelanggan "+name, 201).data(t)["id"].(string)
}

// TestCRMFreelanceCycleToOmzet — DoD Fase 9: prospek → penawaran → DP →
// pelunasan, dan angkanya muncul di laporan omzet yang SAMA dengan POS, tanpa
// entri ganda.
func TestCRMFreelanceCycleToOmzet(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "crmdod")
	cust := makeCustomer(t, f.token, "PT Klien Web")

	// 1. Prospek (deal).
	deal := call(t, "POST", "/api/v1/deals", f.token, map[string]any{
		"title": "Website Toko", "value": 5000000, "customer_id": cust,
	}).mustCode(t, "buat deal", 201).data(t)

	// 2. Penawaran.
	quo := call(t, "POST", "/api/v1/quotations", f.token, map[string]any{
		"customer_id": cust, "deal_id": deal["id"],
		"items": []map[string]any{
			{"description": "Desain & bangun website", "qty": "1", "unit_price": 5000000},
		},
	}).mustCode(t, "buat penawaran", 201).data(t)
	assertI64(t, quo, "total", 5000000)
	quoID := quo["id"].(string)

	call(t, "POST", "/api/v1/quotations/"+quoID+"/send", f.token, nil).mustOK(t, "kirim penawaran")

	// 3. Setujui → proyek otomatis.
	acc := call(t, "POST", "/api/v1/quotations/"+quoID+"/accept", f.token, nil).mustOK(t, "setujui").data(t)
	projID := acc["project_id"].(string)
	if projID == "" {
		t.Fatal("accept tidak membuat proyek")
	}
	proj := call(t, "GET", "/api/v1/projects/"+projID, f.token, nil).mustOK(t, "get proyek").data(t)
	assertI64(t, proj, "contract_value", 5000000)

	// 4. Biaya proyek → jadi modal (cost) saat invoice lunas.
	call(t, "POST", "/api/v1/projects/"+projID+"/expenses", f.token, map[string]any{
		"description": "Hosting & domain setahun", "amount": 500000,
		"spent_at": time.Now().UTC().Format("2006-01-02"),
	}).mustCode(t, "catat biaya", 201)

	// 5. Invoice bertermin.
	inv := call(t, "POST", "/api/v1/invoices", f.token, map[string]any{
		"customer_id": cust, "project_id": projID, "term_label": "Pelunasan",
		"due_date": time.Now().UTC().AddDate(0, 0, 30).Format("2006-01-02"),
		"items": []map[string]any{
			{"description": "Jasa pembuatan website", "qty": "1", "unit_price": 5000000},
		},
	}).mustCode(t, "buat invoice", 201).data(t)
	invID := inv["id"].(string)
	assertI64(t, inv, "total", 5000000)

	call(t, "POST", "/api/v1/invoices/"+invID+"/send", f.token, nil).mustOK(t, "kirim invoice")

	// 6. DP 2 juta → status partial, belum ada penjualan.
	dp := payInvoice(t, f.token, "DP-"+ulid.New(), invID, 2000000)
	if dp["status"] != "partial" || dp["sale_id"] != nil {
		t.Fatalf("setelah DP: status=%v sale_id=%v, mau partial / kosong", dp["status"], dp["sale_id"])
	}
	assertI64(t, dp, "paid_amount", 2000000)

	// 7. Pelunasan 3 juta → lunas + penjualan sintetis.
	paid := payInvoice(t, f.token, "LN-"+ulid.New(), invID, 3000000)
	if paid["status"] != "paid" || paid["sale_id"] == nil {
		t.Fatalf("setelah pelunasan: status=%v sale_id=%v, mau paid / terisi", paid["status"], paid["sale_id"])
	}
	assertI64(t, paid, "outstanding", 0)
	saleID := paid["sale_id"].(string)

	// 8. Penjualan itu = SATU baris di `sales`, laba = total − biaya proyek.
	sale := call(t, "GET", "/api/v1/sales/"+saleID, f.token, nil).mustOK(t, "get sale").data(t)
	assertI64(t, sale, "total", 5000000)
	assertI64(t, sale, "cost_total", 500000)
	assertI64(t, sale, "gross_profit", 4500000)
	bd := sale["business_date"].(string)

	// 9. Muncul di laporan omzet yang sama dengan POS — hanya sekali.
	ls := call(t, "GET", "/api/v1/sales?status=completed", f.token, nil).mustOK(t, "list sales").data(t)
	if n := len(ls["data"].([]any)); n != 1 {
		t.Fatalf("jumlah penjualan = %d, mau 1 (tanpa entri ganda dari 2 pembayaran)", n)
	}
	sum := call(t, "GET", fmt.Sprintf("/api/v1/sales-summary?from=%s&to=%s", bd, bd), f.token, nil).
		mustOK(t, "ringkasan").data(t)
	assertI64(t, sum, "sales_count", 1)
	assertI64(t, sum, "net", 5000000)
	assertI64(t, sum, "cost_total", 500000)
	assertI64(t, sum, "gross_profit", 4500000)

	dash := call(t, "GET", "/api/v1/reports/dashboard?date="+bd, f.token, nil).
		mustOK(t, "dashboard").data(t)["today"].(map[string]any)
	assertI64(t, dash, "net_amount", 5000000)
	assertI64(t, dash, "gross_profit", 4500000)
}

// payInvoice mengirim POST /invoice-payments ber-Idempotency-Key.
func payInvoice(t *testing.T, token, key, invoiceID string, amount int64) map[string]any {
	t.Helper()
	req := jsonRequest(t, "POST", "/api/v1/invoice-payments", token, map[string]any{
		"invoice_id": invoiceID, "amount": amount, "method": "transfer",
	})
	req.Header.Set("Idempotency-Key", key)
	return serve(t, req).mustCode(t, "bayar invoice", 201).data(t)
}

// TestInvoicePaymentIdempotent — kunci sama → replay, satu baris pembayaran.
func TestInvoicePaymentIdempotent(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "crmidem")
	cust := makeCustomer(t, f.token, "Klien Idem")

	inv := call(t, "POST", "/api/v1/invoices", f.token, map[string]any{
		"customer_id": cust, "due_date": time.Now().UTC().AddDate(0, 0, 7).Format("2006-01-02"),
		"items": []map[string]any{{"description": "Jasa", "qty": "1", "unit_price": 1000000}},
	}).mustCode(t, "invoice", 201).data(t)
	invID := inv["id"].(string)
	call(t, "POST", "/api/v1/invoices/"+invID+"/send", f.token, nil).mustOK(t, "kirim")

	key := "K-" + ulid.New()
	a := payInvoice(t, f.token, key, invID, 1000000)
	b := payInvoice(t, f.token, key, invID, 1000000)
	if a["id"] != b["id"] || a["sale_id"] != b["sale_id"] {
		t.Fatalf("replay berbeda: %v vs %v", a["sale_id"], b["sale_id"])
	}

	ls := call(t, "GET", "/api/v1/sales?status=completed", f.token, nil).mustOK(t, "sales").data(t)
	if n := len(ls["data"].([]any)); n != 1 {
		t.Fatalf("replay membuat %d penjualan, mau 1", n)
	}
}

// TestCRMOwnerVisibility — di dalam satu tenant, sales X tidak melihat data
// sales Y; pemegang crm.lead.view.all melihat semua (blueprint E.5).
func TestCRMOwnerVisibility(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "crmvis")

	salesRole := call(t, "POST", "/api/v1/roles", f.token, map[string]any{
		"name":             "Sales Lapangan",
		"permission_codes": []string{"crm.deal.edit", "crm.lead.view.own", "customer.view", "customer.edit"},
	}).mustCode(t, "buat peran sales", 201).data(t)["id"].(string)

	x := staffToken(t, f, salesRole, "sales_x_crmvis")
	y := staffToken(t, f, salesRole, "sales_y_crmvis")

	custX := makeCustomer(t, x, "Toko Milik X")
	dealX := call(t, "POST", "/api/v1/deals", x, map[string]any{"title": "Deal X", "customer_id": custX, "value": 1000}).
		mustCode(t, "deal X", 201).data(t)["id"].(string)

	// Y tidak melihat deal / pelanggan X.
	if n := len(call(t, "GET", "/api/v1/deals", y, nil).mustOK(t, "deals Y").data(t)["data"].([]any)); n != 0 {
		t.Fatalf("sales Y melihat %d deal, mau 0", n)
	}
	call(t, "GET", "/api/v1/deals/"+dealX, y, nil).mustCode(t, "Y baca deal X", 404)
	if n := len(call(t, "GET", "/api/v1/customers", y, nil).mustOK(t, "customers Y").data(t)["data"].([]any)); n != 0 {
		t.Fatalf("sales Y melihat %d pelanggan, mau 0", n)
	}

	// X melihat miliknya.
	if n := len(call(t, "GET", "/api/v1/deals", x, nil).mustOK(t, "deals X").data(t)["data"].([]any)); n != 1 {
		t.Fatalf("sales X melihat %d deal, mau 1", n)
	}

	// Pemilik (punya crm.lead.view.all) melihat semua.
	if n := len(call(t, "GET", "/api/v1/deals", f.token, nil).mustOK(t, "deals owner").data(t)["data"].([]any)); n != 1 {
		t.Fatalf("pemilik melihat %d deal, mau 1", n)
	}
	call(t, "GET", "/api/v1/deals/"+dealX, f.token, nil).mustOK(t, "owner baca deal X")
}

// TestCRMTenantIsolation — tenant B tak melihat data CRM tenant A.
func TestCRMTenantIsolation(t *testing.T) {
	requireDB(t)
	a := registerTenant(t, "crmisoA")
	b := registerTenant(t, "crmisoB")

	custA := makeCustomer(t, a.token, "Klien A")
	dealA := call(t, "POST", "/api/v1/deals", a.token, map[string]any{"title": "Deal A", "customer_id": custA}).
		mustCode(t, "deal A", 201).data(t)["id"].(string)
	quoA := call(t, "POST", "/api/v1/quotations", a.token, map[string]any{
		"customer_id": custA, "items": []map[string]any{{"description": "x", "qty": "1", "unit_price": 1000}},
	}).mustCode(t, "quo A", 201).data(t)["id"].(string)

	call(t, "GET", "/api/v1/deals/"+dealA, b.token, nil).mustCode(t, "B baca deal A", 404)
	call(t, "GET", "/api/v1/quotations/"+quoA, b.token, nil).mustCode(t, "B baca quo A", 404)
	if n := len(call(t, "GET", "/api/v1/deals", b.token, nil).mustOK(t, "deals B").data(t)["data"].([]any)); n != 0 {
		t.Fatalf("tenant B melihat %d deal tenant A", n)
	}
}

// TestDealWinLose — menutup deal memindahkannya ke tahap menang/kalah.
func TestDealWinLose(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "crmwl")

	mk := func(title string) string {
		return call(t, "POST", "/api/v1/deals", f.token, map[string]any{"title": title}).
			mustCode(t, "deal "+title, 201).data(t)["id"].(string)
	}

	won := call(t, "POST", "/api/v1/deals/"+mk("Menangkan")+"/win", f.token, nil).mustOK(t, "win").data(t)
	if won["status"] != "won" || won["closed_at"] == "" {
		t.Fatalf("deal won salah: %v", won)
	}

	d2 := mk("Kalahkan")
	call(t, "POST", "/api/v1/deals/"+d2+"/lose", f.token, map[string]any{}).mustCode(t, "lose tanpa alasan", 422)
	lost := call(t, "POST", "/api/v1/deals/"+d2+"/lose", f.token, map[string]any{"lost_reason": "harga terlalu tinggi"}).
		mustOK(t, "lose").data(t)
	if lost["status"] != "lost" || lost["lost_reason"] != "harga terlalu tinggi" {
		t.Fatalf("deal lost salah: %v", lost)
	}
	// Deal tertutup tak bisa diubah.
	call(t, "PUT", "/api/v1/deals/"+d2, f.token, map[string]any{"title": "ubah"}).mustCode(t, "ubah deal tutup", 409)
}

// TestQuotationAcceptIsIdempotent — accept dua kali → proyek yang sama.
func TestQuotationAcceptIsIdempotent(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "crmacc")
	cust := makeCustomer(t, f.token, "Klien Acc")

	quoID := call(t, "POST", "/api/v1/quotations", f.token, map[string]any{
		"customer_id": cust, "items": []map[string]any{{"description": "x", "qty": "2", "unit_price": 750000}},
	}).mustCode(t, "quo", 201).data(t)["id"].(string)
	call(t, "POST", "/api/v1/quotations/"+quoID+"/send", f.token, nil).mustOK(t, "send")

	p1 := call(t, "POST", "/api/v1/quotations/"+quoID+"/accept", f.token, nil).mustOK(t, "accept 1").data(t)["project_id"].(string)
	p2 := call(t, "POST", "/api/v1/quotations/"+quoID+"/accept", f.token, nil).mustOK(t, "accept 2").data(t)["project_id"].(string)
	if p1 != p2 {
		t.Fatalf("accept kedua membuat proyek baru: %s vs %s", p1, p2)
	}
	proj := call(t, "GET", "/api/v1/projects/"+p1, f.token, nil).mustOK(t, "proyek").data(t)
	assertI64(t, proj, "contract_value", 1500000) // 2 * 750000
}

// TestCRMPermission — endpoint CRM butuh izin crm.*.
func TestCRMPermission(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "crmperm")
	kasir := staffToken(t, f, roleID(t, f, "Kasir"), "kasir_crmperm")

	call(t, "GET", "/api/v1/deals", kasir, nil).mustCode(t, "kasir lihat deal", 403)
	call(t, "POST", "/api/v1/quotations", kasir, map[string]any{
		"customer_id": ulid.New(), "items": []map[string]any{{"description": "x", "qty": "1", "unit_price": 1}},
	}).mustCode(t, "kasir buat penawaran", 403)
	call(t, "POST", "/api/v1/invoices", kasir, map[string]any{
		"customer_id": ulid.New(), "due_date": "2026-12-31",
		"items": []map[string]any{{"description": "x", "qty": "1", "unit_price": 1}},
	}).mustCode(t, "kasir buat invoice", 403)
}

// TestPipelineDefaultSeeded — GET /pipelines menyediakan pipeline "Umum" 6 tahap.
func TestPipelineDefaultSeeded(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "crmpipe")

	rows := call(t, "GET", "/api/v1/pipelines", f.token, nil).mustOK(t, "pipelines").Body["data"].([]any)
	if len(rows) != 1 {
		t.Fatalf("jumlah pipeline = %d, mau 1", len(rows))
	}
	p := rows[0].(map[string]any)
	if p["name"] != "Umum" || len(p["stages"].([]any)) != 6 {
		t.Fatalf("pipeline default salah: %v", p)
	}
	// Idempoten: panggilan kedua tak menggandakan.
	rows2 := call(t, "GET", "/api/v1/pipelines", f.token, nil).mustOK(t, "pipelines 2").Body["data"].([]any)
	if len(rows2) != 1 {
		t.Fatalf("pipeline tergandakan: %d", len(rows2))
	}
}
