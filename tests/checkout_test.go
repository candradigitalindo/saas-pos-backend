package tests

import (
	"fmt"
	"sync"
	"testing"

	"candra/backend-api/internal/ulid"
)

// posFixture: tenant + outlet + unit + 2 produk ber-stok + shift terbuka.
type posFixture struct {
	tenantFixture
	unitID  string
	prodA   string // sell 15000, cost 6000
	prodB   string // sell 8000, cost 3000
	shiftID string
}

func setupPOS(t *testing.T, slug string) posFixture {
	t.Helper()
	return setupPOSDengan(t, slug, registerTenant)
}

// setupPOSDengan seperti setupPOS, dengan cara mendaftar tenant yang dipilih —
// mis. registerTenantPolos untuk menguji kunci paket Gratis.
func setupPOSDengan(t *testing.T, slug string, daftar func(*testing.T, string) tenantFixture) posFixture {
	t.Helper()
	f := daftar(t, slug)
	unit := makeUnit(t, f, "pcs")

	mk := func(name string, sell, cost int64) string {
		d := call(t, "POST", "/api/v1/products", f.token, map[string]any{
			"name": name, "unit_id": unit, "sell_price": sell, "cost_price": cost, "track_stock": true,
		}).mustCode(t, "POST product "+name, 201).data(t)
		return d["id"].(string)
	}
	pf := posFixture{tenantFixture: f, unitID: unit}
	pf.prodA = mk("Produk A "+slug, 15000, 6000)
	pf.prodB = mk("Produk B "+slug, 8000, 3000)

	// Saldo awal stok.
	adjust(t, f, pf.outletID, pf.prodA, "100", "saldo awal")
	adjust(t, f, pf.outletID, pf.prodB, "50", "saldo awal")

	// Shift terbuka.
	d := call(t, "POST", "/api/v1/shifts/open", f.token, map[string]any{
		"outlet_id": pf.outletID, "opening_cash": 100000,
	}).mustCode(t, "open shift", 201).data(t)
	pf.shiftID = d["id"].(string)
	return pf
}

func adjust(t *testing.T, f tenantFixture, outletID, productID, newQty, reason string) {
	t.Helper()
	checkoutLike(t, f.token, ulid.New(), "/api/v1/stock-adjustments", map[string]any{
		"outlet_id": outletID, "product_id": productID, "new_qty": newQty, "reason": reason,
	}).mustCode(t, "stock adjust", 201)
}

// checkout mengirim POST /api/v1/sales dengan Idempotency-Key.
func checkout(t *testing.T, token, idemKey string, payload map[string]any) apiResp {
	t.Helper()
	req := jsonRequest(t, "POST", "/api/v1/sales", token, payload)
	req.Header.Set("Idempotency-Key", idemKey)
	return serve(t, req)
}

func stockQty(t *testing.T, f tenantFixture, outletID, productID string) string {
	t.Helper()
	d := call(t, "GET", "/api/v1/stocks?outlet_id="+outletID, f.token, nil).mustOK(t, "GET stocks").data(t)
	for _, it := range d["data"].([]any) {
		m := it.(map[string]any)
		if m["product_id"] == productID {
			return m["qty"].(string)
		}
	}
	t.Fatalf("produk %s tidak ada di daftar stok", productID)
	return ""
}

func TestCheckoutHappyPath(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "coA")

	res := checkout(t, f.token, "idem-coA-1", map[string]any{
		"outlet_id": f.outletID,
		"items": []map[string]any{
			{"product_id": f.prodA, "qty": "3"},
			{"product_id": f.prodB, "qty": "2"},
		},
		"payments": []map[string]any{{"method": "cash", "amount": 65000}},
	}).mustCode(t, "checkout", 201)
	d := res.data(t)

	assertI64(t, d, "subtotal", 61000) // 3*15000 + 2*8000
	assertI64(t, d, "total", 61000)
	assertI64(t, d, "paid_amount", 65000)
	assertI64(t, d, "change_amount", 4000)
	assertI64(t, d, "cost_total", 24000) // 3*6000 + 2*3000
	assertI64(t, d, "gross_profit", 37000)
	if d["receipt_no"].(string) == "" {
		t.Fatal("receipt_no kosong")
	}
	if n := len(d["items"].([]any)); n != 2 {
		t.Fatalf("items = %d, mau 2", n)
	}

	// Stok berkurang tepat.
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != "97" {
		t.Fatalf("stok A = %s, mau 97", q)
	}
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodB); q != "48" {
		t.Fatalf("stok B = %s, mau 48", q)
	}

	// Kartu stok A: initial + sale, saldo terakhir 97.
	km := call(t, "GET", "/api/v1/stock-movements?product_id="+f.prodA, f.token, nil).
		mustOK(t, "kartu stok").data(t)
	moves := km["data"].([]any)
	if len(moves) != 2 {
		t.Fatalf("gerakan stok A = %d, mau 2 (initial + sale)", len(moves))
	}
	last := moves[0].(map[string]any) // terbaru dulu
	if last["kind"] != "sale" || last["balance_after"] != "97" || last["qty_delta"] != "-3" {
		t.Fatalf("gerakan sale A salah: %v", last)
	}

	// Ringkasan.
	bd := d["business_date"].(string)
	sum := call(t, "GET", fmt.Sprintf("/api/v1/sales-summary?from=%s&to=%s&outlet_id=%s", bd, bd, f.outletID), f.token, nil).
		mustOK(t, "summary").data(t)
	assertI64(t, sum, "net", 61000)
	assertI64(t, sum, "cost_total", 24000)
	assertI64(t, sum, "gross_profit", 37000)
	assertI64(t, sum, "sales_count", 1)
}

func TestCheckoutIdempotent(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "idem")

	body := map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "2"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 30000}},
	}
	r1 := checkout(t, f.token, "K-1", body).mustCode(t, "checkout #1", 201).data(t)
	r2 := checkout(t, f.token, "K-1", body).mustCode(t, "checkout #2 (replay)", 201).data(t)

	if r1["id"] != r2["id"] || r1["receipt_no"] != r2["receipt_no"] {
		t.Fatalf("replay menghasilkan transaksi berbeda: %v vs %v", r1["id"], r2["id"])
	}
	// Stok hanya berkurang sekali.
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != "98" {
		t.Fatalf("stok A = %s, mau 98 (replay tidak boleh mengurangi lagi)", q)
	}
	// Kunci sama, body beda → 409.
	body["order_discount"] = 1000
	checkout(t, f.token, "K-1", body).mustCode(t, "kunci sama body beda", 409)
}

func TestCheckoutRejectsUnderpayAndNoShift(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "rej")

	checkout(t, f.token, "R-1", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "2"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 10000}}, // kurang dari 30000
	}).mustCode(t, "bayar kurang", 422)

	// Outlet lain tanpa shift.
	g := registerTenant(t, "rej2")
	u := makeUnit(t, g, "pcs")
	p := call(t, "POST", "/api/v1/products", g.token, map[string]any{"name": "P", "unit_id": u, "sell_price": 1000}).
		mustCode(t, "prod", 201).data(t)["id"].(string)
	checkout(t, g.token, "R-2", map[string]any{
		"outlet_id": g.outletID,
		"items":     []map[string]any{{"product_id": p, "qty": "1"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 1000}},
	}).mustCode(t, "tanpa shift", 422)
}

func TestVoidRestoresStock(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "void")

	sale := checkout(t, f.token, "V-1", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "5"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 75000}},
	}).mustCode(t, "checkout", 201).data(t)
	saleID := sale["id"].(string)

	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != "95" {
		t.Fatalf("stok setelah jual = %s, mau 95", q)
	}

	v := call(t, "POST", "/api/v1/sales/"+saleID+"/void", f.token, map[string]any{"reason": "salah input"}).
		mustOK(t, "void").data(t)
	if v["status"] != "canceled" {
		t.Fatalf("status setelah void = %v, mau canceled", v["status"])
	}
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != "100" {
		t.Fatalf("stok setelah void = %s, mau 100 (dikembalikan)", q)
	}
	// Void kedua → 409.
	call(t, "POST", "/api/v1/sales/"+saleID+"/void", f.token, map[string]any{"reason": "lagi"}).
		mustCode(t, "void kedua", 409)
}

func TestRefundFullIdempotent(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "ref")

	sale := checkout(t, f.token, "RF-1", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodB, "qty": "4"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 32000}},
	}).mustCode(t, "checkout", 201).data(t)
	saleID := sale["id"].(string)

	r1 := call(t, "POST", "/api/v1/sales/"+saleID+"/refund", f.token, map[string]any{"reason": "rusak"}).
		mustOK(t, "refund").data(t)
	if r1["status"] != "returned" {
		t.Fatalf("status retur = %v, mau returned", r1["status"])
	}
	assertI64(t, r1, "total", -32000)
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodB); q != "50" {
		t.Fatalf("stok setelah refund = %s, mau 50", q)
	}
	// Refund kedua → retur yang sama (idempoten).
	r2 := call(t, "POST", "/api/v1/sales/"+saleID+"/refund", f.token, map[string]any{"reason": "lagi"}).
		mustOK(t, "refund kedua").data(t)
	if r1["id"] != r2["id"] {
		t.Fatalf("refund kedua membuat retur baru: %v vs %v", r1["id"], r2["id"])
	}
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodB); q != "50" {
		t.Fatalf("stok setelah refund kedua = %s, mau tetap 50", q)
	}
}

func TestKasbonCreatesAndSettlesReceivable(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "kasbon")

	cust := call(t, "POST", "/api/v1/customers", f.token, map[string]any{
		"name": "Bu Sari", "phone": "0812kasbon", "credit_limit": 1000000,
	}).mustCode(t, "customer", 201).data(t)["id"].(string)

	// 2*A = 30000; bayar cash 10000 + kasbon 20000.
	checkout(t, f.token, "KB-1", map[string]any{
		"outlet_id":   f.outletID,
		"customer_id": cust,
		"items":       []map[string]any{{"product_id": f.prodA, "qty": "2"}},
		"payments": []map[string]any{
			{"method": "cash", "amount": 10000},
			{"method": "credit", "amount": 20000},
		},
	}).mustCode(t, "checkout kasbon", 201)

	rec := call(t, "GET", "/api/v1/receivables?customer_id="+cust, f.token, nil).mustOK(t, "list piutang").data(t)
	items := rec["data"].([]any)
	if len(items) != 1 {
		t.Fatalf("piutang = %d, mau 1", len(items))
	}
	r := items[0].(map[string]any)
	recID := r["id"].(string)
	assertI64(t, r, "amount", 20000)
	assertI64(t, r, "outstanding", 20000)

	// Bayar sebagian.
	checkoutLike(t, f.token, ulid.New(), "/api/v1/receivable-payments", map[string]any{
		"receivable_id": recID, "amount": 12000, "method": "cash",
	}).mustCode(t, "bayar piutang 1", 201)
	// Pelunasan.
	paid := checkoutLike(t, f.token, ulid.New(), "/api/v1/receivable-payments", map[string]any{
		"receivable_id": recID, "amount": 8000, "method": "cash",
	}).mustCode(t, "bayar piutang 2", 201).data(t)
	if paid["status"] != "paid" {
		t.Fatalf("status piutang = %v, mau paid", paid["status"])
	}
	// Overpay ditolak.
	checkoutLike(t, f.token, ulid.New(), "/api/v1/receivable-payments", map[string]any{
		"receivable_id": recID, "amount": 1, "method": "cash",
	}).mustCode(t, "overpay", 409)
}

func TestShiftCloseReconciles(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "shiftz")

	// 1 penjualan tunai 45000 (3*A).
	checkout(t, f.token, "SZ-1", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "3"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 45000}},
	}).mustCode(t, "checkout", 201)

	// kas masuk 5000, kas keluar 2000.
	call(t, "POST", "/api/v1/cash-movements", f.token, map[string]any{
		"outlet_id": f.outletID, "direction": "in", "amount": 5000, "reason": "top up",
	}).mustCode(t, "cash in", 201)
	call(t, "POST", "/api/v1/cash-movements", f.token, map[string]any{
		"outlet_id": f.outletID, "direction": "out", "amount": 2000, "reason": "beli galon",
	}).mustCode(t, "cash out", 201)

	// expected = 100000 + 45000 + 5000 - 2000 = 148000
	closed := call(t, "POST", "/api/v1/shifts/"+f.shiftID+"/close", f.token, map[string]any{
		"counted_cash": 148000,
	}).mustOK(t, "close shift").data(t)
	assertI64(t, closed, "expected_cash", 148000)
	assertI64(t, closed, "difference", 0)
	if closed["status"] != "closed" {
		t.Fatalf("status shift = %v, mau closed", closed["status"])
	}
}

// TestShiftCloseSubtractsChange menjaga agar kembalian tidak ikut dihitung
// sebagai uang yang masuk laci.
//
// Yang tercatat di sale_payments adalah uang yang DISERAHKAN pembeli. Pembeli
// membayar Rp 50.000 untuk belanja Rp 45.000 hanya menambah Rp 45.000 ke laci —
// Rp 5.000 sisanya kembali ke tangan pembeli. Tanpa pengurangan ini setiap
// shift tampak kurang persis sebesar total kembalian, dan kasir yang jujur
// terus-menerus dituduh selisih.
func TestShiftCloseSubtractsChange(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "shiftkembalian")

	// Belanja 45.000, pembeli menyerahkan 50.000 → kembalian 5.000.
	sale := checkout(t, f.token, "SK-1", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "3"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 50000}},
	}).mustCode(t, "checkout", 201).data(t)
	assertI64(t, sale, "total", 45000)
	assertI64(t, sale, "change_amount", 5000)

	// Laci: 100.000 modal + 45.000 bersih = 145.000 (BUKAN 150.000).
	shift := call(t, "GET", "/api/v1/shifts/"+f.shiftID, f.token, nil).
		mustOK(t, "detail shift").data(t)
	assertI64(t, shift, "cash_sales", 45000)
	assertI64(t, shift, "expected_cash", 145000)

	closed := call(t, "POST", "/api/v1/shifts/"+f.shiftID+"/close", f.token, map[string]any{
		"counted_cash": 145000,
	}).mustOK(t, "close shift").data(t)
	assertI64(t, closed, "expected_cash", 145000)
	// Kasir yang menghitung laci dengan benar harus mendapat selisih NOL.
	assertI64(t, closed, "difference", 0)
}

func TestSaleIsolation(t *testing.T) {
	requireDB(t)
	a := setupPOS(t, "saleisoA")
	b := setupPOS(t, "saleisoB")

	sa := checkout(t, a.token, "SI-1", map[string]any{
		"outlet_id": a.outletID,
		"items":     []map[string]any{{"product_id": a.prodA, "qty": "1"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 15000}},
	}).mustCode(t, "checkout A", 201).data(t)
	saleAID := sa["id"].(string)

	call(t, "GET", "/api/v1/sales/"+saleAID, b.token, nil).mustCode(t, "B baca sale A", 404)
	call(t, "POST", "/api/v1/sales/"+saleAID+"/void", b.token, map[string]any{"reason": "percobaan retas"}).
		mustCode(t, "B void sale A", 404)
	// B tidak melihat sale A di daftar.
	lb := call(t, "GET", "/api/v1/sales", b.token, nil).mustOK(t, "list B").data(t)
	if len(lb["data"].([]any)) != 0 {
		t.Fatalf("B melihat %d transaksi, mau 0", len(lb["data"].([]any)))
	}
}

// TestConcurrentCheckoutSameProduct — dua checkout paralel produk yang sama:
// stok akhir harus tepat, tanpa deadlock (§14). Jalankan dengan -race.
func TestConcurrentCheckoutSameProduct(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "conc")

	const n = 8
	var wg sync.WaitGroup
	wg.Add(n)
	errs := make(chan int, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			r := checkout(t, f.token, fmt.Sprintf("C-%d", i), map[string]any{
				"outlet_id": f.outletID,
				"items":     []map[string]any{{"product_id": f.prodA, "qty": "1"}},
				"payments":  []map[string]any{{"method": "cash", "amount": 15000}},
			})
			errs <- r.Code
		}(i)
	}
	wg.Wait()
	close(errs)
	for code := range errs {
		if code != 201 {
			t.Fatalf("checkout paralel gagal: status %d", code)
		}
	}
	// 100 - 8 = 92
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != "92" {
		t.Fatalf("stok akhir = %s, mau 92 (8 penjualan @1)", q)
	}
}

// TestHundredSalesReconcile — DoD Fase 3: 100 transaksi berurutan; stok, kas,
// dan laporan cocok sampai rupiah terakhir.
func TestHundredSalesReconcile(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "hundred")
	adjust(t, f.tenantFixture, f.outletID, f.prodA, "1000", "top up") // set absolut jadi 1000

	const n = 100
	var wantNet, wantCost int64
	for i := 0; i < n; i++ {
		qty := int64(i%3 + 1) // 1..3
		amount := qty * 15000
		checkout(t, f.token, fmt.Sprintf("H-%d", i), map[string]any{
			"outlet_id": f.outletID,
			"items":     []map[string]any{{"product_id": f.prodA, "qty": fmt.Sprintf("%d", qty)}},
			"payments":  []map[string]any{{"method": "cash", "amount": amount}},
		}).mustCode(t, fmt.Sprintf("checkout %d", i), 201)
		wantNet += amount
		wantCost += qty * 6000
	}

	// Stok: 1000 (+100 dari setup awal) - Σqty.
	soldQty := int64(0)
	for i := 0; i < n; i++ {
		soldQty += int64(i%3 + 1)
	}
	wantStock := 1000 - soldQty
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != fmt.Sprintf("%d", wantStock) {
		t.Fatalf("stok akhir = %s, mau %d", q, wantStock)
	}

	// Ringkasan laporan.
	bd := call(t, "GET", "/api/v1/sales?outlet_id="+f.outletID+"&status=completed", f.token, nil).
		mustOK(t, "list").data(t)["data"].([]any)[0].(map[string]any)["business_date"].(string)
	sum := call(t, "GET", fmt.Sprintf("/api/v1/sales-summary?from=%s&to=%s&outlet_id=%s", bd, bd, f.outletID), f.token, nil).
		mustOK(t, "summary").data(t)
	assertI64(t, sum, "sales_count", n)
	assertI64(t, sum, "net", wantNet)
	assertI64(t, sum, "cost_total", wantCost)
	assertI64(t, sum, "gross_profit", wantNet-wantCost)
	assertI64(t, sum, "paid", wantNet)

	// Kas shift: opening 100000 + Σ tunai.
	closed := call(t, "POST", "/api/v1/shifts/"+f.shiftID+"/close", f.token, map[string]any{
		"counted_cash": 100000 + wantNet,
	}).mustOK(t, "close").data(t)
	assertI64(t, closed, "expected_cash", 100000+wantNet)
	assertI64(t, closed, "difference", 0)
}

func assertI64(t *testing.T, m map[string]any, key string, want int64) {
	t.Helper()
	v, ok := m[key].(float64)
	if !ok || int64(v) != want {
		t.Fatalf("%s = %v, mau %d", key, m[key], want)
	}
}
