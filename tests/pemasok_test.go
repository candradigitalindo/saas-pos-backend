package tests

import (
	"testing"
	"time"

	"candra/backend-api/database"
	"candra/backend-api/internal/ulid"
)

// Layar Pemasok: angka belanja per pemasok (GET /supplier-stats) dan barang
// yang biasa dibeli dari satu pemasok (GET /suppliers/:id/products) — dasar
// "pesan lagi" lewat WhatsApp. Keduanya memuat nilai pembelian → stock.view.
func TestLayarPemasok(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "pemasok") // A: modal 6.000, B: modal 3.000

	sup := call(t, "POST", "/api/v1/suppliers", f.token, map[string]any{
		"name": "CV Sumber Rezeki", "phone": "0812-1111-2222", "address": "Pasar Induk Blok C", "note": "kirim Selasa",
	}).mustCode(t, "pemasok", 201).data(t)
	supID := sup["id"].(string)
	if sup["phone"] != "0812-1111-2222" || sup["address"] != "Pasar Induk Blok C" {
		t.Fatalf("pemasok: %v", sup)
	}
	lain := call(t, "POST", "/api/v1/suppliers", f.token, map[string]any{"name": "UD Lain"}).
		mustCode(t, "pemasok lain", 201).data(t)["id"].(string)

	beli := func(supplier string, items []map[string]any, extra map[string]any) string {
		t.Helper()
		body := map[string]any{"outlet_id": f.outletID, "supplier_id": supplier, "items": items}
		for k, v := range extra {
			body[k] = v
		}
		return checkoutLike(t, f.token, ulid.New(), "/api/v1/purchases", body).mustCode(t, "beli", 201).data(t)["id"].(string)
	}
	lama := beli(supID, []map[string]any{{"product_id": f.prodA, "qty": "10", "unit_cost": 6000}}, map[string]any{"paid_amount": 60000})
	// Pembelian lama: 40 hari lalu — di luar "30 hari", masih di dalam "90 hari".
	database.DB.Exec(`UPDATE purchases SET occurred_at = now() - interval '40 days' WHERE id = ?`, lama)
	kemarin := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	beli(supID, []map[string]any{
		{"product_id": f.prodA, "qty": "5", "unit_cost": 6500},
		{"product_id": f.prodB, "qty": "2", "unit_cost": 3000},
	}, map[string]any{"due_date": kemarin}) // 38.500, belum dibayar, lewat jatuh tempo
	beli(lain, []map[string]any{{"product_id": f.prodB, "qty": "1", "unit_cost": 3000}}, map[string]any{"paid_amount": 3000})

	stats := call(t, "GET", "/api/v1/supplier-stats?outlet_id="+f.outletID, f.token, nil).mustOK(t, "stats")
	per := map[string]map[string]any{}
	for _, x := range stats.Body["data"].([]any) {
		m := x.(map[string]any)
		per[m["supplier_id"].(string)] = m
	}
	s := per[supID]
	if s == nil {
		t.Fatalf("stats tanpa pemasok utama: %v", stats.Body["data"])
	}
	assertI64(t, s, "purchase_count", 2)
	assertI64(t, s, "spent_30d", 38500) // yang 40 hari lalu tidak ikut
	assertI64(t, s, "outstanding", 38500)
	assertI64(t, s, "overdue_count", 1)
	if s["last_purchase_at"] == nil {
		t.Fatalf("tanpa belanja terakhir: %v", s)
	}
	assertI64(t, per[lain], "outstanding", 0)

	barang := call(t, "GET", "/api/v1/suppliers/"+supID+"/products?outlet_id="+f.outletID, f.token, nil).
		mustOK(t, "barang pemasok").Body["data"].([]any)
	if len(barang) != 2 {
		t.Fatalf("barang pemasok: %d, mau 2 (A & B; pembelian pemasok lain tidak ikut)", len(barang))
	}
	for _, x := range barang {
		b := x.(map[string]any)
		switch b["product_id"] {
		case f.prodA:
			// Harga terakhir, dua nota, 15 dalam 90 hari.
			if b["unit_cost"] != float64(6500) || b["times"] != float64(2) || b["qty_90d"] != "15" || b["product_name"] == "" {
				t.Fatalf("barang A: %v", b)
			}
		case f.prodB:
			if b["times"] != float64(1) || b["qty_90d"] != "2" {
				t.Fatalf("barang B: %v", b)
			}
		}
	}

	// Lihat barang boleh (product.view), tapi angka belanja butuh stock.view.
	rid := call(t, "POST", "/api/v1/roles", f.token, map[string]any{
		"name": "Lihat Barang Saja", "permission_codes": []string{"product.view"},
	}).mustCode(t, "peran", 201).data(t)["id"].(string)
	tok := staffToken(t, f.tenantFixture, rid, "lihatbarang_pemasok")
	call(t, "GET", "/api/v1/suppliers/"+supID, tok, nil).mustOK(t, "baca pemasok")
	call(t, "GET", "/api/v1/supplier-stats?outlet_id="+f.outletID, tok, nil).mustCode(t, "stats tanpa stock.view", 403)
	call(t, "GET", "/api/v1/suppliers/"+supID+"/products", tok, nil).mustCode(t, "barang tanpa stock.view", 403)
}
