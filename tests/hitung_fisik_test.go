package tests

import "testing"

// Riwayat hitung fisik yang terbaca: berapa barang dihitung, berapa yang
// berubah, perkiraan nilai selisihnya, siapa yang menghitung — dan rincian
// dengan nama barang (baris opname hanya menyimpan product_id).
func TestRiwayatHitungFisik(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "hitungfisik") // A: 100 (modal 6.000), B: 50 (modal 3.000)

	// B dijual melebihi catatan: 50 → −2.
	checkout(t, f.token, "HF-1", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodB, "qty": "52"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 416000}},
	}).mustCode(t, "jual sampai minus", 201)

	op := call(t, "POST", "/api/v1/stock-opnames", f.token, map[string]any{"outlet_id": f.outletID}).
		mustCode(t, "buat", 201).data(t)["id"].(string)
	call(t, "POST", "/api/v1/stock-opnames/"+op+"/items", f.token, map[string]any{
		"items": []map[string]any{
			{"product_id": f.prodA, "counted_qty": "97"}, // kurang 3
			{"product_id": f.prodB, "counted_qty": "1"},  // catatan −2, di rak 1
		},
	}).mustOK(t, "hitungan")
	call(t, "POST", "/api/v1/stock-opnames/"+op+"/post", f.token, nil).mustOK(t, "posting")

	daftar := call(t, "GET", "/api/v1/stock-opnames?outlet_id="+f.outletID, f.token, nil).
		mustOK(t, "riwayat").data(t)["data"].([]any)
	if len(daftar) != 1 {
		t.Fatalf("riwayat: %d sesi, mau 1", len(daftar))
	}
	r := daftar[0].(map[string]any)
	assertI64(t, r, "item_count", 2)
	assertI64(t, r, "changed_count", 2)
	// Perubahan NILAI STOK: A −3 × 6.000; B dari "minus" (dihitung nol, sama
	// dengan nilai stok di ringkasan) ke 1 → +1 × 3.000. Bukan +3 × 3.000:
	// mencocokkan catatan minus bukan untung.
	assertI64(t, r, "value_diff", -15000)
	if r["status"] != "posted" || r["created_by_name"] == nil || r["created_by_name"] == "" {
		t.Fatalf("sesi: %v", r)
	}

	rinci := call(t, "GET", "/api/v1/stock-opnames/"+op, f.token, nil).mustOK(t, "rincian").data(t)
	assertI64(t, rinci, "value_diff", -15000)
	for _, x := range rinci["items"].([]any) {
		it := x.(map[string]any)
		if it["product_name"] == nil || it["unit_name"] == nil {
			t.Fatalf("baris tanpa nama/satuan: %v", it)
		}
		if it["product_id"] == f.prodA && (it["diff_qty"] != "-3" || it["cost_price"] != float64(6000)) {
			t.Fatalf("baris A: %v", it)
		}
	}
}
