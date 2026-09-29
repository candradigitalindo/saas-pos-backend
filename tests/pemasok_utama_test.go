package tests

import (
	"testing"

	"candra/backend-api/internal/ulid"
)

// Pemasok utama per barang (000050): diatur di formulir barang, terisi
// otomatis dari barang masuk pertama bila belum ada (pilihan yang sudah ada
// tidak ditimpa), ikut di saldo stok, dan "Pesan lagi" pemasok memuat barang
// yang pemasok utamanya dia walau belum pernah dibeli darinya.
func TestPemasokUtama(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "pmsutama") // A: modal 6.000; B: modal 3.000
	pemasok := func(nama string) string {
		t.Helper()
		return call(t, "POST", "/api/v1/suppliers", f.token, map[string]any{"name": nama}).
			mustCode(t, "pemasok", 201).data(t)["id"].(string)
	}
	utama, lain := pemasok("Grosir Utama"), pemasok("Grosir Lain")
	unit := makeUnit(t, f.tenantFixture, "btl")

	// Barang baru dengan pemasok utama dari formulir.
	c := call(t, "POST", "/api/v1/products", f.token, map[string]any{
		"name": "Sirup Baru", "unit_id": unit, "sell_price": 20000, "cost_price": 12000, "supplier_id": utama,
	}).mustCode(t, "barang C", 201).data(t)
	if c["supplier_id"] != utama || c["supplier_name"] != "Grosir Utama" {
		t.Fatalf("barang C: %v", c)
	}
	call(t, "POST", "/api/v1/products", f.token, map[string]any{
		"name": "Salah", "unit_id": unit, "supplier_id": ulid.New(),
	}).mustCode(t, "pemasok tak dikenal", 400)

	beli := func(sup, produk string) {
		t.Helper()
		checkoutLike(t, f.token, ulid.New(), "/api/v1/purchases", map[string]any{
			"outlet_id": f.outletID, "supplier_id": sup, "paid_amount": 6000,
			"items": []map[string]any{{"product_id": produk, "qty": "1", "unit_cost": 6000}},
		}).mustCode(t, "beli", 201)
	}
	// A belum punya pemasok utama → terisi dari barang masuk pertama;
	// barang masuk dari pemasok lain sesudahnya tidak menimpanya.
	beli(utama, f.prodA)
	beli(lain, f.prodA)
	a := call(t, "GET", "/api/v1/products/"+f.prodA, f.token, nil).mustOK(t, "barang A").data(t)
	if a["supplier_id"] != utama {
		t.Fatalf("pemasok utama A: %v, mau Grosir Utama", a["supplier_id"])
	}

	// Saldo stok membawa pemasok utama.
	st := call(t, "GET", "/api/v1/stocks?outlet_id="+f.outletID+"&product_ids="+f.prodA, f.token, nil).
		mustOK(t, "stok A").data(t)["data"].([]any)[0].(map[string]any)
	if st["supplier_id"] != utama || st["supplier_name"] != "Grosir Utama" {
		t.Fatalf("stok A: %v", st)
	}

	// "Pesan lagi": A (pernah dibeli) + C (belum pernah, pemasok utamanya).
	per := map[string]map[string]any{}
	for _, x := range call(t, "GET", "/api/v1/suppliers/"+utama+"/products?outlet_id="+f.outletID, f.token, nil).
		mustOK(t, "barang pemasok").Body["data"].([]any) {
		m := x.(map[string]any)
		per[m["product_id"].(string)] = m
	}
	if len(per) != 2 || per[f.prodA]["main"] != true || per[f.prodA]["last_bought_at"] == nil {
		t.Fatalf("barang pemasok utama: %v", per)
	}
	if cc := per[c["id"].(string)]; cc == nil || cc["main"] != true || cc["last_bought_at"] != nil ||
		cc["unit_cost"] != float64(12000) || cc["times"] != float64(0) {
		t.Fatalf("barang belum pernah dibeli: %v", cc)
	}
	// Di pemasok lain, A tercatat pernah dibeli tapi bukan pemasok utamanya.
	for _, x := range call(t, "GET", "/api/v1/suppliers/"+lain+"/products?outlet_id="+f.outletID, f.token, nil).
		mustOK(t, "barang pemasok lain").Body["data"].([]any) {
		if m := x.(map[string]any); m["product_id"] == f.prodA && m["main"] != false {
			t.Fatalf("A di pemasok lain: %v", m)
		}
	}

	// Dikosongkan dari formulir.
	call(t, "PUT", "/api/v1/products/"+f.prodA, f.token, map[string]any{"supplier_id": ""}).mustOK(t, "kosongkan")
	if a := call(t, "GET", "/api/v1/products/"+f.prodA, f.token, nil).mustOK(t, "A").data(t); a["supplier_id"] != nil {
		t.Fatalf("pemasok utama tidak terhapus: %v", a["supplier_id"])
	}
}
