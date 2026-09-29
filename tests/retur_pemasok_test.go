package tests

import (
	"testing"

	"candra/backend-api/database"
	"candra/backend-api/internal/ulid"
)

// Retur ke pemasok (000049): stok berkurang, total nota berkurang sebesar
// nilai retur (utangnya ikut turun), dan bila yang sudah dibayar melebihi
// total baru, pemasok mengembalikan selisihnya — ke laci (uang masuk shift)
// atau uang lain.
func TestReturKePemasok(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "returpms") // A: 100, B: 50; shift terbuka
	sup := call(t, "POST", "/api/v1/suppliers", f.token, map[string]any{"name": "CV Retur"}).
		mustCode(t, "pemasok", 201).data(t)["id"].(string)

	beli := func(produk string, qty string, harga, dibayar int64) (string, string) {
		t.Helper()
		p := checkoutLike(t, f.token, ulid.New(), "/api/v1/purchases", map[string]any{
			"outlet_id": f.outletID, "supplier_id": sup, "invoice_no": "R-" + qty, "paid_amount": dibayar,
			"items": []map[string]any{{"product_id": produk, "qty": qty, "unit_cost": harga}},
		}).mustCode(t, "beli", 201).data(t)
		return p["id"].(string), p["items"].([]any)[0].(map[string]any)["id"].(string)
	}
	retur := func(tok, kunci, id string, body map[string]any) apiResp {
		t.Helper()
		return checkoutLike(t, tok, kunci, "/api/v1/purchases/"+id+"/returns", body)
	}
	baris := func(item, qty string) []map[string]any {
		return []map[string]any{{"purchase_item_id": item, "qty": qty}}
	}

	// 1) Nota belum dibayar: utang turun, stok turun.
	p1, i1 := beli(f.prodA, "10", 6000, 0) // 60.000; stok A 110
	r := retur(f.token, ulid.New(), p1, map[string]any{"items": baris(i1, "3"), "reason": "Rusak"}).
		mustCode(t, "retur 3", 201).data(t)
	assertI64(t, r, "total", 18000)
	assertI64(t, r, "refund_amount", 0)
	assertI64(t, r, "purchase_outstanding", 42000)
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != "107" {
		t.Fatalf("stok A setelah retur: %s, mau 107", q)
	}
	retur(f.token, ulid.New(), p1, map[string]any{"items": baris(i1, "8"), "reason": "Rusak"}).mustCode(t, "melebihi sisa 7", 422)
	retur(f.token, ulid.New(), p1, map[string]any{"items": baris(i1, "1"), "reason": " "}).mustCode(t, "tanpa alasan", 422)

	rinci := call(t, "GET", "/api/v1/purchases/"+p1, f.token, nil).mustOK(t, "rincian").data(t)
	assertI64(t, rinci, "total", 42000)
	assertI64(t, rinci, "returned_amount", 18000)
	if rt := rinci["returns"].([]any); len(rt) != 1 || rt[0].(map[string]any)["reason"] != "Rusak" {
		t.Fatalf("daftar retur: %v", rinci["returns"])
	}
	if it := rinci["items"].([]any)[0].(map[string]any); it["returned_qty"] != "3" {
		t.Fatalf("returned_qty: %v", it)
	}
	km := call(t, "GET", "/api/v1/stock-movements?product_id="+f.prodA, f.token, nil).mustOK(t, "kartu").data(t)
	if g := km["data"].([]any)[0].(map[string]any); g["kind"] != "purchase_return" || g["qty_delta"] != "-3" {
		t.Fatalf("gerakan retur: %v", g)
	}

	// 2) Nota lunas: pemasok mengembalikan uang — wajib pilih ke mana masuknya.
	p2, i2 := beli(f.prodB, "10", 3000, 30000)
	retur(f.token, ulid.New(), p2, map[string]any{"items": baris(i2, "4"), "reason": "Kedaluwarsa"}).mustCode(t, "tanpa sumber", 422)
	kunci := ulid.New()
	body := map[string]any{"items": baris(i2, "4"), "reason": "Kedaluwarsa", "refund_source": "drawer"}
	r2 := retur(f.token, kunci, p2, body).mustCode(t, "retur lunas ke laci", 201).data(t)
	assertI64(t, r2, "refund_amount", 12000)
	assertI64(t, r2, "purchase_outstanding", 0)
	retur(f.token, kunci, p2, body).mustCode(t, "kirim ulang", 201)
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodB); q != "56" { // 50 + 10 − 4, bukan − 8
		t.Fatalf("stok B: %s, mau 56 (kiriman ulang tidak dobel)", q)
	}
	var kas struct{ N, Jumlah int64 }
	database.DB.Raw(`SELECT COUNT(*) AS n, COALESCE(SUM(amount), 0) AS jumlah FROM cash_movements
		WHERE tenant_id = ? AND shift_id = ? AND direction = 'in' AND reason LIKE 'Retur ke pemasok CV Retur%'`,
		f.tenantID, f.shiftID).Scan(&kas)
	if kas.N != 1 || kas.Jumlah != 12000 {
		t.Fatalf("uang masuk laci: %d × Rp %d, mau 1 × 12.000", kas.N, kas.Jumlah)
	}
	rinci2 := call(t, "GET", "/api/v1/purchases/"+p2, f.token, nil).mustOK(t, "rincian 2").data(t)
	assertI64(t, rinci2, "total", 18000)
	assertI64(t, rinci2, "paid_amount", 18000)

	// 3) Dibayar sebagian, retur lebih besar dari sisa utang: sebagian
	// melunasi, sisanya dikembalikan. 60.000, dibayar 50.000, retur 30.000 →
	// total 30.000, kembali 20.000.
	p3, i3 := beli(f.prodA, "10", 6000, 50000)
	r3 := retur(f.token, ulid.New(), p3, map[string]any{"items": baris(i3, "5"), "reason": "Salah kirim", "refund_source": "other"}).
		mustCode(t, "retur sebagian", 201).data(t)
	assertI64(t, r3, "refund_amount", 20000)
	assertI64(t, r3, "purchase_total", 30000)
	assertI64(t, r3, "purchase_outstanding", 0)

	// Utang pemasok mengikuti total baru.
	s := call(t, "GET", "/api/v1/payables/summary?outlet_id="+f.outletID, f.token, nil).mustOK(t, "utang").data(t)
	assertI64(t, s, "outstanding", 42000)

	// Uang kembali ke laci butuh izin uang laci.
	rid := call(t, "POST", "/api/v1/roles", f.token, map[string]any{
		"name": "Gudang Retur", "permission_codes": []string{"stock.view", "stock.adjust"},
	}).mustCode(t, "peran", 201).data(t)["id"].(string)
	gudang := staffToken(t, f.tenantFixture, rid, "gudang_retur")
	retur(gudang, ulid.New(), p3, map[string]any{"items": baris(i3, "1"), "reason": "Rusak", "refund_source": "drawer"}).
		mustCode(t, "laci tanpa izin", 403)
	retur(gudang, ulid.New(), p3, map[string]any{"items": baris(i3, "1"), "reason": "Rusak", "refund_source": "other"}).
		mustCode(t, "uang lain", 201)
}
