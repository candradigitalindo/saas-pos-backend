package tests

import "testing"

// Daftar kiriman antar toko yang terbaca: nama toko asal & tujuan, pencatat,
// jumlah & cuplikan barang — dan rincian dengan nama barang (baris transfer
// hanya menyimpan product_id; layar dulu menampilkan potongan id mentah).
// ?outlet_id= memuat kiriman keluar MAUPUN masuk toko itu saja.
func TestDaftarKirimAntarToko(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "kirimtoko") // A: 100, B: 50 di outlet utama
	cabang := makeOutlet(t, f.tenantFixture, "KRM2")
	lain := makeOutlet(t, f.tenantFixture, "KRM3")

	kirim := func(dari, ke string, items []map[string]any) string {
		t.Helper()
		return call(t, "POST", "/api/v1/stock-transfers", f.token, map[string]any{
			"from_outlet_id": dari, "to_outlet_id": ke, "items": items,
		}).mustCode(t, "buat kiriman", 201).data(t)["id"].(string)
	}
	keluar := kirim(f.outletID, cabang, []map[string]any{
		{"product_id": f.prodA, "qty": "10"},
		{"product_id": f.prodB, "qty": "5"},
	})
	call(t, "POST", "/api/v1/stock-transfers/"+keluar+"/send", f.token, nil).mustOK(t, "kirim")
	// Kiriman yang tidak melibatkan outlet utama.
	kirim(cabang, lain, []map[string]any{{"product_id": f.prodA, "qty": "1"}})

	daftar := call(t, "GET", "/api/v1/stock-transfers?outlet_id="+f.outletID, f.token, nil).
		mustOK(t, "daftar").data(t)["data"].([]any)
	if len(daftar) != 1 {
		t.Fatalf("kiriman outlet utama: %d, mau 1 (kiriman cabang→lain tidak ikut)", len(daftar))
	}
	k := daftar[0].(map[string]any)
	assertI64(t, k, "item_count", 2)
	if k["status"] != "sent" || k["from_outlet_name"] == nil || k["to_outlet_name"] == nil ||
		k["from_outlet_name"] == k["to_outlet_name"] || k["created_by_name"] == nil {
		t.Fatalf("kiriman: %v", k)
	}
	if n := k["item_names"].([]any); len(n) != 2 {
		t.Fatalf("cuplikan barang: %v", n)
	}
	// Toko tujuan melihat kiriman yang sama sebagai kiriman masuk.
	if d := call(t, "GET", "/api/v1/stock-transfers?outlet_id="+cabang, f.token, nil).
		mustOK(t, "daftar cabang").data(t)["data"].([]any); len(d) != 2 {
		t.Fatalf("kiriman cabang: %d, mau 2 (masuk + keluar)", len(d))
	}

	rinci := call(t, "GET", "/api/v1/stock-transfers/"+keluar, f.token, nil).mustOK(t, "rincian").data(t)
	for _, x := range rinci["items"].([]any) {
		it := x.(map[string]any)
		if it["product_name"] == nil || it["product_name"] == "" || it["unit_name"] == nil {
			t.Fatalf("baris kiriman tanpa nama: %v", it)
		}
	}
}
