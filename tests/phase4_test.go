package tests

import (
	"testing"

	"candra/backend-api/database"
)

// makeOutlet membuat outlet tambahan dan mengembalikan id-nya.
func makeOutlet(t *testing.T, f tenantFixture, name string) string {
	t.Helper()
	d := call(t, "POST", "/api/v1/outlets", f.token, map[string]any{"name": name, "code": name}).
		mustCode(t, "POST /outlets "+name, 201).data(t)
	return d["id"].(string)
}

func TestPurchaseIncreasesStock(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "purchz")

	sup := call(t, "POST", "/api/v1/suppliers", f.token, map[string]any{"name": "PT Sumber"}).
		mustCode(t, "supplier", 201).data(t)["id"].(string)

	body := map[string]any{
		"outlet_id":   f.outletID,
		"supplier_id": sup,
		"invoice_no":  "INV-001",
		"items": []map[string]any{
			{"product_id": f.prodA, "qty": "50", "unit_cost": 7000},
		},
	}
	r := checkoutLike(t, f.token, "PUR-1", "/api/v1/purchases", body).
		mustCode(t, "purchase", 201).data(t)
	assertI64(t, r, "subtotal", 350000) // 50 * 7000
	assertI64(t, r, "total", 350000)

	// Stok naik 100 → 150.
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != "150" {
		t.Fatalf("stok setelah beli = %s, mau 150", q)
	}
	// Harga modal produk = harga beli terakhir.
	pd := call(t, "GET", "/api/v1/products/"+f.prodA, f.token, nil).mustOK(t, "get product").data(t)
	assertI64(t, pd, "cost_price", 7000)

	// Replay idempoten → stok tetap 150.
	checkoutLike(t, f.token, "PUR-1", "/api/v1/purchases", body).mustCode(t, "purchase replay", 201)
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != "150" {
		t.Fatalf("stok setelah replay = %s, mau tetap 150", q)
	}
}

func TestStockOpnamePost(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "opnamez") // prodA stok 100

	op := call(t, "POST", "/api/v1/stock-opnames", f.token, map[string]any{"outlet_id": f.outletID}).
		mustCode(t, "create opname", 201).data(t)["id"].(string)

	// Hitung fisik: 95 (kurang 5).
	set := call(t, "POST", "/api/v1/stock-opnames/"+op+"/items", f.token, map[string]any{
		"items": []map[string]any{{"product_id": f.prodA, "counted_qty": "95"}},
	}).mustOK(t, "set items").data(t)
	it := set["items"].([]any)[0].(map[string]any)
	if it["system_qty"] != "100" || it["counted_qty"] != "95" || it["diff_qty"] != "-5" {
		t.Fatalf("item opname salah: %v", it)
	}

	call(t, "POST", "/api/v1/stock-opnames/"+op+"/post", f.token, nil).mustOK(t, "post opname")
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != "95" {
		t.Fatalf("stok setelah opname = %s, mau 95", q)
	}
	// Post kedua → 409.
	call(t, "POST", "/api/v1/stock-opnames/"+op+"/post", f.token, nil).mustCode(t, "post kedua", 409)

	// Kartu stok: initial + opname.
	km := call(t, "GET", "/api/v1/stock-movements?product_id="+f.prodA, f.token, nil).mustOK(t, "kartu").data(t)
	last := km["data"].([]any)[0].(map[string]any)
	if last["kind"] != "opname" || last["qty_delta"] != "-5" || last["balance_after"] != "95" {
		t.Fatalf("gerakan opname salah: %v", last)
	}
}

func TestStockTransferBetweenOutlets(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "trfz") // prodA stok 100 di outlet utama
	dest := makeOutlet(t, f.tenantFixture, "TRF2")

	tr := call(t, "POST", "/api/v1/stock-transfers", f.token, map[string]any{
		"from_outlet_id": f.outletID, "to_outlet_id": dest,
		"items": []map[string]any{{"product_id": f.prodA, "qty": "30"}},
	}).mustCode(t, "create transfer", 201).data(t)
	trID := tr["id"].(string)
	if tr["status"] != "draft" {
		t.Fatalf("status = %v, mau draft", tr["status"])
	}

	call(t, "POST", "/api/v1/stock-transfers/"+trID+"/send", f.token, nil).mustOK(t, "send")
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != "70" {
		t.Fatalf("stok asal setelah kirim = %s, mau 70", q)
	}
	// Receive sebelum send → tidak bisa (sudah sent, ini receive normal).
	call(t, "POST", "/api/v1/stock-transfers/"+trID+"/receive", f.token, nil).mustOK(t, "receive")
	if q := stockQty(t, f.tenantFixture, dest, f.prodA); q != "30" {
		t.Fatalf("stok tujuan setelah terima = %s, mau 30", q)
	}
	// Receive kedua → 409.
	call(t, "POST", "/api/v1/stock-transfers/"+trID+"/receive", f.token, nil).mustCode(t, "receive kedua", 409)

	// from != to divalidasi.
	call(t, "POST", "/api/v1/stock-transfers", f.token, map[string]any{
		"from_outlet_id": f.outletID, "to_outlet_id": f.outletID,
		"items": []map[string]any{{"product_id": f.prodA, "qty": "1"}},
	}).mustCode(t, "transfer ke outlet sama", 422)
}

// TestReconcileRebuildsFromLedger — DoD Fase 4: hitung ulang `stocks` dari
// `stock_movements` menghasilkan angka yang sama persis. Cache sengaja dirusak
// lalu direkonsiliasi.
func TestReconcileRebuildsFromLedger(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "reconz") // prodA 100, prodB 50

	checkout(t, f.token, "RC-1", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "7"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 105000}},
	}).mustCode(t, "checkout", 201)
	// Saldo benar: 93.

	// Rusak cache langsung di database.
	if err := database.DB.Exec(
		"UPDATE stocks SET qty = 99999 WHERE product_id = ? AND outlet_id = ?", f.prodA, f.outletID,
	).Error; err != nil {
		t.Fatalf("corrupt cache: %v", err)
	}
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != "99999" {
		t.Fatalf("cache belum rusak: %s", q)
	}

	call(t, "POST", "/api/v1/stock-reconcile?outlet_id="+f.outletID, f.token, nil).mustOK(t, "reconcile")

	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != "93" {
		t.Fatalf("stok setelah rekonsiliasi = %s, mau 93 (dari buku besar)", q)
	}
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodB); q != "50" {
		t.Fatalf("stok B setelah rekonsiliasi = %s, mau 50", q)
	}
}

func TestRecipeConsumesIngredients(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "recipez")
	unit := makeUnit(t, f, "pcs")

	mk := func(name string, sell int64, track bool) string {
		d := call(t, "POST", "/api/v1/products", f.token, map[string]any{
			"name": name, "unit_id": unit, "sell_price": sell, "cost_price": 0, "track_stock": track,
		}).mustCode(t, "product "+name, 201).data(t)
		return d["id"].(string)
	}
	teh := mk("Teh", 0, true)
	gula := mk("Gula", 0, true)
	menu := mk("Es Teh Manis", 5000, false) // menu jadi tak di-track sendiri

	adjust(t, f, f.outletID, teh, "100", "awal")
	adjust(t, f, f.outletID, gula, "100", "awal")

	// Resep: 1 porsi = 1 Teh + 2 Gula.
	call(t, "PUT", "/api/v1/products/"+menu+"/recipe", f.token, map[string]any{
		"yield_qty": "1",
		"items": []map[string]any{
			{"ingredient_product_id": teh, "qty": "1"},
			{"ingredient_product_id": gula, "qty": "2"},
		},
	}).mustOK(t, "upsert recipe")

	// Shift + jual 3 porsi.
	call(t, "POST", "/api/v1/shifts/open", f.token, map[string]any{"outlet_id": f.outletID, "opening_cash": 0}).
		mustCode(t, "open shift", 201)
	checkout(t, f.token, "REC-1", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": menu, "qty": "3"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 15000}},
	}).mustCode(t, "checkout menu", 201)

	if q := stockQty(t, f, f.outletID, teh); q != "97" {
		t.Fatalf("stok Teh = %s, mau 97 (3 porsi x 1)", q)
	}
	if q := stockQty(t, f, f.outletID, gula); q != "94" {
		t.Fatalf("stok Gula = %s, mau 94 (3 porsi x 2)", q)
	}

	// Kartu stok Teh: initial + recipe.
	km := call(t, "GET", "/api/v1/stock-movements?product_id="+teh, f.token, nil).mustOK(t, "kartu teh").data(t)
	last := km["data"].([]any)[0].(map[string]any)
	if last["kind"] != "recipe" || last["qty_delta"] != "-3" {
		t.Fatalf("gerakan recipe Teh salah: %v", last)
	}
	// Menu jadi tidak punya gerakan (track_stock=false).
	kmMenu := call(t, "GET", "/api/v1/stock-movements?product_id="+menu, f.token, nil).mustOK(t, "kartu menu").data(t)
	if n := len(kmMenu["data"].([]any)); n != 0 {
		t.Fatalf("menu jadi punya %d gerakan stok, mau 0", n)
	}
}

// checkoutLike mengirim POST ber-Idempotency-Key ke path arbitrer (checkout /
// purchase).
func checkoutLike(t *testing.T, token, idemKey, path string, payload map[string]any) apiResp {
	t.Helper()
	req := jsonRequest(t, "POST", path, token, payload)
	req.Header.Set("Idempotency-Key", idemKey)
	return serve(t, req)
}
