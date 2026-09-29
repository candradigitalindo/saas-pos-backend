package tests

import (
	"testing"
	"time"

	"candra/backend-api/database"
	"candra/backend-api/internal/ulid"
)

// Utang pemasok (000047): barang masuk boleh dibayar sebagian/belum; sisa
// dilunasi kemudian. Sumber uang dipilih tiap pembayaran — dari laci kasir
// (uang keluar shift yang sedang buka) atau uang lain (laci tidak berubah).
func TestUtangPemasok(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "utangpms") // shift terbuka di outlet utama

	sup := call(t, "POST", "/api/v1/suppliers", f.token, map[string]any{"name": "CV Maju"}).
		mustCode(t, "pemasok", 201).data(t)["id"].(string)
	kemarin := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	beli := func(token string, body map[string]any) apiResp {
		t.Helper()
		body["outlet_id"] = f.outletID
		return checkoutLike(t, token, ulid.New(), "/api/v1/purchases", body)
	}
	// A: belum dibayar, jatuh tempo kemarin (terlambat). 10 × 6.000.
	a := beli(f.token, map[string]any{
		"supplier_id": sup, "due_date": kemarin,
		"items": []map[string]any{{"product_id": f.prodA, "qty": "10", "unit_cost": 6000}},
	}).mustCode(t, "beli A", 201).data(t)
	assertI64(t, a, "outstanding", 60000)
	// B: 15.000, dibayar 5.000 DARI LACI saat barang datang.
	b := beli(f.token, map[string]any{
		"supplier_id": sup, "invoice_no": "N-9", "paid_amount": 5000, "payment_source": "drawer",
		"items": []map[string]any{{"product_id": f.prodB, "qty": "5", "unit_cost": 3000}},
	}).mustCode(t, "beli B", 201).data(t)
	assertI64(t, b, "outstanding", 10000)
	// C: lunas dengan uang lain, tanpa pemasok.
	beli(f.token, map[string]any{
		"paid_amount": 3000, "items": []map[string]any{{"product_id": f.prodB, "qty": "1", "unit_cost": 3000}},
	}).mustCode(t, "beli C", 201)
	// Terbayar melebihi total ditolak.
	beli(f.token, map[string]any{
		"paid_amount": 9999, "items": []map[string]any{{"product_id": f.prodB, "qty": "1", "unit_cost": 3000}},
	}).mustCode(t, "terbayar > total", 422)

	kasPemasok := func() (int64, int64) {
		t.Helper()
		var r struct{ N, Jumlah int64 }
		database.DB.Raw(`SELECT COUNT(*) AS n, COALESCE(SUM(amount), 0) AS jumlah FROM cash_movements
			WHERE tenant_id = ? AND shift_id = ? AND direction = 'out' AND reason LIKE 'Bayar pemasok CV Maju%'`,
			f.tenantID, f.shiftID).Scan(&r)
		return r.N, r.Jumlah
	}
	if n, j := kasPemasok(); n != 1 || j != 5000 {
		t.Fatalf("uang keluar laci setelah beli B: %d gerakan Rp %d, mau 1 × 5.000", n, j)
	}

	// Ringkasan: 2 nota belum lunas (A terlambat), semuanya ke CV Maju.
	r := call(t, "GET", "/api/v1/payables/summary?outlet_id="+f.outletID, f.token, nil).mustOK(t, "ringkasan").data(t)
	assertI64(t, r, "outstanding", 70000)
	assertI64(t, r, "count", 2)
	assertI64(t, r, "overdue_count", 1)
	ps := r["suppliers"].([]any)
	if len(ps) != 1 || ps[0].(map[string]any)["supplier_name"] != "CV Maju" {
		t.Fatalf("utang per pemasok: %v", ps)
	}
	// Daftar utang: jatuh tempo terdekat dulu.
	utang := call(t, "GET", "/api/v1/purchases?outlet_id="+f.outletID+"&unpaid=true", f.token, nil).
		mustOK(t, "daftar utang").data(t)["data"].([]any)
	if len(utang) != 2 || utang[0].(map[string]any)["id"] != a["id"] {
		t.Fatalf("daftar utang: %v", utang)
	}

	bayar := func(token, kunci, id string, body map[string]any) apiResp {
		t.Helper()
		return checkoutLike(t, token, kunci, "/api/v1/purchases/"+id+"/payments", body)
	}
	aID, bID := a["id"].(string), b["id"].(string)

	// Lunasi A dengan uang lain; kiriman ulang berkunci sama tidak dobel.
	kunci := ulid.New()
	bayar(f.token, kunci, aID, map[string]any{"amount": 60000, "source": "other"}).mustCode(t, "lunasi A", 201)
	bayar(f.token, kunci, aID, map[string]any{"amount": 60000, "source": "other"}).mustCode(t, "kirim ulang", 201)
	bayar(f.token, ulid.New(), aID, map[string]any{"amount": 1000, "source": "other"}).mustCode(t, "A sudah lunas", 409)
	// B: melebihi sisa ditolak; sisa dilunasi dari laci.
	bayar(f.token, ulid.New(), bID, map[string]any{"amount": 20000, "source": "drawer"}).mustCode(t, "melebihi sisa", 422)
	bayar(f.token, ulid.New(), bID, map[string]any{"amount": 10000, "source": "drawer", "note": "lunas"}).
		mustCode(t, "lunasi B dari laci", 201)
	if n, j := kasPemasok(); n != 2 || j != 15000 {
		t.Fatalf("uang keluar laci: %d gerakan Rp %d, mau 2 × total 15.000", n, j)
	}

	rb := call(t, "GET", "/api/v1/purchases/"+bID, f.token, nil).mustOK(t, "rincian B").data(t)
	assertI64(t, rb, "outstanding", 0)
	if p := rb["payments"].([]any); len(p) != 2 || p[1].(map[string]any)["source"] != "drawer" ||
		p[1].(map[string]any)["note"] != "lunas" || p[1].(map[string]any)["created_by_name"] == nil {
		t.Fatalf("pembayaran B: %v", rb["payments"])
	}
	if r := call(t, "GET", "/api/v1/payables/summary?outlet_id="+f.outletID, f.token, nil).mustOK(t, "ringkasan akhir").data(t); r["outstanding"] != float64(0) || len(r["suppliers"].([]any)) != 0 {
		t.Fatalf("ringkasan setelah lunas: %v", r)
	}

	// Staf dengan izin barang masuk tapi TANPA izin uang laci: boleh bayar
	// dengan uang lain, tidak boleh dari laci.
	d := beli(f.token, map[string]any{"items": []map[string]any{{"product_id": f.prodA, "qty": "1", "unit_cost": 6000}}}).
		mustCode(t, "beli D", 201).data(t)["id"].(string)
	rid := call(t, "POST", "/api/v1/roles", f.token, map[string]any{
		"name": "Gudang Uji Utang", "permission_codes": []string{"stock.view", "stock.adjust"},
	}).mustCode(t, "peran gudang", 201).data(t)["id"].(string)
	gudang := staffToken(t, f.tenantFixture, rid, "gudang_utang")
	bayar(gudang, ulid.New(), d, map[string]any{"amount": 1000, "source": "drawer"}).mustCode(t, "laci tanpa izin", 403)
	bayar(gudang, ulid.New(), d, map[string]any{"amount": 1000, "source": "other"}).mustCode(t, "uang lain", 201)

	// Toko yang kasirnya belum dibuka: dari laci ditolak dengan arahan.
	cabang := makeOutlet(t, f.tenantFixture, "UTG2")
	res := checkoutLike(t, f.token, ulid.New(), "/api/v1/purchases", map[string]any{
		"outlet_id": cabang, "paid_amount": 6000, "payment_source": "drawer",
		"items": []map[string]any{{"product_id": f.prodA, "qty": "1", "unit_cost": 6000}},
	}).mustCode(t, "laci tanpa shift", 422)
	if msg, _ := res.Body["message"].(string); msg == "" {
		t.Fatalf("pesan galat kosong: %v", res.Body)
	}
}
