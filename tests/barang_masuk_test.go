package tests

import (
	"strings"
	"testing"

	"candra/backend-api/internal/ulid"
)

// Layar Barang Masuk: riwayat pembelian yang terbaca (nama pemasok, pencatat,
// dan barangnya — baris pembelian hanya menyimpan id) dan saran belanja
// (?status=restock).
func TestRiwayatDanSaranBarangMasuk(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "brgmasuk") // A: 100, B: 50 — min_stock 0, belum laku
	unit := makeUnit(t, f.tenantFixture, "bks")

	barang := func(nama, min string) string {
		t.Helper()
		return call(t, "POST", "/api/v1/products", f.token, map[string]any{
			"name": nama, "unit_id": unit, "sell_price": 2000, "cost_price": 1000,
			"track_stock": true, "min_stock": min,
		}).mustCode(t, "barang "+nama, 201).data(t)["id"].(string)
	}
	laris := barang("Kerupuk Laris", "0") // di atas batas, tapi habis < seminggu
	tipis := barang("Permen Tipis", "5")  // di bawah batas
	aman := barang("Garam Aman", "5")     // di atas batas, tidak laku

	sup := call(t, "POST", "/api/v1/suppliers", f.token, map[string]any{"name": "CV Grosir Jaya"}).
		mustCode(t, "pemasok", 201).data(t)["id"].(string)
	beli := func(body map[string]any) string {
		t.Helper()
		body["outlet_id"] = f.outletID
		return checkoutLike(t, f.token, ulid.New(), "/api/v1/purchases", body).
			mustCode(t, "beli", 201).data(t)["id"].(string)
	}
	beliBesar := beli(map[string]any{
		"supplier_id": sup, "invoice_no": "NOTA-7",
		"items": []map[string]any{
			{"product_id": laris, "qty": "23", "unit_cost": 1000},
			{"product_id": tipis, "qty": "3", "unit_cost": 1000},
			{"product_id": aman, "qty": "10", "unit_cost": 1000},
			{"product_id": f.prodA, "qty": "1", "unit_cost": 6000},
		},
	})
	beli(map[string]any{"items": []map[string]any{{"product_id": f.prodB, "qty": "2", "unit_cost": 3000}}})

	// 20 dari 23 terjual: sisa 3, laju 20/30 per hari → habis < seminggu.
	checkout(t, f.token, "BM-1", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": laris, "qty": "20"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 40000}},
	}).mustCode(t, "jual laris", 201)

	// Riwayat: terbaru dulu, dengan nama-nama.
	daftar := call(t, "GET", "/api/v1/purchases?outlet_id="+f.outletID, f.token, nil).
		mustOK(t, "riwayat").data(t)["data"].([]any)
	if len(daftar) != 2 {
		t.Fatalf("riwayat: %d baris, mau 2", len(daftar))
	}
	terbaru, besar := daftar[0].(map[string]any), daftar[1].(map[string]any)
	if terbaru["supplier_name"] != nil || terbaru["item_count"] != float64(1) {
		t.Fatalf("pembelian tanpa pemasok: %v", terbaru)
	}
	if besar["id"] != beliBesar || besar["supplier_name"] != "CV Grosir Jaya" || besar["item_count"] != float64(4) ||
		besar["created_by_name"] == nil {
		t.Fatalf("pembelian besar: %v", besar)
	}
	if n := besar["item_names"].([]any); len(n) != 3 || n[0] != "Kerupuk Laris" {
		t.Fatalf("cuplikan barang: %v, mau 3 nama pertama", n)
	}

	// Rincian: tiap baris membawa nama barang & satuan dasarnya.
	rinci := call(t, "GET", "/api/v1/purchases/"+beliBesar, f.token, nil).mustOK(t, "rincian").data(t)
	for _, x := range rinci["items"].([]any) {
		it := x.(map[string]any)
		if it["product_name"] == nil || it["product_name"] == "" {
			t.Fatalf("baris rincian tanpa nama: %v", it)
		}
		if it["product_id"] == laris && (it["product_name"] != "Kerupuk Laris" || it["base_unit_name"] != "bks") {
			t.Fatalf("baris kerupuk: %v", it)
		}
	}

	// Saran belanja: di bawah batas ATAU habis dalam seminggu menurut laju.
	var saran []string
	for _, x := range call(t, "GET", "/api/v1/stocks?outlet_id="+f.outletID+"&status=restock&sort=urgent", f.token, nil).
		mustOK(t, "saran").data(t)["data"].([]any) {
		saran = append(saran, x.(map[string]any)["product_id"].(string))
	}
	if got, mau := strings.Join(saran, ","), strings.Join([]string{tipis, laris}, ","); got != mau {
		t.Fatalf("saran belanja: %v, mau [Permen Tipis, Kerupuk Laris]", saran)
	}
}
