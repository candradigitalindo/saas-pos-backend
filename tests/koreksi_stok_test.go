package tests

import "testing"

// GET /stock-adjustments: koreksi stok terbaru dulu (termasuk stok awal),
// dengan nama barang, satuan, alasan, dan pencatat — daftar "Koreksi
// terakhir" di layar Koreksi Stok. Gerakan lain (penjualan) tidak ikut.
func TestDaftarKoreksiStok(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "koreksistok") // stok awal A: 100, B: 50

	checkout(t, f.token, "KS-1", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodB, "qty": "1"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 8000}},
	}).mustCode(t, "jual", 201)
	adjust(t, f.tenantFixture, f.outletID, f.prodA, "97", "3 pecah")

	r := call(t, "GET", "/api/v1/stock-adjustments?outlet_id="+f.outletID, f.token, nil).mustOK(t, "koreksi").data(t)
	assertI64(t, r, "total", 3) // 2 stok awal + 1 koreksi; penjualan tidak ikut
	baru := r["data"].([]any)[0].(map[string]any)
	if baru["product_id"] != f.prodA || baru["kind"] != "adjustment" || baru["qty_delta"] != "-3" ||
		baru["balance_after"] != "97" || baru["reason"] != "3 pecah" {
		t.Fatalf("koreksi terbaru: %v", baru)
	}
	if baru["product_name"] == "" || baru["unit_name"] == "" || baru["created_by_name"] == nil {
		t.Fatalf("koreksi tanpa nama barang/satuan/pencatat: %v", baru)
	}

	// Tunduk pada stock.view.
	rid := call(t, "POST", "/api/v1/roles", f.token, map[string]any{
		"name": "Tanpa Stok", "permission_codes": []string{"customer.view"},
	}).mustCode(t, "peran", 201).data(t)["id"].(string)
	call(t, "GET", "/api/v1/stock-adjustments?outlet_id="+f.outletID, staffToken(t, f.tenantFixture, rid, "tanpastok_koreksi"), nil).
		mustCode(t, "tanpa stock.view", 403)
}
